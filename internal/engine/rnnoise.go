package engine

import "github.com/xmhrnx/noisefree/internal/rnnoise"

// RNNoise is Xiph's recurrent-network suppressor, built into the app.
type RNNoise struct {
	st *rnnoise.State
	b  *blend
}

// NewRNNoise creates the RNNoise processor.
func NewRNNoise() (*RNNoise, error) {
	st, err := rnnoise.New()
	if err != nil {
		return nil, err
	}
	return &RNNoise{st: st, b: newBlend(rnnoise.FrameSize, rnnoise.FrameSize)}, nil
}

func (r *RNNoise) FrameSize() int        { return rnnoise.FrameSize }
func (r *RNNoise) Latency() int          { return rnnoise.FrameSize }
func (r *RNNoise) Backend() string       { return "CPU" }
func (r *RNNoise) SetStrength(s float32) { r.b.set(s) }
func (r *RNNoise) Close()                { r.st.Close() }

func (r *RNNoise) Process(frame []float32) {
	r.b.keep(frame)
	r.st.Process(frame)
	r.b.mix(frame)
}
