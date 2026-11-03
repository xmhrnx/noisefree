//go:build !linux && !windows

package vcam

func detect() Status {
	return Status{Help: "A virtual camera isn't available on macOS yet: it needs a signed camera extension. The preview shows the effects; screen-share the NoiseFree window as a stopgap."}
}

func enable() error { return ErrUnsupported }

func open(int, int, int) (Output, error) { return nil, ErrUnsupported }
