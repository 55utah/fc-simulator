package ui

import (
	"image"
)

func Resize(source *image.RGBA, w int, h int, ratio int) *image.RGBA {

	tw := w * ratio
	th := h * ratio

	var target *image.RGBA = image.NewRGBA(image.Rect(0, 0, tw, th))

	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			sx := x / ratio
			sy := y / ratio
			target.SetRGBA(x, y, source.RGBAAt(sx, sy))
		}
	}

	return target
}
