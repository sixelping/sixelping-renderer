package canvas

import (
	"errors"
	"image"
	"image/draw"
	"math"
	"sync"
	"time"
)

type Canvas struct {
	R                []uint8
	G                []uint8
	B                []uint8
	LastUpdated      []uint64
	Width            int
	Height           int
	mut              sync.Mutex
	overlay          image.Image
	PixelTimeoutNano uint64
}

func NewCanvas(width int, height int, pixelTimeoutNano uint64) *Canvas {
	return &Canvas{
		Width:            width,
		Height:           height,
		R:                make([]uint8, width*height),
		G:                make([]uint8, width*height),
		B:                make([]uint8, width*height),
		LastUpdated:      make([]uint64, width*height),
		PixelTimeoutNano: pixelTimeoutNano,
	}
}

func (c *Canvas) SetOverlayImage(img image.Image) error {
	if img.Bounds().Max.X != c.Width || img.Bounds().Max.Y != c.Height {
		return errors.New("Invalid width/height")
	}
	c.mut.Lock()
	defer c.mut.Unlock()
	c.overlay = img

	return nil
}

func (c *Canvas) AddDelta(deltaImage image.Image) error {
	if deltaImage.Bounds().Max.X != c.Width || deltaImage.Bounds().Max.Y != c.Height {
		return errors.New("Invalid width/height")
	}

	c.mut.Lock()
	defer c.mut.Unlock()
	for x := 0; x < c.Width; x++ {
		for y := 0; y < c.Height; y++ {
			col := deltaImage.At(x, y)
			r, g, b, a := col.RGBA()
			if a > 0 {
				c.R[y*c.Width+x] = uint8((r * 0xFF) / 0xFFFF)
				c.G[y*c.Width+x] = uint8((g * 0xFF) / 0xFFFF)
				c.B[y*c.Width+x] = uint8((b * 0xFF) / 0xFFFF)
				c.LastUpdated[y*c.Width+x] = uint64(time.Now().UnixNano())
			}
		}
	}

	return nil
}

func (c *Canvas) drawImage(now uint64, img *image.RGBA) error {
	c.mut.Lock()
	defer c.mut.Unlock()
	for x := 0; x < c.Width; x++ {
		for y := 0; y < c.Height; y++ {
			dt := now - c.LastUpdated[y*c.Width+x]
			fac := float64(1.0) - math.Min(float64(dt)/float64(c.PixelTimeoutNano), float64(1.0))

			//If pixel is newer than now, draw it fully
			if c.LastUpdated[y*c.Width+x] > now {
				fac = float64(1.0)
			}

			index := (y-img.Rect.Min.Y)*img.Stride + (x-img.Rect.Min.X)*4

			if fac > 0.0 {
				img.Pix[index] = uint8(fac * float64(c.R[y*c.Width+x]))
				img.Pix[index+1] = uint8(fac * float64(c.G[y*c.Width+x]))
				img.Pix[index+2] = uint8(fac * float64(c.B[y*c.Width+x]))
			} else {
				img.Pix[index] = 0
				img.Pix[index+1] = 0
				img.Pix[index+2] = 0
			}
			img.Pix[index+3] = 255
		}
	}
	return nil
}

func (c *Canvas) GetImage(now time.Time) (*image.RGBA, error) {
	img := image.NewRGBA(image.Rectangle{image.Point{0, 0}, image.Point{c.Width, c.Height}})

	err := c.drawImage(uint64(now.UnixNano()), img)
	if err != nil {
		return nil, err
	}

	// Add overlay image ontop
	if c.overlay != nil {
		draw.Draw(img, img.Bounds(), c.overlay, image.ZP, draw.Over)
	}

	return img, nil
}
