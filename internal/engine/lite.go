package engine

import (
	"math"
	"sync/atomic"

	"github.com/xmhrnx/noisefree/internal/dsp"
)

// Lite is the pure-Go spectral suppressor: no model, no dependencies, and
// the lowest CPU use, for old or low-power machines.
type Lite struct {
	s        *dsp.Spectral
	strength atomic.Uint32
	applied  float32
}

// NewLite creates the Lite processor.
func NewLite() *Lite {
	l := &Lite{s: dsp.NewSpectral(), applied: -1}
	l.SetStrength(1)
	return l
}

func (l *Lite) FrameSize() int  { return l.s.Hop() }
func (l *Lite) Latency() int    { return l.s.Hop() }
func (l *Lite) Backend() string { return "CPU" }
func (l *Lite) Close()          {}

func (l *Lite) SetStrength(s float32) { l.strength.Store(math.Float32bits(s)) }

func (l *Lite) Process(frame []float32) {
	if s := math.Float32frombits(l.strength.Load()); s != l.applied {
		l.s.SetStrength(s)
		l.applied = s
	}
	l.s.Process(frame)
}
