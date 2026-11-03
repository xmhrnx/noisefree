package ui

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"github.com/xmhrnx/giode"
)

// previewBackdrop shows behind letterboxed or missing video.
var previewBackdrop = color.NRGBA{R: 12, G: 12, B: 14, A: 255}

// videoView draws the latest camera preview, fitted into a 16:9 box with
// rounded corners. It uploads each new frame directly: Giode's Image caches
// textures by image, which suits pictures but not live video.
func videoView(frame *image.RGBA, radius int) giode.Element {
	return giode.Raw(func(gtx layout.Context) layout.Dimensions {
		full := gtx.Constraints.Max.X
		w := full
		h := w * 9 / 16
		if h > gtx.Constraints.Max.Y && gtx.Constraints.Max.Y > 0 {
			h = gtx.Constraints.Max.Y
			w = h * 16 / 9
		}
		size := image.Pt(w, h)
		// centre the picture in the available width
		defer op.Offset(image.Pt((full-w)/2, 0)).Push(gtx.Ops).Pop()
		r := gtx.Dp(unit.Dp(radius))
		defer clip.UniformRRect(image.Rectangle{Max: size}, r).Push(gtx.Ops).Pop()
		paint.Fill(gtx.Ops, previewBackdrop)
		dims := layout.Dimensions{Size: image.Pt(full, h)}
		if frame == nil {
			return dims
		}
		fw, fh := float32(frame.Rect.Dx()), float32(frame.Rect.Dy())
		scale := min(float32(w)/fw, float32(h)/fh)
		off := f32.Pt((float32(w)-fw*scale)/2, (float32(h)-fh*scale)/2)
		img := paint.NewImageOp(frame)
		img.Filter = paint.FilterLinear
		img.Add(gtx.Ops)
		defer op.Affine(f32.Affine2D{}.Scale(f32.Point{}, f32.Pt(scale, scale)).Offset(off)).Push(gtx.Ops).Pop()
		paint.PaintOp{}.Add(gtx.Ops)
		return dims
	})
}
