package audio

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/xmhrnx/noisefree/internal/engine"
	"github.com/xmhrnx/noisefree/internal/vmic"
)

// NOISEFREE_TEST_PULSE=1 runs the whole chain on a live PulseAudio server: white
// noise is played into a test sink, NoiseFree captures its monitor, suppresses it
// into the virtual microphone, and parec records the virtual microphone.
func TestEndToEndOnPulseAudio(t *testing.T) {
	if os.Getenv("NOISEFREE_TEST_PULSE") == "" {
		t.Skip("set NOISEFREE_TEST_PULSE=1 with a PulseAudio server running")
	}
	sink, err := exec.Command("pactl", "load-module", "module-null-sink", "sink_name=noise_src", "rate=48000", "channels=1",
		`sink_properties='device.description="Noise Source"'`).Output()
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Command("pactl", "unload-module", string(bytes.TrimSpace(sink))).Run()

	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	_, outputs, _ := p.Devices()
	var names []string
	for _, o := range outputs {
		names = append(names, o.Name)
	}
	st := vmic.Setup(names)
	defer vmic.Teardown()
	if !st.Ready {
		t.Fatalf("virtual mic not ready: %s", st.Help)
	}
	inputs, outputs, _ := p.Devices()
	var in string
	for _, d := range inputs {
		t.Logf("input: %q", d.Name)
		if d.Name == "Monitor of Noise Source" {
			in = d.Name
		}
	}
	out := FindOutput(outputs, st.OutputMatch)
	if in == "" || out == "" {
		t.Fatalf("devices not found: in=%q out=%q", in, out)
	}

	measure := func(enabled bool) float64 {
		p.SetEnabled(enabled)
		play := exec.Command("pacat", "--playback", "-d", "noise_src", "--format=float32le", "--rate=48000", "--channels=1")
		play.Stdin = bytes.NewReader(noise(48000 * 4))
		if err := play.Start(); err != nil {
			t.Fatal(err)
		}
		defer play.Wait()
		time.Sleep(1500 * time.Millisecond) // let the suppressor settle
		rec := exec.Command("parec", "-d", "noisefree_mic", "--format=float32le", "--rate=48000", "--channels=1")
		var buf bytes.Buffer
		rec.Stdout = &buf
		rec.Start()
		time.Sleep(1500 * time.Millisecond)
		rec.Process.Kill()
		rec.Wait()
		return rms(buf.Bytes())
	}

	proc, err := engine.NewRNNoise()
	if err != nil {
		t.Fatal(err)
	}
	p.SetProcessor(proc)
	if err := p.Start(Config{Input: in, Output: out, BufferMs: 10}); err != nil {
		t.Fatal(err)
	}
	raw := measure(false)
	clean := measure(true)
	stats := p.Stats()
	t.Logf("raw %.4f, clean %.4f (%.1f dB), audio thread load %.2f%%, latency %.1f ms",
		raw, clean, 20*math.Log10(raw/clean), stats.Load*100, stats.LatencyMs)
	if raw < 0.01 {
		t.Fatal("no audio reached the virtual microphone")
	}
	if 20*math.Log10(raw/clean) < 6 { // RNNoise removes ~12 dB of synthetic white noise
		t.Fatal("noise reduced by less than 6 dB")
	}
}

func noise(n int) []byte {
	r := rand.New(rand.NewSource(4))
	b := make([]byte, n*4)
	for i := 0; i < n; i++ {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(float32(r.NormFloat64()*0.05)))
	}
	return b
}

func rms(b []byte) float64 {
	var s float64
	n := len(b) / 4
	for i := 0; i < n; i++ {
		v := float64(math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:])))
		s += v * v
	}
	if n == 0 {
		return 0
	}
	return math.Sqrt(s / float64(n))
}
