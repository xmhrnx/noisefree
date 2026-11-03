package vision

import (
	"image"
	"math"
)

const (
	detSize      = 128
	numAnchors   = 896
	lmSize       = 256
	NumLandmarks = 478
)

// Landmark indices (MediaPipe face mesh) used by the effects.
const (
	lmRightEyeOuter = 33
	lmRightEyeInner = 133
	lmRightEyeTop   = 159
	lmRightEyeLow   = 145
	lmLeftEyeInner  = 362
	lmLeftEyeOuter  = 263
	lmLeftEyeTop    = 386
	lmLeftEyeLow    = 374
	lmRightIris     = 468
	lmLeftIris      = 473
	lmNoseTip       = 1
)

// Point is a landmark in frame pixels.
type Point struct{ X, Y float32 }

// Face is one tracked face.
type Face struct {
	Points [NumLandmarks]Point
	// Box is the landmarks' bounding box in frame pixels.
	Box image.Rectangle
}

var anchors = func() [numAnchors][2]float32 {
	var a [numAnchors][2]float32
	i := 0
	for _, l := range []struct{ stride, n int }{{8, 2}, {16, 6}} {
		g := detSize / l.stride
		for y := 0; y < g; y++ {
			for x := 0; x < g; x++ {
				for k := 0; k < l.n; k++ {
					a[i] = [2]float32{(float32(x) + 0.5) / float32(g), (float32(y) + 0.5) / float32(g)}
					i++
				}
			}
		}
	}
	return a
}()

// faceTracker finds the face once with the detector, then follows it with
// the landmark model alone, which is both cheaper and steadier.
type faceTracker struct {
	det, lm *model
	face    Face
	have    bool
	smooth  [NumLandmarks]Point
}

// run updates the face from frame; ok is false when there's no face.
func (t *faceTracker) run(frame *image.RGBA) (f *Face, ok bool, err error) {
	var roi rect
	if t.have {
		b := t.face.Box
		size := float32(max(b.Dx(), b.Dy())) * 1.55
		cx, cy := float32(b.Min.X+b.Max.X)/2, float32(b.Min.Y+b.Max.Y)/2
		roi = rect{cx - size/2, cy - size/2, size, size}
	} else {
		var found bool
		if roi, found, err = t.detect(frame); err != nil || !found {
			return nil, false, err
		}
	}
	in := t.lm.in[0].GetData()
	sampleRGB(in, lmSize, lmSize, frame, roi, 1.0/255, 0)
	if err := t.lm.run(); err != nil {
		return nil, false, err
	}
	if presence := t.lm.out[1].GetData()[0]; presence < 0 { // logit: below 0.5 probability
		t.have = false
		return nil, false, nil
	}
	pts := t.lm.out[0].GetData()
	minX, minY := float32(math.MaxFloat32), float32(math.MaxFloat32)
	maxX, maxY := float32(-math.MaxFloat32), float32(-math.MaxFloat32)
	for i := 0; i < NumLandmarks; i++ {
		p := Point{roi.x + pts[i*3]/lmSize*roi.w, roi.y + pts[i*3+1]/lmSize*roi.h}
		if t.have { // damp jitter, follow real motion
			d := float32(math.Hypot(float64(p.X-t.smooth[i].X), float64(p.Y-t.smooth[i].Y)))
			a := clampf(d/(roi.w*0.02), 0.25, 1)
			p = Point{t.smooth[i].X + (p.X-t.smooth[i].X)*a, t.smooth[i].Y + (p.Y-t.smooth[i].Y)*a}
		}
		t.smooth[i] = p
		t.face.Points[i] = p
		if i < 468 {
			minX, minY, maxX, maxY = min(minX, p.X), min(minY, p.Y), max(maxX, p.X), max(maxY, p.Y)
		}
	}
	t.face.Box = image.Rect(int(minX), int(minY), int(maxX), int(maxY))
	t.have = true
	return &t.face, true, nil
}

// detect looks for a face, first in the centre square of the frame, where
// people sit in front of a webcam (and appear larger to the detector's small
// input), then in the whole frame. It returns the square region the
// landmark model should look at.
func (t *faceTracker) detect(frame *image.RGBA) (rect, bool, error) {
	w, h := float32(frame.Rect.Dx()), float32(frame.Rect.Dy())
	small := min(w, h)
	big := max(w, h)
	for _, r := range []rect{{(w - small) / 2, (h - small) / 2, small, small}, {(w - big) / 2, (h - big) / 2, big, big}} {
		found, err := t.detectIn(frame, r)
		if err != nil || found.w > 0 {
			return found, found.w > 0, err
		}
	}
	return rect{}, false, nil
}

// detectIn runs BlazeFace on square area r of frame.
func (t *faceTracker) detectIn(frame *image.RGBA, r rect) (rect, error) {
	sampleRGB(t.det.in[0].GetData(), detSize, detSize, frame, r, 1.0/127.5, -1)
	if err := t.det.run(); err != nil {
		return rect{}, err
	}
	reg, cls := t.det.out[0].GetData(), t.det.out[1].GetData()
	best, score := -1, float32(0)
	for i, c := range cls {
		if c > score {
			best, score = i, c
		}
	}
	if best < 0 || score < 0.5 { // logit 0.5 ~ 62%
		return rect{}, nil
	}
	b := reg[best*16:]
	cx := (b[0]/detSize+anchors[best][0])*r.w + r.x
	cy := (b[1]/detSize+anchors[best][1])*r.h + r.y
	size := b[2] / detSize * r.w * 1.5 * 1.25
	return rect{cx - size/2, cy - size/2, size, size}, nil
}
