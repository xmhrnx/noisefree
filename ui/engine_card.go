package ui

import (
	"fmt"
	"strings"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/button"
	"github.com/xmhrnx/noisefree/internal/catalog"
)

// installState tracks one running install.
type installState struct {
	progress float64
	status   string
}

// engineCard shows one catalog entry with its install and use actions.
type engineCard struct {
	a                    *App
	id                   string
	install, use, remove *button.Button
}

func newEngineCard(a *App, id string) *engineCard {
	c := &engineCard{a: a, id: id}
	c.install = giode.Button("Install", primaryButton()).Icon(icon("download", 15, "")).
		OnClick(func() { a.engines.install(id) })
	c.use = giode.Button("Use", secondaryButton()).Icon(icon("zap", 15, "")).
		OnClick(func() { a.useEngine(id) })
	c.remove = giode.Button("Remove", giode.Styles{Background: "theme.danger", Color: "theme.danger", BorderRadius: 10,
		Padding: giode.SymmetricInset(12, 9), FontSize: 13}).Variant(giode.VariantGhost).
		OnClick(func() { a.engines.remove(id) })
	return c
}

// View lays out the card for entry e.
func (c *engineCard) View(e catalog.Entry) giode.Element {
	store := c.a.svc.Store
	installed := store.Installed(e)
	active := c.a.svc.Settings().Engine == e.ID
	border := "theme.border"
	if active {
		border = "theme.primary"
	}
	badges := []giode.Element{}
	switch {
	case e.Builtin:
		badges = append(badges, pill("Built in", "theme.accent", tintAccent))
	case e.User:
		badges = append(badges, pill("Custom", "theme.accent", tintAccent))
	}
	if e.Kind != catalog.KindLite {
		badges = append(badges, pill("Neural", "theme.primary", tintPrimary))
	}
	if e.GPU {
		badges = append(badges, iconPill("gpu", "GPU", "theme.success", tintSuccess))
	}
	return giode.Stack(giode.Styles{Width: 296, Gap: 14, Padding: giode.UniformInset(20), Background: "theme.surface",
		BorderRadius: 16, BorderWidth: 1, BorderColor: border, BoxShadow: giode.BoxShadow{Y: 6, Blur: 24, Color: "#00000022"}},
		giode.Row(giode.Styles{Gap: 12, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Width: -1},
			engineMonogram(e.Name),
			giode.Stack(giode.Styles{Gap: 2, FlexGrow: 1},
				giode.Text(e.Name, giode.Styles{FontSize: 15, FontWeight: giode.FontWeightSemiBold, Color: "theme.text"}),
				giode.Text(byline(e), caption)),
			activeBadge(active)),
		giode.Row(giode.Styles{Gap: 6}, badges...),
		giode.Text(e.Description, giode.Styles{FontSize: 12.5, Color: "theme.muted", LineHeight: 18, MaxLines: 5}),
		spacer(),
		giode.Row(giode.Styles{Gap: 16, Width: -1, Wrap: giode.WrapNoWrap},
			rating("Quality", e.Quality, "theme.primary"),
			rating("CPU use", e.CPUCost, "theme.warning"),
			giode.Stack(giode.Styles{Gap: 6},
				giode.Text("Latency", caption),
				giode.Text(fmt.Sprintf("%d ms", e.LatencyMs), giode.Styles{FontSize: 12.5, FontWeight: giode.FontWeightSemiBold, Color: "theme.text"}))),
		giode.Divider(giode.Styles{Color: "theme.border"}),
		c.actions(e, installed, active),
	)
}

func (c *engineCard) actions(e catalog.Entry, installed, active bool) giode.Element {
	if st, busy := c.a.engines.installing[e.ID]; busy {
		return giode.Stack(giode.Styles{Gap: 8, Width: -1},
			giode.Row(giode.Styles{Wrap: giode.WrapNoWrap},
				giode.Text(st.status, caption), spacer(), giode.Text(percent(float32(st.progress)), caption)),
			giode.Progress(float32(st.progress), giode.Styles{Color: "theme.primary", Background: track, Height: 6}))
	}
	row := []giode.Element{}
	switch {
	case !e.Available():
		row = append(row, giode.Text("Not available for "+catalog.Platform(), muted))
	case !installed:
		size := ""
		if mb := c.a.svc.Store.DownloadSizeMB(e); mb > 0 {
			size = fmt.Sprintf("%d MB download", mb)
		}
		row = append(row, giode.Text(size, caption), spacer(), c.install)
	default:
		if !e.Builtin {
			row = append(row, c.remove)
		}
		row = append(row, spacer())
		if active {
			row = append(row, iconPill("audio-lines", "Running now", "theme.primary", tintPrimary))
		} else {
			row = append(row, c.use)
		}
	}
	return giode.Row(giode.Styles{Gap: 8, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Width: -1, MinHeight: 36}, row...)
}

func byline(e catalog.Entry) string {
	parts := []string{e.Vendor}
	if e.License != "" {
		parts = append(parts, e.License)
	}
	return strings.Join(parts, " · ")
}

func activeBadge(active bool) giode.Element {
	if !active {
		return giode.Box(giode.Styles{})
	}
	return iconPill("check", "Active", "theme.onprimary", "theme.primary")
}

// engineMonogram is a gradient tile with the engine's initial.
func engineMonogram(name string) giode.Element {
	initial := "?"
	if name != "" {
		initial = strings.ToUpper(name[:1])
	}
	return giode.Box(giode.Styles{Width: 40, Height: 40, BorderRadius: 12, Align: giode.AlignCenter, Justify: giode.JustifyCenter,
		BackgroundGradient: giode.Linear(135, brandViolet, brandCyan)},
		giode.Text(initial, giode.Styles{FontSize: 17, FontWeight: giode.FontWeightBold, Color: "#FFFFFF"}))
}

// rating shows a 1-5 score as five dots.
func rating(label string, v int, color string) giode.Element {
	dots := make([]giode.Element, 5)
	for i := range dots {
		c := track
		if i < v {
			c = color
		}
		dots[i] = giode.Box(giode.Styles{Width: 14, Height: 6, BorderRadius: 3, Background: c})
	}
	return giode.Stack(giode.Styles{Gap: 6},
		giode.Text(label, caption),
		giode.Row(giode.Styles{Gap: 3, Wrap: giode.WrapNoWrap}, dots...))
}
