package dsp

// Framer adapts the audio device's block size to a processor that needs
// fixed-size frames, adding exactly one frame of latency. It never allocates
// after construction.
type Framer struct {
	size int
	in   []float32
	inN  int
	ring []float32 // processed samples waiting to be played
	r, n int
}

// NewFramer creates a framer for frames of size samples.
func NewFramer(size int) *Framer {
	// primed with one frame of silence, so output never runs dry
	return &Framer{size: size, in: make([]float32, size), ring: make([]float32, 2*size), n: size}
}

// Size returns the frame size.
func (f *Framer) Size() int { return f.size }

// Run feeds buf through process frame by frame and replaces buf with the
// processed stream, delayed by one frame.
func (f *Framer) Run(buf []float32, process func(frame []float32)) {
	for i, v := range buf {
		f.in[f.inN] = v
		f.inN++
		if f.inN == f.size {
			process(f.in)
			w := (f.r + f.n) % len(f.ring)
			for _, s := range f.in {
				f.ring[w] = s
				w++
				if w == len(f.ring) {
					w = 0
				}
			}
			f.n += f.size
			f.inN = 0
		}
		buf[i] = f.ring[f.r]
		f.r++
		if f.r == len(f.ring) {
			f.r = 0
		}
		f.n--
	}
}

// Delay is a fixed sample delay line, used to keep the dry signal aligned
// with a processor's output when blending them.
type Delay struct {
	buf []float32
	pos int
}

// NewDelay creates a delay of n samples (n = 0 passes audio through).
func NewDelay(n int) *Delay { return &Delay{buf: make([]float32, n)} }

// Process delays buf in place.
func (d *Delay) Process(buf []float32) {
	if len(d.buf) == 0 {
		return
	}
	for i, v := range buf {
		buf[i], d.buf[d.pos] = d.buf[d.pos], v
		d.pos++
		if d.pos == len(d.buf) {
			d.pos = 0
		}
	}
}
