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
	"github.com/gordonklaus/portaudio"

	"ines/nes"
)

func keyParse1(ev *fyne.KeyEvent) int {
	var index int = -1
	switch ev.Name {
	// A
	case "F":
		index = 0
		// B
	case "G":
		index = 1
		// Select
	case "R":
		index = 2
		// Start
	case "T":
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

func keyParse2(ev *fyne.KeyEvent) int {
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
	case "Up":
		index = 4
	case "Down":
		index = 5
	case "Left":
		index = 6
	case "Right":
		index = 7
	}
	return index
}

func keyParseSys(ev *fyne.KeyEvent, console *nes.Console) {
	// 重置游戏
	if ev.Name == "Q" {
		console.Reset()
	}
}

var ctrl1 [8]bool
var ctrl2 [8]bool

var frame image.Image = image.NewRGBA(image.Rect(0, 0, 1, 1))

func OpenWindow(console *nes.Console) {

	myApp := app.New()
	w := myApp.NewWindow("FC-55utah")
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
			if console == nil {
				return
			}

			index1 := keyParse1(ev)
			index2 := keyParse2(ev)

			keyParseSys(ev, console)

			if index1 >= 0 {
				ctrl1[index1] = true
				console.SetButton1(ctrl1)
			}
			if index2 >= 0 {
				ctrl2[index2] = true
				console.SetButton2(ctrl2)
			}
		})
		deskCanvas.SetOnKeyUp(func(ev *fyne.KeyEvent) {
			if console == nil {
				return
			}

			index1 := keyParse1(ev)
			index2 := keyParse2(ev)

			if index1 >= 0 {
				ctrl1[index1] = false
				console.SetButton1(ctrl1)
			}
			if index2 >= 0 {
				ctrl2[index2] = false
				console.SetButton2(ctrl2)
			}
		})
	}

	// 使用raster更新canvas画板，性能好一点，大概优化30%
	raster := canvas.NewRaster(func(w, h int) image.Image {
		return frame
	})

	go changeContent(raster, func() image.Image {
		return console.Buffer()
	})

	// 音频API初始化
	// 要将音频API的关闭、流的关闭放在主函数内！
	portaudio.Initialize()
	defer portaudio.Terminate()

	audio := NewAudio()
	audio.RunAudio(console)
	defer audio.Stop()

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
