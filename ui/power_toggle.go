package ui

import (
	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/noisefree/internal/dsp"
)

const ringSegments = 48

// ringColors sweep the brand gradient around the ring.
var ringColors = func() []string {
	c := make([]string, ringSegments)
	for i := range c {
		t := float32(i) / ringSegments
		if t > 0.5 {
			t = 1 - t
		}
		c[i] = giode.Mix(brandGold, brandHot, float64(t*2))
	}
	return c
}()

// Precomputed colors: the view runs every frame, so nothing is mixed there.
var (
	haloColors   = []string{giode.Mix(brandHot, "theme.surface", 0.10), giode.Mix(brandHot, "theme.surface", 0.16), giode.Mix(brandHot, "theme.surface", 0.24)}
	discOff      = giode.Mix("theme.text", "theme.surface", 0.05)
	discOffHover = giode.Mix("theme.text", "theme.surface", 0.09)
	discOn       = giode.Mix(brandHot, "#1C0A05", 0.55)
	discOnHover  = giode.Mix(brandHot, "#1C0A05", 0.65)
)

// powerToggle is the big round on/off control. Its glow breathes with the
// cleaned output level while noise removal is on.
type powerToggle struct {
	press *giode.PressableElement
	on    func() bool
	level func() float32
}

func newPowerToggle(on func() bool, level func() float32, toggle func()) *powerToggle {
	t := &powerToggle{on: on, level: level}
	t.press = giode.Pressable(t.content).Label("Toggle noise removal").
		OnClick(func(giode.ClickEvent) { toggle() })
	return t
}

func (t *powerToggle) content(s giode.PressState) giode.Element {
	on := t.on()
	lvl := float32(0)
	if on {
		lvl = dsp.ToMeter(t.level())
	}
	const size = 176
	glyph := "theme.muted"
	if on {
		glyph = "#FFFFFF"
	}
	ring := giode.Canvas(giode.Styles{Width: size, Height: size}, func(p *giode.Painter) {
		c := float32(size) / 2
		if on { // soft halo, breathing with the voice
			for i, col := range haloColors {
				p.Circle(c, c, c-4-float32(2-i)*7+lvl*6, col)
			}
		}
		disc := discOff
		switch {
		case on && s.Hovered:
			disc = discOnHover
		case on:
			disc = discOn
		case s.Hovered:
			disc = discOffHover
		}
		p.Circle(c, c, c-26, disc)
		if !on {
			p.Arc(c, c, c-20, 5, 0, 360, track)
			return
		}
		sweep := float32(360) / ringSegments
		for i := 0; i < ringSegments; i++ {
			p.Arc(c, c, c-20, 5, float32(i)*sweep, sweep+0.8, ringColors[i])
		}
	})
	return giode.Box(giode.Styles{Width: size, Height: size},
		ring,
		giode.Box(giode.Styles{Position: giode.PositionAbsolute, Top: size/2 - 22, Left: size/2 - 22},
			icon("power", 44, glyph)))
}

// View lays the toggle out.
func (t *powerToggle) View() giode.Element { return t.press }
