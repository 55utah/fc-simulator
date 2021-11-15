package ui

import (
	"ines/nes"
	"time"
)

var stop bool = false
var timestamp float64

func floatSecond() float64 {
	return float64(time.Now().Nanosecond()) * float64(1e-9)
}

func RunView(console *nes.Console) {
	timestamp = floatSecond()
	for !stop {
		RunStep(console)
	}
}

func RunStep(console *nes.Console) {
	current := floatSecond()
	console.StepSeconds(current - timestamp)
	timestamp = current
}
