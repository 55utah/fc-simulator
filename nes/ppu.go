/*
PPU
负责图像处理和输出，最复杂的部分
*/
package nes

import (
	"image"
)

type PPU struct {
	console *Console
	Memory
	Cycle    int
	ScanLine int
	Frame    int

	// 存储
	paletteData [32]byte
	NameTable   [2048]byte
	oamData     [256]byte // sprites内存，每4byte一个
	front       *image.RGBA
	back        *image.RGBA

	// 临时变量
	register byte

	// nmi状态
	nmiOccurred bool // 触发nmi（要在VBlank时触发NMI)
	nmiOutput   bool // $2002 D7 VBank标志位，nmi生成标志位，当VBlank时触发，置为true
	nmiPrevious bool
	nmiDelay    byte

	// internal寄存器
	v uint16 // 当前VRAM地址 15bit
	t uint16 // 临时VRAM地址 15bit
	x byte   // X Scroll 3bit
	w byte   // 第一次还是第二次写的标记 1bit
	f byte

	// 绘制背景时的一些变量
	nameTableByte      byte
	attributeTableByte byte
	lowTileByte        byte
	highTileByte       byte
	tileData           uint64

	// 精灵控制变量
	spriteCount      int
	spritePatterns   [8]uint32
	spritePositions  [8]byte
	spritePriorities [8]byte
	spriteIndexes    [8]byte

	// 0x2000 PPUCTRL 控制寄存器
	flagNameTable       byte // 确定当前使用的名称表 0: $2000; 1: $2400; 2: $2800; 3: $2C00
	flagIncrement       byte // 0: add 1; 1: add 32
	flagSpriteTable     byte // 精灵使用的图样表地址 0: $0000; 1: $1000; ignored in 8x16 mode
	flagBackgroundTable byte // 背景使用的图样表地址 0: $0000; 1: $1000
	flagSpriteSize      byte // 0: 8x8; 1: 8x16
	flagMasterSlave     byte // 0: read EXT; 1: write EXT

	// 0x2001 PPUMASK 掩码寄存器
	flagDisplayMode    byte // 0 colorful 1 gray
	flagShowLeftBack   byte // 0 不显示最左边那列, 8像素的背景
	flagShowLeftSprite byte // 0 不显示最左边那列, 8像素的精灵
	flagShowBack       byte // 1 显示背景
	flagShowSprite     byte // 1 显示精灵

	// 0x2002 PPUSTATUS 状态寄存器
	flagSpriteOverflow byte // 精灵溢出标志位 0(当前扫描线精灵个数小于8)
	flagSpriteZeroHit  byte // 1(#0精灵命中) VBlank之后置0

	// 0x2003 OAMADDR 设置精灵RAM的8位指针, 用来从oamData中定位数据
	oamAddress byte
	// 0x2007 PPUDATA
	bufferedData byte // for buffered reads
}

func NewPPU(console *Console) *PPU {
	ppu := PPU{Memory: NewPPUMemory(console), console: console}
	ppu.front = image.NewRGBA(image.Rect(0, 0, 256, 240))
	ppu.back = image.NewRGBA(image.Rect(0, 0, 256, 240))
	ppu.Reset()
	return &ppu
}

func (ppu *PPU) Reset() {
	// TODO
	ppu.flagNameTable = 0
	ppu.flagBackgroundTable = 0
	ppu.writeOAMAddr(0)
	ppu.Cycle = 340
	ppu.ScanLine = 240
	ppu.Frame = 0
}

