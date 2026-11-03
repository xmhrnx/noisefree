package ui

import (
	"fmt"

	"github.com/xmhrnx/giode"
)

// heroCard holds the power toggle and the live statistics.
type heroCard struct {
	a     *App
	power *powerToggle
}

func newHeroCard(a *App) *heroCard {
	h := &heroCard{a: a}
	h.power = newPowerToggle(
		func() bool { return a.svc.Settings().Enabled && a.live() },
		func() float32 { return a.svc.Pipe.Out.RMS() },
		a.toggleEnabled)
	return h
}

// View lays out the card.
func (h *heroCard) View(st giode.Styles) giode.Element {
	set := h.a.svc.Settings()
	stats := h.a.svc.Pipe.Stats()
	title, sub := "Noise removal is off", "Your voice passes through untouched."
	if set.Enabled {
		title, sub = "Noise removal is on", "Background noise is being removed in real time."
	}
	if !h.a.live() {
		title, sub = "Audio is stopped", "Pick a microphone below to start."
		if err := h.a.svc.AudioError(); err != nil {
			sub = err.Error()
		}
	}
	backend, latency, load := "—", "—", "—"
	if h.a.live() {
		backend, latency, load = stats.Backend, fmt.Sprintf("%.0f ms", stats.LatencyMs), fmt.Sprintf("%.1f%%", stats.Load*100)
	}
	backendIcon, backendTint, backendColor := "cpu", tintAccent, "theme.accent"
	if len(backend) > 3 && backend[:3] == "GPU" {
		backendIcon, backendTint, backendColor = "gpu", tintPrimary, "theme.primary"
	}
	return giode.Box(st,
		h.power.View(),
		giode.Stack(giode.Styles{Gap: 4, Align: giode.AlignCenter},
			giode.Text(title, giode.Styles{FontSize: 18, FontWeight: giode.FontWeightBold, Color: "theme.text"}),
			giode.Text(sub, giode.Styles{FontSize: 13, Color: "theme.muted", TextAlign: giode.TextAlignCenter, MaxWidth: 300})),
		giode.Row(giode.Styles{Gap: 8, Width: -1, Wrap: giode.WrapNoWrap},
			statTile(backendIcon, "Runs on", backend, backendColor, backendTint),
			statTile("timer", "Latency", latency, "theme.success", tintSuccess),
			statTile("gauge", "Load", load, "theme.warning", tintWarning)),
	)
}
