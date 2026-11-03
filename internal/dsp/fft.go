// Package dsp holds the allocation-free signal processing blocks the audio
// pipeline runs on its real-time thread.
package dsp

import (
	"math"
	"math/bits"
)

// FFT is a precomputed radix-2 complex FFT of a fixed power-of-two size.
type FFT struct {
	n      int
	rev    []int
	cos    []float32
	sin    []float32
	re, im []float32
}

// NewFFT prepares an FFT of size n, which must be a power of two.
func NewFFT(n int) *FFT {
	if n < 2 || n&(n-1) != 0 {
		panic("dsp: FFT size must be a power of two")
	}
	f := &FFT{n: n, rev: make([]int, n), cos: make([]float32, n/2), sin: make([]float32, n/2),
		re: make([]float32, n), im: make([]float32, n)}
	shift := 64 - bits.TrailingZeros(uint(n))
	for i := range f.rev {
		f.rev[i] = int(bits.Reverse64(uint64(i)) >> shift)
	}
	for i := range f.cos {
		a := -2 * math.Pi * float64(i) / float64(n)
		f.cos[i], f.sin[i] = float32(math.Cos(a)), float32(math.Sin(a))
	}
	return f
}

// Size returns the transform length.
func (f *FFT) Size() int { return f.n }

// Forward transforms the real signal x (len n) and writes n/2+1 bins to re, im.
func (f *FFT) Forward(x, re, im []float32) {
	for i, r := range f.rev {
		f.re[r], f.im[r] = x[i], 0
	}
	f.transform(false)
	copy(re, f.re[:f.n/2+1])
	copy(im, f.im[:f.n/2+1])
}

// Inverse turns n/2+1 bins back into n real samples, scaled by 1/n.
func (f *FFT) Inverse(re, im, x []float32) {
	h := f.n / 2
	for k := 0; k <= h; k++ {
		f.re[f.rev[k]], f.im[f.rev[k]] = re[k], im[k]
	}
	for k := h + 1; k < f.n; k++ {
		f.re[f.rev[k]], f.im[f.rev[k]] = re[f.n-k], -im[f.n-k]
	}
	f.transform(true)
	scale := 1 / float32(f.n)
	for i := range x[:f.n] {
		x[i] = f.re[i] * scale
	}
}

func (f *FFT) transform(inverse bool) {
	re, im := f.re, f.im
	for size := 2; size <= f.n; size <<= 1 {
		half, step := size/2, f.n/size
		for start := 0; start < f.n; start += size {
			for j := 0; j < half; j++ {
				wr, wi := f.cos[j*step], f.sin[j*step]
				if inverse {
					wi = -wi
				}
				a, b := start+j, start+j+half
				tr := re[b]*wr - im[b]*wi
				ti := re[b]*wi + im[b]*wr
				re[b], im[b] = re[a]-tr, im[a]-ti
				re[a], im[a] = re[a]+tr, im[a]+ti
			}
		}
	}
}