func (ppu *PPU) tick() {
	/*
		扫描线和cycle处理
		0-239 可见扫描线，进行渲染
		241-260 触发VBANK 241触发VBANK
		>260 更新背景和精灵数据，下次渲染预获取等
	*/

	//「TODO 看下这里是否正常」 这里是固定间隔就进行一次中断，让CPU有时间更新
	if ppu.nmiDelay > 0 {
		ppu.nmiDelay--
		if ppu.nmiDelay == 0 && ppu.nmiOutput && ppu.nmiOccurred {
			ppu.console.CPU.TriggerNMI()
		}
	}

	if ppu.flagShowBack != 0 || ppu.flagShowSprite != 0 {
		if ppu.f == 1 && ppu.ScanLine == 261 && ppu.Cycle == 339 {
			ppu.Cycle = 0
			ppu.ScanLine = 0
			ppu.Frame++
			ppu.f ^= 1
			return
		}
	}
	ppu.Cycle++
	if ppu.Cycle > 340 {
		ppu.Cycle = 0
		ppu.ScanLine++
		if ppu.ScanLine > 261 {
			ppu.ScanLine = 0
			ppu.Frame++
			ppu.f ^= 1
		}
	}

	// ppu.Cycle++
	// if ppu.Cycle >= 340 {
	// 	ppu.Cycle = 0
	// 	ppu.ScanLine++
	// 	if ppu.ScanLine >= 261 {
	// 		ppu.ScanLine = 0
	// 		ppu.Frame++
	// 	}
	// }
}

func (ppu *PPU) Step() {
	// TODO 一步耗费一个PPU clock
	// 判断当前扫描线和时钟，选择渲染像素还是其他操作
	// 每个scanline有340cycle，scan 0-239渲染，240不做事情， 241-260VBANK, 	>261 其他处理
	// 渲染的scanline，340周期内每8个时钟看作一个单位，
	// cycle 0 不做事情， cycle 1-256 按下面方式获取数据，256-320 精灵tile数据在这里获取更新
	// 321-340 为下次的渲染预置数据

	/*
		每 8 个点看作一个单位：

		时钟 0 - 1 取 Name table 数据
		时钟 2 - 3 取 Attribute table 数据
		时钟 5 - 6 读取 tile 低 8 位
		时钟 7 - 8 读取 tile 高 8 位
		读取的数据会进入锁存器，然后每一个时钟进入移位寄存器，以提供绘图使用
	*/

	ppu.tick()

	renderEnable := ppu.flagShowBack > 0 || ppu.flagShowSprite > 0

	visibleLine := ppu.ScanLine >= 0 && ppu.ScanLine < 240
	preLine := ppu.ScanLine == 261
	renderLine := visibleLine || preLine

	visibleCycle := ppu.Cycle > 0 && ppu.Cycle <= 256
	preFetchCycle := ppu.Cycle >= 321 && ppu.Cycle <= 336
	fetchCycle := preFetchCycle || visibleCycle

	if renderEnable {
		// 拿数据渲染
		if visibleLine && visibleCycle {
			ppu.renderPixel()
		}
		// 每8个cycle一个循环，每个循环渲染8个点，即一个tile的一行。
		if renderLine && fetchCycle {
			// 注意这里！
			ppu.tileData <<= 4
			switch ppu.Cycle % 8 {
			case 1:
				ppu.fetchNameTableByte()
			case 3:
				ppu.fetchAttributeTableByte()
			case 5:
				ppu.fetchLowTileByte()
			case 7:
				ppu.fetchHighTileByte()
			case 0:
				ppu.storeTileData()
			}
		}

		// 280 - 304期间要将垂直Y位置信息从t copy到v
		if preLine && ppu.Cycle >= 280 && ppu.Cycle <= 304 {
			ppu.copyY()
		}

		if renderLine {
			// 每次进入新的tile需要更新x
			if fetchCycle && ppu.Cycle%8 == 0 {
				ppu.incrementX()
			}

			// nesdev：每行的256点,  Y++ ，相当于为每个tile的下一行，这样才能保证属性表获取正常
			if ppu.Cycle == 256 {
				ppu.incrementY()
			}

			// 257点，将ppu.t的行x位置信息赋给ppu.v
			if ppu.Cycle == 257 {
				ppu.copyX()
			}
		}
	}

	if ppu.ScanLine == 241 && ppu.Cycle == 1 {
		ppu.setVBank()
	}

	// 精灵计算
	if renderEnable {
		if ppu.Cycle == 257 {
			if visibleLine {
				ppu.evaluateSprites()
			} else {
				ppu.spriteCount = 0
			}
		}
	}

	// 超出渲染scan之后，清空中断
	if preLine && ppu.Cycle == 1 {
		ppu.clearVBank()
		ppu.flagSpriteZeroHit = 0
		ppu.flagSpriteOverflow = 0
	}
}

