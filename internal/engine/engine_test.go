package engine

import (
	"math"
	"testing"
)

func TestBlendZeroStrengthReturnsDelayedDry(t *testing.T) {
	b := newBlend(4, 2)
	b.set(0)
	in := [][]float32{{1, 2, 3, 4}, {5, 6, 7, 8}}
	want := [][]float32{{0, 0, 1, 2}, {3, 4, 5, 6}}
	for i, frame := range in {
		f := append([]float32(nil), frame...)
		b.keep(f)
		clear(f) // the "processor" removes everything
		b.mix(f)
		for k := range f {
			if f[k] != want[i][k] {
				t.Fatalf("frame %d = %v, want %v", i, f, want[i])
			}
		}
	}
}

func TestStrengthScalesSuppression(t *testing.T) {
	energyAfter := func(strength float32) float64 {
		p, err := NewRNNoise()
		if err != nil {
			t.Fatal(err)
		}
		defer p.Close()
		p.SetStrength(strength)
		var e float64
		seed := uint32(7)
		frame := make([]float32, p.FrameSize())
		for n := 0; n < 200; n++ {
			for i := range frame {
				seed = seed*1664525 + 1013904223
				frame[i] = (float32(seed>>8)/(1<<24) - 0.5) * 0.1
			}
			p.Process(frame)
			if n > 100 {
				for _, v := range frame {
					e += float64(v * v)
				}
			}
		}
		return e
	}
	full, half := energyAfter(1), energyAfter(0.5)
	if !(full < half) || math.IsNaN(full) {
		t.Fatalf("strength 1 left %.4f, strength 0.5 left %.4f: want less noise at full strength", full, half)
	}
}

func TestLiteFrameContract(t *testing.T) {
	l := NewLite()
	if l.FrameSize() <= 0 || l.Latency() < 0 || l.Backend() != "CPU" {
		t.Fatalf("bad contract: %d %d %q", l.FrameSize(), l.Latency(), l.Backend())
	}
	l.SetStrength(0.3)
	l.Process(make([]float32, l.FrameSize()))
}
