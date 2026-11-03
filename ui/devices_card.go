package ui

import (
	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/button"
	"github.com/xmhrnx/giode/components/dropdown"
	"github.com/xmhrnx/giode/components/toggle"
	"github.com/xmhrnx/noisefree/internal/settings"
)

// devicesCard picks the microphone, where the clean audio goes, and an
// optional monitor to hear yourself.
type devicesCard struct {
	a                      *App
	input, output, monitor *dropdown.Select
	inputs, outputs        []string
	monitorOn              *toggle.Switch
	refreshBtn             *button.Button
}

func newDevicesCard(a *App) *devicesCard {
	c := &devicesCard{a: a}
	c.input = giode.Input.Select(fieldStyle).OnChange(func(i int) {
		c.save(func(s *settings.Settings) { s.Input = pickName(c.inputs, i) })
	})
	c.output = giode.Input.Select(fieldStyle).OnChange(func(i int) {
		c.save(func(s *settings.Settings) { s.Output = pickName(c.outputs, i) })
	})
	c.monitor = giode.Input.Select(fieldStyle).OnChange(func(i int) {
		c.save(func(s *settings.Settings) { s.MonitorDevice = pickName(c.outputs, i) })
	})
	c.monitorOn = giode.Input.Switch("").OnChange(func(on bool) {
		c.save(func(s *settings.Settings) { s.Monitor = on })
	})
	c.monitorOn.SetChecked(a.svc.Settings().Monitor)
	c.refreshBtn = giode.Button("Rescan", secondaryButton()).Icon(icon("refresh-cw", 14, "")).OnClick(c.refresh)
	return c
}

// pickName maps a select index to a device name; index 0 is the default.
func pickName(names []string, i int) string {
	if i <= 0 || i > len(names) {
		return ""
	}
	return names[i-1]
}

func (c *devicesCard) save(fn func(*settings.Settings)) {
	c.a.svc.Update(fn)
	c.a.restartAudio()
}

// refresh rescans the devices in the background.
func (c *devicesCard) refresh() {
	go func() {
		ins, outs, err := c.a.svc.DeviceNames()
		c.a.win.Post(func() {
			if err != nil {
				c.a.toast(giode.ToastError, "Couldn't list audio devices: "+err.Error())
				return
			}
			c.inputs, c.outputs = ins, outs
			set := c.a.svc.Settings()
			fill(c.input, "System default microphone", ins, set.Input)
			fill(c.output, c.autoOutputLabel(), outs, set.Output)
			fill(c.monitor, "System default speakers", outs, set.MonitorDevice)
		})
	}()
}

func (c *devicesCard) autoOutputLabel() string {
	if vm := c.a.svc.VirtualMic(); vm.Ready && c.a.svc.Settings().VirtualMic {
		return "Virtual microphone (" + vm.MicName + ")"
	}
	return "System default output"
}

func fill(s *dropdown.Select, first string, names []string, current string) {
	s.SetOptions(append([]string{first}, names...)...)
	s.SetSelected(0)
	for i, n := range names {
		if n == current {
			s.SetSelected(i + 1)
		}
	}
}

// View lays out the card.
func (c *devicesCard) View(st giode.Styles) giode.Element {
	kids := []giode.Element{
		labelled("Microphone", c.input),
		labelled("Send clean audio to", c.output),
		giode.Divider(giode.Styles{Color: "theme.border"}),
		settingRow("Hear yourself", "Play the cleaned audio to your headphones.", c.monitorOn),
	}
	if c.a.svc.Settings().Monitor {
		kids = append(kids, c.monitor)
	}
	return sectionStyled(st, "headphones", "Devices", "Where audio comes from and goes to.", c.refreshBtn, kids...)
}
