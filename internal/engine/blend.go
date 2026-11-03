package engine

import (
	"math"
	"sync/atomic"

	"github.com/xmhrnx/noisefree/internal/dsp"
)

// blend gives engines with no strength control of their own a strength
// setting: the processed signal is mixed with the latency-aligned original,
// so strength 0.5 lets about 6 dB of the noise through.
type blend struct {
	strength atomic.Uint32
	dry      []float32
	delay    *dsp.Delay
}

func newBlend(frameSize, latency int) *blend {
	b := &blend{dry: make([]float32, frameSize), delay: dsp.NewDelay(latency)}
	b.set(1)
	return b
}

func (b *blend) set(s float32) { b.strength.Store(math.Float32bits(min(max(s, 0), 1))) }

// keep stores the dry frame before processing.
func (b *blend) keep(frame []float32) {
	copy(b.dry, frame)
	b.delay.Process(b.dry)
}

// mix blends the wet frame with the stored dry one.
func (b *blend) mix(frame []float32) {
	s := math.Float32frombits(b.strength.Load())
	if s >= 0.999 {
		return
	}
	for i, w := range frame {
		frame[i] = w*s + b.dry[i]*(1-s)
	}
}
