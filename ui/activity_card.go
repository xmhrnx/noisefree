package ui

import "github.com/xmhrnx/giode"

// activityCard shows the live waveform and the input and output meters.
type activityCard struct {
	a     *App
	graph historyGraph
}

// View lays out the card.
func (c *activityCard) View() giode.Element {
	live := c.a.live()
	p := c.a.svc.Pipe
	legend := giode.Row(giode.Styles{Gap: 12, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
		legendDot(track, "Raw"), legendDot("theme.primary", "Clean"))
	return giode.Box(cardStyle.With(giode.Styles{Width: -1}),
		giode.Row(giode.Styles{Gap: 12, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
			giode.Box(giode.Styles{Background: tintPrimary, BorderRadius: 10, Padding: giode.UniformInset(8)},
				icon("audio-waveform", 18, "theme.primary")),
			giode.Stack(giode.Styles{Gap: 2, FlexGrow: 1},
				giode.Text("Live activity", h2),
				giode.Text("Your raw microphone against what others hear.", muted)),
			legend),
		giode.Box(giode.Styles{Width: -1, Background: raised, BorderRadius: 12, Padding: giode.UniformInset(12),
			BorderWidth: 1, BorderColor: "theme.border"}, c.graph.View(112)),
		levelMeter("Microphone", p.In.RMS(), p.In.Peak(), live),
		levelMeter("Output", p.Out.RMS(), p.Out.Peak(), live),
	)
}

func legendDot(color, label string) giode.Element {
	return giode.Row(giode.Styles{Gap: 6, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
		giode.Box(giode.Styles{Width: 8, Height: 8, BorderRadius: 4, Background: color}),
		giode.Text(label, caption))
}
