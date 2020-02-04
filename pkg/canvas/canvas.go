package canvas

import (
	"errors"
	"image"
	"image/draw"
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

func (c *Canvas) AddDelta(deltaImage []byte) error {
	now := uint64(time.Now().UnixNano())

	c.mut.Lock()
	defer c.mut.Unlock()
	for x := 0; x < c.Width; x++ {
		for y := 0; y < c.Height; y++ {
			i := (y*(c.Width) + x) * 4
			r, g, b, a := deltaImage[i+2], deltaImage[i+1], deltaImage[i], deltaImage[i+3]
			if a > 0 {
				c.R[y*c.Width+x] = r
				c.G[y*c.Width+x] = g
				c.B[y*c.Width+x] = b
				c.LastUpdated[y*c.Width+x] = now
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
			lu := c.LastUpdated[y*c.Width+x]
			dt := now - lu

			fac := float32(0.0)
			if c.LastUpdated[y*c.Width+x] > now {
				//If pixel is newer than now, draw it fully
				fac = float32(1.0)
			} else {
				//Calculate darkness
				fac = float32(1.0) - (float32(dt) / float32(c.PixelTimeoutNano))

				if fac < 0.0 {
					fac = float32(0.0)
				}
			}

			index := (y-img.Rect.Min.Y)*img.Stride + (x-img.Rect.Min.X)*4

			if fac > 0.0 {
				img.Pix[index] = uint8(fac * float32(c.R[y*c.Width+x]))
				img.Pix[index+1] = uint8(fac * float32(c.G[y*c.Width+x]))
				img.Pix[index+2] = uint8(fac * float32(c.B[y*c.Width+x]))
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
