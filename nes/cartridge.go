package nes

type Cartridge struct {
	PRG    []byte
	CHR    []byte
	SRAM   []byte // 卡带SRAM
	Mirror byte   // 0 水平 1 垂直
	Mapper byte   // mapper种类
}

func NewCartridge(prg []byte, chr []byte, mapper byte, mirror byte) *Cartridge {
	sram := make([]byte, 0x2000)
	return &Cartridge{prg, chr, sram, mirror, mapper}
}
