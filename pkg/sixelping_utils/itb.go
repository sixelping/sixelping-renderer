package sixelping_utils

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
)

func ImageToBytes(img image.Image) []byte {
	buffer := new(bytes.Buffer)
	png.Encode(buffer, img)
	return buffer.Bytes()
}

func BlackImage(width int, height int) *image.RGBA {
	upLeft := image.Point{0, 0}
	lowRight := image.Point{width, height}
	rect := image.Rectangle{upLeft, lowRight}

	canvas := image.NewRGBA(rect)

	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{color.Black}, image.ZP, draw.Src)

	return canvas
}
