package ui

import (
	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/styles/properties"
)

// logo draws the NoiseFree mark: a gradient tile with a microphone and
// clean sound waves, matching the app icon.
func logo(size float32) giode.Element {
	return giode.Box(giode.Styles{
		Width: properties.Width(size), Height: properties.Height(size), BorderRadius: properties.BorderRadius(size * 0.3),
		BackgroundGradient: giode.Linear(150, brandHot, brandGold),
		BoxShadow:          giode.BoxShadow{Y: 4, Blur: 14, Color: "#FF5E3A55"},
	}, giode.Canvas(giode.Styles{Width: properties.Width(size), Height: properties.Height(size)}, func(p *giode.Painter) {
		u := size / 100 // the icon is designed on a 100-unit grid
		cx, cy := 50*u, 44*u
		w := 4.6 * u
		p.Rect(cx-9.5*u, 22*u, 19*u, 33*u, 9.5*u, "#FFFFFF") // capsule
		p.Arc(cx, cy, 17*u, w, 90, 180, "#FFFFFF")           // holder
		p.Line(cx, 61*u, cx, 73*u, w, "#FFFFFF")             // stem
		p.Line(cx-9*u, 74*u, cx+9*u, 74*u, w, "#FFFFFF")     // base
		p.Arc(cx, cy, 27*u, 3.6*u, 66, 48, "#FFFFFFEE")      // waves
		p.Arc(cx, cy, 27*u, 3.6*u, 246, 48, "#FFFFFFEE")
	}))
}
