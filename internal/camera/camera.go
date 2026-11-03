// Package camera reads frames from webcams (V4L2 on Linux, DirectShow on
// Windows, AVFoundation on macOS) as RGBA images.
package camera

import (
	"errors"
	"fmt"
	"image"
	"image/draw"
	"math"
	"strings"
	"sync"

	"github.com/pion/mediadevices/pkg/driver"
	_ "github.com/pion/mediadevices/pkg/driver/camera" // registers the platform's cameras
	"github.com/pion/mediadevices/pkg/frame"
	"github.com/pion/mediadevices/pkg/io/video"
	"github.com/pion/mediadevices/pkg/prop"
)

// Device is a camera.
type Device struct {
	ID, Name string
}

// virtual reports cameras that are other apps' outputs, NoiseFree's own
// included: reading from them would loop NoiseFree's output back in.
func virtual(name string) bool {
	n := strings.ToLower(name)
	for _, v := range []string{"noisefree", "obs virtual", "v4l2 loopback", "dummy video device", "virtual camera"} {
		if strings.Contains(n, v) {
			return true
		}
	}
	return false
}

// List returns the real cameras.
func List() []Device {
	var out []Device
	for _, d := range driver.GetManager().Query(driver.FilterVideoRecorder()) {
		name := d.Info().Name
		if name == "" {
			name = d.Info().Label
		}
		if virtual(name) || virtual(d.Info().Label) {
			continue
		}
		out = append(out, Device{ID: d.ID(), Name: name})
	}
	return out
}

// Source delivers frames. Read blocks until the next frame and returns an
// image that stays valid until the following Read.
type Source interface {
	Read() (*image.RGBA, error)
	Close() error
}

type deviceSource struct {
	d      driver.Driver
	r      video.Reader
	rgba   *image.RGBA
	closed sync.Once
}

// Open starts camera id ("" is the first one) close to w x h at 30 fps.
func Open(id string, w, h int) (Source, error) {
	var d driver.Driver
	for _, c := range driver.GetManager().Query(driver.FilterVideoRecorder()) {
		if (id == "" && !virtual(c.Info().Name)) || c.ID() == id {
			d = c
			break
		}
	}
	if d == nil {
		return nil, errors.New("camera not found")
	}
	if err := d.Open(); err != nil {
		return nil, fmt.Errorf("open camera: %w", err)
	}
	media, ok := bestMode(d.Properties(), w, h)
	if !ok {
		d.Close()
		return nil, errors.New("the camera reports no usable video mode")
	}
	rec, ok := d.(driver.VideoRecorder)
	if !ok {
		d.Close()
		return nil, errors.New("not a video device")
	}
	r, err := rec.VideoRecord(media)
	if err != nil {
		d.Close()
		return nil, fmt.Errorf("start camera: %w", err)
	}
	return &deviceSource{d: d, r: r}, nil
}

// bestMode picks the mode closest to w x h, preferring 30 fps and
// formats that decode cheaply.
func bestMode(modes []prop.Media, w, h int) (prop.Media, bool) {
	cost := func(m prop.Media) float64 {
		c := math.Abs(math.Log(float64(m.Width*m.Height) / float64(w*h)))
		if m.FrameRate > 0 && m.FrameRate < 24 {
			c += 2
		}
		switch m.FrameFormat {
		case frame.FormatYUYV, frame.FormatYUY2, frame.FormatNV12, frame.FormatI420, frame.FormatUYVY:
		case frame.FormatMJPEG:
			c += 0.05 // decoding JPEG costs a little more, but it's often the only way to 30 fps
		default:
			c += 0.5
		}
		return c
	}
	best, found := prop.Media{}, false
	for _, m := range modes {
		if m.Width == 0 || m.Height == 0 {
			continue
		}
		if !found || cost(m) < cost(best) {
			best, found = m, true
		}
	}
	return best, found
}

func (s *deviceSource) Read() (*image.RGBA, error) {
	img, release, err := s.r.Read()
	if err != nil {
		return nil, err
	}
	defer release()
	b := img.Bounds()
	if s.rgba == nil || s.rgba.Rect.Dx() != b.Dx() || s.rgba.Rect.Dy() != b.Dy() {
		s.rgba = image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	}
	draw.Draw(s.rgba, s.rgba.Rect, img, b.Min, draw.Src) // fast paths for YCbCr and RGBA
	return s.rgba, nil
}

func (s *deviceSource) Close() error {
	var err error
	s.closed.Do(func() { err = s.d.Close() })
	return err
}
