/*
mapper0和mapper2共用
魂斗罗/沙曼陀蛇都是mapper2
*/

package nes

import (
	"fmt"
)

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

type Mapper0 struct {
	card     *Cartridge
	prgBanks int
	prgBank1 int
	prgBank2 int
}

func NewMapper0(card *Cartridge) Mapper {
	prgBanks := len(card.PRG) / 0x4000
	prgBank1 := 0
	prgBank2 := prgBanks - 1
	return &Mapper0{card, prgBanks, prgBank1, prgBank2}
}

func (mapper *Mapper0) Read(addr uint16) byte {
	card := mapper.card
	switch {
	case addr < 0x2000:
		return card.CHR[addr]
	case addr >= 0xC000:
		index := mapper.prgBank2*0x4000 + int(addr-0xC000)
		return card.PRG[index]
	case addr >= 0x8000:
		index := mapper.prgBank1*0x4000 + int(addr-0x8000)
		return card.PRG[index]
	case addr >= 0x6000:
		index := int(addr) - 0x6000
		return card.SRAM[index]
	default:
		fmt.Printf("unhandled mapper2 read at addr: 0x%04X", addr)
	}
	return 0
}

func (mapper *Mapper0) Write(addr uint16, value byte) {
	card := mapper.card

	switch {
	case addr < 0x2000:
		card.CHR[addr] = value
	case addr >= 0x8000:
		mapper.prgBank1 = int(value) % mapper.prgBanks
	case addr >= 0x6000:
		index := int(addr) - 0x6000
		card.SRAM[index] = value
	default:
		fmt.Printf("unhandled mapper2 write at addr: 0x%04X", addr)
	}
}
