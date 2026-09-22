// The deterministic load simulation: a discrete-event model of seeded
// request arrivals against a pool of identical service workers with a
// FIFO queue. No wall clock, no goroutines — the whole run is a pure
// function of (config, seed), so the report is byte-identical on re-run.
package sim

import (
	"fmt"
	"math"
	"sort"

	"strainlab/internal/prng"
	"strainlab/internal/stats"
)

// Phase is one traffic segment on the virtual timeline.
type Phase struct {
	Name  string
	Ticks int     // virtual ticks the phase lasts
	Rate  float64 // steady-state arrivals per virtual second
	Ramp  bool    // rate rises linearly from 0 to Rate across the phase
}

// SimConfig fully specifies one deterministic load run.
type SimConfig struct {
	Seed   int64
	Tick   float64 // virtual seconds per tick
	Phases []Phase

	Servers    int     // identical service workers
	Base       float64 // base service time, virtual seconds
	Jitter     float64 // service = Base * (1 + Jitter * U)
	ErrorRate  float64 // fraction of requests that end in 500
	SpikeEvery int     // every Nth request is a slow spike (0 = off)
	SpikeMul   float64 // spike service-time multiplier
}

// DemoConfig is the standard configuration the demo and report use:
// sustain runs the pool near saturation (utilization ~0.7) so queueing is
// real but stable, and the spikes carry the tail.
func DemoConfig() SimConfig {
	return SimConfig{
		Seed: 7, Tick: 0.01,
		Phases: []Phase{
			{Name: "ramp", Ticks: 300, Rate: 12, Ramp: true},
			{Name: "sustain", Ticks: 1500, Rate: 12},
			{Name: "soak", Ticks: 1000, Rate: 3},
		},
		Servers:    3,
		Base:       0.12,
		Jitter:     0.5,
		ErrorRate:  0.01,
		SpikeEvery: 50,
		SpikeMul:   8,
	}
}

// Request is one simulated request's fate.
type Request struct {
	Seq      int
	Phase    int
	Arrival  float64
	Complete float64
	Failed   bool
	Service  float64 // virtual seconds the workers spent on this request
}

// Latency in virtual seconds.
func (rq Request) Latency() float64 { return rq.Complete - rq.Arrival }

// Bucket is one time-slice of the latency-over-time series.
type Bucket struct {
	T    float64 // bucket start, virtual seconds
	P95  float64
	Mean float64
	N    int
}

// SimResult is the full outcome of one deterministic run.
type SimResult struct {
	Config   SimConfig
	Reqs     []Request
	Stats    [3]stats.Stats // per phase, aligned with Config.Phases
	Overall  stats.Stats
	Series   []Bucket
	Duration float64
}

// completionEvent is one server slot freeing up.
type completionEvent struct {
	t     float64
	seq   int
	order int // tiebreak: insertion order
}