func (ppu *PPU) renderPixel() {

	x := ppu.Cycle - 1
	y := ppu.ScanLine

	background := ppu.backgroundPixel()
	i, sprite := ppu.spritePixel()

	if x < 8 && ppu.flagShowLeftBack == 0 {
		background = 0
	}

	if x < 8 && ppu.flagShowLeftSprite == 0 {
		sprite = 0
	}

	b := background%4 != 0
	s := sprite%4 != 0

	var color byte
	if !b && !s {
		color = 0
	} else if !b && s {
		color = sprite | 0x10
	} else if b && !s {
		color = background
	} else {
		if ppu.spriteIndexes[i] == 0 && x < 255 {
			ppu.flagSpriteZeroHit = 1
		}
		if ppu.spritePriorities[i] == 0 {
			color = sprite | 0x10
		} else {
			color = background
		}
	}

	paletteIndex := ppu.ReadPalette(uint16(color) % 64)
	c := Palette[paletteIndex]

	ppu.back.SetRGBA(x, y, c)
}

func (ppu *PPU) spritePixel() (byte, byte) {
	if ppu.flagShowSprite == 0 {
		return 0, 0
	}
	// 找到目标精灵
	for i := 0; i < ppu.spriteCount; i++ {
		x := ppu.spritePositions[i]
		offset := ppu.Cycle - 1 - int(x)
		if offset < 0 || offset > 7 {
			continue
		}
		offset = 7 - offset
		color := byte((ppu.spritePatterns[i] >> byte(offset*4)) & 0x0F)
		if color%4 == 0 {
			continue
		}
		return byte(i), color
	}
	return 0, 0
}

// https://github.com/dustpg/BlogFM/issues/17
func (ppu *PPU) evaluateSprites() {
	// 精灵是8*8还是8*16
	var h int
	if ppu.flagSpriteSize == 0 {
		h = 8
	} else {
		h = 16
	}

	count := 0
	for i := 0; i < 64; i++ {
		y := ppu.oamData[i*4+0]
		a := ppu.oamData[i*4+2]
		x := ppu.oamData[i*4+3]
		row := ppu.ScanLine - int(y)
		if row < 0 || row >= h {
			continue
		}
		if count < 8 {
			ppu.spritePatterns[count] = ppu.fetchSpritePattern(i, row)
			ppu.spritePositions[count] = x
			ppu.spritePriorities[count] = (a >> 5) & 1
			ppu.spriteIndexes[count] = byte(i)
		}
		count++
	}
	// 一行扫描线最多8个精灵，超出精灵溢出位置1
	if count > 8 {
		count = 8
		ppu.flagSpriteOverflow = 1
	}
	ppu.spriteCount = count
}

// i表示第几个精灵,row表示这个精灵的y坐标
func (ppu *PPU) fetchSpritePattern(i, row int) uint32 {
	// TODO
	tile := ppu.oamData[i*4+1]
	attribute := ppu.oamData[i*4+2]

	// 计算得到的精灵当前行8位的patternTable地址
	var address uint16
	// 拿到patternTable数据
	// 8*8
	if ppu.flagSpriteSize == 0 {
		// 垂直反转
		if attribute&0x80 == 0x80 {
			row = 7 - row
		}
		// 精灵用的patterTable地址
		table := ppu.flagSpriteTable
		address = 0x1000*uint16(table) + uint16(tile)*16 + uint16(row)
	} else {
		// 垂直反转
		if attribute&0x80 == 0x80 {
			row = 15 - row
		}
		table := tile & 1
		tile &= 0xFE
		if row > 7 {
			tile++
			row -= 8
		}
		address = 0x1000*uint16(table) + uint16(tile)*16 + uint16(row)
	}

	// 和背景一样，代表8个像素调色板颜色4位中的最低位
	lowTileByte := ppu.Read(address)
	highTileByte := ppu.Read(address + 8)

	// 调色板高两位
	high := (attribute & 3) << 2

	// 8个像素分别合成低两位调色板，4位调色板数据完成颜色获取
	// 注意这里要实现水平反转的逻辑
	var data uint32
	for i := 0; i < 8; i++ {
		var p1, p2 byte
		if attribute&0x40 == 0x40 {
			p1 = (lowTileByte & 1) << 0
			p2 = (highTileByte & 1) << 1
			lowTileByte >>= 1
			highTileByte >>= 1
		} else {
			p1 = (lowTileByte & 0x80) >> 7
			p2 = (highTileByte & 0x80) >> 6
			lowTileByte <<= 1
			highTileByte <<= 1
		}
		data <<= 4
		data |= uint32(high | p1 | p2)
	}

	// 32位，每4位代表一个像素的颜色
	return data
}

