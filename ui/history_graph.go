package ui

import (
	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/styles/properties"
	"github.com/xmhrnx/noisefree/internal/dsp"
)

const historyLen = 100 // 5 s at 20 updates per second

// historyGraph is a scrolling mirrored waveform of the last few seconds:
// the raw microphone in grey behind the cleaned signal in brand colors.
type historyGraph struct {
	raw, clean [historyLen]float32
	pos        int
}

// push records one level pair; called on every UI tick.
func (g *historyGraph) push(raw, clean float32) {
	g.raw[g.pos], g.clean[g.pos] = dsp.ToMeter(raw), dsp.ToMeter(clean)
	g.pos = (g.pos + 1) % historyLen
}

// reset clears the history when audio stops.
func (g *historyGraph) reset() { *g = historyGraph{} }

var rawBar = giode.Mix("theme.text", "theme.surface", 0.16)

var historyColors = func() []string {
	c := make([]string, historyLen)
	for i := range c {
		c[i] = giode.Mix(brandGold, brandHot, float64(i)/historyLen)
	}
	return c
}()

// View draws the graph filling its parent's width.
func (g *historyGraph) View(height int) giode.Element {
	return giode.Canvas(giode.Styles{Width: -1, Height: properties.Height(height)}, func(p *giode.Painter) {
		w, h := p.Width(), p.Height()
		step := w / historyLen
		bar := max(step-2, 1)
		mid := h / 2
		p.Line(0, mid, w, mid, 1, track)
		for i := 0; i < historyLen; i++ {
			k := (g.pos + i) % historyLen
			x := float32(i) * step
			if r := g.raw[k] * (h - 4) / 2; r > 0.5 {
				p.Rect(x, mid-r, bar, 2*r, bar/2, rawBar)
			}
			if c := g.clean[k] * (h - 4) / 2; c > 0.5 {
				p.Rect(x, mid-c, bar, 2*c, bar/2, historyColors[i])
			}
		}
	})
}
