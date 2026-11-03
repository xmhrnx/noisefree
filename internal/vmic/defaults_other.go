//go:build !linux && !windows

package vmic

// endpoints isn't implemented here: macOS virtual drivers such as BlackHole
// don't make themselves the default device, so there's nothing to guard.
func endpoints(bool) ([]Endpoint, error) { return nil, ErrUnsupported }

func setDefault(bool, Endpoint) error { return ErrUnsupported }
