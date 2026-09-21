// Latency statistics over a fixed sample. Everything is computed on a
// sorted copy; the input order never matters, so results are stable.
package stats

import (
	"sort"
)

// Stats summarises a latency sample in seconds.
type Stats struct {
	Count  int
	Errors int
	Min    float64
	Max    float64
	Mean   float64
	P50    float64
	P95    float64
	P99    float64
}

// ComputeStats summarises `samples` (seconds). errors = how many failed.
func ComputeStats(samples []float64, errors int) Stats {
	st := Stats{Count: len(samples), Errors: errors}
	if len(samples) == 0 {
		return st
	}
	s := make([]float64, len(samples))
	copy(s, samples)
	sort.Float64s(s)
	st.Min = s[0]
	st.Max = s[len(s)-1]
	var sum float64
	for _, v := range s {
		sum += v
	}
	st.Mean = sum / float64(len(s))
	st.P50 = Percentile(s, 0.50)
	st.P95 = Percentile(s, 0.95)
	st.P99 = Percentile(s, 0.99)
	return st
}

// Percentile returns the p-quantile (p in [0,1]) of a pre-sorted sample
// using linear interpolation (type 7, the numpy default).
func Percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 {
		return sorted[0]
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[n-1]
	}
	idx := (float64(n) - 1) * p
	lo := int(idx)
	frac := idx - float64(lo)
	if lo+1 >= n {
		return sorted[n-1]
	}
	return sorted[lo] + (sorted[lo+1]-sorted[lo])*frac
}