func (ppu *PPU) backgroundPixel() byte {
	if ppu.flagShowBack == 0 {
		return 0
	}
	// 前32bit用于当前渲染
	renderTileData := uint32(ppu.tileData >> 32)
	// ppu.x 0-7 是当前8个像素的x坐标，表示第几个像素 tiledata 4个bit处理一个像素
	data := renderTileData >> ((7 - ppu.x) * 4)
	return byte(data & 0x0F)
}

func (ppu *PPU) fetchNameTableByte() {
	// 已经将当前VRAM地址存入了 uint16 的ppu.v
	// 名称表每个tile图块使用1字节 每个tile负责8*8像素，共32*30个tile，刚好960byte
	// 值用来索引pattern table
	v := ppu.v
	// 这边防止ppu.v首次为0情况
	address := 0x2000 | (v & 0x0fff)
	ppu.nameTableByte = ppu.Read(address)
}

func (ppu *PPU) fetchAttributeTableByte() {
	// 是用来描述使用的调色板是哪个
	// 64字节的属性表，每个byte管理16个tile，即4*4个tile，32 * 32像素区域
	// 根据nametable地址确定是哪个属性表，以及在属性表中的位置
	v := ppu.v
	address := 0x23C0 | (v & 0x0C00) | ((v >> 4) & 0x38) | ((v >> 2) & 0x07)
	shift := ((v >> 4) & 4) | (v & 2)
	ppu.attributeTableByte = ((ppu.Read(address) >> shift) & 3) << 2
}

func (ppu *PPU) copyX() {
	// If rendering is enabled, the PPU copies all bits related to horizontal position from t to v:
	// v: ....A.. ...BCDEF <- t: ....A.. ...BCDEF
	ppu.v = (ppu.v & 0xfbe0) | (ppu.t & 0x41f)
}

func (ppu *PPU) copyY() {
	// 同步下Y信息
	// v: GHIA.BC DEF..... <- t: GHIA.BC DEF.....
	ppu.v = (ppu.v & 0x841f) | (ppu.t & 0x7be0)
}

func (ppu *PPU) incrementX() {
	if (ppu.v & 0x001F) == 31 {
		// coarse X = 0
		ppu.v &= 0xFFE0
		// switch horizontal nametable
		ppu.v ^= 0x0400
	} else {
		ppu.v++
	}
}

func (ppu *PPU) incrementY() {
	// nesdev上有伪代码
	// Y指fineY，每个tile渲染的第几行
	v := ppu.v
	// fineY < 7 => Y++
	if v&0x7000 != 0x7000 {
		ppu.v += 0x1000
	} else {
		// fine Y = 0
		ppu.v &= 0x8fff
		y := (v & 0x03e0) >> 5
		if y == 29 {
			y = 0
			// switch vertical nametable
			ppu.v ^= 0x0800
		} else if y == 31 {
			y = 0
		} else {
			y++
		}
		// put coarse Y back into v
		ppu.v = (ppu.v & 0xFC1F) | (y << 5)
	}
}

