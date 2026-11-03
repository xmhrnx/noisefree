// Package engine turns every installed noise suppressor into a Processor the
// audio pipeline can run, whatever its technology.
package engine

// SampleRate is the rate every processor runs at.
const SampleRate = 48000

// Processor removes noise from mono 48 kHz audio. Process is only called
// from the audio thread, with exactly FrameSize samples.
type Processor interface {
	// FrameSize is the number of samples Process expects.
	FrameSize() int
	// Latency is the processor's own delay in samples, excluding framing.
	Latency() int
	// SetStrength sets how much noise is removed, 0 (none) to 1 (all). It
	// may be called from any goroutine.
	SetStrength(s float32)
	// Process denoises frame in place.
	Process(frame []float32)
	// Backend names where the work runs, e.g. "CPU" or "GPU · DirectML".
	Backend() string
	// Close releases the processor.
	Close()
}
