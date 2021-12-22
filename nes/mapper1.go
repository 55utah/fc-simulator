package nes

// 中东战争可以用来测试

type Mapper1 struct {
	card          *Cartridge
	shiftRegister byte
	ctrlRegister  byte
	prgMode       byte
	chrMode       byte
	chrBank0      byte
	chrBank1      byte
	prgBank       byte
	prgOffsets    [2]int
	chrOffsets    [2]int
}

func NewMapper1(card *Cartridge) Mapper {
	m := Mapper1{}
	m.card = card
	m.shiftRegister = 0x10
	m.prgOffsets[1] = m.getPrgOffset(-1)
	return &m
}

func (m *Mapper1) writeRegister(addr uint16, value byte) {
	switch {
	case addr <= 0x9fff:
		m.writeControl(value)
	case addr <= 0xbfff:
		m.writeCHRBank0(value)
	case addr <= 0xdfff:
		m.writeCHRBank1(value)
	case addr <= 0xffff:
		m.writePRGBank(value)
	}
	// 写寄存器后更新偏移信息
}

func (m *Mapper1) writeControl(value byte) {
	m.ctrlRegister = value
	m.prgMode = (value >> 2) & 0x3
	m.chrMode = (value >> 4) & 1
	mirror := value & 0x3
	// 这里mirror值和Mirror并不是直接对应的
	switch mirror {
	case 0:
		m.card.Mirror = MirrorSingle0
	case 1:
		m.card.Mirror = MirrorSingle1
	case 2:
		m.card.Mirror = MirrorVertical
	case 3:
		m.card.Mirror = MirrorHorizontal
	}
	m.updateOffsets()
}

// CHR bank 0 (internal, $A000-$BFFF)
func (m *Mapper1) writeCHRBank0(value byte) {
	m.chrBank0 = value
	m.updateOffsets()
}

// CHR bank 1 (internal, $C000-$DFFF)
func (m *Mapper1) writeCHRBank1(value byte) {
	m.chrBank1 = value
	m.updateOffsets()
}

// PRG bank (internal, $E000-$FFFF)
func (m *Mapper1) writePRGBank(value byte) {
	m.prgBank = value & 0x0f
	m.updateOffsets()
}

func (m *Mapper1) loadRegister(addr uint16, value byte) {
	// D7==1
	if value&0x80 == 0x80 {
		m.shiftRegister = 0x10
		m.writeControl(m.ctrlRegister | 0x0c)
	} else {
		complete := m.shiftRegister&1 == 1
		m.shiftRegister |= ((value & 1) << 5)
		m.shiftRegister >>= 1
		if complete {
			// 5次了
			m.writeRegister(addr, m.shiftRegister)
			m.shiftRegister = 0x10
		}
	}
}

// prg 8k 0x4000
func (m *Mapper1) getPrgOffset(value int) int {
	// 没看懂的操作。
	if value >= 0x80 {
		value -= 0x100
	}
	count := len(m.card.PRG) / 0x4000
	offset := (value % count) * 0x4000
	if offset < 0 {
		offset += len(m.card.PRG)
	}
	return offset
}

// chr 1k 0x1000
func (m *Mapper1) getChrOffset(value int) int {
	if value >= 0x80 {
		value -= 0x100
	}
	count := len(m.card.CHR) / 0x1000
	offset := (value % count) * 0x1000
	if offset < 0 {
		offset += len(m.card.CHR)
	}
	return offset
}

// PRG ROM bank mode (0, 1: switch 32 KB at $8000, ignoring low bit of bank number;
//                    2: fix first bank at $8000 and switch 16 KB bank at $C000;
//                    3: fix last bank at $C000 and switch 16 KB bank at $8000)
// CHR ROM bank mode (0: switch 8 KB at a time; 1: switch two separate 4 KB banks)
func (m *Mapper1) updateOffsets() {
	switch m.prgMode {
	case 0, 1:
		m.prgOffsets[0] = m.getPrgOffset(int(m.prgBank & 0xFE))
		m.prgOffsets[1] = m.getPrgOffset(int(m.prgBank | 0x01))
	case 2:
		m.prgOffsets[0] = 0
		m.prgOffsets[1] = m.getPrgOffset(int(m.prgBank))
	case 3:
		m.prgOffsets[0] = m.getPrgOffset(int(m.prgBank))
		m.prgOffsets[1] = m.getPrgOffset(-1)
	}
	switch m.chrMode {
	case 0:
		m.chrOffsets[0] = m.getChrOffset(int(m.chrBank0 & 0xFE))
		m.chrOffsets[1] = m.getChrOffset(int(m.chrBank0 | 0x01))
	case 1:
		m.chrOffsets[0] = m.getChrOffset(int(m.chrBank0))
		m.chrOffsets[1] = m.getChrOffset(int(m.chrBank1))
	}
}

func (m *Mapper1) Read(addr uint16) byte {
	switch {
	case addr < 0x2000:
		bank := addr / 0x1000
		offset := addr % 0x1000
		return m.card.CHR[m.chrOffsets[bank]+int(offset)]
	case addr >= 0x8000:
		addr = addr - 0x8000
		bank := addr / 0x4000
		offset := addr % 0x4000
		return m.card.PRG[m.prgOffsets[bank]+int(offset)]
	case addr >= 0x6000:
		return m.card.SRAM[addr-0x6000]
	default:
	}
	return 0
}

func (m *Mapper1) Write(addr uint16, value byte) {
	switch {
	case addr < 0x2000:
		bank := addr / 0x400
		offset := addr % 0x400
		m.card.CHR[m.chrOffsets[bank]+int(offset)] = value
	case addr >= 0x8000:
		m.loadRegister(addr, value)
	case addr >= 0x6000:
		m.card.SRAM[addr-0x6000] = value
	default:
	}
}

func (m *Mapper1) Step() {}
