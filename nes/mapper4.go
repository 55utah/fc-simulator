package nes

/**
BANK是每个Mapper载入的单位
根据地址划分为每8KB(0x2000 byte)一个BANK, 一共8个区块:

0: [$0000, $2000) cpu 内存
1: [$2000, $4000) PPU 寄存器
2: [$4000, $6000) pAPU寄存器以及扩展区域
3: [$6000, $8000) 存档用SRAM区
剩下的全是 程序代码区 PRG-ROM [$8000, $0x10000)
0xfff0附近有中断相关信息

$FFFA-FFFB = NMI
$FFFC-FFFD = RESET
$FFFE-FFFF = IRQ/BRK

两byte存的是中断触发时跳转到的制定位置的16位地址

*/

type Mapper4 struct {
	card       *Cartridge
	console    *Console
	regIndex   byte    // 寄存器索引
	registers  [8]byte //
	prgMode    byte    // prg bank mode 0/1
	chrMode    byte    // chr倒置逻辑。 0/1
	reload     byte    // 计数器总时长
	timerValue byte    // 计数器当前值
	irqEnable  bool    // IRQ中断开关
	// 这里采用和参考项目同样的方法，计算出每个bank对应的地址offset
	prgOffsets [4]int
	chrOffsets [8]int
}

func (m *Mapper4) Step() {
	ppu := m.console.PPU
	if ppu.ScanLine > 239 && ppu.ScanLine < 261 {
		return
	}
	if ppu.flagShowBack == 0 && ppu.flagShowSprite == 0 {
		return
	}
	// nesdev:  presumably at PPU cycle 260 of the current scanline.
	if ppu.Cycle != 260 {
		return
	}
	m.StepScanLineCounter()
}

func (m *Mapper4) StepScanLineCounter() {
	if m.timerValue == 0 {
		m.timerValue = m.reload
	} else {
		m.timerValue--
		if m.timerValue == 0 && m.irqEnable {
			m.console.CPU.TriggerIRQ()
		}
	}
}

func NewMapper4(card *Cartridge, console *Console) Mapper {
	m := Mapper4{card: card, console: console}
	// 这里注意要先把prg预制好
	m.prgOffsets[0] = m.getPrgOffset(0)
	m.prgOffsets[1] = m.getPrgOffset(1)
	m.prgOffsets[2] = m.getPrgOffset(-2)
	m.prgOffsets[3] = m.getPrgOffset(-1)
	return &m
}

/*
文档描述不清楚，D0D1D2三位合起来范围是0-7，用来选择8个bank寄存器
bank寄存器是下次写入到bank data寄存器内的；
*/
func (m *Mapper4) setBankSelect(value byte) {
	m.regIndex = value & 7
	m.prgMode = (value >> 6) & 1
	m.chrMode = (value >> 7) & 1
	// 这里需要更新bank
	m.calculateBank()
}

func (m *Mapper4) setBankData(value byte) {
	m.registers[m.regIndex] = value
	// 这里需要更新bank
	m.calculateBank()
}

// 这个居然是用来更改卡带的Mirror属性的
func (m *Mapper4) setMirroring(value byte) {
	if value&1 > 0 {
		m.card.Mirror = 0
	} else {
		m.card.Mirror = 1
	}
}

func (m *Mapper4) setIRQLatch(value byte) {
	m.reload = value
}

func (m *Mapper4) setIRQReload(value byte) {
	m.timerValue = 0
	// m.reload = value
}

func (m *Mapper4) setIRQDisable(value byte) {
	m.irqEnable = false
}

func (m *Mapper4) setIRQEnable(value byte) {
	m.irqEnable = true
}

/*
4对寄存器，光看资料很难明白指的是什么。
寄存器都在>0x8000的地址
奇数地址对应一个寄存器，偶数地址就是另一个寄存器；
比如：$8000-$9FFE地址中的偶数地址就是对应一个寄存器；
如何对应? 其实就是给这个地址范围偶地址写value时，就是给这个寄存器赋值/处理
*/
func (m *Mapper4) writeRegister(addr uint16, value byte) {
	switch {
	case addr <= uint16(0x9fff) && addr%2 == 0:
		m.setBankSelect(value)
	case addr <= uint16(0x9fff) && addr%2 == 1:
		m.setBankData(value)
	case addr <= uint16(0xbfff) && addr%2 == 0:
		m.setMirroring(value)
	case addr <= uint16(0xbfff) && addr%2 == 1:
		// PRG RAM 保护，可不实现
	case addr <= uint16(0xdfff) && addr%2 == 0:
		m.setIRQLatch(value)
	case addr <= uint16(0xdfff) && addr%2 == 1:
		m.setIRQReload(value)
	case addr <= uint16(0xffff) && addr%2 == 0:
		m.setIRQDisable(value)
	case addr <= uint16(0xffff) && addr%2 == 1:
		m.setIRQEnable(value)
	}
}

/*
老规矩
<0x2000是CHR
>0x8000是PRG
*/

func (m *Mapper4) Read(addr uint16) byte {
	switch {
	case addr < 0x2000:
		bank := addr / 0x0400
		offset := addr % 0x0400
		return m.card.CHR[m.chrOffsets[bank]+int(offset)]
	case addr >= 0x8000:
		newAddr := addr - 0x8000
		bank := newAddr / 0x2000
		offset := newAddr % 0x2000
		return m.card.PRG[m.prgOffsets[bank]+int(offset)]
	case addr >= 0x6000:
		return m.card.SRAM[addr-0x6000]
	default:
	}
	return 0
}

