package dsp

import "math"

// Resampler3 converts between 48 kHz and 16 kHz (an exact factor of 3) with
// a windowed-sinc low-pass, for models trained on 16 kHz speech.
type Resampler3 struct {
	taps    []float32
	downBuf []float32
	upBuf   []float32
}

// NewResampler3 builds the 48-tap anti-aliasing filter.
func NewResampler3() *Resampler3 {
	const n = 48
	r := &Resampler3{taps: make([]float32, n), downBuf: make([]float32, n), upBuf: make([]float32, n)}
	cutoff := 7600.0 / 48000
	var sum float64
	t := make([]float64, n)
	for i := range t {
		m := float64(i) - float64(n-1)/2
		sinc := 2 * cutoff
		if m != 0 {
			sinc = math.Sin(2*math.Pi*cutoff*m) / (math.Pi * m)
		}
		w := 0.42 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(n-1)) + 0.08*math.Cos(4*math.Pi*float64(i)/float64(n-1))
		t[i] = sinc * w
		sum += t[i]
	}
	for i := range t {
		r.taps[i] = float32(t[i] / sum)
	}
	return r
}

// Latency is the filter delay at 48 kHz, in samples, for one pass.
func (r *Resampler3) Latency() int { return len(r.taps) / 2 }

// Down writes len(in)/3 samples at 16 kHz to out.
func (r *Resampler3) Down(in, out []float32) {
	for i := range out[:len(in)/3] {
		for j := 0; j < 3; j++ {
			push(r.downBuf, in[i*3+j])
		}
		out[i] = dot(r.downBuf, r.taps)
	}
}

// Up writes 3*len(in) samples at 48 kHz to out.
func (r *Resampler3) Up(in, out []float32) {
	for i, v := range in {
		for j := 0; j < 3; j++ {
			if j == 0 {
				push(r.upBuf, v*3)
			} else {
				push(r.upBuf, 0)
			}
			out[i*3+j] = dot(r.upBuf, r.taps)
		}
	}
}

func push(buf []float32, v float32) {
	copy(buf, buf[1:])
	buf[len(buf)-1] = v
}

func dot(a, b []float32) float32 {
	var s float32
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}
