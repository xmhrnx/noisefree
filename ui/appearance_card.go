package ui

import (
	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/slider"
	"github.com/xmhrnx/giode/components/toggle"
	"github.com/xmhrnx/noisefree/internal/settings"
)

// appearanceCard holds eye contact and lighting.
type appearanceCard struct {
	a                *App
	eyes, light      *toggle.Switch
	eyeAmt, lightAmt *slider.Slider
}

func newAppearanceCard(a *App) *appearanceCard {
	c := &appearanceCard{a: a}
	set := a.svc.Settings().Video
	st := giode.Styles{Color: "theme.primary", Background: track, BorderColor: "theme.primary"}
	upd := func(fn func(*settings.Video)) {
		a.svc.Update(func(s *settings.Settings) { fn(&s.Video) })
		a.svc.Video.Apply()
	}
	c.eyes = giode.Input.Switch("").OnChange(func(on bool) { upd(func(v *settings.Video) { v.EyeContact = on }) })
	c.eyes.SetChecked(set.EyeContact)
	c.eyeAmt = giode.Input.Slider(st).OnChange(func(x float32) { upd(func(v *settings.Video) { v.EyeStrength = x }) })
	c.eyeAmt.SetValue(set.EyeStrength)
	c.light = giode.Input.Switch("").OnChange(func(on bool) { upd(func(v *settings.Video) { v.Lighting = on }) })
	c.light.SetChecked(set.Lighting)
	c.lightAmt = giode.Input.Slider(st).OnChange(func(x float32) { upd(func(v *settings.Video) { v.LightStrength = x }) })
	c.lightAmt.SetValue(set.LightStrength)
	return c
}

// View lays out the card.
func (c *appearanceCard) View(st giode.Styles, vs videoStatusView) giode.Element {
	set := c.a.svc.Settings().Video
	c.eyes.Disabled(!vs.ModelsReady)
	eyeDesc := "Moves your eyes towards the camera while you read the screen. Stands down when you blink or turn away."
	if !vs.ModelsReady {
		eyeDesc = "Needs the AI effects."
	}
	kids := []giode.Element{settingRow("Eye contact", eyeDesc, c.eyes)}
	if set.EyeContact {
		kids = append(kids, sliderRow("Eye contact strength", percent(set.EyeStrength), c.eyeAmt))
	}
	kids = append(kids, giode.Divider(giode.Styles{Color: "theme.border"}),
		settingRow("Studio lighting", "Evens out exposure and colour, and adds a soft light on your face.", c.light))
	if set.Lighting {
		kids = append(kids, sliderRow("Lighting strength", percent(set.LightStrength), c.lightAmt))
	}
	return sectionStyled(st, "eye", "Appearance", "Look your best without a ring light.", nil, kids...)
}
