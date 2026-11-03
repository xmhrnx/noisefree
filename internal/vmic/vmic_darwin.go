package vmic

// macOS needs an audio server plug-in for a virtual device; NoiseFree plays into
// the free BlackHole driver (or Loopback) when present.
func setup(outputs []string) Status {
	if n := find(outputs, "BlackHole", "Loopback Audio"); n != "" {
		return Status{Ready: true, OutputMatch: n, MicName: n}
	}
	return Status{
		Help:       "Install the free BlackHole 2ch driver, then restart NoiseFree. Pick \"BlackHole 2ch\" as the microphone in your apps.",
		InstallURL: "https://existential.audio/blackhole/",
	}
}

func teardown() {}