func (ppu *PPU) storeTileData() {
	// attributeTableByte是作为颜色的高两bit，从pattern table获取的LowTileByte HightTileByte是低2bit
	// 共同决定整个tile 8*8像素的颜色
	// storeTileData时，已经拿到8像素的结果，就是当前tile的当前行，4bit决定一个点颜色，共32bit，所以可以存两个周期的数据，渲染上一个周期的数据
	// 并更新下一个周期的数据

	var data uint32

	for i := 0; i < 8; i++ {
		a := ppu.attributeTableByte
		p1 := (ppu.lowTileByte & 0x80) >> 7
		p2 := (ppu.highTileByte & 0x80) >> 6
		ppu.lowTileByte <<= 1
		ppu.highTileByte <<= 1
		data <<= 4
		data |= uint32(a | p1 | p2)
	}
	ppu.tileData |= uint64(data)
}

func (ppu *PPU) fetchLowTileByte() {
	// 决定像素颜色的4位低两位来自patternTable，每个tile使用16字节，这里获取低8位
	table := ppu.flagBackgroundTable // patternTable 背景用是 0还是0x1000的pattern table
	tile := ppu.nameTableByte        // 用来索引pattern table

	// Y方向偏移 0 - 7
	fineY := (ppu.v >> 12) & 7

	// 读取pattern table(0-0x2000是pattern table)
	address := 0x1000*uint16(table) + uint16(tile)*16 + fineY
	ppu.lowTileByte = ppu.Read(address)
}

func (ppu *PPU) fetchHighTileByte() {
	// 基本同fetchLowTileByte
	table := ppu.flagBackgroundTable
	tile := ppu.nameTableByte

	fineY := (ppu.v >> 12) & 7

	address := 0x1000*uint16(table) + uint16(tile)*16 + fineY
	ppu.highTileByte = ppu.Read(address + 8)
}

func (ppu *PPU) setVBank() {
	ppu.front, ppu.back = ppu.back, ppu.front
	ppu.nmiOccurred = true
	ppu.nmiChange()
}

func (ppu *PPU) clearVBank() {
	ppu.nmiOccurred = false
	ppu.nmiChange()
}

func (ppu *PPU) readRegister(address uint16) byte {
	switch address {
	case 0x2002:
		return ppu.readStatus()
	case 0x2004:
		return ppu.readOAMData()
	case 0x2007:
		return ppu.readData()
	}
	return 0
}

// https://wiki.nesdev.org/w/index.php?title=PPU_registers
// cpu修改ppu寄存器，其中几个寄存器从0x2000-0x2007
func (ppu *PPU) writeRegister(addr uint16, value byte) {
	ppu.register = value
	switch addr {
	// PPUCTRL
	case 0x2000:
		ppu.writeControl(value)
	case 0x2001:
		ppu.writeMask(value)
	// case 0x2002:
	// 	// 不存在
	case 0x2003:
		ppu.writeOAMAddr(value) // 0x2003是OAM精灵内存指针位
	case 0x2004:
		ppu.writeOAMData(value) // 0x2004是OAM精灵内存数据位
	case 0x2005:
		ppu.writeScroll(value)
	case 0x2006:
		ppu.writeAddress(value)
	case 0x2007:
		ppu.writeData(value)
	case 0x4014:
		ppu.writeDMA(value) // 将0xXX00-0xXXff的内存复制到精灵OAM256byte内存
	}
}

// 0x4014
func (ppu *PPU) writeDMA(value byte) {
	cpu := ppu.console.CPU
	address := uint16(value) << 8
	for i := 0; i < 256; i++ {
		ppu.oamData[ppu.oamAddress] = cpu.Read(address)
		ppu.oamAddress++
		address++
	}
}

func (ppu *PPU) readOAMData() byte {
	data := ppu.oamData[ppu.oamAddress]
	if (ppu.oamAddress & 0x03) == 0x02 {
		data = data & 0xE3
	}
	return data
}

func (ppu *PPU) writeOAMData(value byte) {
	ppu.oamData[ppu.oamAddress] = value
	ppu.oamAddress++
}

func (ppu *PPU) writeOAMAddr(value byte) {
	ppu.oamAddress = value
}

