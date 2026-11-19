package catalog

import (
	"errors"
	"os"
	"runtime"
	"strings"
)

// ErrSandboxed is returned when an engine would be downloaded or added
// from a file in the Mac App Store build.
var ErrSandboxed = errors.New("the Mac App Store version of NoiseFree runs only its built-in engines")

// Sandboxed reports whether NoiseFree runs in the macOS App Sandbox, as
// Mac App Store builds do. The store forbids downloading code (guideline
// 2.5.2) and the sandbox's library validation refuses other developers'
// plugins, so only the engines compiled into the app are offered there.
func Sandboxed() bool {
	return sandboxed(runtime.GOOS, os.Getenv)
}

// sandboxed tells from the environment: macOS doesn't always set
// APP_SANDBOX_CONTAINER_ID, but a sandboxed app's home is always its
// container, ~/Library/Containers/<id>/Data.
func sandboxed(goos string, getenv func(string) string) bool {
	if goos != "darwin" {
		return false
	}
	return getenv("APP_SANDBOX_CONTAINER_ID") != "" || strings.Contains(getenv("HOME"), "/Library/Containers/")
}

// builtinOnly keeps the entries that ship inside the app.
func builtinOnly(entries []Entry) []Entry {
	var kept []Entry
	for _, e := range entries {
		if e.Builtin {
			kept = append(kept, e)
		}
	}
	return kept
}
