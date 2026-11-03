// Package dtln runs the DTLN speech enhancement network (Westhausen & Meyer,
// MIT license) through ONNX Runtime, on the CPU or a GPU.
package dtln

import "github.com/xmhrnx/noisefree/internal/onnx"

// Accelerator selects where ONNX Runtime runs the network.
type Accelerator = onnx.Accelerator

// Accelerators, see package onnx.
const (
	CPU      = onnx.CPU
	DirectML = onnx.DirectML
	CoreML   = onnx.CoreML
	CUDA     = onnx.CUDA
)

// ErrRestartRequired means another ONNX Runtime build is already loaded.
var ErrRestartRequired = onnx.ErrRestartRequired
