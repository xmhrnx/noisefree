package ui

import (
	"context"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/button"
	"github.com/xmhrnx/giode/components/toggle"
	"github.com/xmhrnx/noisefree/internal/settings"
)

// virtualCameraCard sends the result to other apps.
type virtualCameraCard struct {
	a       *App
	on      *toggle.Switch
	enable  *button.Button
	install *button.Button
}

func newVirtualCameraCard(a *App) *virtualCameraCard {
	c := &virtualCameraCard{a: a}
	c.on = giode.Input.Switch("").OnChange(func(on bool) {
		a.svc.Update(func(s *settings.Settings) { s.Video.VirtualCamera = on })
		a.camera.restart()
	})
	c.on.SetChecked(a.svc.Settings().Video.VirtualCamera)
	c.enable = giode.Button("Turn on virtual camera", primaryButton()).Icon(icon("video", 15, "")).OnClick(func() {
		c.enable.Loading(true)
		go func() {
			err := a.svc.Video.EnableVirtualCamera()
			a.win.Post(func() {
				c.enable.Loading(false)
				if err != nil {
					a.toast(giode.ToastError, err.Error())
				}
			})
		}()
	})
	c.install = giode.Button("Get it", secondaryButton()).Icon(icon("external-link", 15, "")).OnClick(func() {
		url := a.svc.Video.Status().VirtualCam.InstallURL
		go func() {
			if err := a.win.OpenURL(context.Background(), url); err != nil {
				a.win.Post(func() { a.toast(giode.ToastError, err.Error()) })
			}
		}()
	})
	return c
}

// View lays out the card.
func (c *virtualCameraCard) View(st giode.Styles, vs videoStatusView) giode.Element {
	vc := vs.VirtualCam
	if !vs.Running {
		vc = c.a.svc.Video.DetectVirtualCamera()
	}
	kids := []giode.Element{settingRow("Send to other apps", "Zoom, Teams, Meet, Discord, OBS and the browser see the processed video.", c.on)}
	switch {
	case vs.VirtualOn:
		kids = append(kids, statusRow("shield-check", "theme.success", tintSuccess, "Live", "Pick this camera in your apps: "+vc.Name))
	case vc.Ready:
		kids = append(kids, statusRow("shield-check", "theme.success", tintSuccess, "Ready", "Pick \""+vc.Name+"\" as the camera in your apps."))
	default:
		kids = append(kids, statusRow("circle-alert", "theme.warning", tintWarning, "Not set up", vc.Help))
		if vc.CanEnable {
			kids = append(kids, c.enable)
		} else if vc.InstallURL != "" {
			kids = append(kids, c.install)
		}
	}
	if vs.VirtualError != nil && vc.Ready { // otherwise the help above already says what's missing
		kids = append(kids, giode.Text(vs.VirtualError.Error(), giode.Styles{FontSize: 12.5, Color: "theme.danger"}))
	}
	return sectionStyled(st, "video", "Virtual camera", "How other apps receive your video.", nil, kids...)
}

// statusRow is a tinted status banner.
func statusRow(iconName, color, tint, title, text string) giode.Element {
	return giode.Row(giode.Styles{Gap: 10, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Background: tint,
		BorderRadius: 12, Padding: giode.SymmetricInset(14, 12), Width: -1},
		icon(iconName, 20, color),
		giode.Stack(giode.Styles{Gap: 2, FlexGrow: 1},
			giode.Text(title, giode.Styles{FontSize: 13.5, FontWeight: giode.FontWeightSemiBold, Color: "theme.text"}),
			giode.Text(text, muted)))
}
