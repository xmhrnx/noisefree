package audio

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"

	"github.com/gen2brain/malgo"
	"github.com/xmhrnx/noisefree/internal/engine"
)

// Preview lengths.
const (
	// PreviewSeconds is the longest recording.
	PreviewSeconds = 10
	// EnvelopeBuckets is how many level points a full recording has.
	EnvelopeBuckets = PreviewSeconds * 50 // one per 20 ms
	previewSamples  = PreviewSeconds * engine.SampleRate
	bucketSamples   = previewSamples / EnvelopeBuckets
)

// Preview states.
const (
	PreviewEmpty int32 = iota
	PreviewRecording
	PreviewReady
	PreviewPlaying
)

// preview records the microphone before and after noise removal, so the
// user can hear what others hear, and plays either take back.
type preview struct {
	state atomic.Int32
	// written by the audio callback while recording, read after
	raw, clean []float32
	n          atomic.Int64
	// per-bucket peaks, safe to read while recording
	rawEnv, cleanEnv [EnvelopeBuckets]atomic.Uint32
	scratch          []float32

	mu     sync.Mutex // guards the playback device
	player *malgo.Device
	pos    atomic.Int64
	which  atomic.Int32 // 0 clean, 1 raw
	ended  chan struct{}
}

// keepRaw stores the unprocessed block before the chain runs.
func (v *preview) keepRaw(in []float32) {
	if v.state.Load() != PreviewRecording {
		return
	}
	if cap(v.scratch) < len(in) {
		v.scratch = make([]float32, len(in)) // first callback only
	}
	v.scratch = v.scratch[:len(in)]
	copy(v.scratch, in)
}

// record appends the raw (kept) and processed blocks.
func (v *preview) record(clean []float32) {
	if v.state.Load() != PreviewRecording {
		return
	}
	n := int(v.n.Load())
	k := min(len(clean), previewSamples-n, len(v.scratch))
	copy(v.raw[n:], v.scratch[:k])
	copy(v.clean[n:], clean[:k])
	for i := 0; i < k; i++ {
		b := (n + i) / bucketSamples
		peak(&v.rawEnv[b], v.raw[n+i])
		peak(&v.cleanEnv[b], v.clean[n+i])
	}
	v.n.Store(int64(n + k))
	if n+k >= previewSamples {
		v.state.Store(PreviewReady)
	}
}

func peak(a *atomic.Uint32, s float32) {
	s = float32(math.Abs(float64(s)))
	if s > math.Float32frombits(a.Load()) {
		a.Store(math.Float32bits(s))
	}
}

// StartRecording begins a new preview take. Audio must be running.
func (p *Pipeline) StartRecording() error {
	if !p.Running() {
		return errors.New("start audio first")
	}
	v := &p.preview
	p.StopPlayback()
	if v.raw == nil {
		v.raw, v.clean = make([]float32, previewSamples), make([]float32, previewSamples)
	}
	for i := range v.rawEnv {
		v.rawEnv[i].Store(0)
		v.cleanEnv[i].Store(0)
	}
	v.n.Store(0)
	v.state.Store(PreviewRecording)
	return nil
}

// StopRecording ends the take early.
func (p *Pipeline) StopRecording() {
	v := &p.preview
	if v.state.CompareAndSwap(PreviewRecording, PreviewReady) && v.n.Load() == 0 {
		v.state.Store(PreviewEmpty)
	}
}

// PreviewState returns PreviewEmpty, PreviewRecording, PreviewReady or
// PreviewPlaying.
func (p *Pipeline) PreviewState() int32 { return p.preview.state.Load() }

// PreviewProgress returns the recorded length and, while playing, the
// playback position, in seconds.
func (p *Pipeline) PreviewProgress() (recorded, played float64) {
	v := &p.preview
	return float64(v.n.Load()) / engine.SampleRate, float64(v.pos.Load()) / engine.SampleRate
}

// PreviewEnvelope fills raw and clean (each EnvelopeBuckets long) with the
// take's peak levels.
func (p *Pipeline) PreviewEnvelope(raw, clean []float32) {
	v := &p.preview
	for i := range v.rawEnv {
		raw[i] = math.Float32frombits(v.rawEnv[i].Load())
		clean[i] = math.Float32frombits(v.cleanEnv[i].Load())
	}
}

// PlayPreview plays the take on the named output ("" is the system
// default): the processed audio, or the original when original is true.
func (p *Pipeline) PlayPreview(output string, original bool) error {
	v := &p.preview
	p.StopPlayback()
	if v.state.Load() != PreviewReady {
		return errors.New("nothing recorded yet")
	}
	_, outputs, err := p.Devices()
	if err != nil {
		return err
	}
	cfg := malgo.DefaultDeviceConfig(malgo.Playback)
	cfg.SampleRate = engine.SampleRate
	cfg.Playback.Format, cfg.Playback.Channels = malgo.FormatF32, 1
	cfg.Playback.DeviceID = pick(outputs, output)
	v.pos.Store(0)
	p.SetPreviewOriginal(original)
	ended := make(chan struct{})
	var once sync.Once
	dev, err := malgo.InitDevice(p.ctx.Context, cfg, malgo.DeviceCallbacks{Data: func(out, _ []byte, frames uint32) {
		o := floats(out)[:frames]
		src := v.clean
		if v.which.Load() == 1 {
			src = v.raw
		}
		pos, end := int(v.pos.Load()), int(v.n.Load())
		k := copy(o, src[pos:end])
		clear(o[k:])
		v.pos.Store(int64(pos + k))
		if pos+k >= end {
			once.Do(func() { close(ended) })
		}
	}})
	if err != nil {
		return fmt.Errorf("open speakers: %w", err)
	}
	v.mu.Lock()
	v.player, v.ended = dev, ended
	v.mu.Unlock()
	v.state.Store(PreviewPlaying)
	if err := dev.Start(); err != nil {
		p.StopPlayback()
		return fmt.Errorf("play: %w", err)
	}
	go func() {
		<-ended
		p.stopPlayer(dev)
	}()
	return nil
}

// SetPreviewOriginal switches playback between the processed take and the
// original, live, for an instant A/B comparison.
func (p *Pipeline) SetPreviewOriginal(original bool) {
	if original {
		p.preview.which.Store(1)
	} else {
		p.preview.which.Store(0)
	}
}

// StopPlayback stops a playing take.
func (p *Pipeline) StopPlayback() {
	v := &p.preview
	v.mu.Lock()
	dev := v.player
	v.mu.Unlock()
	if dev != nil {
		p.stopPlayer(dev)
	}
}

func (p *Pipeline) stopPlayer(dev *malgo.Device) {
	v := &p.preview
	v.mu.Lock()
	if v.player != dev {
		v.mu.Unlock()
		return
	}
	v.player = nil
	v.mu.Unlock()
	dev.Uninit()
	v.state.CompareAndSwap(PreviewPlaying, PreviewReady)
}
