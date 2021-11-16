package nes

import "fmt"

type Mapper interface {
	Read(address uint16) byte
	Write(address uint16, value byte)
}

func NewMapper(card *Cartridge) (Mapper, error) {
	switch card.Mapper {
	case 0:
		return NewMapper0(card), nil
	case 2:
		return NewMapper0(card), nil
	case 3:
		return NewMapper3(card), nil
	default:
		fmt.Printf("unsupported mapper \n")
		return nil, nil
	}
}
