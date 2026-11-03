package ui

import (
	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/slider"
	"github.com/xmhrnx/giode/components/toggle"
	"github.com/xmhrnx/noisefree/internal/settings"
)

// Slider ranges, in dB.
const (
	gateMin, gateRange = -80, 60
	gainMin, gainRange = -12, 24
)

// enhancementsCard holds the extra voice processing: rumble filter, noise
// gate and output gain.
type enhancementsCard struct {
	a         *App
	highPass  *toggle.Switch
	gate      *toggle.Switch
	gateLevel *slider.Slider
	gain      *slider.Slider
}

func newEnhancementsCard(a *App) *enhancementsCard {
	c := &enhancementsCard{a: a}
	set := a.svc.Settings()
	sliderStyle := giode.Styles{Color: "theme.accent", Background: track, BorderColor: "theme.accent"}
	c.highPass = giode.Input.Switch("").OnChange(func(on bool) { a.svc.Update(func(s *settings.Settings) { s.HighPass = on }) })
	c.highPass.SetChecked(set.HighPass)
	c.gate = giode.Input.Switch("").OnChange(func(on bool) { a.svc.Update(func(s *settings.Settings) { s.Gate = on }) })
	c.gate.SetChecked(set.Gate)
	c.gateLevel = giode.Input.Slider(sliderStyle).OnChange(func(v float32) {
		a.svc.Update(func(s *settings.Settings) { s.GateDB = gateMin + v*gateRange })
	})
	c.gateLevel.SetValue((set.GateDB - gateMin) / gateRange)
	c.gain = giode.Input.Slider(sliderStyle).OnChange(func(v float32) {
		a.svc.Update(func(s *settings.Settings) { s.GainDB = gainMin + v*gainRange })
	})
	c.gain.SetValue((set.GainDB - gainMin) / gainRange)
	return c
}

// View lays out the card.
func (c *enhancementsCard) View(st giode.Styles) giode.Element {
	set := c.a.svc.Settings()
	kids := []giode.Element{
		settingRow("Rumble filter", "Cuts desk thumps, traffic rumble and hum below 80 Hz.", c.highPass),
		giode.Divider(giode.Styles{Color: "theme.border"}),
		settingRow("Noise gate", "Mutes the microphone completely between words.", c.gate),
	}
	if set.Gate {
		kids = append(kids, sliderRow("Gate threshold", dB(set.GateDB), c.gateLevel))
	}
	kids = append(kids, giode.Divider(giode.Styles{Color: "theme.border"}),
		sliderRow("Output volume", dB(set.GainDB), c.gain))
	return sectionStyled(st, "sliders-horizontal", "Voice enhancements", "Lightweight extras, applied after noise removal.", nil, kids...)
}

func sliderRow(label, value string, s *slider.Slider) giode.Element {
	return giode.Stack(giode.Styles{Gap: 10, Width: -1},
		giode.Row(giode.Styles{Wrap: giode.WrapNoWrap, Align: giode.AlignCenter},
			giode.Text(label, overline), spacer(),
			giode.Text(value, giode.Styles{FontSize: 13, FontWeight: giode.FontWeightSemiBold, Color: "theme.accent"})),
		s)
}
