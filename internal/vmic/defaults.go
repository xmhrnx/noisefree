package vmic

import (
	"errors"
	"strings"
)

// Endpoint is one system audio device: ID is what the OS addresses it by,
// Name what people see.
type Endpoint struct {
	ID, Name string
	Default  bool
}

// virtualNames are substrings of virtual audio devices: NoiseFree's own and
// the cable drivers it plays into. Recording from or playing into one by
// accident gives silence or feedback.
var virtualNames = []string{"noisefree", "cable input", "cable output", "vb-audio", "voicemeeter", "blackhole", "loopback audio"}

// IsVirtual reports whether a device is a virtual cable rather than a real
// speaker or microphone.
func IsVirtual(name string) bool {
	n := strings.ToLower(name)
	for _, v := range virtualNames {
		if strings.Contains(n, v) {
			return true
		}
	}
	return false
}

func (e Endpoint) virtual() bool {
	return IsVirtual(e.Name) || IsVirtual(e.ID) || strings.HasSuffix(e.ID, ".monitor")
}

// Repair says what GuardDefaults found and changed.
type Repair struct {
	// FixedOutput and FixedInput name the real devices made the default
	// again, or are empty when nothing needed fixing.
	FixedOutput, FixedInput string
	// Was names the virtual devices that had taken over.
	WasOutput, WasInput string
	// RealOutput and RealInput are the system defaults after the repair,
	// worth remembering for the next one.
	RealOutput, RealInput string
}

// ErrUnsupported means this system's defaults can't be changed by NoiseFree.
var ErrUnsupported = errors.New("changing the default audio device isn't supported here")

// GuardDefaults makes sure the system's default speaker and microphone are
// real devices. Virtual cable installers (VB-Audio's in particular) make
// themselves the default, which silences the computer; this switches back,
// to preferOut/preferIn (the last real defaults) when they're still there,
// else to the first real device.
func GuardDefaults(preferOut, preferIn string) (Repair, error) {
	var r Repair
	var errs []error
	for _, output := range []bool{true, false} {
		eps, err := endpoints(output)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		prefer, fixed, was, real := preferIn, &r.FixedInput, &r.WasInput, &r.RealInput
		if output {
			prefer, fixed, was, real = preferOut, &r.FixedOutput, &r.WasOutput, &r.RealOutput
		}
		def, ok := defaultOf(eps)
		if ok && !def.virtual() {
			*real = def.Name
			continue
		}
		if !ok {
			continue // no default at all: nothing to restore from
		}
		target, ok := realTarget(eps, prefer)
		if !ok {
			continue
		}
		if err := setDefault(output, target); err != nil {
			errs = append(errs, err)
			continue
		}
		*fixed, *was, *real = target.Name, def.Name, target.Name
	}
	return r, errors.Join(errs...)
}

func defaultOf(eps []Endpoint) (Endpoint, bool) {
	for _, e := range eps {
		if e.Default {
			return e, true
		}
	}
	return Endpoint{}, false
}

func realTarget(eps []Endpoint, prefer string) (Endpoint, bool) {
	for _, e := range eps {
		if prefer != "" && e.Name == prefer && !e.virtual() {
			return e, true
		}
	}
	for _, e := range eps {
		if !e.virtual() {
			return e, true
		}
	}
	return Endpoint{}, false
}
