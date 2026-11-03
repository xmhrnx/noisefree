# Models

These ONNX models are converted from Google's MediaPipe models, licensed under
the Apache License 2.0 (https://www.apache.org/licenses/LICENSE-2.0):

| File | Source |
| --- | --- |
| `selfie_segmenter_landscape.onnx` | MediaPipe Selfie Segmenter (landscape), `selfie_segmenter_landscape.tflite` |
| `face_detector.onnx` | MediaPipe Face Detector (BlazeFace short range), from `face_landmarker.task` |
| `face_landmarks.onnx` | MediaPipe Face Landmarker (478 points with irises), from `face_landmarker.task` |

Converted with tf2onnx (opset 17). MediaPipe's custom
`Convolution2DTransposeBias` op in the segmenter was rewritten as a standard
ONNX `ConvTranspose` (stride 2, 2×2 kernel) followed by an NCHW→NHWC transpose.
Copyright 2023 The MediaPipe Authors.

`testdata/portrait.jpg` is MediaPipe's test image (official White House portrait,
public domain).
