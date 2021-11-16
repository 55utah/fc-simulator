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

var frame image.Image = image.NewRGBA(image.Rect(0, 0, 1, 1))

func OpenWindow(console *nes.Console) {

	myApp := app.New()
	w := myApp.NewWindow("TinyFC")
	width := 256
	height := 240
	buf := 20
	w.Resize(fyne.NewSize(width+buf, height+buf))

	// 禁止用户缩放窗口
	w.SetFixedSize(true)

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
			if console != nil {
				console.SetButton1(ctrl1)
			}
		})
		deskCanvas.SetOnKeyUp(func(ev *fyne.KeyEvent) {
			index := keyParse(ev)
			if index < 0 {
				return
			}
			ctrl1[index] = false
			if console != nil {
				console.SetButton1(ctrl1)
			}
		})
	}

	// 使用raster更新canvas画板，性能好一点
	raster := canvas.NewRaster(func(w, h int) image.Image {
		return frame
	})
	// wrap := container.NewGridWithRows(1, btn1)
	// layout := layout.NewGridWrapLayout(fyne.NewSize(int((float32(width) * zoom)), int(float32(height)*zoom)))
	// winRaster := fyne.NewContainerWithLayout(layout, raster)
	// document := container.NewVBox(wrap, winRaster)

	go changeContent(raster, func() image.Image {
		return console.Buffer()
	})

	w.SetContent(raster)
	w.ShowAndRun()
}

func changeContent(raster *canvas.Raster, getFrame func() image.Image) {
	for {
		// 模拟接近60fps的图像刷新率
		time.Sleep(time.Millisecond * 20)
		frame = getFrame()
		raster.Refresh()
	}
}
