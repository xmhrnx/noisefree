// Package vmic provides the virtual microphone other apps record from: NoiseFree
// plays its cleaned audio into a virtual output whose other end shows up as
// a microphone in Discord, Zoom, Teams, OBS and the browser.
package vmic

import "strings"

// Status describes the virtual microphone on this computer.
type Status struct {
	// Ready means OutputMatch names an output device NoiseFree can play into.
	Ready bool
	// OutputMatch is a substring of the output device's name.
	OutputMatch string
	// MicName is what users pick as the microphone in other apps.
	MicName string
	// Help explains how to get a virtual microphone when there's none.
	Help string
	// InstallURL points to a free virtual audio driver, when one is needed.
	InstallURL string
	// Managed means NoiseFree created the device itself and removes it on exit.
	Managed bool
}

// Setup prepares the virtual microphone; outputs are the output device
// names currently present, used to detect installed drivers.
func Setup(outputs []string) Status { return setup(outputs) }

// Teardown removes a device Setup created.
func Teardown() { teardown() }

// find returns the first output whose name contains one of the needles.
func find(outputs []string, needles ...string) string {
	for _, o := range outputs {
		for _, n := range needles {
			if strings.Contains(strings.ToLower(o), strings.ToLower(n)) {
				return n
			}
		}
	}
	return ""
}
