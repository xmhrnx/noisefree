package dtln

import (
	"math"
	"math/rand"
	"os"
	"testing"
	"time"
)

// Set NOISEFREE_TEST_ORT to libonnxruntime and NOISEFREE_TEST_DTLN to the folder with
// model_1.onnx and model_2.onnx to run this test.
func TestDTLNSuppressesNoise(t *testing.T) {
	lib, dir := os.Getenv("NOISEFREE_TEST_ORT"), os.Getenv("NOISEFREE_TEST_DTLN")
	if lib == "" || dir == "" {
		t.Skip("NOISEFREE_TEST_ORT / NOISEFREE_TEST_DTLN not set")
	}
	m, err := Open(dir, lib, CPU)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	r := rand.New(rand.NewSource(3))
	frame := make([]float32, FrameSize)
	var in, out float64
	start := time.Now()
	const frames = 500
	for n := 0; n < frames; n++ {
		for i := range frame {
			frame[i] = float32(r.NormFloat64() * 0.03)
		}
		if n > 100 {
			in += energy(frame)
		}
		m.Process(frame)
		if n > 100 {
			out += energy(frame)
		}
	}
	perFrame := time.Since(start) / frames
	t.Logf("%.1f dB reduction, %v per 8 ms frame (%.1f%% of one core)",
		10*math.Log10(in/out), perFrame, 100*perFrame.Seconds()/0.008)
	if 10*math.Log10(in/out) < 10 {
		t.Fatal("DTLN reduced white noise by less than 10 dB")
	}
}

func energy(x []float32) float64 {
	var s float64
	for _, v := range x {
		s += float64(v * v)
	}
	return s
}
