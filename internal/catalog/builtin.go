package catalog

// ONNX Runtime release the neural engines are built against (the C API
// version of github.com/yalue/onnxruntime_go v1.27).
const ortVersion = "1.24.4"

const ortBase = "https://github.com/microsoft/onnxruntime/releases/download/v" + ortVersion + "/"

// Builtin returns the engines NoiseFree knows out of the box.
func Builtin() []Entry {
	return []Entry{
		{
			ID: "lite", Name: "NoiseFree Lite", Vendor: "NoiseFree", Version: "1.0", Kind: KindLite, Builtin: true,
			Summary:     "Featherweight spectral suppressor",
			Description: "A Wiener filter with adaptive noise tracking, written in pure Go. Removes steady noise such as fans, hum and air conditioning with almost no CPU. Best for old laptops and low-power devices.",
			License:     "MIT", Quality: 2, CPUCost: 1, LatencyMs: 11,
		},
		{
			ID: "rnnoise", Name: "RNNoise", Vendor: "Xiph.Org", Version: "0.1", Kind: KindRNNoise, Builtin: true,
			Summary:     "Neural suppressor, light enough for any CPU",
			Description: "A small recurrent neural network trained on speech and noise. Handles keyboards, traffic and chatter far better than classic filters while using about 1% of one CPU core.",
			License:     "BSD-3-Clause", Homepage: "https://jmvalin.ca/demo/rnnoise/", Quality: 3, CPUCost: 2, LatencyMs: 20,
		},
		{
			ID: "deepfilternet", Name: "DeepFilterNet 3", Vendor: "Hendrik Schröter", Version: "0.5.6", Kind: KindLADSPA,
			Summary:     "Studio-grade neural noise removal",
			Description: "A state-of-the-art deep filtering network for full-band 48 kHz speech. The closest open match to NVIDIA Broadcast's quality: removes dogs, keyboards, crying babies and reverb-heavy noise while keeping the voice natural.",
			License:     "MIT / Apache-2.0", Homepage: "https://github.com/Rikorose/DeepFilterNet",
			Quality: 5, CPUCost: 4, LatencyMs: 40,
			Plugin: &PluginSpec{File: "deepfilter_ladspa", Label: "deep_filter_mono",
				StrengthControl: "Attenuation Limit", StrengthMin: 6, StrengthMax: 100},
			Downloads: map[string][]Download{
				"linux/amd64":   {dfn("libdeep_filter_ladspa", "x86_64-unknown-linux-gnu", "so")},
				"linux/arm64":   {dfn("libdeep_filter_ladspa", "aarch64-unknown-linux-gnu", "so")},
				"windows/amd64": {dfn("deep_filter_ladspa", "x86_64-pc-windows-msvc", "dll")},
			},
		},
		{
			ID: "rnnoise-vad", Name: "RNNoise Voice Gate", Vendor: "werman", Version: "1.10", Kind: KindLADSPA,
			Summary:     "RNNoise with voice-activity gating",
			Description: "RNNoise plus a voice detector that fully mutes the microphone between words, so breathing, clicks and room tone disappear. Strength sets how sure the detector must be that you're speaking.",
			License:     "GPL-3.0", Homepage: "https://github.com/werman/noise-suppression-for-voice",
			Quality: 3, CPUCost: 2, LatencyMs: 20,
			Plugin: &PluginSpec{File: "rnnoise_ladspa", Label: "noise_suppressor_mono",
				StrengthControl: "VAD Threshold", StrengthMin: 0, StrengthMax: 95,
				Controls: map[string]float32{"VAD Grace Period": 200}, LatencySamples: 480},
			Downloads: map[string][]Download{
				"linux":   {werman("linux", "librnnoise_ladspa.so")},
				"windows": {werman("win", "rnnoise_ladspa.dll")},
				"darwin":  {werman("macos", "librnnoise_ladspa.dylib")},
			},
		},
		{
			ID: "dtln", Name: "DTLN Neural", Vendor: "Westhausen & Meyer", Version: "1.0", Kind: KindDTLN, GPU: true,
			Summary:     "Dual-signal LSTM network, GPU accelerated",
			Description: "A dual-signal transformation LSTM network that ranked among the best real-time models of the DNS challenge. Runs on ONNX Runtime and can move to your GPU (any DirectX 12 card on Windows, Apple silicon on macOS, NVIDIA CUDA on Linux) to free the CPU completely.",
			License:     "MIT", Homepage: "https://github.com/breizhn/DTLN",
			Quality: 4, CPUCost: 3, LatencyMs: 32, Requires: []string{"onnxruntime"},
			Downloads: map[string][]Download{"*": {
				{URL: "https://raw.githubusercontent.com/breizhn/DTLN/master/pretrained_model/model_1.onnx", Pick: []Pick{{As: "model_1.onnx"}}, SizeMB: 2},
				{URL: "https://raw.githubusercontent.com/breizhn/DTLN/master/pretrained_model/model_2.onnx", Pick: []Pick{{As: "model_2.onnx"}}, SizeMB: 3},
			}},
		},
		{
			ID: "onnxruntime", Name: "ONNX Runtime", Vendor: "Microsoft", Version: ortVersion, Kind: KindRuntime, Hidden: true,
			Summary: "Runs neural engines on the CPU (and Core ML on macOS)", License: "MIT",
			Downloads: map[string][]Download{
				"linux/amd64":   {ort("onnxruntime-linux-x64-"+ortVersion+".tgz", "tgz", "so", 8)},
				"linux/arm64":   {ort("onnxruntime-linux-aarch64-"+ortVersion+".tgz", "tgz", "so", 8)},
				"darwin/arm64":  {ort("onnxruntime-osx-arm64-"+ortVersion+".tgz", "tgz", "dylib", 30)},
				"windows/amd64": {ort("onnxruntime-win-x64-"+ortVersion+".zip", "zip", "dll", 71)},
			},
		},
		{
			ID: "onnxruntime-gpu", Name: "GPU acceleration pack", Vendor: "Microsoft", Version: ortVersion, Kind: KindRuntime, Hidden: true,
			Summary: "ONNX Runtime with DirectML (Windows) or CUDA (Linux, NVIDIA)", License: "MIT",
			Downloads: map[string][]Download{
				"windows/amd64": {
					{URL: "https://api.nuget.org/v3-flatcontainer/microsoft.ml.onnxruntime.directml/" + ortVersion + "/microsoft.ml.onnxruntime.directml." + ortVersion + ".nupkg",
						Archive: "zip", Pick: []Pick{
							{Match: "runtimes/win-x64/native/onnxruntime.dll", As: "onnxruntime.dll"},
							{Match: "runtimes/win-x64/native/onnxruntime_providers_shared.dll", As: "onnxruntime_providers_shared.dll"},
						}, SizeMB: 12},
					{URL: "https://api.nuget.org/v3-flatcontainer/microsoft.ai.directml/1.15.4/microsoft.ai.directml.1.15.4.nupkg",
						Archive: "zip", Pick: []Pick{{Match: "bin/x64-win/DirectML.dll", As: "DirectML.dll"}}, SizeMB: 193},
				},
				"linux/amd64": {{URL: ortBase + "onnxruntime-linux-x64-gpu-" + ortVersion + ".tgz", Archive: "tgz", Pick: []Pick{
					{Match: "lib/libonnxruntime.so." + ortVersion, As: "libonnxruntime.so." + ortVersion},
					{Match: "lib/libonnxruntime_providers_shared.so", As: "libonnxruntime_providers_shared.so"},
					{Match: "lib/libonnxruntime_providers_cuda.so", As: "libonnxruntime_providers_cuda.so"},
				}, SizeMB: 196}},
			},
		},
	}
}

