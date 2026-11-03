package vision

import (
	"image"
	"image/color"
	"math"
	"math/rand"
	"sync"
)

// Preset is a built-in replacement background, drawn on first use.
type Preset struct {
	Name string
	draw func(w, h int) *image.RGBA
	once sync.Once
	img  image.Image
}

// Image returns the background at 1280x720.
func (p *Preset) Image() image.Image {
	p.once.Do(func() { p.img = p.draw(1280, 720) })
	return p.img
}

// Presets are the built-in backgrounds.
var Presets = []*Preset{
	{Name: "Studio", draw: func(w, h int) *image.RGBA {
		return radial(w, h, color.RGBA{92, 96, 104, 255}, color.RGBA{28, 30, 34, 255}, 0.45, 0.4)
	}},
	{Name: "Bokeh", draw: func(w, h int) *image.RGBA {
		img := radial(w, h, color.RGBA{58, 52, 60, 255}, color.RGBA{18, 17, 22, 255}, 0.5, 0.45)
		bokeh(img, 7, []color.RGBA{{255, 196, 140, 255}, {255, 230, 190, 255}, {170, 190, 255, 255}})
		return img
	}},
	{Name: "Dusk", draw: func(w, h int) *image.RGBA {
		return linear(w, h, color.RGBA{58, 70, 110, 255}, color.RGBA{214, 140, 120, 255})
	}},
	{Name: "Mint", draw: func(w, h int) *image.RGBA {
		return radial(w, h, color.RGBA{190, 222, 208, 255}, color.RGBA{108, 150, 138, 255}, 0.5, 0.35)
	}},
}

func mix(a, b color.RGBA, t float64) color.RGBA {
	m := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t + 0.5) }
	return color.RGBA{m(a.R, b.R), m(a.G, b.G), m(a.B, b.B), 255}
}

// radial is a soft light centred at (cx, cy) in frame fractions.
func radial(w, h int, inner, outer color.RGBA, cx, cy float64) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := float64(x)/float64(w)-cx, (float64(y)/float64(h)-cy)*float64(h)/float64(w)
			t := math.Min(1, math.Sqrt(dx*dx+dy*dy)/0.75)
			img.SetRGBA(x, y, mix(inner, outer, t*t*(3-2*t)))
		}
	}
	return img
}

func linear(w, h int, top, bottom color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		c := mix(top, bottom, float64(y)/float64(h-1))
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

// bokeh scatters soft out-of-focus lights.
func bokeh(img *image.RGBA, seed int64, colors []color.RGBA) {
	r := rand.New(rand.NewSource(seed))
	w, h := img.Rect.Dx(), img.Rect.Dy()
	for n := 0; n < 26; n++ {
		cx, cy := r.Float64()*float64(w), r.Float64()*float64(h)*0.8
		rad := 30 + r.Float64()*70
		c := colors[r.Intn(len(colors))]
		alpha := 0.10 + r.Float64()*0.18
		for y := int(cy - rad); y < int(cy+rad); y++ {
			for x := int(cx - rad); x < int(cx+rad); x++ {
				if !image.Pt(x, y).In(img.Rect) {
					continue
				}
				d := math.Hypot(float64(x)-cx, float64(y)-cy) / rad
				if d >= 1 {
					continue
				}
				a := alpha * math.Min(1, (1-d)*4)
				img.SetRGBA(x, y, mix(img.RGBAAt(x, y), c, a))
			}
		}
	}
}
