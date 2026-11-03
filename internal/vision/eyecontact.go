package vision

import (
	"image"
	"math"
)

// eyeContact redirects the eyes towards the camera: when you read the
// screen, your eyes sit lower (and often to the side) than when you look
// into the lens. Each iris is moved back to the middle of the eye opening
// with a smooth local warp that fades out at the eye's edge, and the effect
// stands down during blinks and when the head turns away.
//
// It's a geometric correction, not a generative one: it can't invent the
// part of the iris an eyelid covers, so the shift is capped to look natural.
type eyeContact struct {
	src   []uint8 // copy of the eye region being warped
	shift [2]Point
}

type eye struct{ outer, inner, top, low, iris int }

var eyes = [2]eye{
	{lmRightEyeOuter, lmRightEyeInner, lmRightEyeTop, lmRightEyeLow, lmRightIris},
	{lmLeftEyeOuter, lmLeftEyeInner, lmLeftEyeTop, lmLeftEyeLow, lmLeftIris},
}

func dist(a, b Point) float32 { return float32(math.Hypot(float64(a.X-b.X), float64(a.Y-b.Y))) }

// apply warps both eyes of face in frame.
func (e *eyeContact) apply(frame *image.RGBA, f *Face, strength float32) {
	p := f.Points
	// stand down when the head is turned: the nose drifts off the eyes' midpoint
	mid := Point{(p[lmRightEyeInner].X + p[lmLeftEyeInner].X) / 2, (p[lmRightEyeInner].Y + p[lmLeftEyeInner].Y) / 2}
	inter := dist(p[lmRightEyeOuter], p[lmLeftEyeOuter])
	if inter < 12 {
		return // face too small to correct
	}
	yaw := abs32(p[lmNoseTip].X-mid.X) / inter
	turn := clampf(1-(yaw-0.12)/0.15, 0, 1)
	for k, ey := range eyes {
		a, b := p[ey.outer], p[ey.inner]
		width := dist(a, b)
		gap := dist(p[ey.top], p[ey.low])
		open := clampf((gap/width-0.14)/0.08, 0, 1) // 0 during a blink
		centre := Point{(a.X + b.X) / 2, (p[ey.top].Y + p[ey.low].Y) / 2}
		shape := eyeShape(p, ey)
		iris := measureIris(frame, shape)
		target := Point{centre.X, centre.Y - 0.04*width}
		d := Point{(target.X - iris.X) * strength, (target.Y - iris.Y) * strength}
		if l := float32(math.Hypot(float64(d.X), float64(d.Y))); l > 0.25*width {
			d.X, d.Y = d.X*0.25*width/l, d.Y*0.25*width/l
		}
		w := open * turn
		d = Point{d.X * w, d.Y * w}
		// ease the shift in and out over a few frames
		e.shift[k] = Point{e.shift[k].X + (d.X-e.shift[k].X)*0.5, e.shift[k].Y + (d.Y-e.shift[k].Y)*0.5}
		e.warp(frame, shape, e.shift[k])
	}
}

// eyeOpening is one eye's outline: the corners, the eyelid curves between
// them and the iris.
type eyeOpening struct {
	left, right Point // corners, left.X < right.X
	top, low    Point // middle of the upper and lower lid
	iris        Point
	width       float32
}

func eyeShape(p [NumLandmarks]Point, e eye) eyeOpening {
	l, r := p[e.outer], p[e.inner]
	if l.X > r.X {
		l, r = r, l
	}
	return eyeOpening{left: l, right: r, top: p[e.top], low: p[e.low], iris: p[e.iris], width: dist(l, r)}
}

// lid returns the y of the parabola through the two corners and mid at x.
func (o eyeOpening) lid(mid Point, x float32) float32 {
	x0, y0, x1, y1, x2, y2 := o.left.X, o.left.Y, mid.X, mid.Y, o.right.X, o.right.Y
	return y0*(x-x1)*(x-x2)/((x0-x1)*(x0-x2)) + y1*(x-x0)*(x-x2)/((x1-x0)*(x1-x2)) + y2*(x-x0)*(x-x1)/((x2-x0)*(x2-x1))
}