func dfn(lib, target, ext string) Download {
	v := "0.5.6"
	return Download{
		URL:  "https://github.com/Rikorose/DeepFilterNet/releases/download/v" + v + "/" + lib + "-" + v + "-" + target + "." + ext,
		Pick: []Pick{{As: "deepfilter_ladspa"}}, SizeMB: 10,
	}
}

func werman(prefix, lib string) Download {
	return Download{
		URL:     "https://github.com/werman/noise-suppression-for-voice/releases/download/v1.10/" + prefix + "-rnnoise.zip",
		Archive: "zip", Pick: []Pick{{Match: "ladspa/" + lib, As: "rnnoise_ladspa"}}, SizeMB: 8,
	}
}

func ort(file, archive, ext string, sizeMB int) Download {
	var picks []Pick
	switch ext {
	case "so":
		picks = []Pick{{Match: "lib/libonnxruntime.so." + ortVersion, As: "libonnxruntime.so." + ortVersion}}
	case "dylib":
		picks = []Pick{{Match: "lib/libonnxruntime." + ortVersion + ".dylib", As: "libonnxruntime." + ortVersion + ".dylib"}}
	case "dll":
		picks = []Pick{
			{Match: "lib/onnxruntime.dll", As: "onnxruntime.dll"},
			{Match: "lib/onnxruntime_providers_shared.dll", As: "onnxruntime_providers_shared.dll"},
		}
	}
	return Download{URL: ortBase + file, Archive: archive, Pick: picks, SizeMB: sizeMB}
}
