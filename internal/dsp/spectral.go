package dsp

import "math"

// Spectral is a lightweight, pure-Go speech enhancer: an STFT Wiener filter
// with a minimum-tracking noise estimate. It costs a fraction of a percent of
// one CPU core at 48 kHz and needs no model files.
type Spectral struct {
	fft              *FFT
	n, hop           int
	window           []float32
	in, out          []float32 // analysis history and overlap-add accumulator
	frame            []float32
	re, im           []float32
	power, smooth    []float32
	noise, prevClean []float32
	gain, tmp        []float32
	floor            float32
	overSubtract     float32
	frames           int
}

// NewSpectral returns an enhancer that consumes and produces hop-sized
// blocks (256 samples at 48 kHz, 5.3 ms).
func NewSpectral() *Spectral {
	const n = 512
	bins := n/2 + 1
	s := &Spectral{fft: NewFFT(n), n: n, hop: n / 2, window: make([]float32, n),
		in: make([]float32, n), out: make([]float32, n), frame: make([]float32, n),
		re: make([]float32, bins), im: make([]float32, bins),
		power: make([]float32, bins), smooth: make([]float32, bins), noise: make([]float32, bins),
		prevClean: make([]float32, bins), gain: make([]float32, bins), tmp: make([]float32, bins)}
	for i := range s.window { // sqrt-Hann: analysis x synthesis sums to 1 at 50% overlap
		s.window[i] = float32(math.Sqrt(0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(n))))
	}
	s.SetStrength(0.8)
	return s
}

// Hop returns the block size Process expects.
func (s *Spectral) Hop() int { return s.hop }

// SetStrength maps 0..1 to the maximum attenuation (0 to 36 dB) and how
// aggressively the noise estimate is subtracted.
func (s *Spectral) SetStrength(v float32) {
	v = clamp(v, 0, 1)
	s.floor = float32(math.Pow(10, -float64(v)*36/20))
	s.overSubtract = 1 + v
}

// Process enhances one hop-sized block in place.
func (s *Spectral) Process(block []float32) {
	n, h := s.n, s.hop
	copy(s.in, s.in[h:])
	copy(s.in[n-h:], block[:h])
	for i := range s.frame {
		s.frame[i] = s.in[i] * s.window[i]
	}
	s.fft.Forward(s.frame, s.re, s.im)
	s.estimate()
	for k := range s.re {
		s.re[k] *= s.gain[k]
		s.im[k] *= s.gain[k]
	}
	s.fft.Inverse(s.re, s.im, s.frame)
	for i := range s.out {
		s.out[i] += s.frame[i] * s.window[i]
	}
	copy(block[:h], s.out[:h])
	copy(s.out, s.out[h:])
	clear(s.out[n-h:])
}

func (s *Spectral) estimate() {
	const (
		smoothing = 0.7
		fall      = 0.85
		rise      = 1.004 // ~4 dB/s at 187 frames/s: follows changing noise, not speech
		ddAlpha   = 0.96
		warmup    = 40
	)
	s.frames++
	for k := range s.re {
		p := s.re[k]*s.re[k] + s.im[k]*s.im[k] + 1e-12
		s.power[k] = p
		s.smooth[k] = smoothing*s.smooth[k] + (1-smoothing)*p
		switch {
		case s.frames <= warmup:
			s.noise[k] += (s.smooth[k] - s.noise[k]) / float32(s.frames)
		case s.smooth[k] < s.noise[k]:
			s.noise[k] = fall*s.noise[k] + (1-fall)*s.smooth[k]
		default:
			s.noise[k] *= rise
		}
		nz := s.noise[k]*s.overSubtract + 1e-12
		post := p/nz - 1
		if post < 0 {
			post = 0
		}
		prior := ddAlpha*s.prevClean[k]/nz + (1-ddAlpha)*post
		g := prior / (1 + prior)
		if g < s.floor {
			g = s.floor
		}
		s.tmp[k] = g
		s.prevClean[k] = g * g * p
	}
	last := len(s.tmp) - 1
	for k := range s.tmp { // light smoothing across frequency tames musical noise
		l, r := s.tmp[max(k-1, 0)], s.tmp[min(k+1, last)]
		s.gain[k] = 0.25*l + 0.5*s.tmp[k] + 0.25*r
	}
}

func clamp(v, lo, hi float32) float32 {
	return min(max(v, lo), hi)
}
