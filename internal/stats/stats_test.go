package stats

import (
	"math"
	"testing"
)

func TestPercentileEmptyIsZero(t *testing.T) {
	if got := Percentile(nil, 0.5); got != 0 {
		t.Fatalf("empty: got %v", got)
	}
}

func TestPercentileSingle(t *testing.T) {
	if got := Percentile([]float64{42}, 0.9); got != 42 {
		t.Fatalf("single: got %v", got)
	}
}

func TestPercentileEndpoints(t *testing.T) {
	s := []float64{3, 1, 4, 1, 5} // sorted: 1 1 3 4 5
	sortCopy := func() []float64 {
		s2 := append([]float64(nil), s...)
		for i := 1; i < len(s2); i++ {
			for j := i; j > 0 && s2[j] < s2[j-1]; j-- {
				s2[j], s2[j-1] = s2[j-1], s2[j]
			}
		}
		return s2
	}
	got := Percentile(sortCopy(), 0)
	if got != 1 {
		t.Fatalf("p0: got %v want 1", got)
	}
	got = Percentile(sortCopy(), 1)
	if got != 5 {
		t.Fatalf("p1: got %v want 5", got)
	}
}

func TestPercentileKnown(t *testing.T) {
	s := make([]float64, 10)
	for i := range s {
		s[i] = float64(i + 1) // 1..10
	}
	cases := []struct {
		p, want float64
	}{
		{0.50, 5.5},
		{0.95, 9.55},
		{0.99, 9.91},
		{0.10, 1.9},
	}
	for _, c := range cases {
		if got := Percentile(s, c.p); math.Abs(got-c.want) > 1e-9 {
			t.Fatalf("p%.2f: got %v want %v", c.p, got, c.want)
		}
	}
}

func TestPercentileLinearInterpolation(t *testing.T) {
	if got := Percentile([]float64{10, 20}, 0.5); got != 15 {
		t.Fatalf("interp: got %v want 15", got)
	}
}

func TestComputeStatsKnown(t *testing.T) {
	st := ComputeStats([]float64{1, 2, 3, 4, 5}, 0)
	if st.Count != 5 || st.Min != 1 || st.Max != 5 {
		t.Fatalf("basic: %+v", st)
	}
	if math.Abs(st.Mean-3) > 1e-9 || math.Abs(st.P50-3) > 1e-9 {
		t.Fatalf("mean/p50: %+v", st)
	}
}

func TestComputeStatsEmpty(t *testing.T) {
	st := ComputeStats(nil, 0)
	if st.Count != 0 || st.Min != 0 || st.Max != 0 || st.Mean != 0 ||
		st.P50 != 0 || st.P95 != 0 || st.P99 != 0 {
		t.Fatalf("empty: %+v", st)
	}
}

func TestComputeStatsErrorsPassThrough(t *testing.T) {
	st := ComputeStats([]float64{1, 2, 3}, 2)
	if st.Errors != 2 {
		t.Fatalf("errors: %+v", st)
	}
}

func TestComputeStatsInputOrderInsensitive(t *testing.T) {
	a := ComputeStats([]float64{5, 1, 4, 2, 3}, 0)
	b := ComputeStats([]float64{1, 2, 3, 4, 5}, 0)
	if a != b {
		t.Fatalf("order sensitive: %+v vs %+v", a, b)
	}
}

func TestComputeStatsSingleSample(t *testing.T) {
	st := ComputeStats([]float64{0.42}, 0)
	if st.Min != 0.42 || st.Max != 0.42 || st.Mean != 0.42 ||
		st.P50 != 0.42 || st.P95 != 0.42 || st.P99 != 0.42 {
		t.Fatalf("single: %+v", st)
	}
}
