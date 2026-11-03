package vcam

import (
	"image"
	"image/color"
	"testing"
)

func TestYUVOfGreyAndRed(t *testing.T) {
	if y, u, v := yuv(128, 128, 128); y != 126 || u != 128 || v != 128 {
		t.Fatalf("grey -> %d %d %d", y, u, v)
	}
	if y, u, v := yuv(255, 0, 0); y != 82 || u != 90 || v != 240 {
		t.Fatalf("red -> %d %d %d", y, u, v)
	}
}

func TestPacking(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for i := 0; i < 8; i++ {
		img.Set(i%4, i/4, color.RGBA{255, 0, 0, 255})
	}
	yuyv := make([]byte, 4*2*2)
	toYUYV(yuyv, img)
	if yuyv[0] != 82 || yuyv[1] != 90 || yuyv[3] != 240 {
		t.Fatalf("yuyv = %v", yuyv[:4])
	}
	nv12 := make([]byte, 4*2*3/2)
	toNV12(nv12, img)
	if nv12[0] != 82 || nv12[8] != 90 || nv12[9] != 240 {
		t.Fatalf("nv12 = %v", nv12)
	}
}
