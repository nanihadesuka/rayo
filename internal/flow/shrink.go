package flow

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
)

// shrink makes a screenshot small, as it is only to see what happened: the short side is at most 480,
// as in the videos. A real one goes from megabytes to some 30 KB.
func shrink(data []byte) ([]byte, error) {
	full, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	size := full.Bounds().Size()
	const most = 480
	w, h := size.X, size.Y
	if short := min(w, h); short > most {
		w, h = w*most/short, h*most/short
	}
	small := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			small.Set(x, y, average(full, image.Rect(x*size.X/w, y*size.Y/h, (x+1)*size.X/w, (y+1)*size.Y/h).Add(full.Bounds().Min)))
		}
	}
	var b bytes.Buffer
	err = jpeg.Encode(&b, small, &jpeg.Options{Quality: 60})
	return b.Bytes(), err
}

// average is the colour of a part, so that thin lines and small text don't vanish.
func average(from image.Image, part image.Rectangle) color.Color {
	var r, g, b, n uint32
	for y := part.Min.Y; y < part.Max.Y; y++ {
		for x := part.Min.X; x < part.Max.X; x++ {
			cr, cg, cb, _ := from.At(x, y).RGBA()
			r, g, b, n = r+cr>>8, g+cg>>8, b+cb>>8, n+1
		}
	}
	if n == 0 {
		return color.Black
	}
	return color.RGBA{uint8(r / n), uint8(g / n), uint8(b / n), 255}
}
