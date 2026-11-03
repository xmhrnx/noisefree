package vmic

import "testing"

func TestIsVirtual(t *testing.T) {
	for name, want := range map[string]bool{
		"CABLE Input (VB-Audio Virtual Cable)":          true,
		"CABLE Output (VB-Audio Virtual Cable)":         true,
		"VoiceMeeter Input (VB-Audio VoiceMeeter VAIO)": true,
		"BlackHole 2ch":                         true,
		"NoiseFree Microphone":                  true,
		"Speakers (Realtek(R) Audio)":           false,
		"Microphone Array (Intel® Smart Sound)": false,
		"MacBook Pro Speakers":                  false,
	} {
		if IsVirtual(name) != want {
			t.Errorf("IsVirtual(%q) = %v, want %v", name, !want, want)
		}
	}
}

func TestRealTargetPrefersRememberedDevice(t *testing.T) {
	eps := []Endpoint{
		{ID: "1", Name: "CABLE Input (VB-Audio Virtual Cable)", Default: true},
		{ID: "2", Name: "HDMI Output"},
		{ID: "3", Name: "Speakers (Realtek(R) Audio)"},
	}
	if e, _ := realTarget(eps, "Speakers (Realtek(R) Audio)"); e.ID != "3" {
		t.Fatalf("picked %q, want the remembered speakers", e.Name)
	}
	if e, _ := realTarget(eps, "Gone Headset"); e.ID != "2" {
		t.Fatalf("picked %q, want the first real device", e.Name)
	}
	if _, ok := realTarget(eps[:1], ""); ok {
		t.Fatal("picked a virtual device")
	}
}
