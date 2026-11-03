package catalog

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/xmhrnx/noisefree/internal/dtln"
	"github.com/xmhrnx/noisefree/internal/engine"
)

// GPUAccelerator is the GPU backend neural engines use on this platform.
func GPUAccelerator() dtln.Accelerator {
	switch runtime.GOOS {
	case "windows":
		return dtln.DirectML
	case "darwin":
		return dtln.CoreML
	}
	return dtln.CUDA
}

// GPUNeedsPack reports whether GPU use needs the GPU acceleration pack (on
// macOS the standard runtime already includes Core ML).
func GPUNeedsPack() bool { return runtime.GOOS != "darwin" }

// GPUSupported reports whether this platform has any GPU backend.
func GPUSupported() bool {
	if runtime.GOOS == "darwin" {
		return runtime.GOARCH == "arm64"
	}
	e, ok := findBuiltin("onnxruntime-gpu")
	return ok && e.Available()
}

func findBuiltin(id string) (Entry, bool) {
	for _, e := range Builtin() {
		if e.ID == id {
			return e, true
		}
	}
	return Entry{}, false
}

// Open creates a running processor for an installed entry. useGPU asks
// neural engines to use the graphics card; the returned warning explains a
// fallback to the CPU.
func (s *Store) Open(id string, useGPU bool) (engine.Processor, string, error) {
	e, ok := s.Get(id)
	if !ok {
		return nil, "", fmt.Errorf("unknown engine %q", id)
	}
	if !s.Installed(e) {
		return nil, "", fmt.Errorf("%s isn't installed", e.Name)
	}
	switch e.Kind {
	case KindLite:
		return engine.NewLite(), "", nil
	case KindRNNoise:
		p, err := engine.NewRNNoise()
		return p, "", err
	case KindLADSPA:
		spec := e.Plugin
		p, err := engine.NewPlugin(engine.PluginOptions{
			Path: filepath.Join(s.EngineDir(id), spec.File), Label: spec.Label,
			StrengthControl: spec.StrengthControl, StrengthMin: spec.StrengthMin, StrengthMax: spec.StrengthMax,
			Controls: spec.Controls, Latency: spec.LatencySamples,
		})
		return p, "", err
	case KindDTLN:
		lib, acc, warn := s.RuntimeFor(useGPU)
		if lib == "" {
			return nil, "", errors.New("ONNX Runtime isn't installed; reinstall the engine")
		}
		p, w, err := engine.NewNeural(s.EngineDir(id), lib, acc)
		if w != "" {
			warn = w
		}
		if errors.Is(err, dtln.ErrRestartRequired) {
			return nil, "", err
		}
		return p, warn, err
	}
	return nil, "", fmt.Errorf("%s can't be run (kind %q)", e.Name, e.Kind)
}

// RuntimeFor picks the ONNX Runtime library and accelerator to use; the
// library is empty when ONNX Runtime isn't installed, and the warning
// explains a fallback to the CPU.
func (s *Store) RuntimeFor(useGPU bool) (string, dtln.Accelerator, string) {
	cpu := findLibrary(s.EngineDir("onnxruntime"))
	if !useGPU {
		return cpu, dtln.CPU, ""
	}
	if !GPUSupported() {
		return cpu, dtln.CPU, "No GPU backend for this computer; running on the CPU"
	}
	if !GPUNeedsPack() {
		return cpu, GPUAccelerator(), ""
	}
	if gpu := findLibrary(s.EngineDir("onnxruntime-gpu")); gpu != "" {
		return gpu, GPUAccelerator(), ""
	}
	return cpu, dtln.CPU, "Install the GPU acceleration pack to use the GPU; running on the CPU"
}

func findLibrary(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		n := e.Name()
		if strings.Contains(n, "providers") {
			continue
		}
		if n == "onnxruntime.dll" || strings.HasPrefix(n, "libonnxruntime.so.") ||
			(strings.HasPrefix(n, "libonnxruntime.") && strings.HasSuffix(n, ".dylib")) {
			return filepath.Join(dir, n)
		}
	}
	return ""
}