// $2005 屏幕滚动 双写
func (ppu *PPU) writeScroll(value byte) {
	if ppu.w == 0 {
		// t: ....... ...ABCDE <- d: ABCDE...
		// x:              FGH <- d: .....FGH
		// w:                  <- 1
		ppu.t = (ppu.t & uint16(0xffe0)) | (uint16(value) >> 3)
		ppu.x = value & 0x7
		ppu.w = 1
	} else {
		// t: .CBA..HG FED..... = d: HGFEDCBA
		// w:                   = 0
		ppu.t = (ppu.t & 0x8FFF) | ((uint16(value) & 0x07) << 12)
		ppu.t = (ppu.t & 0xFC1F) | ((uint16(value) & 0xF8) << 2)
		ppu.w = 0
	}
}

// $2006 显存指针 双写
func (ppu *PPU) writeAddress(value byte) {
	// first
	if ppu.w == 0 {
		// t: ..FEDCBA ........ = d: ..FEDCBA
		// t: .X...... ........ = 0
		// w:                   = 1
		ppu.t = (ppu.t & 0x80ff) | (uint16(value&0x3f) << 8)
		ppu.w = 1
	} else {
		// t: ....... ABCDEFGH <- d: ABCDEFGH
		// v: <...all bits...> <- t: <...all bits...>
		// w:                  <- 0
		ppu.t = (ppu.t & uint16(0xff00)) | uint16(value)
		ppu.v = ppu.t
		ppu.w = 0
	}
}

// $2007: PPUDATA (read)
func (ppu *PPU) readData() byte {
	// ppu.v是当前VRAM地址
	value := ppu.Read(ppu.v)
	if ppu.v%0x4000 < 0x3F00 {
		buffered := ppu.bufferedData
		ppu.bufferedData = value
		value = buffered
	} else {
		ppu.bufferedData = ppu.Read(ppu.v - 0x1000)
	}
	// $2000的D2决定PPUDATA被访问后增加1还是32
	if ppu.flagIncrement == 0 {
		ppu.v += 1
	} else {
		ppu.v += 32
	}
	return value
}

// $2007: PPUDATA (write)
func (ppu *PPU) writeData(value byte) {
	ppu.Write(ppu.v, value)
	if ppu.flagIncrement == 0 {
		ppu.v += 1
	} else {
		ppu.v += 32
	}
}

// $2002: PPUSTATUS
func (ppu *PPU) readStatus() byte {
	result := ppu.register & 0x1f
	result |= ppu.flagSpriteOverflow << 5
	result |= ppu.flagSpriteZeroHit << 6
	if ppu.nmiOccurred {
		result |= 1 << 7
	}
	ppu.nmiOccurred = false
	ppu.nmiChange()

	// $2002读 修改w
	// w:                   = 0
	ppu.w = 0
	return result
}

func (ppu *PPU) writeMask(value byte) {
	ppu.flagDisplayMode = value & 1
	ppu.flagShowLeftBack = (value >> 1) & 1
	ppu.flagShowLeftSprite = (value >> 2) & 1
	ppu.flagShowBack = (value >> 3) & 1
	ppu.flagShowSprite = (value >> 4) & 1
}

// https://github.com/dustpg/BlogFM/issues/15
// PPUCTRL修改
func (ppu *PPU) writeControl(value byte) {
	// 确定名称表
	ppu.flagNameTable = value & 0b11
	// 显存增量
	ppu.flagIncrement = (value >> 2) & 1
	ppu.flagSpriteTable = (value >> 3) & 1
	ppu.flagBackgroundTable = (value >> 4) & 1
	ppu.flagSpriteSize = (value >> 5) & 1
	ppu.flagMasterSlave = (value >> 6) & 1

	ppu.nmiOutput = (value>>7)&1 == 1
	ppu.nmiChange()

	// $2000 write 修改t
	// t: ....BA.. ........ = d: ......BA
	ppu.t = (ppu.t & uint16(0xf3ff)) | (uint16((value & 0x3)) << 10)
}

