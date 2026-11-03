package vmic

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// NOISEFREE_TEST_PULSE=1: makes NoiseFree's virtual sink the default on a
// live PulseAudio server and checks GuardDefaults switches back.
func TestGuardDefaultsOnPulseAudio(t *testing.T) {
	if os.Getenv("NOISEFREE_TEST_PULSE") == "" {
		t.Skip("set NOISEFREE_TEST_PULSE=1 with a PulseAudio server running")
	}
	st := Setup(nil)
	defer Teardown()
	if !st.Ready {
		t.Fatal(st.Help)
	}
	before, _ := pactl("get-default-sink")
	if strings.HasPrefix(before, sinkName) {
		t.Skip("no real sink to restore to")
	}
	if err := exec.Command("pactl", "set-default-sink", sinkName).Run(); err != nil {
		t.Fatal(err)
	}
	r, err := GuardDefaults("", "")
	if err != nil {
		t.Fatal(err)
	}
	after, _ := pactl("get-default-sink")
	t.Logf("repair %+v, default sink %q -> %q", r, sinkName, after)
	if after != before || r.FixedOutput == "" {
		t.Fatalf("default sink is %q, want %q back", after, before)
	}
}
