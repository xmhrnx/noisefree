package dsp

import (
	"math"
	"math/rand"
	"testing"
)

func TestFFTRoundTrip(t *testing.T) {
	f := NewFFT(64)
	x := make([]float32, 64)
	for i := range x {
		x[i] = float32(math.Sin(float64(i)*0.3) + 0.2*math.Cos(float64(i)*1.7))
	}
	re, im, y := make([]float32, 33), make([]float32, 33), make([]float32, 64)
	f.Forward(x, re, im)
	f.Inverse(re, im, y)
	for i := range x {
		if d := math.Abs(float64(x[i] - y[i])); d > 1e-4 {
			t.Fatalf("sample %d: got %f want %f", i, y[i], x[i])
		}
	}
}

func TestFFTBinOfSine(t *testing.T) {
	f := NewFFT(128)
	x := make([]float32, 128)
	for i := range x {
		x[i] = float32(math.Cos(2 * math.Pi * 8 * float64(i) / 128))
	}
	re, im := make([]float32, 65), make([]float32, 65)
	f.Forward(x, re, im)
	if math.Abs(float64(re[8])-64) > 1e-2 {
		t.Fatalf("bin 8 = %f, want 64", re[8])
	}
}

func TestFramerDelaysByOneFrame(t *testing.T) {
	f := NewFramer(5)
	var got []float32
	for b := 0; b < 6; b++ {
		buf := make([]float32, 3)
		for i := range buf {
			buf[i] = float32(b*3 + i + 1)
		}
		f.Run(buf, func([]float32) {})
		got = append(got, buf...)
	}
	for i, v := range got {
		want := float32(0)
		if i >= 5 {
			want = float32(i - 5 + 1)
		}
		if v != want {
			t.Fatalf("sample %d = %v, want %v (%v)", i, v, want, got)
		}
	}
}

func noisyTone(r *rand.Rand, n int, noise float64) ([]float32, []float32) {
	clean, noisy := make([]float32, n), make([]float32, n)
	for i := range clean {
		on := (i/24000)%2 == 1 // speech-like bursts
		if on {
			clean[i] = float32(0.3 * math.Sin(2*math.Pi*440*float64(i)/48000))
		}
		noisy[i] = clean[i] + float32(r.NormFloat64()*noise)
	}
	return clean, noisy
}

func TestSpectralReducesNoiseKeepsTone(t *testing.T) {
	s := NewSpectral()
	s.SetStrength(1)
	_, noisy := noisyTone(rand.New(rand.NewSource(2)), 48000*4, 0.03)
	out := append([]float32(nil), noisy...)
	for i := 0; i+s.Hop() <= len(out); i += s.Hop() {
		s.Process(out[i : i+s.Hop()])
	}
	// silent segment 48000..72000 (noise only), tone segment 72000..96000
	silentIn, silentOut := energy(noisy[48000:72000]), energy(out[48000+256:72000+256])
	toneIn, toneOut := energy(noisy[72000:96000]), energy(out[72000+256:96000+256])
	if r := 10 * math.Log10(silentIn/silentOut); r < 15 {
		t.Errorf("noise reduced by %.1f dB, want >= 15", r)
	}
	if l := 10 * math.Log10(toneIn/toneOut); l > 2 {
		t.Errorf("tone attenuated by %.1f dB, want <= 2", l)
	}
}

func TestResamplerRoundTrip(t *testing.T) {
	r := NewResampler3()
	in := make([]float32, 4800)
	for i := range in {
		in[i] = float32(math.Sin(2 * math.Pi * 1000 * float64(i) / 48000))
	}
	mid, out := make([]float32, 1600), make([]float32, 4800)
	r.Down(in, mid)
	r.Up(mid, out)
	if e := energy(out[2400:]) / energy(in[2400:]); e < 0.8 || e > 1.2 {
		t.Fatalf("energy ratio after round trip = %.2f", e)
	}
}

func TestGateClosesOnSilence(t *testing.T) {
	g := NewGate(48000, -40)
	buf := make([]float32, 48000)
	for i := range buf {
		buf[i] = 0.001
	}
	g.Process(buf)
	if g.Open() || buf[len(buf)-1] > 0.0001 {
		t.Fatal("gate stayed open on -60 dBFS input")
	}
}

func TestHighPassRemovesDC(t *testing.T) {
	q := HighPass(80, 48000)
	buf := make([]float32, 48000)
	for i := range buf {
		buf[i] = 0.5
	}
	q.Process(buf)
	if math.Abs(float64(buf[len(buf)-1])) > 1e-3 {
		t.Fatalf("DC leaked: %f", buf[len(buf)-1])
	}
}

func energy(x []float32) float64 {
	var s float64
	for _, v := range x {
		s += float64(v * v)
	}
	return s
}

func BenchmarkSpectral(b *testing.B) {
	s := NewSpectral()
	buf := make([]float32, s.Hop())
	for i := 0; i < b.N; i++ {
		s.Process(buf)
	}
}
