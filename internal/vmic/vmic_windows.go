package vmic

// Windows can't create audio endpoints without a signed kernel driver, so
// NoiseFree plays into VB-Audio's free Virtual Cable (or VoiceMeeter) when present.
func setup(outputs []string) Status {
	if n := find(outputs, "CABLE Input", "VoiceMeeter Input", "VoiceMeeter VAIO"); n != "" {
		mic := "CABLE Output (VB-Audio Virtual Cable)"
		if n != "CABLE Input" {
			mic = "VoiceMeeter Output"
		}
		return Status{Ready: true, OutputMatch: n, MicName: mic}
	}
	return Status{
		Help:       "Install the free VB-Audio Virtual Cable. NoiseFree finds it automatically, keeps your speakers as the default output, and sends your clean voice to the \"CABLE Output\" microphone your apps record from.",
		InstallURL: "https://vb-audio.com/Cable/",
	}
}

func teardown() {}
