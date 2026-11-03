package ladspa

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func buildTestPlugin(t *testing.T) string {
	if runtime.GOOS == "windows" {
		t.Skip("test plugin build uses gcc -shared")
	}
	out := filepath.Join(t.TempDir(), "gain.so")
	cmd := exec.Command("cc", "-shared", "-fPIC", "-o", out, "testdata/gain.c")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("no C compiler: %v %s", err, b)
	}
	return out
}

func TestHostRunsPlugin(t *testing.T) {
	p, err := Open(buildTestPlugin(t), "", 48000, 64)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if p.Label != "test_gain" {
		t.Fatalf("label = %q", p.Label)
	}
	buf := make([]float32, 100) // larger than maxBlock: processed in chunks
	for i := range buf {
		buf[i] = 1
	}
	p.Process(buf)
	if buf[0] != 0.5 || buf[99] != 0.5 {
		t.Fatalf("default gain (middle of 0..1) not applied: %v %v", buf[0], buf[99])
	}
	if !p.SetControl("gain", 0.25) {
		t.Fatal("gain control not found")
	}
	buf[0] = 1
	p.Process(buf[:1])
	if buf[0] != 0.25 {
		t.Fatalf("gain = %v, want 0.25", buf[0])
	}
}

func TestMissingLabel(t *testing.T) {
	if _, err := Open(buildTestPlugin(t), "nope", 48000, 64); err == nil {
		t.Fatal("expected an error for a missing label")
	}
}
