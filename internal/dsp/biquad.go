package dsp

import "math"

// Biquad is a direct-form-II-transposed second-order IIR filter.
type Biquad struct {
	b0, b1, b2, a1, a2 float32
	z1, z2             float32
}

// HighPass returns a Butterworth high-pass at cutoff Hz, removing rumble,
// desk thumps and DC without touching the voice.
func HighPass(cutoff, sampleRate float64) *Biquad {
	w := 2 * math.Pi * cutoff / sampleRate
	cw, alpha := math.Cos(w), math.Sin(w)/math.Sqrt2 // Q = 1/sqrt(2)
	a0 := 1 + alpha
	return &Biquad{
		b0: float32((1 + cw) / 2 / a0), b1: float32(-(1 + cw) / a0), b2: float32((1 + cw) / 2 / a0),
		a1: float32(-2 * cw / a0), a2: float32((1 - alpha) / a0),
	}
}

// Process filters buf in place.
func (q *Biquad) Process(buf []float32) {
	for i, x := range buf {
		y := q.b0*x + q.z1
		q.z1 = q.b1*x - q.a1*y + q.z2
		q.z2 = q.b2*x - q.a2*y
		buf[i] = y
	}
}
