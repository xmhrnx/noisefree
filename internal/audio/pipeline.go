// Package audio captures the microphone, runs the noise suppressor and
// plays the result into the virtual microphone (and optionally a monitor).
package audio

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/gen2brain/malgo"
	"github.com/xmhrnx/noisefree/internal/dsp"
	"github.com/xmhrnx/noisefree/internal/engine"
)

// Device is an audio input or output.
type Device struct {
	Name    string
	Default bool
	id      malgo.DeviceID
}

// Config selects the devices to run on. Empty names mean the system default.
type Config struct {
	Input, Output, Monitor string
	MonitorOn              bool
	BufferMs               int
}

// Pipeline is the running audio graph. Its exported setters are safe to
// call from any goroutine.
type Pipeline struct {
	ctx *malgo.AllocatedContext

	mu       sync.Mutex // guards everything the audio callback touches below
	dev, mon *malgo.Device
	proc     engine.Processor
	run      func([]float32) // proc.Process, bound once
	framer   *dsp.Framer
	hp       *dsp.Biquad
	gate     *dsp.Gate
	monitor  *ring
	period   int

	enabled, highPass, gateOn atomic.Bool
	gain                      atomic.Uint32
	gateDB                    atomic.Uint32
	load                      atomic.Uint32
	stopped                   atomic.Bool

	In, Out dsp.Meter
	// OnStop is called (from an audio thread) when a device stops on its
	// own, for example when it's unplugged.
	OnStop func()
}

// New opens the system audio backend.
func New() (*Pipeline, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, fmt.Errorf("open audio system: %w", err)
	}
	p := &Pipeline{ctx: ctx, hp: dsp.HighPass(80, engine.SampleRate), gate: dsp.NewGate(engine.SampleRate, -50)}
	p.enabled.Store(true)
	p.SetGain(0)
	p.SetGate(false, -50)
	return p, nil
}

// Close stops audio and releases the backend.
func (p *Pipeline) Close() {
	p.Stop()
	p.mu.Lock()
	if p.proc != nil {
		p.proc.Close()
		p.proc = nil
	}
	p.mu.Unlock()
	_ = p.ctx.Uninit()
	p.ctx.Free()
}

// Devices lists the inputs and outputs, refreshed on every call.
func (p *Pipeline) Devices() (inputs, outputs []Device, err error) {
	list := func(kind malgo.DeviceType) ([]Device, error) {
		infos, err := p.ctx.Devices(kind)
		if err != nil {
			return nil, err
		}
		out := make([]Device, 0, len(infos))
		for _, info := range infos {
			out = append(out, Device{Name: info.Name(), Default: info.IsDefault != 0, id: info.ID})
		}
		return out, nil
	}
	if inputs, err = list(malgo.Capture); err != nil {
		return nil, nil, err
	}
	outputs, err = list(malgo.Playback)
	return inputs, outputs, err
}

// IsOwnDevice reports whether a device belongs to NoiseFree's virtual
// microphone; recording from it would feed NoiseFree its own output.
func IsOwnDevice(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "noisefree") || strings.Contains(n, "cable output") || strings.Contains(n, "blackhole")
}

// FindOutput returns the name of the first output containing match.
func FindOutput(outputs []Device, match string) string {
	for _, d := range outputs {
		if match != "" && strings.Contains(strings.ToLower(d.Name), strings.ToLower(match)) {
			return d.Name
		}
	}
	return ""
}

func pick(devs []Device, name string) unsafe.Pointer {
	for i := range devs {
		if devs[i].Name == name {
			return devs[i].id.Pointer()
		}
	}
	return nil // system default
}

// Start opens the devices in cfg and begins processing. A running pipeline
// is restarted.
func (p *Pipeline) Start(cfg Config) error {
	p.Stop()
	inputs, outputs, err := p.Devices()
	if err != nil {
		return err
	}
	if cfg.BufferMs <= 0 {
		cfg.BufferMs = 10
	}
	dc := malgo.DefaultDeviceConfig(malgo.Duplex)
	dc.SampleRate = engine.SampleRate
	dc.PeriodSizeInMilliseconds = uint32(cfg.BufferMs)
	dc.PerformanceProfile = malgo.LowLatency
	dc.Capture.Format, dc.Capture.Channels = malgo.FormatF32, 1
	dc.Playback.Format, dc.Playback.Channels = malgo.FormatF32, 1
	dc.Capture.DeviceID = pick(inputs, cfg.Input)
	dc.Playback.DeviceID = pick(outputs, cfg.Output)

	p.mu.Lock()
	p.period = engine.SampleRate * cfg.BufferMs / 1000
	p.monitor = nil
	if cfg.MonitorOn {
		p.monitor = newRing(engine.SampleRate / 5)
	}
	p.mu.Unlock()
	p.stopped.Store(false)

	dev, err := malgo.InitDevice(p.ctx.Context, dc, malgo.DeviceCallbacks{Data: p.process, Stop: p.deviceStopped})
	if err != nil {
		return fmt.Errorf("open audio devices: %w", err)
	}
	if err := dev.Start(); err != nil {
		dev.Uninit()
		return fmt.Errorf("start audio: %w", err)
	}
	var mon *malgo.Device
	if cfg.MonitorOn {
		mc := malgo.DefaultDeviceConfig(malgo.Playback)
		mc.SampleRate = engine.SampleRate
		mc.PeriodSizeInMilliseconds = uint32(cfg.BufferMs)
		mc.Playback.Format, mc.Playback.Channels = malgo.FormatF32, 1
		mc.Playback.DeviceID = pick(outputs, cfg.Monitor)
		mon, err = malgo.InitDevice(p.ctx.Context, mc, malgo.DeviceCallbacks{Data: p.playMonitor})
		if err == nil {
			err = mon.Start()
		}
		if err != nil {
			dev.Uninit()
			if mon != nil {
				mon.Uninit()
			}
			return fmt.Errorf("start monitor: %w", err)
		}
	}
	p.mu.Lock()
	p.dev, p.mon = dev, mon
	p.mu.Unlock()
	return nil
}

