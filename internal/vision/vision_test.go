package vision

import (
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/xmhrnx/noisefree/internal/onnx"
)

// NOISEFREE_TEST_ORT points at libonnxruntime; NOISEFREE_TEST_OUT, when set,
// receives the processed images for a look.
func loadTest(t *testing.T) (*Models, *image.RGBA) {
	t.Helper()
	lib := os.Getenv("NOISEFREE_TEST_ORT")
	if lib == "" {
		t.Skip("NOISEFREE_TEST_ORT not set")
	}
	m, err := LoadModels(lib, onnx.CPU)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open("testdata/portrait.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	src, err := jpeg.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	// a 16:9 webcam-like frame: the portrait enlarged so the face is about
	// as big as someone sitting at a laptop
	frame := image.NewRGBA(image.Rect(0, 0, 1280, 720))
	draw.Draw(frame, frame.Rect, image.NewUniform(color.RGBA{90, 80, 70, 255}), image.Point{}, draw.Src)
	sb := src.Bounds()
	dst := image.Rect(640-512, -40, 640+512, -40+1280)
	scaleInto(frame, dst.Intersect(frame.Rect), src, sb, dst)
	return m, frame
}

// scaleInto draws src, scaled to cover full, into the visible part vis.
func scaleInto(dst *image.RGBA, vis image.Rectangle, src image.Image, sb, full image.Rectangle) {
	for y := vis.Min.Y; y < vis.Max.Y; y++ {
		for x := vis.Min.X; x < vis.Max.X; x++ {
			sx := sb.Min.X + (x-full.Min.X)*sb.Dx()/full.Dx()
			sy := sb.Min.Y + (y-full.Min.Y)*sb.Dy()/full.Dy()
			dst.Set(x, y, src.At(sx, sy))
		}
	}
}

func save(t *testing.T, name string, img image.Image) {
	dir := os.Getenv("NOISEFREE_TEST_OUT")
	if dir == "" {
		return
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	png.Encode(f, img)
}

func clone(img *image.RGBA) *image.RGBA {
	c := image.NewRGBA(img.Rect)
	copy(c.Pix, img.Pix)
	return c
}

func TestEffects(t *testing.T) {
	m, frame := loadTest(t)
	defer m.Close()
	cases := map[string]Settings{
		"blur":    {Background: BackgroundBlur, BlurStrength: 0.7},
		"replace": {Background: BackgroundReplace},
		"remove":  {Background: BackgroundRemove, RemoveColor: color.RGBA{0, 177, 64, 255}},
		"studio":  {Lighting: true, LightStrength: 0.8, Denoise: true, Sharpen: true, EyeContact: true, EyeStrength: 1, Background: BackgroundBlur, BlurStrength: 0.5},
	}
	for name, set := range cases {
		p := NewProcessor(m)
		p.SetSettings(set)
		var out *image.RGBA
		for i := 0; i < 8; i++ { // let the temporal filters settle
			var err error
			if out, err = p.Process(frame); err != nil {
				t.Fatal(err)
			}
		}
		st := p.Stats()
		t.Logf("%-8s %v per 720p frame (%v in networks), face %v", name, st.Frame, st.Inference, st.FaceFound)
		save(t, name+".png", out)
		// the corner is background, the centre is the person
		corner, centre := out.RGBAAt(40, 40), frame.RGBAAt(640, 500)
		if name == "remove" && corner != (color.RGBA{0, 177, 64, 255}) {
			t.Errorf("remove: corner = %v, want the solid colour", corner)
		}
		if got := out.RGBAAt(640, 500); name != "studio" && got != centre {
			t.Errorf("%s: person pixel changed from %v to %v", name, centre, got)
		}
	}
}

// irisY finds each iris as the centroid of the darkest pixels inside the
// eye opening of o, and returns its height relative to the corners, in eye
// widths (positive: below the corners).
func irisY(img *image.RGBA, o eyeOpening) float32 {
	var sum, n float32
	thr := float32(1e9)
	// threshold: the darkest fifth of the opening
	var vals []float32
	each := func(fn func(x, y int, l float32)) {
		for x := int(o.left.X + o.width*0.15); x < int(o.right.X-o.width*0.15); x++ {
			for y := int(o.lid(o.top, float32(x))) + 1; y < int(o.lid(o.low, float32(x))); y++ {
				c := img.RGBAAt(x, y)
				fn(x, y, 0.299*float32(c.R)+0.587*float32(c.G)+0.114*float32(c.B))
			}
		}
	}
	each(func(_, _ int, l float32) { vals = append(vals, l) })
	if len(vals) == 0 {
		return 0
	}
	sortFloats(vals)
	thr = vals[len(vals)/5]
	each(func(_, y int, l float32) {
		if l <= thr {
			sum += float32(y)
			n++
		}
	})
	return (sum/n - (o.left.Y+o.right.Y)/2) / o.width
}

func sortFloats(v []float32) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && v[j] < v[j-1]; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}

func TestEyeContactLiftsLoweredGaze(t *testing.T) {
	m, frame := loadTest(t)
	defer m.Close()
	// simulate reading the screen: push both irises down
	var tr faceTracker
	tr.det, tr.lm = m.det, m.lm
	f, ok, _ := tr.run(frame)
	if !ok {
		t.Fatal("no face")
	}
	down := clone(frame)
	var ec eyeContact
	for _, e := range eyes {
		o := eyeShape(f.Points, e)
		ec.warp(down, o, Point{0, 0.12 * o.width})
	}
	save(t, "gaze_down.png", down)

	p := NewProcessor(m)
	p.SetSettings(Settings{EyeContact: true, EyeStrength: 1})
	var out *image.RGBA
	for i := 0; i < 6; i++ {
		out, _ = p.Process(down)
	}
	save(t, "gaze_fixed.png", out)
	measure := func(img *image.RGBA) float32 {
		var s float32
		for _, e := range eyes {
			s += irisY(img, eyeShape(f.Points, e))
		}
		return s / 2
	}
	orig, before, after := measure(frame), measure(down), measure(out)
	t.Logf("iris height below the eye corners: original %.3f, looking down %.3f, corrected %.3f eye widths", orig, before, after)
	if !(before > orig+0.02) {
		t.Fatalf("test setup didn't lower the gaze: %.3f -> %.3f", orig, before)
	}
	// A geometric warp can't restore iris pixels the simulated downward look
	// destroyed, so the bar is a clear upward move rather than a full return.
	if !(after < before-0.01) {
		t.Fatalf("eye contact didn't lift the gaze: %.3f -> %.3f", before, after)
	}
}

func TestLightingBrightensDarkFrame(t *testing.T) {
	m, frame := loadTest(t)
	defer m.Close()
	dark := clone(frame)
	for i := range dark.Pix {
		if i%4 != 3 {
			dark.Pix[i] = uint8(float32(dark.Pix[i]) * 0.35)
		}
	}
	p := NewProcessor(m)
	p.SetSettings(Settings{Lighting: true, LightStrength: 1})
	var out *image.RGBA
	for i := 0; i < 60; i++ {
		out, _ = p.Process(dark)
	}
	save(t, "dark.png", dark)
	save(t, "dark_lit.png", out)
	luma := func(img *image.RGBA) float32 {
		var s float32
		for y := 150; y < 350; y++ {
			c := img.RGBAAt(640, y)
			s += 0.299*float32(c.R) + 0.587*float32(c.G) + 0.114*float32(c.B)
		}
		return s / 200
	}
	if b, a := luma(dark), luma(out); a < b*1.4 {
		t.Fatalf("face luma %.0f -> %.0f: want a clear lift", b, a)
	}
}

func BenchmarkStudio720p(b *testing.B) {
	t := &testing.T{}
	m, frame := loadTest(t)
	if m == nil {
		b.Skip("NOISEFREE_TEST_ORT not set")
	}
	defer m.Close()
	p := NewProcessor(m)
	p.SetSettings(Settings{Lighting: true, LightStrength: 0.8, Denoise: true, Sharpen: true, EyeContact: true, EyeStrength: 1, Background: BackgroundBlur, BlurStrength: 0.5})
	for i := 0; i < b.N; i++ {
		p.Process(frame)
	}
}
