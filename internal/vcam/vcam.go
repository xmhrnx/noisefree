// Package vcam publishes the processed video as a camera other apps can
// pick: "NoiseFree Camera" through v4l2loopback on Linux, and OBS Virtual
// Camera on Windows.
package vcam

import (
	"errors"
	"image"
)

// Status describes the virtual camera on this computer.
type Status struct {
	// Ready means Open will work.
	Ready bool
	// Name is what to pick as the camera in other apps.
	Name string
	// Help explains what's missing when not ready.
	Help string
	// InstallURL points to what to install, when something is missing.
	InstallURL string
	// CanEnable means Enable can make it ready (Linux: loads the kernel
	// module, asking for the administrator password).
	CanEnable bool
}

// Output receives frames.
type Output interface {
	Write(img *image.RGBA) error
	Close() error
}

// ErrUnsupported means this system has no virtual camera support.
var ErrUnsupported = errors.New("virtual cameras aren't supported on this system yet")

// Detect reports whether a virtual camera can be opened.
func Detect() Status { return detect() }

// Enable tries to make the virtual camera available.
func Enable() error { return enable() }

// Open starts a w x h virtual camera at fps frames per second.
func Open(w, h, fps int) (Output, error) { return open(w, h, fps) }

// BT.601 limited-range conversion, in 8.8 fixed point.
func yuv(r, g, b int32) (y, u, v uint8) {
	yy := (66*r + 129*g + 25*b + 128) >> 8
	uu := (-38*r - 74*g + 112*b + 128) >> 8
	vv := (112*r - 94*g - 18*b + 128) >> 8
	return uint8(yy + 16), uint8(uu + 128), uint8(vv + 128)
}

// toYUYV packs img (even width) as YUY2/YUYV into dst (w*h*2 bytes).
func toYUYV(dst []byte, img *image.RGBA) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	for y := 0; y < h; y++ {
		src := img.Pix[y*img.Stride:]
		row := dst[y*w*2:]
		for x := 0; x+1 < w; x += 2 {
			i := x * 4
			y0, u0, v0 := yuv(int32(src[i]), int32(src[i+1]), int32(src[i+2]))
			y1, u1, v1 := yuv(int32(src[i+4]), int32(src[i+5]), int32(src[i+6]))
			o := x * 2
			row[o], row[o+1], row[o+2], row[o+3] = y0, uint8((int(u0)+int(u1))/2), y1, uint8((int(v0)+int(v1))/2)
		}
	}
}

// toNV12 writes img (even size) as NV12: the Y plane, then interleaved
// U and V at half resolution.
func toNV12(dst []byte, img *image.RGBA) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	uv := dst[w*h:]
	for y := 0; y < h; y++ {
		src := img.Pix[y*img.Stride:]
		for x := 0; x < w; x++ {
			i := x * 4
			yy, u, v := yuv(int32(src[i]), int32(src[i+1]), int32(src[i+2]))
			dst[y*w+x] = yy
			if y%2 == 0 && x%2 == 0 {
				o := (y/2)*w + x
				uv[o], uv[o+1] = u, v
			}
		}
	}
}