// Simulate runs the discrete-event model and returns the result.
func Simulate(cfg SimConfig) *SimResult {
	if cfg.Servers <= 0 {
		panic("Simulate: Servers must be >= 1")
	}
	if cfg.SpikeEvery < 0 {
		panic("Simulate: SpikeEvery must be >= 0")
	}
	rng := prng.NewRng(uint64(cfg.Seed))

	// ---- phase geometry -------------------------------------------------
	phaseStart := make([]float64, len(cfg.Phases)) // virtual seconds
	totalTicks := 0
	for i, ph := range cfg.Phases {
		phaseStart[i] = float64(totalTicks) * cfg.Tick
		totalTicks += ph.Ticks
	}
	duration := float64(totalTicks) * cfg.Tick

	// ---- arrival schedule (seeded thinning, deterministic) --------------
	type arrival struct {
		t     float64
		phase int
	}
	var arrs []arrival
	for pi, ph := range cfg.Phases {
		var acc float64
		for tick := 0; tick < ph.Ticks; tick++ {
			rate := ph.Rate
			if ph.Ramp && ph.Ticks > 0 {
				rate = ph.Rate * float64(tick+1) / float64(ph.Ticks)
			}
			lam := rate * cfg.Tick
			acc += lam
			n := int(acc)
			if rng.NextFloat() < acc-float64(n) {
				n++
			}
			acc -= float64(n)
			for k := 0; k < n; k++ {
				// arrival time uniform inside the tick
				off := rng.NextFloat()
				arrs = append(arrs, arrival{
					t:     phaseStart[pi] + float64(tick+1)*cfg.Tick - off*cfg.Tick,
					phase: pi,
				})
			}
		}
	}
	sort.Slice(arrs, func(i, j int) bool {
		if arrs[i].t != arrs[j].t {
			return arrs[i].t < arrs[j].t
		}
		return i < j
	})

	// ---- pre-compute each request's service time and outcome -------------
	svc := make([]float64, len(arrs))
	failed := make([]bool, len(arrs))
	for i := range arrs {
		u := rng.NextFloat()
		s := cfg.Base * (1 + cfg.Jitter*u)
		isSpike := cfg.SpikeEvery > 0 && (i+1)%cfg.SpikeEvery == 0
		if isSpike {
			s *= cfg.SpikeMul
		}
		svc[i] = s
		failed[i] = rng.NextFloat() < cfg.ErrorRate
	}

	// ---- discrete event loop ---------------------------------------------
	// Server slots free at the completion times tracked in `heap` (a sorted
	// slice; n is small, so linear insertion stays clearer than a heap).
	// The FIFO queue is the contiguous block [qHead, ai) of arrivals that
	// arrived while every server was busy; qHead == -1 means the queue is
	// empty. A completion starts the OLDEST queued request, never a later
	// arrival.
	var heap []completionEvent
	order := 0
	free := cfg.Servers
	qHead := -1
	queueLen := 0
	reqs := make([]Request, len(arrs))
	ai := 0
	for ai < len(arrs) || len(heap) > 0 {
		var nextArr float64
		if ai < len(arrs) {
			nextArr = arrs[ai].t
		} else {
			nextArr = math.Inf(1)
		}
		var nextComp float64
		if len(heap) > 0 {
			nextComp = heap[0].t
		} else {
			nextComp = math.Inf(1)
		}

		if nextComp <= nextArr {
			// a server frees up
			ev := heap[0]
			heap = heap[1:]
			if queueLen > 0 {
				j := qHead
				reqs[j] = Request{
					Seq: j, Phase: arrs[j].phase,
					Arrival:  arrs[j].t,
					Complete: ev.t + svc[j],
					Failed:   failed[j],
				}
				heap = pushEvent(heap, completionEvent{t: ev.t + svc[j], seq: j, order: order})
				order++
				queueLen--
				if queueLen == 0 {
					qHead = -1
				} else {
					qHead++
				}
			} else {
				free++
			}
			continue
		}

		// an arrival
		reqs[ai] = Request{Seq: ai, Phase: arrs[ai].phase, Arrival: arrs[ai].t}
		if free > 0 {
			free--
			reqs[ai].Complete = arrs[ai].t + svc[ai]
			reqs[ai].Failed = failed[ai]
			reqs[ai].Service = svc[ai]
			heap = pushEvent(heap, completionEvent{t: arrs[ai].t + svc[ai], seq: ai, order: order})
			order++
		} else {
			if queueLen == 0 {
				qHead = ai
			}
			queueLen++
		}
		ai++
	}
	if queueLen != 0 {
		panic("Simulate: queue not drained (internal error)")
	}

	// ---- aggregate -------------------------------------------------------
	byPhase := make([][]float64, len(cfg.Phases))
	phaseErrs := make([]int, len(cfg.Phases))
	var all []float64
	errs := 0
	for _, rq := range reqs {
		l := rq.Latency()
		byPhase[rq.Phase] = append(byPhase[rq.Phase], l)
		all = append(all, l)
		if rq.Failed {
			phaseErrs[rq.Phase]++
			errs++
		}
	}
	res := &SimResult{
		Config:   cfg,
		Reqs:     reqs,
		Duration: duration,
	}
	for i := range cfg.Phases {
		res.Stats[i] = stats.ComputeStats(byPhase[i], phaseErrs[i])
	}
	res.Overall = stats.ComputeStats(all, errs)

	// latency-over-time series: p95 of requests that ARRIVED per bucket
	const nb = 60
	width := duration / float64(nb)
	perBucket := make([][]float64, nb)
	for _, rq := range reqs {
		b := int(rq.Arrival / width)
		if b >= nb {
			b = nb - 1
		}
		perBucket[b] = append(perBucket[b], rq.Latency())
	}
	series := make([]Bucket, 0, nb)
	last := 0.0
	for b := 0; b < nb; b++ {
		vs := perBucket[b]
		if len(vs) == 0 {
			// carry the previous p95 so the chart line is continuous
			p95 := last
			if b > 0 {
				p95 = series[b-1].P95
			}
			series = append(series, Bucket{T: float64(b) * width, P95: p95, Mean: seriesOrZero(series, b), N: 0})
			continue
		}
		s2 := make([]float64, len(vs))
		copy(s2, vs)
		sort.Float64s(s2)
		p95 := stats.Percentile(s2, 0.95)
		mean := 0.0
		for _, v := range s2 {
			mean += v
		}
		mean /= float64(len(s2))
		series = append(series, Bucket{T: float64(b) * width, P95: p95, Mean: mean, N: len(s2)})
		last = p95
	}
	res.Series = series
	return res
}

func seriesOrZero(series []Bucket, b int) float64 {
	if b > 0 {
		return series[b-1].Mean
	}
	return 0
}

// pushEvent inserts an event keeping the slice sorted by (t, order).
func pushEvent(h []completionEvent, ev completionEvent) []completionEvent {
	h = append(h, ev)
	i := len(h) - 1
	for i > 0 && lessEvent(h[i], h[i-1]) {
		h[i], h[i-1] = h[i-1], h[i]
		i--
	}
	return h
}

func lessEvent(a, b completionEvent) bool {
	if a.t != b.t {
		return a.t < b.t
	}
	return a.order < b.order
}

// PhaseWindow is the [start, end) virtual-time span of a phase.
func (r *SimResult) PhaseWindow(i int) (float64, float64) {
	start := 0.0
	for j := 0; j < i; j++ {
		start += float64(r.Config.Phases[j].Ticks) * r.Config.Tick
	}
	end := start + float64(r.Config.Phases[i].Ticks)*r.Config.Tick
	return start, end
}

// PeakRate is the highest steady-state arrival rate across phases.
func (r *SimResult) PeakRate() float64 {
	peak := 0.0
	for _, ph := range r.Config.Phases {
		if ph.Rate > peak {
			peak = ph.Rate
		}
	}
	return peak
}

// fmtDur renders virtual seconds with a fixed, deterministic format.
func fmtDur(s float64) string {
	if s >= 100 {
		return fmt.Sprintf("%.0f s", s)
	}
	if s >= 1 {
		return fmt.Sprintf("%.2f s", s)
	}
	return fmt.Sprintf("%.1f ms", s*1000)
}
