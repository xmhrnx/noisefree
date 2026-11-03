package ui

import (
	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/noisefree/internal/dsp"
)

const meterSegments = 36

// meterColors grade the lit segments from the brand violet to cyan, with
// the last segments warning of clipping.
var meterColors = func() []string {
	c := make([]string, meterSegments)
	for i := range c {
		t := float32(i) / float32(meterSegments-1)
		switch {
		case t > 0.92:
			c[i] = "theme.danger"
		case t > 0.8:
			c[i] = "theme.warning"
		default:
			c[i] = giode.Mix(brandCyan, brandViolet, float64(t/0.8))
		}
	}
	return c
}()

// levelMeter draws a segmented level bar for a linear level and peak.
func levelMeter(label string, level, peak float32, active bool) giode.Element {
	lit := int(dsp.ToMeter(level) * meterSegments)
	pk := int(dsp.ToMeter(peak) * meterSegments)
	return giode.Stack(giode.Styles{Gap: 8, Width: -1},
		giode.Row(giode.Styles{Wrap: giode.WrapNoWrap, Align: giode.AlignCenter},
			giode.Text(label, overline), spacer(), giode.Text(levelText(peak), caption)),
		giode.Canvas(giode.Styles{Width: -1, Height: 10}, func(p *giode.Painter) {
			w := (p.Width() - float32(meterSegments-1)*3) / meterSegments
			for i := 0; i < meterSegments; i++ {
				color := track
				if active && (i < lit || i == pk-1) {
					color = meterColors[i]
				}
				p.Rect(float32(i)*(w+3), 0, w, 10, 3, color)
			}
		}))
}

func levelText(peak float32) string {
	if peak < 0.001 {
		return "−∞ dB"
	}
	return dB(dsp.ToMeter(peak)*60 - 60)
}
