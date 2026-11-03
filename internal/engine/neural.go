package engine

import (
	"github.com/xmhrnx/noisefree/internal/dtln"
)

// Neural runs the DTLN network through ONNX Runtime on the CPU or a GPU.
type Neural struct {
	m *dtln.Model
	b *blend
}

// NewNeural loads DTLN from modelDir with the ONNX Runtime at libPath.
// The returned warning is non-empty when the GPU was requested but the
// model fell back to the CPU.
func NewNeural(modelDir, libPath string, acc dtln.Accelerator) (*Neural, string, error) {
	m, err := dtln.Open(modelDir, libPath, acc)
	if err != nil {
		return nil, "", err
	}
	return &Neural{m: m, b: newBlend(dtln.FrameSize, m.Latency())}, m.Warning, nil
}

func (n *Neural) FrameSize() int        { return dtln.FrameSize }
func (n *Neural) Latency() int          { return n.m.Latency() }
func (n *Neural) SetStrength(s float32) { n.b.set(s) }
func (n *Neural) Close()                { n.m.Close() }

func (n *Neural) Backend() string {
	switch n.m.Accelerator() {
	case dtln.DirectML:
		return "GPU · DirectML"
	case dtln.CoreML:
		return "GPU · Core ML"
	case dtln.CUDA:
		return "GPU · CUDA"
	}
	return "CPU"
}

func (n *Neural) Process(frame []float32) {
	n.b.keep(frame)
	n.m.Process(frame)
	n.b.mix(frame)
}
