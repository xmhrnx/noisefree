package audio

import (
	"bytes"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/xmhrnx/noisefree/internal/engine"
)

// NOISEFREE_TEST_PULSE=1: records a preview of noise through RNNoise on a
// live PulseAudio server and plays it back.
func TestPreviewRecordsAndPlays(t *testing.T) {
	if os.Getenv("NOISEFREE_TEST_PULSE") == "" {
		t.Skip("set NOISEFREE_TEST_PULSE=1 with a PulseAudio server running")
	}
	sink, err := exec.Command("pactl", "load-module", "module-null-sink", "sink_name=preview_src", "rate=48000", "channels=1").Output()
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Command("pactl", "unload-module", string(bytes.TrimSpace(sink))).Run()
	play := exec.Command("pacat", "--playback", "-d", "preview_src", "--format=float32le", "--rate=48000", "--channels=1")
	play.Stdin = bytes.NewReader(noise(48000 * 10))
	if err := play.Start(); err != nil {
		t.Fatal(err)
	}
	defer play.Process.Kill()

	p, err := New()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	proc, _ := engine.NewRNNoise()
	p.SetProcessor(proc)
	if err := p.Start(Config{Input: "Monitor of Null Output"}); err != nil {
		inputs, _, _ := p.Devices()
		t.Fatalf("%v (inputs %v)", err, inputs)
	}
	time.Sleep(3 * time.Second) // let the devices start streaming
	if err := p.StartRecording(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2500 * time.Millisecond)
	p.StopRecording()
	rec, _ := p.PreviewProgress()
	if p.PreviewState() != PreviewReady || rec < 2 {
		t.Fatalf("state %d after recording %.2fs", p.PreviewState(), rec)
	}
	raw, clean := make([]float32, EnvelopeBuckets), make([]float32, EnvelopeBuckets)
	p.PreviewEnvelope(raw, clean)
	var r, c float32
	for i := 50; i < 100; i++ { // 1-2 s, after the suppressor settled
		r += raw[i]
		c += clean[i]
	}
	t.Logf("recorded %.2fs; envelope raw %.3f clean %.3f", rec, r/50, c/50)
	if r == 0 || c >= r {
		t.Fatalf("envelope raw %.3f, clean %.3f: want a recording with less noise after processing", r, c)
	}
	if err := p.PlayPreview("", false); err != nil {
		t.Fatal(err)
	}
	if p.PreviewState() != PreviewPlaying {
		t.Fatal("not playing")
	}
	deadline := time.Now().Add(time.Duration(rec*float64(time.Second)) + 2*time.Second)
	for p.PreviewState() == PreviewPlaying && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	if _, played := p.PreviewProgress(); p.PreviewState() != PreviewReady || played < rec-0.01 {
		t.Fatalf("playback state %d, played %.2fs of %.2fs", p.PreviewState(), played, rec)
	}
}
