// Package settings persists the user's choices as JSON.
package settings

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Settings are the user's saved choices.
type Settings struct {
	Enabled       bool    `json:"enabled"`       // noise removal on
	Engine        string  `json:"engine"`        // catalog entry ID
	Strength      float32 `json:"strength"`      // 0..1
	UseGPU        bool    `json:"useGPU"`        // run neural engines on the GPU
	Input         string  `json:"input"`         // microphone name, "" = system default
	Output        string  `json:"output"`        // output device name, "" = virtual mic or default
	Monitor       bool    `json:"monitor"`       // play the result to Monitor too
	MonitorDevice string  `json:"monitorDevice"` // "" = system default speakers
	HighPass      bool    `json:"highPass"`
	Gate          bool    `json:"gate"`
	GateDB        float32 `json:"gateDB"`
	GainDB        float32 `json:"gainDB"`
	BufferMs      int     `json:"bufferMs"` // device period: 10 (low latency) or 20 (low CPU)
	Theme         string  `json:"theme"`    // "dark", "light" or "system"
	CloseToTray   bool    `json:"closeToTray"`
	VirtualMic    bool    `json:"virtualMic"`
	// ProtectDefaults switches the system's default speaker and microphone
	// back to real devices when a virtual cable driver takes them over.
	ProtectDefaults bool `json:"protectDefaults"`
	// LastOutput and LastInput are the last real system defaults seen, the
	// devices ProtectDefaults restores.
	LastOutput string `json:"lastOutput"`
	LastInput  string `json:"lastInput"`

	Video Video `json:"video"`
}

// Video are the camera effect settings.
type Video struct {
	Enabled       bool    `json:"enabled"`       // camera running
	Camera        string  `json:"camera"`        // device name, "" = first
	Height        int     `json:"height"`        // 720, 540 or 360
	VirtualCamera bool    `json:"virtualCamera"` // publish to the virtual camera
	Background    int     `json:"background"`    // vision.BackgroundMode
	BlurStrength  float32 `json:"blurStrength"`
	Preset        string  `json:"preset"`    // built-in background name
	ImagePath     string  `json:"imagePath"` // custom background, overrides Preset
	RemoveColor   string  `json:"removeColor"`
	EyeContact    bool    `json:"eyeContact"`
	EyeStrength   float32 `json:"eyeStrength"`
	Lighting      bool    `json:"lighting"`
	LightStrength float32 `json:"lightStrength"`
	Denoise       bool    `json:"denoise"`
	Sharpen       bool    `json:"sharpen"`
}

// Defaults are the settings on first launch.
func Defaults() Settings {
	return Settings{
		Enabled: true, Engine: "rnnoise", Strength: 0.9, HighPass: true,
		GateDB: -50, BufferMs: 10, Theme: "dark", CloseToTray: true, VirtualMic: true,
		ProtectDefaults: true,
		Video: Video{Height: 720, VirtualCamera: true, Background: 1, BlurStrength: 0.6, Preset: "Studio",
			RemoveColor: "#00B140", EyeStrength: 0.8, Lighting: true, LightStrength: 0.6, Denoise: true},
	}
}

// Dir is the folder NoiseFree keeps its settings and engines in.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "NoiseFree"), nil
}

// Load reads the settings in dir, or returns the defaults on first launch.
func Load(dir string) (Settings, error) {
	s := Defaults()
	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	err = json.Unmarshal(data, &s)
	return s, err
}

// Save writes the settings atomically.
func Save(dir string, s Settings) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, "settings.json.tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, "settings.json"))
}
