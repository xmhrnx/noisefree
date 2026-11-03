// Package vision runs NoiseFree's camera effects: background blur and
// replacement, eye contact, lighting and image quality. The neural parts
// (person segmentation, face landmarks) are MediaPipe models run by ONNX
// Runtime on the CPU or a GPU; the pixel work is parallel Go.
package vision

import (
	_ "embed"
	"fmt"

	"github.com/xmhrnx/noisefree/internal/onnx"
	ort "github.com/yalue/onnxruntime_go"
)

var (
	//go:embed models/selfie_segmenter_landscape.onnx
	segmenterONNX []byte
	//go:embed models/face_detector.onnx
	faceDetectorONNX []byte
	//go:embed models/face_landmarks.onnx
	faceLandmarksONNX []byte
)

// model is one ONNX session with preallocated input and output tensors.
type model struct {
	s   *ort.AdvancedSession
	in  []*ort.Tensor[float32]
	out []*ort.Tensor[float32]
}

type tensorSpec struct {
	name  string
	shape []int64
}

func newModel(data []byte, opts *ort.SessionOptions, inputs, outputs []tensorSpec) (*model, error) {
	m := &model{}
	var inNames, outNames []string
	var inVals, outVals []ort.Value
	for _, t := range inputs {
		x, err := ort.NewEmptyTensor[float32](ort.NewShape(t.shape...))
		if err != nil {
			m.close()
			return nil, err
		}
		m.in = append(m.in, x)
		inNames, inVals = append(inNames, t.name), append(inVals, x)
	}
	for _, t := range outputs {
		x, err := ort.NewEmptyTensor[float32](ort.NewShape(t.shape...))
		if err != nil {
			m.close()
			return nil, err
		}
		m.out = append(m.out, x)
		outNames, outVals = append(outNames, t.name), append(outVals, x)
	}
	s, err := ort.NewAdvancedSessionWithONNXData(data, inNames, outNames, inVals, outVals, opts)
	if err != nil {
		m.close()
		return nil, fmt.Errorf("load model: %w", err)
	}
	m.s = s
	return m, nil
}

func (m *model) run() error { return m.s.Run() }

func (m *model) close() {
	if m == nil {
		return
	}
	if m.s != nil {
		m.s.Destroy()
	}
	for _, t := range append(m.in, m.out...) {
		t.Destroy()
	}
}

// Models holds the loaded networks.
type Models struct {
	seg, det, lm *model
	accel        onnx.Accelerator
	Warning      string
}

// LoadModels starts ONNX Runtime from libPath and loads the networks on acc,
// falling back to the CPU (with Warning set) when acc isn't available.
func LoadModels(libPath string, acc onnx.Accelerator) (*Models, error) {
	if err := onnx.Init(libPath); err != nil {
		return nil, err
	}
	opts, used, warn := onnx.SessionOptions(acc, 2)
	if opts == nil {
		return nil, warn
	}
	m := &Models{accel: used}
	if warn != nil {
		m.Warning = warn.Error()
		opts.Destroy()
		if opts, m.accel, _ = onnx.SessionOptions(onnx.CPU, 2); opts == nil {
			return nil, warn
		}
	}
	defer func() { opts.Destroy() }()
	var err error
	if m.seg, err = newModel(segmenterONNX, opts,
		[]tensorSpec{{"input_1", []int64{1, segH, segW, 3}}},
		[]tensorSpec{{"segment_back", []int64{1, segH, segW, 1}}}); err != nil {
		return nil, err
	}
	if m.det, err = newModel(faceDetectorONNX, opts,
		[]tensorSpec{{"input", []int64{1, detSize, detSize, 3}}},
		[]tensorSpec{{"regressors", []int64{1, numAnchors, 16}}, {"classificators", []int64{1, numAnchors, 1}}}); err != nil {
		m.Close()
		return nil, err
	}
	if m.lm, err = newModel(faceLandmarksONNX, opts,
		[]tensorSpec{{"input_12", []int64{1, lmSize, lmSize, 3}}},
		[]tensorSpec{{"Identity", []int64{1, 1, 1, NumLandmarks * 3}}, {"Identity_1", []int64{1, 1, 1, 1}}, {"Identity_2", []int64{1, 1}}}); err != nil {
		m.Close()
		return nil, err
	}
	return m, nil
}

// Accelerator reports where the networks run.
func (m *Models) Accelerator() onnx.Accelerator { return m.accel }

// Close frees the networks.
func (m *Models) Close() {
	m.seg.close()
	m.det.close()
	m.lm.close()
}
