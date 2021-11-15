package main

import (
	"image"
	"ines/nes"
	"ines/ui"
)

func main() {

	console, err := nes.NewConsole("mario.nes")

	if err != nil {
		panic(err)
	}
	console.Reset()

	ui.OpenWindow(console, func() image.Image {
		frame := console.Buffer()
		return frame
	})

}
