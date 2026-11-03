package vision

import (
	"image"
	"runtime"
	"sync"
)

// workers is how many goroutines split per-pixel work.
var workers = min(runtime.GOMAXPROCS(0), 8)

// parallelRows calls fn on disjoint row ranges covering [0, h).
func parallelRows(h int, fn func(y0, y1 int)) {
	n := min(workers, max(h/16, 1))
	if n == 1 {
		fn(0, h)
		return
	}
	var wg sync.WaitGroup
	step := (h + n - 1) / n
	for y := 0; y < h; y += step {
		wg.Add(1)
		go func(y0, y1 int) {
			defer wg.Done()
			fn(y0, y1)
		}(y, min(y+step, h))
	}
	wg.Wait()
}

// rect is an area of the source image in pixels, possibly reaching outside
// it (those parts read as black).
type rect struct{ x, y, w, h float32 }

// sampleRGB fills dst (dw x dh x 3, row-major RGB) from area r of src with
// bilinear sampling, mapping each 0-255 value v to v*scale+offset.
func sampleRGB(dst []float32, dw, dh int, src *image.RGBA, r rect, scale, offset float32) {
	sw, sh := src.Rect.Dx(), src.Rect.Dy()
	for y := 0; y < dh; y++ {
		fy := r.y + (float32(y)+0.5)*r.h/float32(dh) - 0.5
		for x := 0; x < dw; x++ {
			fx := r.x + (float32(x)+0.5)*r.w/float32(dw) - 0.5
			var c [3]float32
			if fx >= -0.5 && fy >= -0.5 && fx <= float32(sw)-0.5 && fy <= float32(sh)-0.5 {
				c = bilinear(src, fx, fy, sw, sh)
			}
			i := (y*dw + x) * 3
			dst[i] = c[0]*scale + offset
			dst[i+1] = c[1]*scale + offset
			dst[i+2] = c[2]*scale + offset
		}
	}
}

// bilinear returns the RGB at (fx, fy), clamped to the image.
func bilinear(src *image.RGBA, fx, fy float32, sw, sh int) [3]float32 {
	x0, y0 := int(fx), int(fy)
	if fx < 0 {
		x0 = -1
	}
	if fy < 0 {
		y0 = -1
	}
	tx, ty := fx-float32(x0), fy-float32(y0)
	x0c, x1c := clampi(x0, 0, sw-1), clampi(x0+1, 0, sw-1)
	y0c, y1c := clampi(y0, 0, sh-1), clampi(y0+1, 0, sh-1)
	p := src.Pix
	r0, r1 := y0c*src.Stride, y1c*src.Stride
	var c [3]float32
	for k := 0; k < 3; k++ {
		a := float32(p[r0+x0c*4+k])*(1-tx) + float32(p[r0+x1c*4+k])*tx
		b := float32(p[r1+x0c*4+k])*(1-tx) + float32(p[r1+x1c*4+k])*tx
		c[k] = a*(1-ty) + b*ty
	}
	return c
}

func clampi(v, lo, hi int) int { return min(max(v, lo), hi) }

func clampf(v, lo, hi float32) float32 { return min(max(v, lo), hi) }

func clamp8(v float32) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(v + 0.5)
}

// grid maps output pixels to a smaller field (a mask, a blurred image) for
// bilinear sampling: index and weight per column and per row, computed once
// per size.
type grid struct {
	w, h, fw, fh int
	x0, x1       []int32
	tx           []float32
	y0, y1       []int32
	ty           []float32
}

func (g *grid) prepare(w, h, fw, fh int) {
	if g.w == w && g.h == h && g.fw == fw && g.fh == fh {
		return
	}
	*g = grid{w: w, h: h, fw: fw, fh: fh}
	axis := func(n, fn int) (a, b []int32, t []float32) {
		a, b, t = make([]int32, n), make([]int32, n), make([]float32, n)
		for i := 0; i < n; i++ {
			f := (float32(i)+0.5)*float32(fn)/float32(n) - 0.5
			f = clampf(f, 0, float32(fn-1))
			i0 := int(f)
			a[i], b[i], t[i] = int32(i0), int32(min(i0+1, fn-1)), f-float32(i0)
		}
		return
	}
	g.x0, g.x1, g.tx = axis(w, fw)
	g.y0, g.y1, g.ty = axis(h, fh)
}

// row interpolates field f vertically for output row y into dst (fw long),
// so the pixels of that row need only a horizontal lerp (atRow).
func (g *grid) row(f []float32, y int, dst []float32) {
	r0, r1, t := int(g.y0[y])*g.fw, int(g.y1[y])*g.fw, g.ty[y]
	for i := range dst[:g.fw] {
		dst[i] = f[r0+i] + (f[r1+i]-f[r0+i])*t
	}
}

// atRow samples a row prepared by row at output column x.
func (g *grid) atRow(row []float32, x int) float32 {
	a := row[g.x0[x]]
	return a + (row[g.x1[x]]-a)*g.tx[x]
}

// at samples field f (fw x fh) at output pixel (x, y).
func (g *grid) at(f []float32, x, y int) float32 {
	r0, r1 := int(g.y0[y])*g.fw, int(g.y1[y])*g.fw
	a := f[r0+int(g.x0[x])]*(1-g.tx[x]) + f[r0+int(g.x1[x])]*g.tx[x]
	b := f[r1+int(g.x0[x])]*(1-g.tx[x]) + f[r1+int(g.x1[x])]*g.tx[x]
	return a*(1-g.ty[y]) + b*g.ty[y]
}
