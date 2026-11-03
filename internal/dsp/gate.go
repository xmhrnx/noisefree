package dsp

import "math"

// Gate silences the signal between words: it opens when the short-term level
// rises above the threshold and closes smoothly after a hold time.
type Gate struct {
	threshold       float32
	env, gain       float32
	attack, release float32
	envCoef         float32
	hold, holdLeft  int
	floor           float32
}

// NewGate creates a gate at sampleRate with the threshold in dBFS.
func NewGate(sampleRate float64, thresholdDB float32) *Gate {
	g := &Gate{
		attack:  coef(0.002, sampleRate),
		release: coef(0.120, sampleRate),
		envCoef: coef(0.010, sampleRate),
		hold:    int(0.200 * sampleRate),
		floor:   float32(math.Pow(10, -40.0/20)),
		gain:    1,
	}
	g.SetThreshold(thresholdDB)
	return g
}

// SetThreshold sets the opening level in dBFS (for example -50).
func (g *Gate) SetThreshold(db float32) {
	g.threshold = float32(math.Pow(10, float64(db)/20))
}

// Open reports whether the gate is passing audio.
func (g *Gate) Open() bool { return g.gain > 0.5 }

// Process gates buf in place.
func (g *Gate) Process(buf []float32) {
	for i, x := range buf {
		a := x
		if a < 0 {
			a = -a
		}
		g.env += (a - g.env) * (1 - g.envCoef)
		target := g.floor
		if g.env >= g.threshold {
			target, g.holdLeft = 1, g.hold
		} else if g.holdLeft > 0 {
			target = 1
			g.holdLeft--
		}
		c := g.release
		if target > g.gain {
			c = g.attack
		}
		g.gain = target + (g.gain-target)*c
		buf[i] = x * g.gain
	}
}

func coef(seconds, sampleRate float64) float32 {
	return float32(math.Exp(-1 / (seconds * sampleRate)))
}
