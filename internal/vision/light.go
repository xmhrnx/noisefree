package vision

import "image"

// lighting evens out exposure and white balance and adds a soft key light
// on the face, adapting slowly so the picture never pumps.
type lighting struct {
	gain   float32          // exposure gain, smoothed
	wb     [3]float32       // white balance gains, smoothed
	wbLUT  [3][256]uint16   // channel value after white balance, x256
	tone   [256 * 4]float32 // brightness factor by luma (luma x4 for precision)
	primed bool
}

// update measures frame (weighted to the face when there is one) and
// rebuilds the colour lookup tables.
func (l *lighting) update(frame *image.RGBA, face *Face, strength float32) {
	b := frame.Rect
	w, h := b.Dx(), b.Dy()
	area := image.Rect(w/4, h/6, w*3/4, h*5/6) // centre, where a person usually is
	if face != nil {
		area = face.Box.Intersect(image.Rect(0, 0, w, h))
	}
	var luma, n float64
	var sums [3]float64
	var cnt float64
	for y := 0; y < h; y += 6 {
		for x := 0; x < w; x += 6 {
			i := y*frame.Stride + x*4
			r, g, bl := float64(frame.Pix[i]), float64(frame.Pix[i+1]), float64(frame.Pix[i+2])
			if max(r, g, bl) < 245 && max(r, g, bl) > 10 { // ignore clipped and black pixels
				sums[0], sums[1], sums[2], cnt = sums[0]+r, sums[1]+g, sums[2]+bl, cnt+1
			}
			if image.Pt(x, y).In(area) {
				luma += 0.299*r + 0.587*g + 0.114*bl
				n++
			}
		}
	}
	gain, wb := float32(1), [3]float32{1, 1, 1}
	if n > 0 {
		mean := float32(luma / n / 255)
		// lift dark faces towards a pleasant mid-tone, pull hot ones down
		gain = clampf(0.47/max(mean, 0.02), 0.75, 2.6)
	}
	if cnt > 0 { // grey world, applied partially: keeps the room's mood
		avg := (sums[0] + sums[1] + sums[2]) / 3
		for k := range wb {
			wb[k] = clampf(float32(avg/max(sums[k], 1)), 0.75, 1.35)
		}
	}
	gain = 1 + (gain-1)*strength
	for k := range wb {
		wb[k] = 1 + (wb[k]-1)*strength*0.7
	}
	if !l.primed {
		l.gain, l.wb, l.primed = gain, wb, true
	}
	const follow = 0.08 // about a second to settle at 30 fps
	l.gain += (gain - l.gain) * follow
	for k := range wb {
		l.wb[k] += (wb[k] - l.wb[k]) * follow
	}
	for k := 0; k < 3; k++ {
		for v := 0; v < 256; v++ {
			l.wbLUT[k][v] = uint16(clampf(float32(v)*l.wb[k], 0, 255) * 256)
		}
	}
	// Exposure and contrast act on brightness only; colours are scaled by
	// the same factor, so a lifted face keeps its saturation instead of
	// turning grey.
	for i := range l.tone {
		y := (float32(i) + 0.5) / float32(len(l.tone))
		x := y * l.gain / (1 + (l.gain-1)*y) // brightens shadows, rolls off before clipping
		x += (x*x*(3-2*x) - x) * 0.55 * strength
		l.tone[i] = x / y
	}
}

// apply maps frame through the white balance and tone curve and adds the
// key light on the face.
func (l *lighting) apply(frame *image.RGBA, face *Face, strength float32) {
	w, h := frame.Rect.Dx(), frame.Rect.Dy()
	var cx, cy, rad2, lift float32
	if face != nil {
		b := face.Box
		cx, cy = float32(b.Min.X+b.Max.X)/2, float32(b.Min.Y+b.Max.Y)/2-float32(b.Dy())*0.05
		r := float32(max(b.Dx(), b.Dy())) * 0.85
		rad2, lift = r*r, 0.22*strength
	}
	parallelRows(h, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			o := y * frame.Stride
			for x := 0; x < w; x++ {
				i := o + x*4
				r := float32(l.wbLUT[0][frame.Pix[i]]) / 256
				g := float32(l.wbLUT[1][frame.Pix[i+1]]) / 256
				b := float32(l.wbLUT[2][frame.Pix[i+2]]) / 256
				luma := 0.299*r + 0.587*g + 0.114*b
				f := l.tone[min(int(luma*4), len(l.tone)-1)]
				r, g, b = r*f, g*f, b*f
				if lift > 0 {
					dx, dy := float32(x)-cx, float32(y)-cy
					if d := (dx*dx + dy*dy) / rad2; d < 1 {
						k := lift * (1 - d) * (1 - d)
						r, g, b = r+(255-r)*k, g+(255-g)*k, b+(255-b)*k
					}
				}
				frame.Pix[i], frame.Pix[i+1], frame.Pix[i+2] = clamp8(r), clamp8(g), clamp8(b)
			}
		}
	})
}

// denoise blends each pixel with the previous frame where little moved,
// which removes most webcam grain without smearing motion.
func denoise(frame, prev *image.RGBA, strength float32) {
	thr := int32(4 + 14*strength)
	keep := int32((0.35 + 0.35*strength) * 256) // share of the previous frame kept, /256
	w, h := frame.Rect.Dx(), frame.Rect.Dy()
	parallelRows(h, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			cur := frame.Pix[y*frame.Stride : y*frame.Stride+w*4]
			old := prev.Pix[y*prev.Stride : y*prev.Stride+w*4]
			for i := 0; i < len(cur); i += 4 {
				r, g, b := int32(cur[i]), int32(cur[i+1]), int32(cur[i+2])
				pr, pg, pb := int32(old[i]), int32(old[i+1]), int32(old[i+2])
				d := max(iabs(r-pr), iabs(g-pg), iabs(b-pb))
				if d < thr {
					k := keep * (thr - d) / thr
					cur[i] = uint8((r*(256-k) + pr*k) >> 8)
					cur[i+1] = uint8((g*(256-k) + pg*k) >> 8)
					cur[i+2] = uint8((b*(256-k) + pb*k) >> 8)
				}
				old[i], old[i+1], old[i+2] = cur[i], cur[i+1], cur[i+2]
			}
		}
	})
}

func iabs(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

// sharpen writes an unsharp-masked copy of src to dst: detail lost to webcam
// compression and denoising comes back.
func sharpen(dst, src *image.RGBA, amount float32) {
	w, h := src.Rect.Dx(), src.Rect.Dy()
	s := src.Stride
	amt := int32(amount * 64) // applied to (4v - neighbours), /256
	parallelRows(h, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			o := y * s
			if y == 0 || y == h-1 {
				copy(dst.Pix[o:o+w*4], src.Pix[o:o+w*4])
				continue
			}
			copy(dst.Pix[o:o+4], src.Pix[o:o+4])
			copy(dst.Pix[o+(w-1)*4:o+w*4], src.Pix[o+(w-1)*4:o+w*4])
			p := src.Pix
			for i := o + 4; i < o+(w-1)*4; i += 4 {
				for c := i; c < i+3; c++ {
					v := int32(p[c])
					lap := 4*v - int32(p[c-4]) - int32(p[c+4]) - int32(p[c-s]) - int32(p[c+s])
					n := v + (lap*amt)>>8
					if n < 0 {
						n = 0
					} else if n > 255 {
						n = 255
					}
					dst.Pix[c] = uint8(n)
				}
				dst.Pix[i+3] = 255
			}
		}
	})
}
