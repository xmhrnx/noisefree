// Package catalog lists the noise suppression engines NoiseFree can run, installs
// and removes them, and loads extra engines from JSON manifests.
package catalog

import (
	"runtime"
)

// Kind is the technology an engine is built on, which decides how NoiseFree runs it.
type Kind string

// Engine kinds.
const (
	KindLite    Kind = "lite"    // built-in spectral suppressor
	KindRNNoise Kind = "rnnoise" // built-in RNNoise
	KindLADSPA  Kind = "ladspa"  // a LADSPA plugin library
	KindDTLN    Kind = "dtln"    // DTLN ONNX models, run by ONNX Runtime
	KindRuntime Kind = "runtime" // a support library other engines need
)

// Entry describes one engine or support component. Entries are also the
// format of user manifests (JSON), so the field names are part of that format.
type Entry struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Vendor      string `json:"vendor"`
	Version     string `json:"version"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	License     string `json:"license"`
	Homepage    string `json:"homepage"`
	Kind        Kind   `json:"kind"`
	// Quality and CPUCost rate the engine from 1 to 5, for the store cards.
	Quality int `json:"quality"`
	CPUCost int `json:"cpuCost"`
	// LatencyMs is the delay the engine adds, for display.
	LatencyMs int `json:"latencyMs"`
	// GPU marks engines that can run on a graphics card.
	GPU bool `json:"gpu"`
	// Builtin engines ship inside the app and can't be removed.
	Builtin bool `json:"builtin,omitempty"`
	// Hidden entries (support runtimes) don't show as engines.
	Hidden bool `json:"hidden,omitempty"`
	// Requires lists entry IDs installed along with this one.
	Requires []string `json:"requires,omitempty"`
	// Downloads maps "goos/goarch" (or "goos" or "*") to the files to fetch.
	Downloads map[string][]Download `json:"downloads,omitempty"`
	// Plugin configures KindLADSPA engines.
	Plugin *PluginSpec `json:"plugin,omitempty"`
	// User marks entries loaded from a manifest file.
	User bool `json:"-"`
}

// Download is one file to fetch, optionally an archive to pick files from.
type Download struct {
	URL string `json:"url"`
	// Archive is "zip" (also .nupkg) or "tgz"; empty saves the file as is.
	Archive string `json:"archive,omitempty"`
	// Pick lists files to take from the archive (or the name to save a plain
	// download under).
	Pick []Pick `json:"pick"`
	// SHA256 of the downloaded file; checked when set.
	SHA256 string `json:"sha256,omitempty"`
	// SizeMB is the approximate download size, for display.
	SizeMB int `json:"sizeMB,omitempty"`
}

// Pick selects a file by the end of its path inside an archive.
type Pick struct {
	Match string `json:"match"`
	As    string `json:"as"`
}

// PluginSpec says how to drive a LADSPA engine.
type PluginSpec struct {
	// File is the installed library's name (an As of a Pick).
	File  string `json:"file"`
	Label string `json:"label,omitempty"`
	// StrengthControl is the control port the strength slider drives,
	// mapping 0..1 onto StrengthMin..StrengthMax.
	StrengthControl string             `json:"strengthControl,omitempty"`
	StrengthMin     float32            `json:"strengthMin,omitempty"`
	StrengthMax     float32            `json:"strengthMax,omitempty"`
	Controls        map[string]float32 `json:"controls,omitempty"`
	LatencySamples  int                `json:"latencySamples,omitempty"`
}

// Platform is this computer's download key.
func Platform() string { return runtime.GOOS + "/" + runtime.GOARCH }

// DownloadsHere returns the files to fetch on this computer, and whether
// the entry is available here at all.
func (e *Entry) DownloadsHere() ([]Download, bool) {
	if e.Builtin {
		return nil, true
	}
	for _, key := range []string{Platform(), runtime.GOOS, "*"} {
		if d, ok := e.Downloads[key]; ok {
			return d, true
		}
	}
	return nil, false
}

// Available reports whether the entry can run on this computer.
func (e *Entry) Available() bool {
	_, ok := e.DownloadsHere()
	return ok
}
