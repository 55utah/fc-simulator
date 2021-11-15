package nes

import "image"

/**
这个模块作为cpu/ppu/apu/mapper/card/RAM的封装
*/

type Console struct {
	CPU *CPU
	// APU         *APU
	PPU         *PPU
	Card        *Cartridge
	Controller1 *Controller
	Controller2 *Controller
	Mapper      Mapper
	RAM         []byte
}

func NewConsole(path string) (*Console, error) {
	card, err := LoadNESRom(path)
	if err != nil {
		return nil, err
	}

	ram := make([]byte, 2048)
	ctrl1 := NewController()
	ctrl2 := NewController()

	console := &Console{
		nil, nil, card, ctrl1, ctrl2, nil, ram,
	}
	mapper, err := NewMapper(card)
	if err != nil {
		return nil, err
	}
	console.Mapper = mapper
	console.CPU = NewCPU(console)
	console.PPU = NewPPU(console)

	return console, nil
}

func (console *Console) Reset() {
	console.CPU.Reset()
}

func (console *Console) Step() int64 {
	// PPU的时钟是CPU三倍
	cpuCycles := console.CPU.Step()
	ppuCycles := cpuCycles * 3
	for i := 0; int64(i) < ppuCycles; i++ {
		console.PPU.Step()
	}
	return cpuCycles
}

func (console *Console) StepSeconds(seconds float64) {
	cycles := int64(CPUFrequency * seconds)
	for cycles > 0 {
		cycles -= console.Step()
	}
}

func (console *Console) SetButton1(buttons [8]bool) {
	console.Controller1.SetButtons(buttons)
}

func (console *Console) SetButton2(buttons [8]bool) {
	console.Controller2.SetButtons(buttons)
}

func (console *Console) Buffer() *image.RGBA {
	return console.PPU.front
}
