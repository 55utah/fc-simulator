package main

import (
	"io/ioutil"
	"os"

	"github.com/55utah/fc-simulator/nes"
	"github.com/55utah/fc-simulator/ui"
)

func main() {
	args := os.Args
	if len(args) <= 1 {
		panic("need rom path.")
	}
	filePath := args[1]
	info, err := os.Stat(filePath)
	if err != nil {
		panic(err)
	}
	if info.IsDir() {
		panic("invalid path.")
	}

	// 调试用
	// filePath := "/Users/utahcoder/Desktop/nes-roms/中东战争.nes"

	file, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	fileData, err2 := ioutil.ReadAll(file)
	if err2 != nil {
		panic(err2)
	}

	console, err := nes.NewConsole(fileData)
	if err != nil {
		panic(err)
	}
	ui.OpenWindow(console)
}
