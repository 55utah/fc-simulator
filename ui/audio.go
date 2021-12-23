package ui

import (
	"github.com/55utah/fc-simulator/nes"

	"github.com/gordonklaus/portaudio"
)

type Audio struct {
	stream         *portaudio.Stream
	sampleRate     float64
	outputChannels int
	channel        chan float32
}

func NewAudio() *Audio {
	a := Audio{}
	// channel通道作为缓存区，越大声音延迟越大
	a.channel = make(chan float32, 8192)
	return &a
}

func (audio *Audio) RunAudio(console *nes.Console) {

	api, err := portaudio.DefaultHostApi()
	Check(err)

	parameters := portaudio.HighLatencyParameters(nil, api.DefaultOutputDevice)
	stream, err1 := portaudio.OpenStream(parameters, audio.Callback)
	Check(err1)

	audio.stream = stream
	audio.sampleRate = parameters.SampleRate
	audio.outputChannels = parameters.Output.Channels

	// 给apu设置输出通道、和帧率
	console.SetAudioSampleRate(audio.sampleRate)
	// console.SetAudioChannel(audio.channel)
	// 这里改为回调
	console.SetAudioOutputWork(func(f float32) {
		audio.channel <- f
	})

	err2 := stream.Start()
	Check(err2)
}

func (a *Audio) Stop() error {
	return a.stream.Close()
}

func (audio *Audio) Callback(out []float32) {
	var output float32
	for i := range out {
		if i%audio.outputChannels == 0 {
			select {
			case sample := <-audio.channel:
				output = sample
			default:
				output = 0
			}
		}
		out[i] = output
	}
}

func Check(err error) {
	if err != nil {
		panic(err)
	}
}
