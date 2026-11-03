package engine

import (
	"math"
	"sync/atomic"

	"github.com/xmhrnx/noisefree/internal/ladspa"
)

// PluginOptions describe how to drive an installed LADSPA suppressor.
type PluginOptions struct {
	Path  string
	Label string
	// StrengthControl names the control port strength maps onto, from
	// StrengthMin (strength 0) to StrengthMax (strength 1). Without one,
	// strength blends the processed and original signals.
	StrengthControl          string
	StrengthMin, StrengthMax float32
	// Controls are fixed control values applied on load.
	Controls map[string]float32
	// Latency is the plugin's internal delay in samples, if known.
	Latency int
}

// Plugin runs a LADSPA suppressor such as DeepFilterNet or RNNoise-VAD.
type Plugin struct {
	p        *ladspa.Plugin
	opt      PluginOptions
	b        *blend
	strength atomic.Uint32
	applied  float32
}

const pluginFrame = 480

// NewPlugin loads a LADSPA plugin.
func NewPlugin(opt PluginOptions) (*Plugin, error) {
	p, err := ladspa.Open(opt.Path, opt.Label, SampleRate, pluginFrame)
	if err != nil {
		return nil, err
	}
	for name, v := range opt.Controls {
		p.SetControl(name, v)
	}
	pl := &Plugin{p: p, opt: opt, b: newBlend(pluginFrame, opt.Latency), applied: -1}
	pl.SetStrength(1)
	return pl, nil
}

func (p *Plugin) FrameSize() int  { return pluginFrame }
func (p *Plugin) Latency() int    { return p.opt.Latency }
func (p *Plugin) Backend() string { return "CPU" }
func (p *Plugin) Close()          { p.p.Close() }

func (p *Plugin) SetStrength(s float32) {
	p.strength.Store(math.Float32bits(s))
	p.b.set(s)
}

func (p *Plugin) Process(frame []float32) {
	if p.opt.StrengthControl != "" {
		if s := math.Float32frombits(p.strength.Load()); s != p.applied {
			p.p.SetControl(p.opt.StrengthControl, p.opt.StrengthMin+(p.opt.StrengthMax-p.opt.StrengthMin)*s)
			p.applied = s
		}
		p.p.Process(frame)
		return
	}
	p.b.keep(frame)
	p.p.Process(frame)
	p.b.mix(frame)
}
