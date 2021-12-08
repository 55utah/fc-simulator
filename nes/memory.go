package nes

/*
[$0000, $2000) cpu 内存{0-0x0800，[0x0800-0x1000, 0x1000-0x1800, 0x1800-0x2000]都是0-0x0800的镜像}
[$2000, $4000) PPU 寄存器
[$4000, $6000) pAPU寄存器以及扩展区域
[$6000, $8000) 存档用SRAM区
[$8000, $0x10000) 程序代码区 PRG-ROM
*/

type Memory interface {
	Write(addr uint16, value byte)
	Read(addr uint16) byte
}

type CPUMemory struct {
	RAM     []byte
	console *Console
}

func NewCPUMemory(console *Console) Memory {
	ram := make([]byte, 0x2000)
	return &CPUMemory{console: console, RAM: ram}
}

func (mem *CPUMemory) Read(addr uint16) byte {
	switch {
	case addr < 0x2000:
		return mem.RAM[addr%0x0800]
	case addr < 0x4000:
		// 这边addr访问ppu寄存器，存在镜像，需要对8取余
		return mem.console.PPU.readRegister(0x2000 + addr%8)
	case addr == 0x4014:
		return mem.console.PPU.readRegister(addr)
	case addr == 0x4015:
		return mem.console.APU.ReadRegister(addr)
	case addr == 0x4016:
		return mem.console.Controller1.Read()
	case addr == 0x4017:
		return mem.console.Controller2.Read()
	case addr < 0x6000:
		// TODO
		return 0
	case addr >= 0x6000:
		return mem.console.Mapper.Read(addr)
	default:
		return 0
	}
}

func (mem *CPUMemory) Write(addr uint16, value byte) {
	switch {
	case addr < 0x2000:
		mem.RAM[addr%0x0800] = value
	case addr < 0x4000:
		mem.console.PPU.writeRegister(0x2000+addr%8, value)
	case addr >= 0x4000 && addr < 0x4014:
		mem.console.APU.writeRegister(addr, value)
	case addr == 0x4014:
		mem.console.PPU.writeRegister(addr, value)
	case addr == 0x4015:
		mem.console.APU.writeRegister(addr, value)
	case addr == 0x4016:
		mem.console.Controller1.Write(value)
		mem.console.Controller2.Write(value)
	case addr == 0x4017:
		mem.console.APU.writeRegister(addr, value)
	case addr < 0x6000:
		// TODO IO寄存器？
	case addr >= 0x6000:
		mem.console.Mapper.Write(addr, value)
	default:
		panic("to finish")
	}
}

type PPUMemory struct {
	console *Console
}

func NewPPUMemory(console *Console) Memory {
	return &PPUMemory{console}
}

func (mem *PPUMemory) Read(addr uint16) byte {
	// PPU的地址空间. PPU拥有16kb的地址空间， 也就是从0-0x3fff, 完全独立于CPU. 再高的地址会被镜像.
	// 高于0x3fff的会被镜像，所以需要对0x4000取余；

	addr = addr % 0x4000
	switch {
	// 0-0x2000是pattern table图样表，这部分来自卡带的CHR-ROM，除了这部分，其他的都是PPU自己的内存（读+写）
	case addr < 0x2000:
		return mem.console.Mapper.Read(addr)
	case addr < 0x3f00:
		mode := mem.console.Card.Mirror
		return mem.console.PPU.NameTable[MirrorAddress(mode, addr)%2048]
	case addr < 0x4000:
		return mem.console.PPU.ReadPalette(addr % 32)
	default:
		return 0
	}
}

func (mem *PPUMemory) Write(addr uint16, value byte) {
	addr = addr % 0x4000
	switch {
	// 0-0x2000是pattern table图样表，这部分来自卡带的CHR-ROM，除了这部分，其他的都是PPU自己的内存（读+写）
	case addr < 0x2000:
		mem.console.Mapper.Write(addr, value)
	case addr < 0x3f00:
		mode := mem.console.Card.Mirror
		mem.console.PPU.NameTable[MirrorAddress(mode, addr)%2048] = value
	case addr < 0x4000:
		mem.console.PPU.WritePalette(addr%32, value)
	default:
		break
	}
}

// Mirroring Modes
// 这里直接用开源的这部分代码，镜像模式的各种类型，一般常用的是 MirrorHorizontal、MirrorVertical

const (
	MirrorHorizontal = 0
	MirrorVertical   = 1
	MirrorSingle0    = 2
	MirrorSingle1    = 3
	MirrorFour       = 4
)

var MirrorLookup = [...][4]uint16{
	{0, 0, 1, 1},
	{0, 1, 0, 1},
	{0, 0, 0, 0},
	{1, 1, 1, 1},
	{0, 1, 2, 3},
}

func MirrorAddress(mode byte, address uint16) uint16 {
	address = (address - 0x2000) % 0x1000
	table := address / 0x0400
	offset := address % 0x0400
	return 0x2000 + MirrorLookup[mode][table]*0x0400 + offset
}
