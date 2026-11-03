// Package rnnoise wraps Xiph's RNNoise (BSD-3-Clause, see COPYING), compiled
// into the app so the neural suppressor works offline on every platform.
package rnnoise

/*
#cgo CFLAGS: -O2 -I${SRCDIR} -Wno-unused-variable -Wno-unused-function
#cgo !windows LDFLAGS: -lm
#include <stdlib.h>
#include "rnnoise.h"
*/
import "C"

import (
	"errors"
	"unsafe"
)

// FrameSize is the number of 48 kHz samples RNNoise processes per call.
const FrameSize = 480

// State is one RNNoise denoiser instance. It is not safe for concurrent use.
type State struct {
	st  *C.DenoiseState
	buf *C.float
}

// New allocates a denoiser.
func New() (*State, error) {
	st := C.rnnoise_create(nil)
	if st == nil {
		return nil, errors.New("rnnoise: allocation failed")
	}
	buf := (*C.float)(C.malloc(C.size_t(FrameSize * 4)))
	return &State{st: st, buf: buf}, nil
}

// Process denoises one frame of FrameSize samples in place and returns the
// voice probability RNNoise computed for it. Samples are in [-1, 1].
func (s *State) Process(frame []float32) float32 {
	b := unsafe.Slice((*float32)(unsafe.Pointer(s.buf)), FrameSize)
	for i, v := range frame[:FrameSize] {
		b[i] = v * 32768 // RNNoise works on 16-bit scaled floats
	}
	vad := C.rnnoise_process_frame(s.st, s.buf, s.buf)
	for i := range frame[:FrameSize] {
		frame[i] = b[i] / 32768
	}
	return float32(vad)
}

// Close frees the denoiser.
func (s *State) Close() {
	if s.st != nil {
		C.rnnoise_destroy(s.st)
		C.free(unsafe.Pointer(s.buf))
		s.st, s.buf = nil, nil
	}
}
