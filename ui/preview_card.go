package ui

import (
	"fmt"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/components/button"
	"github.com/xmhrnx/giode/components/segmented"
	"github.com/xmhrnx/noisefree/internal/audio"
	"github.com/xmhrnx/noisefree/internal/dsp"
)

const previewBars = 100

// previewCard records a few seconds through the active engine and plays
// them back, switching between the cleaned and the original take.
type previewCard struct {
	a                         *App
	record, stop, play, again *button.Button
	source                    *segmented.Control
	raw, clean                []float32
}

func newPreviewCard(a *App) *previewCard {
	c := &previewCard{a: a, raw: make([]float32, audio.EnvelopeBuckets), clean: make([]float32, audio.EnvelopeBuckets)}
	pipe := a.svc.Pipe
	c.record = giode.Button("Record a sample", primaryButton()).Icon(icon("circle", 14, "")).OnClick(func() {
		if err := pipe.StartRecording(); err != nil {
			a.toast(giode.ToastError, "Couldn't record: "+err.Error())
		}
	})
	c.again = giode.Button("Record again", secondaryButton()).Icon(icon("rotate-ccw", 14, "")).OnClick(c.record.Click)
	c.stop = giode.Button("Stop", giode.Styles{Background: "theme.danger", Color: "#FFFFFF", BorderRadius: 10,
		Padding: giode.SymmetricInset(16, 9), FontSize: 13, FontWeight: giode.FontWeightSemiBold}).
		Icon(icon("square", 13, "")).OnClick(pipe.StopRecording)
	c.play = giode.Button("Play", primaryButton()).Icon(icon("play", 14, "")).OnClick(func() {
		if pipe.PreviewState() == audio.PreviewPlaying {
			pipe.StopPlayback()
			return
		}
		original := c.source.Selected() == 1
		go func() {
			if err := a.svc.PlayPreview(original); err != nil {
				a.win.Post(func() { a.toast(giode.ToastError, "Couldn't play: "+err.Error()) })
			}
		}()
	})
	c.source = giode.Input.Segmented("With NoiseFree", "Original").
		OnChange(func(i int) { pipe.SetPreviewOriginal(i == 1) })
	return c
}

// View lays out the card with st (the card's own style).
func (c *previewCard) View(st giode.Styles) giode.Element {
	pipe := c.a.svc.Pipe
	state := pipe.PreviewState()
	recorded, played := pipe.PreviewProgress()
	pipe.PreviewEnvelope(c.raw, c.clean)

	var controls, trailing giode.Element
	scale := float64(audio.PreviewSeconds)
	switch state {
	case audio.PreviewEmpty:
		controls = giode.Row(giode.Styles{Gap: 12, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Width: -1},
			c.record, giode.Text(fmt.Sprintf("Up to %d seconds. Talk as usual, then listen back.", audio.PreviewSeconds), muted))
	case audio.PreviewRecording:
		controls = giode.Row(giode.Styles{Gap: 12, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Width: -1},
			c.stop,
			iconPill("circle", "Recording", "theme.danger", tintDanger),
			spacer(),
			giode.Text(fmt.Sprintf("%s / %s", clock(recorded), clock(scale)), giode.Styles{FontSize: 13, FontWeight: giode.FontWeightSemiBold, Color: "theme.text"}))
	default:
		scale = max(recorded, 0.5)
		label, iconName := "Play", "play"
		if state == audio.PreviewPlaying {
			label, iconName = "Stop", "square"
		}
		c.play.Label(label).Icon(icon(iconName, 14, ""))
		controls = giode.Row(giode.Styles{Gap: 10, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap, Width: -1},
			c.play, c.source, spacer(), giode.Text(clock(recorded), caption))
		trailing = c.again
	}
	playhead := -1.0
	if state == audio.PreviewPlaying {
		playhead = played / scale
	}
	wave := giode.Box(giode.Styles{Width: -1, Background: raised, BorderRadius: 12, Padding: giode.UniformInset(12),
		BorderWidth: 1, BorderColor: "theme.border"},
		c.waveform(scale, playhead, state == audio.PreviewEmpty))
	return sectionStyled(st, "audio-lines", "Preview", "Record yourself and hear exactly what others will hear.", trailing, wave, controls)
}

// waveform draws the take's envelope over scale seconds, with a playhead
// at fraction playhead (negative: none).
func (c *previewCard) waveform(scale, playhead float64, empty bool) giode.Element {
	return giode.Canvas(giode.Styles{Width: -1, Height: 72}, func(p *giode.Painter) {
		w, h := p.Width(), p.Height()
		mid := h / 2
		p.Line(0, mid, w, mid, 1, track)
		if empty {
			return
		}
		step := w / previewBars
		bar := max(step-2, 1)
		per := int(scale * float64(audio.EnvelopeBuckets) / audio.PreviewSeconds / previewBars)
		per = max(per, 1)
		for i := 0; i < previewBars; i++ {
			var r, cl float32
			for k := i * per; k < (i+1)*per && k < len(c.raw); k++ {
				r, cl = max(r, c.raw[k]), max(cl, c.clean[k])
			}
			x := float32(i) * step
			if rh := dsp.ToMeter(r) * (h - 4) / 2; rh > 0.5 {
				p.Rect(x, mid-rh, bar, 2*rh, bar/2, rawBar)
			}
			if ch := dsp.ToMeter(cl) * (h - 4) / 2; ch > 0.5 {
				p.Rect(x, mid-ch, bar, 2*ch, bar/2, previewColors[i])
			}
		}
		if playhead >= 0 {
			x := float32(min(playhead, 1)) * w
			p.Line(x, 0, x, h, 2, "theme.text")
		}
	})
}

var previewColors = func() []string {
	c := make([]string, previewBars)
	for i := range c {
		c[i] = giode.Mix(brandGold, brandHot, float64(i)/previewBars)
	}
	return c
}()

func clock(sec float64) string {
	s := int(sec + 0.5)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}
