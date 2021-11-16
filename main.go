package main

import (
	"ines/nes"
	"ines/ui"
	"os"
)

func main() {
	args := os.Args
	if len(args) <= 1 {
		panic("need rom path")
	}
	file := args[1]
	info, err := os.Stat(file)
	if err != nil {
		panic(err)
	}
	if info.IsDir() {
		panic("invalid path")
	}
	console, err := nes.NewConsole(file)
	if err != nil {
		panic(err)
	}
	ui.OpenWindow(console)
}
