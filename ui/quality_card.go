package ui

import (
	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/toggle"
	"github.com/xmhrnx/noisefree/internal/settings"
)

// qualityCard cleans up the webcam image.
type qualityCard struct {
	a                *App
	denoise, sharpen *toggle.Switch
}

func newQualityCard(a *App) *qualityCard {
	c := &qualityCard{a: a}
	set := a.svc.Settings().Video
	upd := func(fn func(*settings.Video)) {
		a.svc.Update(func(s *settings.Settings) { fn(&s.Video) })
		a.svc.Video.Apply()
	}
	c.denoise = giode.Input.Switch("").OnChange(func(on bool) { upd(func(v *settings.Video) { v.Denoise = on }) })
	c.denoise.SetChecked(set.Denoise)
	c.sharpen = giode.Input.Switch("").OnChange(func(on bool) { upd(func(v *settings.Video) { v.Sharpen = on }) })
	c.sharpen.SetChecked(set.Sharpen)
	return c
}

// View lays out the card.
func (c *qualityCard) View(st giode.Styles) giode.Element {
	return sectionStyled(st, "sparkles", "Image quality", "Clean up a grainy or soft webcam.", nil,
		settingRow("Video denoise", "Removes the grain webcams add in dim rooms, without smearing movement.", c.denoise),
		giode.Divider(giode.Styles{Color: "theme.border"}),
		settingRow("Sharpen", "Brings back detail lost to webcam compression.", c.sharpen))
}
