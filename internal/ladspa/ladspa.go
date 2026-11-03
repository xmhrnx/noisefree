// Package ladspa hosts LADSPA audio plugins (.so, .dll, .dylib), the common
// format RNNoise and DeepFilterNet suppressors ship in.
package ladspa

/*
#cgo linux LDFLAGS: -ldl
#include <stdlib.h>
#include "host.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"unsafe"
)

const (
	portInput   = 0x1
	portOutput  = 0x2
	portControl = 0x4
	portAudio   = 0x8

	hintBelow       = 0x1
	hintAbove       = 0x2
	hintSampleRate  = 0x8
	hintLogarithmic = 0x10
	hintDefaultMask = 0x3C0
)

// Port describes one plugin port.
type Port struct {
	Index    int
	Name     string
	Control  bool
	Input    bool
	Min, Max float32
	Default  float32
}

// Plugin is one instantiated mono LADSPA effect, processing up to maxBlock
// samples per call.
type Plugin struct {
	lib      unsafe.Pointer
	desc     *C.LADSPA_Descriptor
	handle   C.LADSPA_Handle
	in, out  *C.float
	controls map[string]*C.float
	ports    []Port
	allocs   []unsafe.Pointer
	maxBlock int
	Label    string
	Name     string
}

// Open loads the library at path and instantiates the plugin with the given
// label (or the first mono effect when label is empty) at sampleRate.
func Open(path, label string, sampleRate, maxBlock int) (*Plugin, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	var errBuf [512]C.char
	lib := C.noisefree_open(cpath, &errBuf[0], C.int(len(errBuf)))
	if lib == nil {
		return nil, fmt.Errorf("load %s: %s", path, C.GoString(&errBuf[0]))
	}
	p := &Plugin{lib: lib, maxBlock: maxBlock, controls: map[string]*C.float{}}
	for i := C.ulong(0); ; i++ {
		d := C.noisefree_descriptor(lib, i)
		if d == nil {
			break
		}
		l := C.GoString(d.Label)
		if (label == "" && isMonoEffect(d)) || l == label {
			p.desc, p.Label, p.Name = d, l, C.GoString(d.Name)
			break
		}
	}
	if p.desc == nil {
		C.noisefree_close(lib)
		if label == "" {
			return nil, errors.New("plugin has no mono effect")
		}
		return nil, fmt.Errorf("plugin has no effect labelled %q", label)
	}
	if err := p.instantiate(sampleRate); err != nil {
		C.noisefree_close(lib)
		return nil, err
	}
	return p, nil
}

func isMonoEffect(d *C.LADSPA_Descriptor) bool {
	var ins, outs int
	for _, pd := range unsafe.Slice(d.PortDescriptors, d.PortCount) {
		if pd&portAudio != 0 && pd&portInput != 0 {
			ins++
		}
		if pd&portAudio != 0 && pd&portOutput != 0 {
			outs++
		}
	}
	return ins == 1 && outs == 1
}

func (p *Plugin) instantiate(rate int) error {
	d := p.desc
	p.handle = C.noisefree_instantiate(d, C.ulong(rate))
	if p.handle == nil {
		return errors.New("plugin refused to instantiate (unsupported sample rate?)")
	}
	n := int(d.PortCount)
	descs := unsafe.Slice(d.PortDescriptors, n)
	names := unsafe.Slice(d.PortNames, n)
	hints := unsafe.Slice(d.PortRangeHints, n)
	p.in, p.out = p.alloc(p.maxBlock), p.alloc(p.maxBlock)
	audioIn, audioOut := false, false
	for i := 0; i < n; i++ {
		pd := int(descs[i])
		port := Port{Index: i, Name: C.GoString(names[i]), Control: pd&portControl != 0, Input: pd&portInput != 0}
		switch {
		case pd&portAudio != 0 && port.Input && !audioIn:
			C.noisefree_connect(d, p.handle, C.ulong(i), p.in)
			audioIn = true
		case pd&portAudio != 0 && !port.Input && !audioOut:
			C.noisefree_connect(d, p.handle, C.ulong(i), p.out)
			audioOut = true
		case port.Control:
			port.Min, port.Max, port.Default = rangeOf(hints[i], rate)
			v := p.alloc(1)
			*v = C.float(port.Default)
			C.noisefree_connect(d, p.handle, C.ulong(i), v)
			if port.Input {
				p.controls[port.Name] = v
			}
		default: // extra audio ports get a scratch buffer
			C.noisefree_connect(d, p.handle, C.ulong(i), p.alloc(p.maxBlock))
		}
		p.ports = append(p.ports, port)
	}
	C.noisefree_activate(d, p.handle)
	return nil
}

// alloc returns zeroed C memory for n samples; plugins keep port pointers, so
// buffers must live outside the Go heap.
func (p *Plugin) alloc(n int) *C.float {
	ptr := C.calloc(C.size_t(n), 4)
	p.allocs = append(p.allocs, ptr)
	return (*C.float)(ptr)
}

func rangeOf(h C.LADSPA_PortRangeHint, rate int) (lo, hi, def float32) {
	hint := int(h.HintDescriptor)
	lo, hi = float32(h.LowerBound), float32(h.UpperBound)
	if hint&hintSampleRate != 0 {
		lo, hi = lo*float32(rate), hi*float32(rate)
	}
	if hint&hintBelow == 0 {
		lo = 0
	}
	if hint&hintAbove == 0 {
		hi = max(lo, 1)
	}
	mid := func(t float64) float32 {
		if hint&hintLogarithmic != 0 && lo > 0 && hi > 0 {
			return float32(math.Exp(math.Log(float64(lo))*(1-t) + math.Log(float64(hi))*t))
		}
		return lo + (hi-lo)*float32(t)
	}
	switch hint & hintDefaultMask {
	case 0x40:
		def = lo
	case 0x80:
		def = mid(0.25)
	case 0xC0:
		def = mid(0.5)
	case 0x100:
		def = mid(0.75)
	case 0x140:
		def = hi
	case 0x200:
		def = 0
	case 0x240:
		def = 1
	case 0x280:
		def = 100
	case 0x2C0:
		def = 440
	default:
		def = lo
	}
	return lo, hi, def
}

// Ports lists the plugin's ports.
func (p *Plugin) Ports() []Port { return p.ports }

// SetControl sets an input control port by (case-insensitive prefix of its)
// name, clamped to the port's range. It reports whether the port exists.
func (p *Plugin) SetControl(name string, v float32) bool {
	for _, port := range p.ports {
		if port.Control && port.Input && strings.HasPrefix(strings.ToLower(port.Name), strings.ToLower(name)) {
			*p.controls[port.Name] = C.float(min(max(v, port.Min), port.Max))
			return true
		}
	}
	return false
}

// Process runs the plugin over buf in place, in chunks of at most maxBlock.
func (p *Plugin) Process(buf []float32) {
	in := unsafe.Slice((*float32)(unsafe.Pointer(p.in)), p.maxBlock)
	out := unsafe.Slice((*float32)(unsafe.Pointer(p.out)), p.maxBlock)
	for len(buf) > 0 {
		n := min(len(buf), p.maxBlock)
		copy(in, buf[:n])
		C.noisefree_run(p.desc, p.handle, C.ulong(n))
		copy(buf[:n], out[:n])
		buf = buf[n:]
	}
}

// Close deactivates and frees the plugin instance. The library itself stays
// loaded: plugins written in Rust or C++ may keep threads or thread-locals
// that crash if their code is unmapped, and reopening it is then free.
func (p *Plugin) Close() {
	if p.handle == nil {
		return
	}
	C.noisefree_cleanup(p.desc, p.handle)
	p.handle = nil
	for _, ptr := range p.allocs {
		C.free(ptr)
	}
	p.allocs = nil
}