// Stop closes the devices; the processor stays loaded.
func (p *Pipeline) Stop() {
	p.mu.Lock()
	dev, mon := p.dev, p.mon
	p.dev, p.mon = nil, nil
	p.mu.Unlock()
	p.stopped.Store(true)
	// Uninit waits for the callbacks, which take mu, so it runs unlocked.
	for _, d := range []*malgo.Device{dev, mon} {
		if d != nil {
			d.Uninit()
		}
	}
	p.In.Reset()
	p.Out.Reset()
	p.load.Store(0)
}

// Running reports whether audio is flowing.
func (p *Pipeline) Running() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.dev != nil
}

func (p *Pipeline) deviceStopped() {
	if !p.stopped.Load() && p.OnStop != nil {
		p.OnStop()
	}
}

// SetProcessor swaps in a new noise suppressor and closes the old one.
func (p *Pipeline) SetProcessor(proc engine.Processor) {
	var framer *dsp.Framer
	var run func([]float32)
	if proc != nil {
		framer, run = dsp.NewFramer(proc.FrameSize()), proc.Process
	}
	p.mu.Lock()
	old := p.proc
	p.proc, p.framer, p.run = proc, framer, run
	p.mu.Unlock()
	if old != nil {
		old.Close()
	}
}

// SetEnabled turns noise removal on or off without stopping audio.
func (p *Pipeline) SetEnabled(on bool) { p.enabled.Store(on) }

// SetStrength forwards the strength to the processor.
func (p *Pipeline) SetStrength(s float32) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.proc != nil {
		p.proc.SetStrength(s)
	}
}

// SetHighPass toggles the 80 Hz rumble filter.
func (p *Pipeline) SetHighPass(on bool) { p.highPass.Store(on) }

// SetGate toggles the noise gate and sets its threshold in dBFS.
func (p *Pipeline) SetGate(on bool, db float32) {
	p.gateOn.Store(on)
	p.gateDB.Store(math.Float32bits(db))
}

// SetGain sets the output gain in dB.
func (p *Pipeline) SetGain(db float32) {
	p.gain.Store(math.Float32bits(float32(math.Pow(10, float64(db)/20))))
}

// Stats describe the running pipeline.
type Stats struct {
	Load      float32 // share of real time the audio thread spends processing
	LatencyMs float32
	Backend   string
}

// Stats returns live statistics.
func (p *Pipeline) Stats() Stats {
	p.mu.Lock()
	defer p.mu.Unlock()
	st := Stats{Load: math.Float32frombits(p.load.Load())}
	samples := 2 * p.period
	if p.proc != nil {
		samples += p.proc.FrameSize() + p.proc.Latency()
		st.Backend = p.proc.Backend()
	}
	st.LatencyMs = float32(samples) * 1000 / engine.SampleRate
	return st
}

func passThrough([]float32) {}

func floats(b []byte) []float32 {
	if len(b) < 4 {
		return nil
	}
	return unsafe.Slice((*float32)(unsafe.Pointer(&b[0])), len(b)/4)
}

func (p *Pipeline) process(out, in []byte, frames uint32) {
	start := time.Now()
	o, i := floats(out), floats(in)
	if len(o) < int(frames) {
		return
	}
	o = o[:frames]
	if len(i) >= int(frames) {
		copy(o, i[:frames])
	} else {
		clear(o)
	}
	p.In.Measure(o)

	p.mu.Lock()
	if p.highPass.Load() {
		p.hp.Process(o)
	}
	if p.proc != nil {
		run := p.run
		if !p.enabled.Load() {
			run = passThrough // keep the latency constant: no click on toggle
		}
		p.framer.Run(o, run)
	}
	if p.gateOn.Load() {
		p.gate.SetThreshold(math.Float32frombits(p.gateDB.Load()))
		p.gate.Process(o)
	}
	mon := p.monitor
	p.mu.Unlock()

	if g := math.Float32frombits(p.gain.Load()); g != 1 {
		for k, v := range o {
			o[k] = min(max(v*g, -1), 1)
		}
	}
	p.Out.Measure(o)
	if mon != nil {
		mon.write(o)
	}
	budget := float64(frames) / engine.SampleRate
	l := float32(time.Since(start).Seconds() / budget)
	prev := math.Float32frombits(p.load.Load())
	p.load.Store(math.Float32bits(prev*0.95 + l*0.05))
}

func (p *Pipeline) playMonitor(out, _ []byte, frames uint32) {
	o := floats(out)
	p.mu.Lock()
	mon := p.monitor
	p.mu.Unlock()
	if mon == nil || len(o) < int(frames) {
		clear(o)
		return
	}
	mon.read(o[:frames])
}
