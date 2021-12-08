package nes

import "image"

/**
这个模块作为cpu/ppu/apu/mapper/card/RAM的封装
*/

type Console struct {
	CPU         *CPU
	APU         *APU
	PPU         *PPU
	Card        *Cartridge
	Controller1 *Controller
	Controller2 *Controller
	Mapper      Mapper
	RAM         []byte
}

func NewConsole(info []byte) (*Console, error) {
	card, err := LoadNESRom(info)
	if err != nil {
		return nil, err
	}

	ram := make([]byte, 2048)
	ctrl1 := NewController()
	ctrl2 := NewController()

	console := &Console{
		nil, nil, nil, card, ctrl1, ctrl2, nil, ram,
	}
	mapper, err := NewMapper(card, console)
	if err != nil {
		return nil, err
	}
	console.Mapper = mapper
	console.CPU = NewCPU(console)
	console.PPU = NewPPU(console)
	console.APU = NewAPU(console)

	return console, nil
}

func (console *Console) Reset() {
	console.CPU.Reset()
	console.PPU.Reset()
}

func (console *Console) Step() int64 {
	// PPU的时钟是CPU三倍
	cpuCycles := console.CPU.Step()
	ppuCycles := cpuCycles * 3
	for i := 0; int64(i) < ppuCycles; i++ {
		console.PPU.Step()
		// 部分mapper需要时钟信息
		console.Mapper.Step()
	}
	for j := 0; int64(j) < cpuCycles; j++ {
		console.APU.Step()
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

// func (console *Console) SetAudioChannel(channel chan float32) {
// 	console.APU.channel = channel
// }

// 将音频通过回调输出
func (console *Console) SetAudioOutputWork(callback func(float32)) {
	console.APU.outputWork = callback
}

func (console *Console) SetAudioSampleRate(sampleRate float64) {
	if sampleRate != 0 {
		// 将每秒帧率设置为每秒cpu步长
		console.APU.sampleRate = CPUFrequency / sampleRate
	}
}

func (console *Console) Buffer() *image.RGBA {
	return console.PPU.front
}
