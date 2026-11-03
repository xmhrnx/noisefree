package vision

import (
	"image"
	"image/color"
	"sync"
	"time"
)

// Settings are the user's camera effect choices.
type Settings struct {
	Background      BackgroundMode
	BlurStrength    float32     // 0..1
	BackgroundImage image.Image // for BackgroundReplace; nil is a plain studio grey
	RemoveColor     color.RGBA  // for BackgroundRemove
	EyeContact      bool
	EyeStrength     float32 // 0..1
	Lighting        bool    // auto exposure, white balance and a key light on the face
	LightStrength   float32 // 0..1
	Denoise         bool
	Sharpen         bool
}

// Stats describe the last processed frame.
type Stats struct {
	Frame     time.Duration // processing time
	Inference time.Duration // of which spent in the networks
	FaceFound bool
	Backend   string
	Warning   string
}

// Processor applies the effects to camera frames. Process is called from
// one goroutine; SetSettings and Stats from any.
type Processor struct {
	mu    sync.Mutex
	set   Settings
	stats Stats

	models *Models
	seg    segmenter
	face   faceTracker
	light  lighting
	eyes   eyeContact
	bg     background

	work, out, prev *image.RGBA
}

// NewProcessor creates a processor. models may be nil: then only lighting
// and quality, which need no networks, are applied.
func NewProcessor(models *Models) *Processor {
	p := &Processor{models: models}
	if models != nil {
		p.seg.m = models.seg
		p.face.det, p.face.lm = models.det, models.lm
		p.stats.Backend = string(models.accel)
		p.stats.Warning = models.Warning
	}
	return p
}

// SetSettings changes the effects from the next frame.
func (p *Processor) SetSettings(s Settings) {
	p.mu.Lock()
	p.set = s
	p.mu.Unlock()
}

// Stats returns statistics of the last frame.
func (p *Processor) Stats() Stats {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stats
}

func ensure(img **image.RGBA, r image.Rectangle) {
	if *img == nil || (*img).Rect != r {
		*img = image.NewRGBA(r)
	}
}

// Process applies the effects to src and returns the result, which stays
// valid until the next call.
func (p *Processor) Process(src *image.RGBA) (*image.RGBA, error) {
	start := time.Now()
	p.mu.Lock()
	set := p.set
	p.mu.Unlock()

	r := image.Rect(0, 0, src.Rect.Dx(), src.Rect.Dy())
	ensure(&p.work, r)
	ensure(&p.out, r)
	for y := 0; y < r.Dy(); y++ {
		so := src.PixOffset(src.Rect.Min.X, src.Rect.Min.Y+y)
		copy(p.work.Pix[y*p.work.Stride:y*p.work.Stride+r.Dx()*4], src.Pix[so:so+r.Dx()*4])
	}
	if set.Denoise {
		if p.prev == nil || p.prev.Rect != r {
			p.prev = image.NewRGBA(r)
			copy(p.prev.Pix, p.work.Pix)
		}
		// denoise also stores the result as the next frame's reference
		denoise(p.work, p.prev, 0.6)
	}

	// The two networks only read the frame, so they run side by side before
	// the pixel effects change it.
	needFace := p.models != nil && (set.EyeContact || set.Lighting)
	needSeg := p.models != nil && set.Background != BackgroundOff
	var face *Face
	var faceErr, segErr error
	var faceTime, segTime time.Duration
	var wg sync.WaitGroup
	if needSeg {
		wg.Add(1)
		go func() {
			defer wg.Done()
			t := time.Now()
			segErr = p.seg.run(p.work)
			segTime = time.Since(t)
		}()
	}
	if needFace {
		t := time.Now()
		f, ok, err := p.face.run(p.work)
		faceTime, faceErr = time.Since(t), err
		if ok {
			face = f
		}
	}
	wg.Wait()
	if faceErr != nil {
		return nil, faceErr
	}
	if segErr != nil {
		return nil, segErr
	}
	inference := max(faceTime, segTime)

	if set.Lighting {
		p.light.update(p.work, face, set.LightStrength)
		p.light.apply(p.work, face, set.LightStrength)
	}
	if set.EyeContact && face != nil {
		p.eyes.apply(p.work, face, set.EyeStrength)
	}
	if set.Sharpen {
		sharpen(p.out, p.work, 0.45)
		p.work, p.out = p.out, p.work // the sharpened frame is the new work frame
	}

	result := p.work
	if needSeg {
		var bg *image.RGBA
		switch set.Background {
		case BackgroundBlur:
			p.bg.blurred(p.work, p.seg.mask, set.BlurStrength)
		case BackgroundReplace:
			bg = p.bg.replacement(set.BackgroundImage, r.Dx(), r.Dy())
		}
		p.bg.composite(p.out, p.work, p.seg.mask, set.Background, bg, set.RemoveColor)
		result = p.out
	}

	p.mu.Lock()
	p.stats.Frame, p.stats.Inference, p.stats.FaceFound = time.Since(start), inference, face != nil
	p.mu.Unlock()
	return result, nil
}
