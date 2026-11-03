package vision

import (
	"image"
	"image/color"
	"image/draw"

	xdraw "golang.org/x/image/draw"
)

// BackgroundMode is what happens behind the person.
type BackgroundMode int

// Background modes.
const (
	BackgroundOff BackgroundMode = iota
	BackgroundBlur
	BackgroundReplace
	BackgroundRemove
)

const blurScale = 4 // the background is blurred at a quarter of the size

// background composites the person over a blurred, replaced or removed
// background.
type background struct {
	maskGrid, smallGrid grid
	sw, sh              int
	planes              [4][]float32 // r, g, b (background-weighted), weight
	replaced            *image.RGBA
	replacedSrc         image.Image
}

// blurred computes the mask-aware blurred background of frame at a quarter
// size: the person's pixels are weighted out, so they don't bleed into the
// blur as a halo.
func (b *background) blurred(frame *image.RGBA, mask []float32, strength float32) {
	w, h := frame.Rect.Dx(), frame.Rect.Dy()
	sw, sh := max(w/blurScale, 1), max(h/blurScale, 1)
	if b.sw != sw || b.sh != sh {
		b.sw, b.sh = sw, sh
		for i := range b.planes {
			b.planes[i] = make([]float32, sw*sh)
		}
	}
	b.smallGrid.prepare(sw, sh, segW, segH)
	r, g, bl, wt := b.planes[0], b.planes[1], b.planes[2], b.planes[3]
	parallelRows(sh, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			for x := 0; x < sw; x++ {
				var sr, sg, sb float32
				for dy := 0; dy < blurScale; dy++ {
					o := (y*blurScale+dy)*frame.Stride + x*blurScale*4
					for dx := 0; dx < blurScale; dx++ {
						sr += float32(frame.Pix[o])
						sg += float32(frame.Pix[o+1])
						sb += float32(frame.Pix[o+2])
						o += 4
					}
				}
				k := 1 - b.smallGrid.at(mask, x, y)
				k = k*k + 0.002 // never exactly zero: a fully covered area still averages
				i := y*sw + x
				const n = blurScale * blurScale
				r[i], g[i], bl[i], wt[i] = sr/n*k, sg/n*k, sb/n*k, k
			}
		}
	})
	radius := 2 + int(strength*14)
	for pass := 0; pass < 3; pass++ { // three box passes ~ a Gaussian
		for _, p := range [][]float32{r, g, bl, wt} {
			boxBlur(p, sw, sh, radius)
		}
	}
	for i := range wt {
		inv := 1 / wt[i]
		r[i], g[i], bl[i] = r[i]*inv, g[i]*inv, bl[i]*inv
	}
}

// boxBlur blurs plane p (w x h) in place with a box of the given radius,
// horizontally then vertically, using running sums.
func boxBlur(p []float32, w, h, radius int) {
	parallelRows(h, func(y0, y1 int) {
		line := make([]float32, w)
		for y := y0; y < y1; y++ {
			row := p[y*w : (y+1)*w]
			copy(line, row)
			runningBox(line, row, radius)
		}
	})
	parallelRows(w, func(x0, x1 int) {
		line, out := make([]float32, h), make([]float32, h)
		for x := x0; x < x1; x++ {
			for y := 0; y < h; y++ {
				line[y] = p[y*w+x]
			}
			runningBox(line, out, radius)
			for y := 0; y < h; y++ {
				p[y*w+x] = out[y]
			}
		}
	})
}

// runningBox writes the box average of in (edges clamped) to out.
func runningBox(in, out []float32, r int) {
	n := len(in)
	var sum float32
	for i := -r; i <= r; i++ {
		sum += in[clampi(i, 0, n-1)]
	}
	inv := 1 / float32(2*r+1)
	for i := 0; i < n; i++ {
		out[i] = sum * inv
		sum += in[clampi(i+r+1, 0, n-1)] - in[clampi(i-r, 0, n-1)]
	}
}

// replacement returns bg scaled to cover a w x h frame, cached.
func (b *background) replacement(bg image.Image, w, h int) *image.RGBA {
	if b.replaced != nil && b.replacedSrc == bg && b.replaced.Rect.Dx() == w && b.replaced.Rect.Dy() == h {
		return b.replaced
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	if bg == nil {
		draw.Draw(dst, dst.Rect, image.NewUniform(color.RGBA{40, 40, 44, 255}), image.Point{}, draw.Src)
	} else {
		sb := bg.Bounds()
		// cover: scale to fill, crop the overflow evenly
		scale := max(float64(w)/float64(sb.Dx()), float64(h)/float64(sb.Dy()))
		cw, ch := int(float64(w)/scale), int(float64(h)/scale)
		crop := image.Rect(sb.Min.X+(sb.Dx()-cw)/2, sb.Min.Y+(sb.Dy()-ch)/2, 0, 0)
		crop.Max = crop.Min.Add(image.Pt(cw, ch))
		xdraw.ApproxBiLinear.Scale(dst, dst.Rect, bg, crop, xdraw.Src, nil)
	}
	b.replaced, b.replacedSrc = dst, bg
	return dst
}

// composite writes the person from fg over the background into out.
func (b *background) composite(out, fg *image.RGBA, mask []float32, mode BackgroundMode, bg *image.RGBA, solid color.RGBA) {
	w, h := fg.Rect.Dx(), fg.Rect.Dy()
	b.maskGrid.prepare(w, h, segW, segH)
	var blurGrid grid
	if mode == BackgroundBlur {
		blurGrid.prepare(w, h, b.sw, b.sh)
	}
	planes := b.planes
	parallelRows(h, func(y0, y1 int) {
		mrow := make([]float32, segW)
		var brow [3][]float32
		if mode == BackgroundBlur {
			for c := range brow {
				brow[c] = make([]float32, b.sw)
			}
		}
		for y := y0; y < y1; y++ {
			b.maskGrid.row(mask, y, mrow)
			if mode == BackgroundBlur {
				for c := range brow {
					blurGrid.row(planes[c], y, brow[c])
				}
			}
			o := y * fg.Stride
			for x := 0; x < w; x++ {
				i := o + x*4
				m := b.maskGrid.atRow(mrow, x)
				if m > 0.996 { // clearly the person
					out.Pix[i], out.Pix[i+1], out.Pix[i+2], out.Pix[i+3] = fg.Pix[i], fg.Pix[i+1], fg.Pix[i+2], 255
					continue
				}
				var br, bgc, bb float32
				switch mode {
				case BackgroundBlur:
					br, bgc, bb = blurGrid.atRow(brow[0], x), blurGrid.atRow(brow[1], x), blurGrid.atRow(brow[2], x)
				case BackgroundReplace:
					q := y*bg.Stride + x*4
					br, bgc, bb = float32(bg.Pix[q]), float32(bg.Pix[q+1]), float32(bg.Pix[q+2])
				default:
					br, bgc, bb = float32(solid.R), float32(solid.G), float32(solid.B)
				}
				out.Pix[i] = clamp8(br + (float32(fg.Pix[i])-br)*m)
				out.Pix[i+1] = clamp8(bgc + (float32(fg.Pix[i+1])-bgc)*m)
				out.Pix[i+2] = clamp8(bb + (float32(fg.Pix[i+2])-bb)*m)
				out.Pix[i+3] = 255
			}
		}
	})
}
