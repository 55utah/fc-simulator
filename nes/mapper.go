package nes

import "fmt"

type Mapper interface {
	Read(address uint16) byte
	Write(address uint16, value byte)
	// 由于Mapper4需要在PPU每条扫描线结束时更新一次计数器，这里增加Step接口
	Step()
}

func NewMapper(card *Cartridge, console *Console) (Mapper, error) {
	switch card.Mapper {
	case 0:
		return NewMapper0(card), nil
	case 1:
		return NewMapper1(card), nil
	case 2:
		return NewMapper0(card), nil
	case 3:
		return NewMapper3(card), nil
	case 4:
		return NewMapper4(card, console), nil
	default:
		fmt.Printf("unsupported mapper \n")
		return nil, nil
	}
}
