/*
负责ui渲染，声音输出，接受控制的模块
*/

package ui

import (
	"image"
	"time"

	"fyne.io/fyne"
	"fyne.io/fyne/app"
	"fyne.io/fyne/canvas"
	"fyne.io/fyne/driver/desktop"

	"ines/nes"
)

func keyParse(ev *fyne.KeyEvent) int {
	var index int = -1
	switch ev.Name {
	// A
	case "J":
		index = 0
		// B
	case "K":
		index = 1
		// Select
	case "U":
		index = 2
		// Start
	case "I":
		index = 3
	case "W":
		index = 4
	case "S":
		index = 5
	case "A":
		index = 6
	case "D":
		index = 7
	}
	return index
}

var ctrl1 [8]bool

func OpenWindow(console *nes.Console, getFrame func() image.Image) {

	myApp := app.New()
	w := myApp.NewWindow("TinyFC")
	w.Resize(fyne.NewSize(260, 260))
	myCanvas := w.Canvas()

	// 禁止用户缩放窗口
	// w.SetFixedSize(true)

	go func() {
		RunView(console)
	}()

	if deskCanvas, ok := w.Canvas().(desktop.Canvas); ok {
		deskCanvas.SetOnKeyDown(func(ev *fyne.KeyEvent) {
			index := keyParse(ev)
			if index < 0 {
				return
			}
			ctrl1[index] = true
			console.SetButton1(ctrl1)
		})
		deskCanvas.SetOnKeyUp(func(ev *fyne.KeyEvent) {
			index := keyParse(ev)
			if index < 0 {
				return
			}
			ctrl1[index] = false
			console.SetButton1(ctrl1)
		})
	}

	go changeContent(myCanvas, getFrame)

	w.ShowAndRun()
}

func changeContent(can fyne.Canvas, getFrame func() image.Image) {
	for {
		// 模拟接近60fps的图像刷新率
		time.Sleep(time.Millisecond * 20)
		result := getFrame()
		res := canvas.NewImageFromImage(result)
		can.SetContent(res)
	}
}
