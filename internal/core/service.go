// Package core ties settings, the engine catalog, the audio pipeline and the
// virtual microphone together. It has no UI code: the UI calls it and shows
// what it reports.
package core

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/xmhrnx/noisefree/internal/audio"
	"github.com/xmhrnx/noisefree/internal/catalog"
	"github.com/xmhrnx/noisefree/internal/settings"
	"github.com/xmhrnx/noisefree/internal/vmic"
)

// Service is the running app backend. Methods are safe from any goroutine;
// the ones documented as blocking belong in a goroutine, not a click handler.
type Service struct {
	Dir   string
	Store *catalog.Store
	Pipe  *audio.Pipeline

	mu        sync.Mutex
	set       settings.Settings
	dirty     bool
	vmic      vmic.Status
	engine    string // ID of the loaded engine
	backend   string
	warning   string
	audioErr  error
	saveErr   error
	done      chan struct{}
	closeOnce sync.Once
}

// Open loads settings and the catalog and opens the audio system. A broken
// user manifest is returned as a warning error alongside a usable service.
func Open() (*Service, error) {
	dir, err := settings.Dir()
	if err != nil {
		return nil, err
	}
	set, setErr := settings.Load(dir)
	store, storeErr := catalog.NewStore(dir)
	pipe, err := audio.New()
	if err != nil {
		return nil, err
	}
	s := &Service{Dir: dir, Store: store, Pipe: pipe, set: set, done: make(chan struct{})}
	pipe.SetEnabled(set.Enabled)
	pipe.SetHighPass(set.HighPass)
	pipe.SetGate(set.Gate, set.GateDB)
	pipe.SetGain(set.GainDB)
	go s.autosave()
	if setErr != nil {
		setErr = fmt.Errorf("settings were reset: %w", setErr)
	}
	return s, errors.Join(setErr, storeErr)
}

// Close stops audio, removes the virtual microphone and saves settings.
func (s *Service) Close() {
	s.closeOnce.Do(func() {
		close(s.done)
		s.Pipe.Close()
		vmic.Teardown()
		s.flush()
	})
}

// Settings returns a copy of the current settings.
func (s *Service) Settings() settings.Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set
}

// Update changes settings in memory and applies the live ones; they're saved
// shortly after. Device or engine changes need Restart or LoadEngine.
func (s *Service) Update(fn func(*settings.Settings)) {
	s.mu.Lock()
	fn(&s.set)
	set := s.set
	s.dirty = true
	s.mu.Unlock()
	s.Pipe.SetEnabled(set.Enabled)
	s.Pipe.SetHighPass(set.HighPass)
	s.Pipe.SetGate(set.Gate, set.GateDB)
	s.Pipe.SetGain(set.GainDB)
	s.Pipe.SetStrength(set.Strength)
}

func (s *Service) autosave() {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			s.flush()
		case <-s.done:
			return
		}
	}
}

func (s *Service) flush() {
	s.mu.Lock()
	if !s.dirty {
		s.mu.Unlock()
		return
	}
	set := s.set
	s.dirty = false
	s.mu.Unlock()
	err := settings.Save(s.Dir, set)
	s.mu.Lock()
	s.saveErr = err
	s.mu.Unlock()
}

// SaveError reports the last failure to save settings.
func (s *Service) SaveError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveErr
}

// LoadEngine loads the engine named in the settings and swaps it into the
// pipeline. It blocks while models load. The returned warning explains a
// GPU-to-CPU fallback.
func (s *Service) LoadEngine() (string, error) {
	set := s.Settings()
	e, ok := s.Store.Get(set.Engine)
	if !ok || !s.Store.Installed(e) {
		set.Engine = "rnnoise"
		s.Update(func(st *settings.Settings) { st.Engine = "rnnoise" })
	}
	proc, warn, err := s.Store.Open(set.Engine, set.UseGPU)
	if err != nil {
		return "", err
	}
	proc.SetStrength(set.Strength)
	s.Pipe.SetProcessor(proc)
	s.mu.Lock()
	s.engine, s.backend, s.warning = set.Engine, proc.Backend(), warn
	s.mu.Unlock()
	return warn, nil
}

// ActiveEngine returns the loaded engine's ID, where it runs and any
// fallback warning.
func (s *Service) ActiveEngine() (id, backend, warning string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.engine, s.backend, s.warning
}

// DeviceNames lists the microphones and outputs.
func (s *Service) DeviceNames() (inputs, outputs []string, err error) {
	in, out, err := s.Pipe.Devices()
	for _, d := range in {
		if !audio.IsOwnDevice(d.Name) {
			inputs = append(inputs, d.Name)
		}
	}
	for _, d := range out {
		outputs = append(outputs, d.Name)
	}
	return inputs, outputs, err
}

// VirtualMic returns the virtual microphone status from the last Restart.
func (s *Service) VirtualMic() vmic.Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.vmic
}

// OutputName is the device NoiseFree plays into: the user's choice, else the
// virtual microphone, else the system default ("").
func (s *Service) OutputName() string {
	set := s.Settings()
	if set.Output != "" {
		return set.Output
	}
	st := s.VirtualMic()
	if !set.VirtualMic || !st.Ready {
		return ""
	}
	_, outs, _ := s.Pipe.Devices()
	return audio.FindOutput(outs, st.OutputMatch)
}

// Restart (re)opens the audio devices from the settings, setting up the
// virtual microphone first. It blocks briefly.
func (s *Service) Restart() error {
	set := s.Settings()
	if set.VirtualMic {
		_, outs, _ := s.DeviceNames()
		st := vmic.Setup(outs)
		s.mu.Lock()
		s.vmic = st
		s.mu.Unlock()
	}
	err := s.Pipe.Start(audio.Config{
		Input: s.inputName(set.Input), Output: s.OutputName(), Monitor: s.monitorName(set.MonitorDevice),
		MonitorOn: set.Monitor, BufferMs: set.BufferMs,
	})
	s.mu.Lock()
	s.audioErr = err
	s.mu.Unlock()
	return err
}

// inputName guards against feedback: when the system default microphone is
// NoiseFree's own virtual microphone, the first real microphone is used instead.
func (s *Service) inputName(chosen string) string {
	if chosen != "" {
		return chosen
	}
	ins, _, err := s.Pipe.Devices()
	if err != nil {
		return ""
	}
	return realDefault(ins)
}

// monitorName keeps "hear yourself" out of the virtual microphone when the
// system default output is NoiseFree's own virtual device.
func (s *Service) monitorName(chosen string) string {
	if chosen != "" {
		return chosen
	}
	_, outs, err := s.Pipe.Devices()
	if err != nil {
		return ""
	}
	return realDefault(outs)
}

// realDefault returns "" (the system default) unless the default device is
// one of NoiseFree's own, in which case it names the first other device.
func realDefault(devs []audio.Device) string {
	for _, d := range devs {
		if d.Default && !audio.IsOwnDevice(d.Name) {
			return ""
		}
	}
	for _, d := range devs {
		if !audio.IsOwnDevice(d.Name) {
			return d.Name
		}
	}
	return ""
}

// AudioError is the last failure to start audio.
func (s *Service) AudioError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.audioErr
}