// warp moves what's inside the eye opening by d, most at the iris and
// fading towards the corners. The lids stay put, so the iris slides under
// the upper lid and the white shows below it, as when looking up at a camera.
func (e *eyeContact) warp(frame *image.RGBA, o eyeOpening, d Point) {
	if abs32(d.X) < 0.15 && abs32(d.Y) < 0.15 {
		return
	}
	if o.top.X <= o.left.X || o.top.X >= o.right.X || o.low.X <= o.left.X || o.low.X >= o.right.X {
		return // degenerate outline (profile view)
	}
	feather := o.width * 0.05
	minY := min(o.left.Y, o.right.Y, o.top.Y) - o.width*0.15
	maxY := max(o.left.Y, o.right.Y, o.low.Y) + o.width*0.15
	bounds := image.Rect(int(o.left.X)-1, int(minY)-1, int(o.right.X)+2, int(maxY)+2).Intersect(frame.Rect)
	if bounds.Empty() {
		return
	}
	bw, bh := bounds.Dx(), bounds.Dy()
	if cap(e.src) < bw*bh*4 {
		e.src = make([]uint8, bw*bh*4)
	}
	src := e.src[:bw*bh*4]
	for y := 0; y < bh; y++ {
		off := (bounds.Min.Y+y)*frame.Stride + bounds.Min.X*4
		copy(src[y*bw*4:(y+1)*bw*4], frame.Pix[off:off+bw*4])
	}
	span := o.right.X - o.left.X
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		fx := float32(x)
		// along the eye: strongest under the iris, nothing at the corners
		u := (fx - o.left.X) / span
		if u <= 0 || u >= 1 {
			continue
		}
		along := smoothstep(0, 0.35, min(u, 1-u)) * clampf(1.3-abs32(fx-o.iris.X)/(o.width*0.45), 0, 1)
		upper, lower := o.lid(o.top, fx), o.lid(o.low, fx)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			fy := float32(y)
			inside := smoothstep(-feather, feather, fy-upper) * smoothstep(-feather, feather, lower-fy)
			wgt := along * inside
			if wgt <= 0.002 {
				continue
			}
			sx := fx - d.X*wgt - float32(bounds.Min.X)
			sy := fy - d.Y*wgt - float32(bounds.Min.Y)
			c := bilinearBuf(src, bw, bh, sx, sy)
			i := y*frame.Stride + x*4
			frame.Pix[i], frame.Pix[i+1], frame.Pix[i+2] = clamp8(c[0]), clamp8(c[1]), clamp8(c[2])
		}
	}
}

// measureIris finds the iris from the pixels: the centroid of the darkest
// part of the eye opening, weighted towards the landmark model's estimate.
// The model's iris points lean towards the middle of the eye, so on their
// own they under-report how far someone is looking down.
func measureIris(frame *image.RGBA, o eyeOpening) Point {
	var hist [256]int
	var n int
	x0, x1 := int(o.left.X+o.width*0.12), int(o.right.X-o.width*0.12)
	type px struct {
		x, y int
		l    uint8
	}
	var pts [512]px
	k := 0
	for x := x0; x < x1; x++ {
		y0, y1 := int(o.lid(o.top, float32(x)))+1, int(o.lid(o.low, float32(x)))
		for y := y0; y < y1; y++ {
			if !image.Pt(x, y).In(frame.Rect) {
				continue
			}
			i := frame.PixOffset(x, y)
			l := uint8((299*int(frame.Pix[i]) + 587*int(frame.Pix[i+1]) + 114*int(frame.Pix[i+2])) / 1000)
			hist[l]++
			n++
			if k < len(pts) {
				pts[k] = px{x, y, l}
				k++
			}
		}
	}
	if n < 12 || k < n { // too small to measure, or too big to sample cheaply
		return o.iris
	}
	thr, acc := 0, 0
	for ; thr < 255 && acc < n/4; thr++ {
		acc += hist[thr]
	}
	sigma := o.width * 0.3
	var sx, sy, sw float32
	for _, q := range pts[:k] {
		if int(q.l) > thr {
			continue
		}
		dx, dy := float32(q.x)-o.iris.X, float32(q.y)-o.iris.Y
		w := float32(math.Exp(float64(-(dx*dx + dy*dy) / (2 * sigma * sigma))))
		sx, sy, sw = sx+float32(q.x)*w, sy+float32(q.y)*w, sw+w
	}
	if sw == 0 {
		return o.iris
	}
	return Point{sx / sw, sy / sw}
}

func bilinearBuf(p []uint8, w, h int, fx, fy float32) [3]float32 {
	fx, fy = clampf(fx, 0, float32(w-1)), clampf(fy, 0, float32(h-1))
	x0, y0 := int(fx), int(fy)
	x1, y1 := min(x0+1, w-1), min(y0+1, h-1)
	tx, ty := fx-float32(x0), fy-float32(y0)
	var c [3]float32
	for k := 0; k < 3; k++ {
		a := float32(p[(y0*w+x0)*4+k])*(1-tx) + float32(p[(y0*w+x1)*4+k])*tx
		b := float32(p[(y1*w+x0)*4+k])*(1-tx) + float32(p[(y1*w+x1)*4+k])*tx
		c[k] = a*(1-ty) + b*ty
	}
	return c
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
