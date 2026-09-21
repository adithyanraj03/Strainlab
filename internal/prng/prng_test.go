package prng

import (
	"math"
	"testing"
)

// These known-answer values match the VecLab (Rust) crate's KATs exactly —
// the same SplitMix64 stream, verified across two languages and two test
// suites. If this ever drifts, the lab's cross-language determinism broke.
func TestKatSeed0FirstSix(t *testing.T) {
	r := NewRng(0)
	want := []uint64{
		0xe220a8397b1dcdaf,
		0x6e789e6aa1b965f4,
		0x06c45d188009454f,
		0xf88bb8a8724c81ec,
		0x1b39896a51a8749b,
		0x53cb9f0c747ea2ea,
	}
	for i, w := range want {
		if got := r.NextU64(); got != w {
			t.Fatalf("draw %d: got 0x%016x want 0x%016x", i+1, got, w)
		}
	}
	if got := r.State(); got != 0xb54cda58fbbee87e {
		t.Fatalf("state after 6 draws: got 0x%016x want 0xb54cda58fbbee87e", got)
	}
}

func TestKatSeed7FirstFour(t *testing.T) {
	r := NewRng(7)
	want := []uint64{
		0x63cbe1e459320dd7,
		0x044c3cd7f43c661c,
		0xe6984080bab12a02,
		0x953aeb70673e29cb,
	}
	for i, w := range want {
		if got := r.NextU64(); got != w {
			t.Fatalf("draw %d: got 0x%016x want 0x%016x", i+1, got, w)
		}
	}
}

func TestTwoStreamsSameSeedBitIdentical(t *testing.T) {
	a, b := NewRng(1234), NewRng(1234)
	for i := 0; i < 1000; i++ {
		if a.NextU64() != b.NextU64() {
			t.Fatalf("streams diverged at draw %d", i)
		}
	}
}

func TestDifferentSeedsDiffer(t *testing.T) {
	a, b := NewRng(1), NewRng(2)
	same := 0
	for i := 0; i < 64; i++ {
		if a.NextU64() == b.NextU64() {
			same++
		}
	}
	if same > 1 {
		t.Fatalf("seeds 1 and 2 matched %d/64 draws", same)
	}
}

func TestFloatInHalfOpenUnit(t *testing.T) {
	r := NewRng(9)
	for i := 0; i < 200000; i++ {
		v := r.NextFloat()
		if v < 0 || v >= 1 {
			t.Fatalf("draw %d out of range: %v", i, v)
		}
	}
}

func TestFloatMeanNearHalf(t *testing.T) {
	r := NewRng(11)
	var sum float64
	const n = 100000
	for i := 0; i < n; i++ {
		sum += r.NextFloat()
	}
	mean := sum / float64(n)
	if math.Abs(mean-0.5) > 0.005 {
		t.Fatalf("uniform mean %.5f far from 0.5", mean)
	}
}

func TestIntInRange(t *testing.T) {
	r := NewRng(5)
	const n = 97
	for i := 0; i < 100000; i++ {
		v := r.NextInt(n)
		if v < 0 || v >= n {
			t.Fatalf("draw %d out of range: %d", i, v)
		}
	}
}

func TestIntCoversAllResidues(t *testing.T) {
	r := NewRng(3)
	const n = 7
	seen := make([]bool, n)
	for i := 0; i < 7000; i++ {
		seen[r.NextInt(n)] = true
	}
	for v := 0; v < n; v++ {
		if !seen[v] {
			t.Fatalf("residue %d never drawn", v)
		}
	}
}

func TestIntPanicsOnNonPositive(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NextInt(0) did not panic")
		}
	}()
	NewRng(1).NextInt(0)
}

func TestExpMeanNearOne(t *testing.T) {
	r := NewRng(17)
	var sum float64
	const n = 100000
	for i := 0; i < n; i++ {
		sum += r.NextExp()
	}
	mean := sum / float64(n)
	if math.Abs(mean-1.0) > 0.01 {
		t.Fatalf("exponential mean %.4f far from 1.0", mean)
	}
}

func TestExpAlwaysPositive(t *testing.T) {
	r := NewRng(19)
	for i := 0; i < 10000; i++ {
		if v := r.NextExp(); v <= 0 {
			t.Fatalf("exponential draw %d not positive: %v", i, v)
		}
	}
}