func (ppu *PPU) ReadPalette(addr uint16) byte {
	if addr >= 16 && addr%4 == 0 {
		addr -= 16
	}
	return ppu.paletteData[addr]
}

func (ppu *PPU) WritePalette(addr uint16, value byte) {
	if addr >= 16 && addr%4 == 0 {
		addr -= 16
	}
	ppu.paletteData[addr] = value
}

func (ppu *PPU) nmiChange() {
	nmi := ppu.nmiOutput && ppu.nmiOccurred
	if nmi && !ppu.nmiPrevious {
		ppu.nmiDelay = 15
	}
	ppu.nmiPrevious = nmi
}

/*
关于PPU地址分配
两个图样表pattern table 大小0x1000即4kb
0-0x0fff 图样表0
0x1000-0x1fff 图样表1
两个pattern Table各4kb，由0x2000 PPUCTRL的D3\D4分别控制精灵和背景各自使用哪块图样表


四个名称表 name table 大小0x0400即1kb
0x2000-0x23ff 名称表0
0x2400-0x27ff 名称表1
0x2800-0x2bff 名称表2
0x3000-0x3eff 名称表3
四个表每个1kb共4kb
name Table实际作为pattern Table的索引：
属性表控制当前图像高2bit的palette偏移量，
而name table索引到pattern table数据之后，控制低2bit的palette偏移量
(name table与pattern table的对应：
name table一个字节对应一个tile，值用来索引到pattern table中，pattern table 大小0x1000即4kb,
每16byte表示一个tile，则 0x1000/16 = 256 刚好是name table一个字节可以表示的范围)
(attribute table的逻辑：
table共64字节，要表示960个tile，每四个tile使用两bit，即16 * 64 = 1024，即覆盖了960个tile，用来控制
指定tile颜色的高2bit，也就是每四个tile使用同样的高2bit。
)

0x3000-0x3eff 是0x2000-0x2eff的镜像

0x3f00-0x3f1f 0x0020 调色板Palette内存索引
0x3f20-0x3fff 0x00e0 0x3f00-0x3f1f镜像

每个名称表最后64字节是属性表，



*/

/*
屏幕分辨率 256 * 240
PPU使用2kb RAM 作为屏幕图像存储
另有2kb他的镜像

palette调色板
共支持64种颜色

地址中的0x3f00-0x3f1f 调色板Palette内存索引 大小是0x20也就是32.
调色板索引是32字节，前面16字节背景使用，后面16字节精灵使用
16字节存32个颜色->一个颜色占4位
背景调色板在 0x3f00-0x3f0f
精灵调色板在 0x3f10-0x3f1f

*/

/*
关于时序
fc每帧0-261共262根扫描线，每个扫描线341个时钟，每个时钟对应一个像素点
大于256*240, 所以扫描线 0 - 239 的 1 - 256 周期可见，另外，0时钟空闲。
从241-261为垂直消隐VBank时间，这段时间内不渲染，给cpu时间计算，1-241期间设置VBank标志，
241-261时清除。

PPU时钟是CPU三倍，每个PPU时钟扫描一个点，每行341个点，每帧扫描262行
扫描线的0-239是可见
0，0 点不做任何事

*/

/*
ppu会暴露8个寄存器给cpu， 从$2000-$2007,
cpu会读写这些寄存器！
*/

/*
PPU存在一些内部寄存器
v Current VRAM address (15 bits)
t Temporary VRAM address (15 bits); can also be thought of as the address of the top left onscreen tile.
x Fine X scroll (3 bits)
w First or second write toggle (1 bit) 这个寄存器可以用来为双写的寄存器标识含义

PPU使用$2007读写方式使用VRAM地址，并且可获取nametable来绘制背景，在绘制背景时，更新地址指出当前要绘制的nametable数据


$2007	读写	访问显存数据	指针会在读写后+1或者+32

调试技巧：
执行足够多的指令(比如定一个小目标: 执行™一万次)
触发VBlank(NMI)中断(需要检查$2000:D7)
渲染图像
回到第一步

*/
