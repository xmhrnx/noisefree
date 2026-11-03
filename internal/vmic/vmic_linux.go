package vmic

import (
	"os/exec"
	"strings"
	"sync"
)

// On Linux, NoiseFree creates the virtual microphone itself through the
// PulseAudio API (also served by PipeWire): a null sink to play into, and a
// remapped source over its monitor that apps see as a microphone.
const (
	sinkName   = "noisefree_sink"
	sourceName = "noisefree_mic"
	outputDesc = "NoiseFree Virtual Output"
	micDesc    = "NoiseFree Microphone"
)

var (
	mu      sync.Mutex
	modules []string
)

func setup(outputs []string) Status {
	st := Status{OutputMatch: outputDesc, MicName: micDesc, Managed: true}
	if _, err := exec.LookPath("pactl"); err != nil {
		st.Help = "Install pactl (pulseaudio-utils or pipewire-pulse) so NoiseFree can create its virtual microphone."
		return st
	}
	if find(outputs, outputDesc) != "" || loaded(sinkName) {
		st.Ready = true
		return st
	}
	mu.Lock()
	defer mu.Unlock()
	sink, err := pactl("load-module", "module-null-sink", "sink_name="+sinkName, "rate=48000", "channels=1",
		`sink_properties='device.description="`+outputDesc+`"'`)
	if err != nil {
		st.Help = "Couldn't create the virtual microphone: " + err.Error()
		return st
	}
	modules = append(modules, sink)
	src, err := pactl("load-module", "module-remap-source", "master="+sinkName+".monitor", "source_name="+sourceName,
		`source_properties='device.description="`+micDesc+`"'`)
	if err == nil {
		modules = append(modules, src)
	}
	st.Ready = true
	return st
}

func teardown() {
	mu.Lock()
	defer mu.Unlock()
	for i := len(modules) - 1; i >= 0; i-- {
		_, _ = pactl("unload-module", modules[i])
	}
	modules = nil
}

func loaded(name string) bool {
	out, err := pactl("list", "short", "sinks")
	return err == nil && strings.Contains(out, name)
}

func pactl(args ...string) (string, error) {
	out, err := exec.Command("pactl", args...).CombinedOutput()
	if err != nil {
		return "", &pactlError{strings.TrimSpace(string(out)), err}
	}
	return strings.TrimSpace(string(out)), nil
}

type pactlError struct {
	out string
	err error
}

func (e *pactlError) Error() string {
	if e.out != "" {
		return e.out
	}
	return e.err.Error()
}
