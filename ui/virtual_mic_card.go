package ui

import (
	"context"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/button"
)

// virtualMicCard tells the user what to pick in Discord, Zoom or OBS, or
// how to get a virtual microphone driver.
type virtualMicCard struct {
	a   *App
	get *button.Button
}

func newVirtualMicCard(a *App) *virtualMicCard {
	c := &virtualMicCard{a: a}
	c.get = giode.Button("Get the free driver", primaryButton()).Icon(icon("external-link", 15, "")).OnClick(func() {
		url := a.svc.VirtualMic().InstallURL
		go func() {
			if err := a.win.OpenURL(context.Background(), url); err != nil {
				a.win.Post(func() { a.toast(giode.ToastError, "Couldn't open the browser: "+err.Error()) })
			}
		}()
	})
	return c
}

// View lays out the card.
func (c *virtualMicCard) View(st giode.Styles) giode.Element {
	vm := c.a.svc.VirtualMic()
	set := c.a.svc.Settings()
	var kids []giode.Element
	switch {
	case !set.VirtualMic:
		kids = append(kids, giode.Text("The virtual microphone is turned off in Settings. NoiseFree plays the clean audio to the output you chose.", muted))
	case vm.Ready:
		kids = append(kids,
			giode.Row(giode.Styles{Gap: 10, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Background: tintSuccess,
				BorderRadius: 12, Padding: giode.SymmetricInset(14, 12), Width: -1},
				icon("shield-check", 20, "theme.success"),
				giode.Stack(giode.Styles{Gap: 2, FlexGrow: 1},
					giode.Text("Ready", giode.Styles{FontSize: 13.5, FontWeight: giode.FontWeightSemiBold, Color: "theme.text"}),
					giode.Text("In Discord, Zoom, Teams, OBS or your browser, choose this microphone:", muted))),
			giode.Row(giode.Styles{Gap: 10, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Background: raised, BorderRadius: 10,
				BorderWidth: 1, BorderColor: "theme.border", Padding: giode.SymmetricInset(14, 10), Width: -1},
				icon("mic", 16, "theme.primary"),
				giode.Text(vm.MicName, giode.Styles{FontSize: 14, FontWeight: giode.FontWeightSemiBold, Color: "theme.text"})))
	default:
		kids = append(kids,
			giode.Row(giode.Styles{Gap: 10, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Background: tintWarning,
				BorderRadius: 12, Padding: giode.SymmetricInset(14, 12), Width: -1},
				icon("circle-alert", 20, "theme.warning"),
				giode.Text(vm.Help, giode.Styles{FontSize: 13, Color: "theme.text", LineHeight: 19, FlexGrow: 1})))
		if vm.InstallURL != "" {
			kids = append(kids, c.get)
		}
	}
	return sectionStyled(st, "mic", "Virtual microphone", "How other apps receive your clean voice.", nil, kids...)
}
