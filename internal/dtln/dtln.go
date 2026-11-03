package dtln

import (
	"fmt"
	"math"
	"path/filepath"

	"github.com/xmhrnx/noisefree/internal/dsp"
	ort "github.com/yalue/onnxruntime_go"
)

const (
	blockLen   = 512 // samples at 16 kHz
	blockShift = 128
	bins       = blockLen/2 + 1
)

// FrameSize is how many 48 kHz samples Process takes per call.
const FrameSize = blockShift * 3

// Model is a running DTLN instance.
type Model struct {
	s1, s2             *ort.AdvancedSession
	mag, mask          *ort.Tensor[float32]
	st1In, st1Out      *ort.Tensor[float32]
	frameIn, frameOut  *ort.Tensor[float32]
	st2In, st2Out      *ort.Tensor[float32]
	fft                *dsp.FFT
	rs                 *dsp.Resampler3
	inBuf, outBuf      []float32
	re, im, phRe, phIm []float32
	time               []float32
	low                []float32
	accel              Accelerator
	Warning            string // set when the requested accelerator fell back
}

// Open loads model_1.onnx and model_2.onnx from dir with the ONNX Runtime
// library at libPath, on the requested accelerator.
func Open(dir, libPath string, acc Accelerator) (*Model, error) {
	if err := initRuntime(libPath); err != nil {
		return nil, err
	}
	m := &Model{fft: dsp.NewFFT(blockLen), rs: dsp.NewResampler3(),
		inBuf: make([]float32, blockLen), outBuf: make([]float32, blockLen),
		re: make([]float32, bins), im: make([]float32, bins), phRe: make([]float32, bins), phIm: make([]float32, bins),
		time: make([]float32, blockLen), low: make([]float32, blockShift)}
	var err error
	newT := func(dims ...int64) *ort.Tensor[float32] {
		if err != nil {
			return nil
		}
		var t *ort.Tensor[float32]
		t, err = ort.NewEmptyTensor[float32](ort.NewShape(dims...))
		return t
	}
	m.mag, m.mask = newT(1, 1, bins), newT(1, 1, bins)
	m.st1In, m.st1Out = newT(1, 2, 128, 2), newT(1, 2, 128, 2)
	m.frameIn, m.frameOut = newT(1, 1, blockLen), newT(1, 1, blockLen)
	m.st2In, m.st2Out = newT(1, 2, 128, 2), newT(1, 2, 128, 2)
	if err != nil {
		m.Close()
		return nil, err
	}
	opts, used, warn := sessionOptions(acc)
	if opts == nil {
		m.Close()
		return nil, warn
	}
	defer func() { opts.Destroy() }()
	if warn != nil {
		m.Warning = warn.Error()
		opts.Destroy()
		if opts, used, _ = sessionOptions(CPU); opts == nil {
			m.Close()
			return nil, warn
		}
	}
	m.accel = used
	m.s1, err = ort.NewAdvancedSession(filepath.Join(dir, "model_1.onnx"),
		[]string{"input_2", "input_3"}, []string{"activation_2", "tf_op_layer_stack_2"},
		[]ort.Value{m.mag, m.st1In}, []ort.Value{m.mask, m.st1Out}, opts)
	if err == nil {
		m.s2, err = ort.NewAdvancedSession(filepath.Join(dir, "model_2.onnx"),
			[]string{"input_4", "input_5"}, []string{"conv1d_3", "tf_op_layer_stack_5"},
			[]ort.Value{m.frameIn, m.st2In}, []ort.Value{m.frameOut, m.st2Out}, opts)
	}
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("load DTLN: %w", err)
	}
	return m, nil
}

// Accelerator reports where the model actually runs.
func (m *Model) Accelerator() Accelerator { return m.accel }

// Latency is the model's algorithmic delay at 48 kHz, excluding framing.
func (m *Model) Latency() int { return (blockLen-blockShift)*3 + 2*m.rs.Latency() }

// Process enhances FrameSize samples at 48 kHz in place.
func (m *Model) Process(frame []float32) {
	m.rs.Down(frame, m.low)
	copy(m.inBuf, m.inBuf[blockShift:])
	copy(m.inBuf[blockLen-blockShift:], m.low)

	m.fft.Forward(m.inBuf, m.re, m.im)
	mag := m.mag.GetData()
	for k := range m.re {
		a := float32(math.Hypot(float64(m.re[k]), float64(m.im[k])))
		mag[k] = a
		if a > 0 {
			m.phRe[k], m.phIm[k] = m.re[k]/a, m.im[k]/a
		} else {
			m.phRe[k], m.phIm[k] = 1, 0
		}
	}
	if m.s1.Run() != nil {
		return // leave the audio untouched rather than glitching
	}
	copy(m.st1In.GetData(), m.st1Out.GetData())
	mask := m.mask.GetData()
	for k := range m.re {
		a := mag[k] * mask[k]
		m.re[k], m.im[k] = a*m.phRe[k], a*m.phIm[k]
	}
	m.fft.Inverse(m.re, m.im, m.frameIn.GetData())
	if m.s2.Run() != nil {
		return
	}
	copy(m.st2In.GetData(), m.st2Out.GetData())

	copy(m.outBuf, m.outBuf[blockShift:])
	clear(m.outBuf[blockLen-blockShift:])
	for i, v := range m.frameOut.GetData() {
		m.outBuf[i] += v
	}
	m.rs.Up(m.outBuf[:blockShift], frame)
}

// Close releases the sessions and tensors.
func (m *Model) Close() {
	for _, s := range []*ort.AdvancedSession{m.s1, m.s2} {
		if s != nil {
			s.Destroy()
		}
	}
	for _, t := range []*ort.Tensor[float32]{m.mag, m.mask, m.st1In, m.st1Out, m.frameIn, m.frameOut, m.st2In, m.st2Out} {
		if t != nil {
			t.Destroy()
		}
	}
	m.s1, m.s2 = nil, nil
}
