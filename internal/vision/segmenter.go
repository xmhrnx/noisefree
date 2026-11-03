package vision

import "image"

const (
	segW, segH = 256, 144
)

// segmenter estimates, per pixel, how likely it belongs to a person.
type segmenter struct {
	m    *model
	mask []float32 // segW x segH, smoothed over time
	have bool
}

// run updates the mask from frame.
func (s *segmenter) run(frame *image.RGBA) error {
	in := s.m.in[0].GetData()
	// The whole frame is fitted to the model's 16:9 input; a 4:3 camera is
	// stretched a little, which the model tolerates, and the mask then maps
	// straight back onto the frame.
	b := frame.Rect
	sampleRGB(in, segW, segH, frame, rect{w: float32(b.Dx()), h: float32(b.Dy())}, 1.0/255, 0)
	if err := s.m.run(); err != nil {
		return err
	}
	out := s.m.out[0].GetData()
	if s.mask == nil {
		s.mask = make([]float32, segW*segH)
	}
	// a little temporal smoothing removes flicker at the edges without
	// visible lag; sharpen the transition so hair and shoulders stay crisp
	for i, v := range out {
		v = smoothstep(0.2, 0.8, v)
		if s.have {
			v = 0.55*v + 0.45*s.mask[i]
		}
		s.mask[i] = v
	}
	// one small blur pass rounds off the stair-steps the low-resolution
	// mask would otherwise show along hair and shoulders once enlarged
	boxBlur(s.mask, segW, segH, 1)
	s.have = true
	return nil
}

func smoothstep(a, b, x float32) float32 {
	t := clampf((x-a)/(b-a), 0, 1)
	return t * t * (3 - 2*t)
}
