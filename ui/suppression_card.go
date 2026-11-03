package ui

import (
	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/button"
	"github.com/xmhrnx/giode/components/dropdown"
	"github.com/xmhrnx/giode/components/slider"
	"github.com/xmhrnx/giode/components/toggle"
	"github.com/xmhrnx/noisefree/internal/catalog"
	"github.com/xmhrnx/noisefree/internal/settings"
)

// suppressionCard picks the engine, its strength and where it runs.
type suppressionCard struct {
	a        *App
	engine   *dropdown.Select
	ids      []string
	strength *slider.Slider
	gpu      *toggle.Switch
	more     *button.Button
	getPack  *button.Button
}

// packMissing reports whether GPU use waits on the GPU acceleration pack.
func (c *suppressionCard) packMissing() bool {
	e, ok := c.a.svc.Store.Get("onnxruntime-gpu")
	return ok && catalog.GPUSupported() && catalog.GPUNeedsPack() && !c.a.svc.Store.Installed(e)
}

func newSuppressionCard(a *App) *suppressionCard {
	c := &suppressionCard{a: a}
	set := a.svc.Settings()
	c.engine = giode.Input.Select(fieldStyle).OnChange(func(i int) {
		if i < len(c.ids) {
			id := c.ids[i]
			a.svc.Update(func(s *settings.Settings) { s.Engine = id })
			a.applyEngine()
		}
	})
	c.strength = giode.Input.Slider(giode.Styles{Color: "theme.primary", Background: track, BorderColor: "theme.primary"}).
		OnChange(func(v float32) { a.svc.Update(func(s *settings.Settings) { s.Strength = v }) })
	c.strength.SetValue(set.Strength)
	c.gpu = giode.Input.Switch("").OnChange(func(on bool) {
		a.svc.Update(func(s *settings.Settings) { s.UseGPU = on })
		a.applyEngine()
	})
	c.gpu.SetChecked(set.UseGPU)
	c.gpu.Disabled(!catalog.GPUSupported())
	c.more = giode.Button("Get more engines", secondaryButton()).Icon(icon("download", 15, "")).
		OnClick(func() { a.page = pageEngines })
	c.getPack = giode.Button("Get pack", secondaryButton()).OnClick(func() {
		a.page = pageEngines
		a.engines.install("onnxruntime-gpu")
	})
	c.refresh()
	return c
}

// refresh reloads the installed engines into the picker.
func (c *suppressionCard) refresh() {
	c.ids = c.ids[:0]
	var names []string
	selected := 0
	cur := c.a.svc.Settings().Engine
	for _, e := range c.a.svc.Store.Entries() {
		if e.Hidden || !c.a.svc.Store.Installed(e) || !e.Available() {
			continue
		}
		if e.ID == cur {
			selected = len(c.ids)
		}
		c.ids = append(c.ids, e.ID)
		label := e.Name
		if e.GPU {
			label += "  ·  GPU ready"
		}
		names = append(names, label)
	}
	c.engine.SetOptions(names...)
	c.engine.SetSelected(selected)
}

// View lays out the card.
func (c *suppressionCard) View() giode.Element {
	set := c.a.svc.Settings()
	e, _ := c.a.svc.Store.Get(set.Engine)
	gpuDesc := "Run neural engines on your graphics card to free the CPU."
	switch {
	case !catalog.GPUSupported():
		gpuDesc = "No GPU backend is available for this computer."
	case !e.GPU:
		gpuDesc = e.Name + " runs on the CPU; pick DTLN Neural to use the GPU."
	}
	kids := []giode.Element{
		labelled("Engine", c.engine),
		giode.Stack(giode.Styles{Gap: 10, Width: -1},
			giode.Row(giode.Styles{Wrap: giode.WrapNoWrap, Align: giode.AlignCenter},
				giode.Text("Strength", overline), spacer(),
				giode.Text(percent(set.Strength), giode.Styles{FontSize: 13, FontWeight: giode.FontWeightSemiBold, Color: "theme.primary"})),
			c.strength,
			giode.Text(e.Summary, muted)),
		giode.Divider(giode.Styles{Color: "theme.border"}),
		settingRow("GPU acceleration", gpuDesc, c.gpu),
	}
	if _, _, warn := c.a.svc.ActiveEngine(); warn != "" {
		row := []giode.Element{icon("circle-alert", 15, "theme.warning"),
			giode.Text(warn, giode.Styles{FontSize: 12.5, Color: "theme.text", FlexGrow: 1})}
		if c.packMissing() {
			row = append(row, c.getPack)
		}
		kids = append(kids, giode.Row(giode.Styles{Gap: 8, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Width: -1,
			Background: tintWarning, BorderRadius: 10, Padding: giode.SymmetricInset(12, 8)}, row...))
	}
	return section("sparkles", "Noise removal", "Choose how your voice is cleaned.", c.more, kids...)
}
