// Package onnx loads ONNX Runtime once per process and creates sessions on
// the CPU or a GPU, shared by the audio and video models.
package onnx

import (
	"fmt"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

// Accelerator selects where ONNX Runtime runs the network.
type Accelerator string

// Accelerators. DirectML runs on any DirectX 12 GPU (NVIDIA, AMD, Intel,
// Qualcomm), CoreML on Apple silicon GPUs and the Neural Engine, CUDA on
// NVIDIA GPUs.
const (
	CPU      Accelerator = "cpu"
	DirectML Accelerator = "directml"
	CoreML   Accelerator = "coreml"
	CUDA     Accelerator = "cuda"
)

var (
	runtimeMu   sync.Mutex
	runtimePath string
)

// Init loads the ONNX Runtime library once per process. ONNX Runtime
// can't be unloaded, so switching to another build needs a restart.
func Init(libPath string) error {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	if runtimePath != "" {
		if runtimePath != libPath {
			return ErrRestartRequired
		}
		return nil
	}
	ort.SetSharedLibraryPath(libPath)
	if err := ort.InitializeEnvironment(); err != nil {
		return fmt.Errorf("load ONNX Runtime: %w", err)
	}
	runtimePath = libPath
	return nil
}

// ErrRestartRequired means another ONNX Runtime build is already loaded.
var ErrRestartRequired = fmt.Errorf("a different ONNX Runtime is loaded; restart NoiseFree to switch")

// SessionOptions returns options for a session on acc using threads CPU
// threads, and the accelerator actually set up: on failure the options fall
// back to the CPU and the error says why. Small models want one thread:
// spinning thread pools would cost more than the inference itself.
func SessionOptions(acc Accelerator, threads int) (*ort.SessionOptions, Accelerator, error) {
	o, err := ort.NewSessionOptions()
	if err != nil {
		return nil, CPU, err
	}
	_ = o.SetIntraOpNumThreads(max(threads, 1))
	// Idle worker threads would otherwise spin between frames, burning a
	// core for nothing in an app that runs all day.
	_ = o.AddSessionConfigEntry("session.intra_op.allow_spinning", "0")
	_ = o.AddSessionConfigEntry("session.inter_op.allow_spinning", "0")
	_ = o.SetInterOpNumThreads(1)
	_ = o.SetGraphOptimizationLevel(ort.GraphOptimizationLevelEnableAll)
	var perr error
	switch acc {
	case DirectML:
		_ = o.SetMemPattern(false) // required by DirectML
		_ = o.SetExecutionMode(ort.ExecutionModeSequential)
		perr = o.AppendExecutionProviderDirectML(0)
	case CoreML:
		perr = o.AppendExecutionProviderCoreMLV2(map[string]string{"MLComputeUnits": "ALL"})
	case CUDA:
		var c *ort.CUDAProviderOptions
		if c, perr = ort.NewCUDAProviderOptions(); perr == nil {
			perr = o.AppendExecutionProviderCUDA(c)
			c.Destroy()
		}
	}
	if perr != nil {
		return o, CPU, fmt.Errorf("%s unavailable, using CPU: %w", acc, perr)
	}
	return o, acc, nil
}
