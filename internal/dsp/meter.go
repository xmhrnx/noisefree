package dsp

import (
	"math"
	"sync/atomic"
)

// Meter publishes a block's peak and RMS level from the audio thread for the
// UI to read without locks.
type Meter struct {
	peak, rms atomic.Uint32
}

// Measure records the levels of buf, with a falling peak hold.
func (m *Meter) Measure(buf []float32) {
	var peak, sum float32
	for _, v := range buf {
		sum += v * v
		if v < 0 {
			v = -v
		}
		peak = max(peak, v)
	}
	prev := math.Float32frombits(m.peak.Load()) * 0.85
	m.peak.Store(math.Float32bits(max(peak, prev)))
	if len(buf) > 0 {
		m.rms.Store(math.Float32bits(float32(math.Sqrt(float64(sum / float32(len(buf)))))))
	}
}

// Reset zeroes the levels.
func (m *Meter) Reset() { m.peak.Store(0); m.rms.Store(0) }

// Peak returns the held peak level in [0, 1+].
func (m *Meter) Peak() float32 { return math.Float32frombits(m.peak.Load()) }

// RMS returns the last block's RMS level.
func (m *Meter) RMS() float32 { return math.Float32frombits(m.rms.Load()) }

// ToMeter maps a linear level onto 0..1 over a -60..0 dBFS scale.
func ToMeter(level float32) float32 {
	if level <= 1e-6 {
		return 0
	}
	db := 20 * math.Log10(float64(level))
	return clamp(float32((db+60)/60), 0, 1)
}
