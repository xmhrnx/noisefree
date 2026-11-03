//go:build !linux && !windows && !darwin

package vmic

func setup([]string) Status {
	return Status{Help: "Virtual microphones aren't supported on this system; choose an output device instead."}
}

func teardown() {}
