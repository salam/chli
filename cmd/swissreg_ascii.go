package cmd

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"
)

// asciiRamp maps 0 (black/transparent) → 10 (white). Trademark images are
// typically dark-ink-on-white, so dense glyphs represent ink.
const asciiRamp = " .:-=+*#%@"

// renderASCII decodes an image (PNG/JPEG/GIF) and returns a string rendering.
// cols controls the output width; the row count is derived from the image
// aspect ratio assuming a ~2:1 char height:width so circles stay circular.
func renderASCII(data []byte, cols int) (string, error) {
	if cols <= 0 {
		cols = 60
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("decode image: %w", err)
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w == 0 || h == 0 {
		return "", fmt.Errorf("empty image")
	}
	rows := h * cols / w / 2
	if rows < 1 {
		rows = 1
	}

	// Invert the ramp: on white backgrounds, dark ink should be rendered with
	// dense characters. Here we output density ∝ (1 - luminance).
	// First pass: box-average luminance into a cells grid so thin strokes
	// don't get lost to nearest-neighbor sampling.
	cells := make([]float64, cols*rows)
	for row := 0; row < rows; row++ {
		y0 := bounds.Min.Y + row*h/rows
		y1 := bounds.Min.Y + (row+1)*h/rows
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for col := 0; col < cols; col++ {
			x0 := bounds.Min.X + col*w/cols
			x1 := bounds.Min.X + (col+1)*w/cols
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var sum float64
			var n int
			for y := y0; y < y1; y++ {
				for x := x0; x < x1; x++ {
					sum += luminance(img.At(x, y))
					n++
				}
			}
			cells[row*cols+col] = sum / float64(n)
		}
	}

	// Contrast-stretch: rescale observed [min,max] → [0,1] so faint ink on
	// large white backgrounds still produces glyphs.
	lo, hi := 1.0, 0.0
	for _, v := range cells {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	span := hi - lo
	if span < 0.01 {
		span = 0.01
	}

	rampLen := len(asciiRamp)
	var b strings.Builder
	b.Grow((cols + 1) * rows)
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			norm := (cells[row*cols+col] - lo) / span
			// Dark = dense. 0.0 → last glyph, 1.0 → first (space).
			idx := int((1.0 - norm) * float64(rampLen-1))
			if idx < 0 {
				idx = 0
			}
			if idx >= rampLen {
				idx = rampLen - 1
			}
			b.WriteByte(asciiRamp[idx])
		}
		b.WriteByte('\n')
	}
	return b.String(), nil
}

// luminance returns perceived brightness in [0,1]. Fully transparent pixels
// read as white (background) so the rendering matches what a viewer shows.
func luminance(c color.Color) float64 {
	r, g, bl, a := c.RGBA()
	if a == 0 {
		return 1.0
	}
	// Un-premultiply (image/color returns premultiplied alpha).
	if a > 0 && a < 0xffff {
		r = r * 0xffff / a
		g = g * 0xffff / a
		bl = bl * 0xffff / a
	}
	// Rec. 709 luma, normalized to [0,1].
	y := 0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(bl)
	return y / 0xffff
}