func (m *Mapper4) Write(addr uint16, value byte) {
	switch {
	case addr < 0x2000:
		bank := addr / 0x0400
		offset := addr % 0x0400
		m.card.CHR[m.chrOffsets[bank]+int(offset)] = value
	case addr >= 0x8000:
		m.writeRegister(addr, value)
	case addr >= 0x6000:
		m.card.SRAM[addr-0x6000] = value
	default:
	}
}

// prg 8k 0x2000
func (m *Mapper4) getPrgOffset(value int) int {
	// 没看懂的操作。
	if value >= 0x80 {
		value -= 0x100
	}
	count := len(m.card.PRG) / 0x2000
	offset := (value % count) * 0x2000
	if offset < 0 {
		offset += len(m.card.PRG)
	}
	return offset
}

// chr 1k 0x0400
func (m *Mapper4) getChrOffset(value int) int {
	if value >= 0x80 {
		value -= 0x100
	}
	count := len(m.card.CHR) / 0x400
	offset := (value % count) * 0x400
	if offset < 0 {
		offset += len(m.card.CHR)
	}
	return offset
}

func (m *Mapper4) calculateBank() {
	// 这里就直接参考 https://wiki.nesdev.org/w/index.php/MMC3 的表格
	if m.prgMode == 0 {
		// R6
		m.prgOffsets[0] = m.getPrgOffset(int(m.registers[6]))
		// R7
		m.prgOffsets[1] = m.getPrgOffset(int(m.registers[7]))
		// -2
		m.prgOffsets[2] = m.getPrgOffset(-2)
		// -1
		m.prgOffsets[3] = m.getPrgOffset(-1)
	} else {
		// -2
		m.prgOffsets[0] = m.getPrgOffset(-2)
		// R7
		m.prgOffsets[1] = m.getPrgOffset(int(m.registers[7]))
		// R6
		m.prgOffsets[2] = m.getPrgOffset(int(m.registers[6]))
		// -1
		m.prgOffsets[3] = m.getPrgOffset(-1)
	}

	if m.chrMode == 0 {
		m.chrOffsets[0] = m.getChrOffset(int(m.registers[0]) & 0xFE)
		m.chrOffsets[1] = m.getChrOffset(int(m.registers[0]) | 0x01)
		m.chrOffsets[2] = m.getChrOffset(int(m.registers[1]) & 0xFE)
		m.chrOffsets[3] = m.getChrOffset(int(m.registers[1]) | 0x01)
		m.chrOffsets[4] = m.getChrOffset(int(m.registers[2]))
		m.chrOffsets[5] = m.getChrOffset(int(m.registers[3]))
		m.chrOffsets[6] = m.getChrOffset(int(m.registers[4]))
		m.chrOffsets[7] = m.getChrOffset(int(m.registers[5]))
	} else {
		m.chrOffsets[0] = m.getChrOffset(int(m.registers[2]))
		m.chrOffsets[1] = m.getChrOffset(int(m.registers[3]))
		m.chrOffsets[2] = m.getChrOffset(int(m.registers[4]))
		m.chrOffsets[3] = m.getChrOffset(int(m.registers[5]))
		m.chrOffsets[4] = m.getChrOffset(int(m.registers[0]) & 0xFE)
		m.chrOffsets[5] = m.getChrOffset(int(m.registers[0]) | 0x01)
		m.chrOffsets[6] = m.getChrOffset(int(m.registers[1]) & 0xFE)
		m.chrOffsets[7] = m.getChrOffset(int(m.registers[1]) | 0x01)
	}
}

/*
关于

Bank select ($8000-$9FFE, even)
7  bit  0
---- ----
CPMx xRRR
|||   |||
|||   +++- Specify which bank register to update on next write to Bank Data register
|||          000: R0: Select 2 KB CHR bank at PPU $0000-$07FF (or $1000-$17FF)
|||          001: R1: Select 2 KB CHR bank at PPU $0800-$0FFF (or $1800-$1FFF)
|||          010: R2: Select 1 KB CHR bank at PPU $1000-$13FF (or $0000-$03FF)
|||          011: R3: Select 1 KB CHR bank at PPU $1400-$17FF (or $0400-$07FF)
|||          100: R4: Select 1 KB CHR bank at PPU $1800-$1BFF (or $0800-$0BFF)
|||          101: R5: Select 1 KB CHR bank at PPU $1C00-$1FFF (or $0C00-$0FFF)
|||          110: R6: Select 8 KB PRG ROM bank at $8000-$9FFF (or $C000-$DFFF)
|||          111: R7: Select 8 KB PRG ROM bank at $A000-$BFFF
||+------- Nothing on the MMC3, see MMC6
|+-------- PRG ROM bank mode (0: $8000-$9FFF swappable,
|                                $C000-$DFFF fixed to second-last bank;
|                             1: $C000-$DFFF swappable,
|                                $8000-$9FFF fixed to second-last bank)
+--------- CHR A12 inversion (0: two 2 KB banks at $0000-$0FFF,
                                 four 1 KB banks at $1000-$1FFF;
                              1: two 2 KB banks at $1000-$1FFF,
                                 four 1 KB banks at $0000-$0FFF)

这个是bank选择寄存器，像这个范围内地址写入的value(一个字节)，低三位用来确定是哪个寄存器，说明里面的“R0/R1/../R7”就是8个内部寄存器
确定的寄存器是下次写入Bank Data时给哪个寄存器写！
Bank Data存的数据为使用哪块Bank， PRG 4块 每块0x2000; CHR 8块 每块0x0400;

*/
