package rnnoise

import (
	"math"
	"math/rand"
	"testing"
)

func TestReducesWhiteNoise(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := rand.New(rand.NewSource(1))
	var in, out float64
	frame := make([]float32, FrameSize)
	for n := 0; n < 300; n++ {
		for i := range frame {
			frame[i] = float32(r.NormFloat64() * 0.05)
		}
		if n > 100 {
			for _, v := range frame {
				in += float64(v * v)
			}
		}
		s.Process(frame)
		if n > 100 {
			for _, v := range frame {
				out += float64(v * v)
			}
		}
	}
	reduction := 10 * math.Log10(in/out)
	t.Logf("reduction %.1f dB", reduction)
	if reduction < 10 {
		t.Fatalf("noise reduced by %.1f dB, want at least 10", reduction)
	}
}
