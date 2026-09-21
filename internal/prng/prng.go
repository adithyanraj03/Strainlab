// SplitMix64 — the same stream family the VecLab crate uses, so the whole
// lab behaves like one instrument. No math/rand: Go's math/rand source
// implementations are version-pinned, but a hand-rolled SplitMix64 is the
// identical 64-bit sequence on every Go release.
package prng

import (
	"math"
)

const (
	smInc = 0x9E3779B97F4A7C15
	smM1  = 0xBF58476D1CE4E5B9
	smM2  = 0x94D049BB133111EB
)

// Rng is a SplitMix64 state, advancing by the golden-ratio increment per draw.
type Rng struct {
	state uint64
}

// NewRng seeds a fresh stream.
func NewRng(seed uint64) *Rng {
	return &Rng{state: seed}
}

// State exposes the current state (fully determines the remaining stream).
func (r *Rng) State() uint64 { return r.state }

// NextU64 returns the next full-64-bit SplitMix64 output.
func (r *Rng) NextU64() uint64 {
	r.state += smInc
	z := r.state
	z = (z ^ (z >> 30)) * smM1
	z = (z ^ (z >> 27)) * smM2
	return z ^ (z >> 31)
}

// NextFloat returns a uniform float64 in [0, 1) with 53 bits of resolution.
func (r *Rng) NextFloat() float64 {
	return float64(r.NextU64()>>11) / float64(1<<53)
}

// NextInt returns a uniform int in [0, n). Panics for n <= 0.
// Rejection sampling on the low 32 bits keeps the bias under 1e-9.
func (r *Rng) NextInt(n int) int {
	if n <= 0 {
		panic("Rng.NextInt(n<=0)")
	}
	for {
		x := int(r.NextU64() & 0xFFFFFFFF)
		// values below t = (-n) mod n are biased and rejected
		t := int((^uint(n)) % uint(n))
		if x >= t {
			return x % n
		}
	}
}

// NextExp returns an exponential(1) sample via the inverse transform.
func (r *Rng) NextExp() float64 {
	u := r.NextFloat()
	if u < 1e-15 {
		u = 1e-15
	}
	return -math.Log(1.0 - u)
}
