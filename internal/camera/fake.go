package camera

import (
	"image"
	"image/draw"
	"math"
	"os"
	"time"

	_ "image/jpeg" // decodes the still
	_ "image/png"

	xdraw "golang.org/x/image/draw"
)

// fakeSource plays a still image as a 30 fps camera with a slow drift, so
// effects can be shown and tested without a webcam (NOISEFREE_FAKE_CAMERA).
type fakeSource struct {
	still *image.RGBA
	out   *image.RGBA
	start time.Time
	next  time.Time
}

// OpenFake opens the image at path as a w x h camera.
func OpenFake(path string, w, h int) (Source, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	// cover a slightly larger canvas, leaving room to drift
	still := image.NewRGBA(image.Rect(0, 0, w+w/10, h+h/10))
	sb := img.Bounds()
	scale := max(float64(still.Rect.Dx())/float64(sb.Dx()), float64(still.Rect.Dy())/float64(sb.Dy()))
	cw, ch := int(float64(still.Rect.Dx())/scale), int(float64(still.Rect.Dy())/scale)
	crop := image.Rect(sb.Min.X+(sb.Dx()-cw)/2, sb.Min.Y, 0, 0)
	crop.Max = crop.Min.Add(image.Pt(cw, ch))
	xdraw.ApproxBiLinear.Scale(still, still.Rect, img, crop, xdraw.Src, nil)
	return &fakeSource{still: still, out: image.NewRGBA(image.Rect(0, 0, w, h)), start: time.Now(), next: time.Now()}, nil
}

func (s *fakeSource) Read() (*image.RGBA, error) {
	if d := time.Until(s.next); d > 0 {
		time.Sleep(d)
	}
	s.next = s.next.Add(time.Second / 30)
	t := time.Since(s.start).Seconds()
	dx := int((math.Sin(t*0.6) + 1) / 2 * float64(s.still.Rect.Dx()-s.out.Rect.Dx()))
	dy := int((math.Sin(t*0.4) + 1) / 2 * float64(s.still.Rect.Dy()-s.out.Rect.Dy()))
	draw.Draw(s.out, s.out.Rect, s.still, image.Pt(dx, dy), draw.Src)
	return s.out, nil
}

func (s *fakeSource) Close() error { return nil }
