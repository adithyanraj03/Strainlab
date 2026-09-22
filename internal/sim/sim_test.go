package sim

import (
	"math"
	"testing"
)

// A single server with constant service 0.1s and arrivals at 1 req/s
// (utilization 0.1) must have p50 ≈ service time. No spikes, no errors.
func TestControlledSingleServer(t *testing.T) {
	cfg := SimConfig{
		Seed: 7, Tick: 0.01,
		Phases:     []Phase{{Name: "flat", Ticks: 500, Rate: 1}},
		Servers:    1,
		Base:       0.1,
		Jitter:     0,
		ErrorRate:  0,
		SpikeEvery: 0,
		SpikeMul:   0,
	}
	r := Simulate(cfg)
	if r.Overall.Count == 0 {
		t.Fatal("no requests")
	}
	if r.Overall.P50 > 0.25 {
		t.Fatalf("p50=%.4f but should be near 0.10", r.Overall.P50)
	}
}

// Two servers, constant service 0.1s, 10 req/s => utilization 0.5.
// p50 should stay near the service time.
func TestControlledTwoServersHalfUtil(t *testing.T) {
	cfg := SimConfig{
		Seed: 7, Tick: 0.01,
		Phases:     []Phase{{Name: "flat", Ticks: 1000, Rate: 10}},
		Servers:    2,
		Base:       0.1,
		Jitter:     0,
		ErrorRate:  0,
		SpikeEvery: 0,
		SpikeMul:   0,
	}
	r := Simulate(cfg)
	if r.Overall.P50 > 0.35 {
		t.Fatalf("p50=%.4f but should be near 0.10", r.Overall.P50)
	}
}

// A queueing discipline check: with 1 server and constant service, every
// later arrival must complete no earlier than the arrival before it.
func TestFIFOBurst(t *testing.T) {
	cfg := SimConfig{
		Seed: 7, Tick: 0.01,
		Phases:     []Phase{{Name: "burst", Ticks: 1, Rate: 1000}},
		Servers:    1,
		Base:       0.1,
		Jitter:     0,
		ErrorRate:  0,
		SpikeEvery: 0,
		SpikeMul:   0,
	}
	r := Simulate(cfg)
	if r.Overall.Count == 0 {
		t.Fatal("no requests")
	}
	for i := 1; i < len(r.Reqs); i++ {
		if r.Reqs[i].Complete < r.Reqs[i-1].Complete {
			t.Fatalf("FIFO violated: req %d complete %.6f < req %d complete %.6f",
				i, r.Reqs[i].Complete, i-1, r.Reqs[i-1].Complete)
		}
	}
}

// The determinism contract: two runs of the standard demo are identical at
// the request level.
func TestDeterminismSameConfigSameSeed(t *testing.T) {
	a := Simulate(DemoConfig())
	b := Simulate(DemoConfig())
	if len(a.Reqs) != len(b.Reqs) {
		t.Fatalf("request counts differ: %d vs %d", len(a.Reqs), len(b.Reqs))
	}
	for i := range a.Reqs {
		if a.Reqs[i] != b.Reqs[i] {
			t.Fatalf("request %d differs: %+v vs %+v", i, a.Reqs[i], b.Reqs[i])
		}
	}
	if a.Overall != b.Overall {
		t.Fatalf("overall stats differ: %+v vs %+v", a.Overall, b.Overall)
	}
}

func TestDifferentSeedsChangeSimulation(t *testing.T) {
	cfg := DemoConfig()
	a := Simulate(cfg)
	cfg.Seed = 8
	b := Simulate(cfg)
	if len(a.Reqs) != len(b.Reqs) {
		t.Fatalf("request counts differ: %d vs %d", len(a.Reqs), len(b.Reqs))
	}
	same := 0
	for i := range a.Reqs {
		if a.Reqs[i] == b.Reqs[i] {
			same++
		}
	}
	if same > len(a.Reqs)/2 {
		t.Fatalf("seeds 7 and 8 agree on %d/%d requests", same, len(a.Reqs))
	}
}

func TestCountsConsistent(t *testing.T) {
	r := Simulate(DemoConfig())
	sum := 0
	for i := range r.Config.Phases {
		if r.Stats[i].Count < 0 {
			t.Fatalf("negative phase count %d", i)
		}
		sum += r.Stats[i].Count
	}
	if sum != r.Overall.Count {
		t.Fatalf("phase sum %d != overall %d", sum, r.Overall.Count)
	}
	if sum != len(r.Reqs) {
		t.Fatalf("phase sum %d != len(reqs) %d", sum, len(r.Reqs))
	}
	for _, rq := range r.Reqs {
		if rq.Phase < 0 || rq.Phase >= len(r.Config.Phases) {
			t.Fatalf("request %d has out-of-range phase %d", rq.Seq, rq.Phase)
		}
	}
}

func TestLatenciesNonNegativeAndComplete(t *testing.T) {
	r := Simulate(DemoConfig())
	for _, rq := range r.Reqs {
		if rq.Complete <= 0 {
			t.Fatalf("request %d never completed (Complete=%v)", rq.Seq, rq.Complete)
		}
		if rq.Complete < rq.Arrival {
			t.Fatalf("request %d completed before arriving: %.6f < %.6f",
				rq.Seq, rq.Complete, rq.Arrival)
		}
		if rq.Latency() < 0 {
			t.Fatalf("request %d negative latency", rq.Seq)
		}
		if rq.Service < 0 {
			t.Fatalf("request %d negative service", rq.Seq)
		}
		// latency = queue wait + service, so it can't be below service
		// (tiny float tolerance for (a+s)-a rounding)
		if rq.Latency() < rq.Service-1e-9 {
			t.Fatalf("request %d latency %.6f < service %.6f",
				rq.Seq, rq.Latency(), rq.Service)
		}
	}
}

// Every Nth request is a spike with service multiplied by SpikeMul.
func TestSpikePlacement(t *testing.T) {
	cfg := DemoConfig()
	cfg.SpikeEvery = 10
	cfg.SpikeMul = 8
	cfg.Jitter = 0 // make the check exact: normal = Base, spike = 8*Base
	cfg.ErrorRate = 0
	r := Simulate(cfg)
	if r.Overall.Count == 0 {
		t.Fatal("no requests")
	}
	for _, rq := range r.Reqs {
		isSpike := (rq.Seq+1)%10 == 0
		if isSpike {
			if math.Abs(rq.Service-cfg.Base*cfg.SpikeMul) > 1e-12 {
				t.Fatalf("spike req %d service %.6f want %.6f",
					rq.Seq, rq.Service, cfg.Base*cfg.SpikeMul)
			}
		} else if math.Abs(rq.Service-cfg.Base) > 1e-12 {
			t.Fatalf("normal req %d service %.6f want %.6f",
				rq.Seq, rq.Service, cfg.Base)
		}
	}
}

// A 50% error rate must produce roughly half failures.
func TestErrorRateApproximate(t *testing.T) {
	cfg := SimConfig{
		Seed: 13, Tick: 0.01,
		Phases:     []Phase{{Name: "flat", Ticks: 500, Rate: 400}},
		Servers:    8,
		Base:       0.01,
		Jitter:     0,
		ErrorRate:  0.5,
		SpikeEvery: 0,
		SpikeMul:   0,
	}
	r := Simulate(cfg)
	if r.Overall.Count < 1500 {
		t.Fatalf("too few requests: %d", r.Overall.Count)
	}
	if float64(r.Overall.Errors) < 0.40*float64(r.Overall.Count) ||
		float64(r.Overall.Errors) > 0.60*float64(r.Overall.Count) {
		t.Fatalf("errors %d/%d far from 50%%", r.Overall.Errors, r.Overall.Count)
	}
}

// A zero-length phase must not break the run.
func TestZeroTickPhase(t *testing.T) {
	cfg := SimConfig{
		Seed: 7, Tick: 0.01,
		Phases: []Phase{
			{Name: "empty", Ticks: 0, Rate: 5},
			{Name: "work", Ticks: 100, Rate: 2},
		},
		Servers:    2,
		Base:       0.05,
		Jitter:     0,
		ErrorRate:  0,
		SpikeEvery: 0,
		SpikeMul:   0,
	}
	r := Simulate(cfg)
	if r.Stats[0].Count != 0 {
		t.Fatalf("empty phase has %d requests", r.Stats[0].Count)
	}
	if r.Stats[1].Count == 0 {
		t.Fatal("work phase has no requests")
	}
	s, e := r.PhaseWindow(1)
	if s != 0 || e <= 0 {
		t.Fatalf("phase window: %v..%v", s, e)
	}
}

// Even an over-utilised system must finish: every request completes and the
// queue drains. (This is the config that looked "broken" before tuning.)
func TestOverutilizedStillDrains(t *testing.T) {
	cfg := SimConfig{
		Seed: 7, Tick: 0.01,
		Phases:     []Phase{{Name: "hot", Ticks: 500, Rate: 15}},
		Servers:    2,
		Base:       0.12,
		Jitter:     0.5,
		ErrorRate:  0,
		SpikeEvery: 40,
		SpikeMul:   10,
	}
	r := Simulate(cfg)
	for _, rq := range r.Reqs {
		if rq.Complete <= 0 {
			t.Fatalf("request %d never completed", rq.Seq)
		}
	}
}

func TestPhaseWindowGeometry(t *testing.T) {
	cfg := SimConfig{
		Seed: 7, Tick: 0.01,
		Phases: []Phase{
			{Name: "a", Ticks: 100, Rate: 1},
			{Name: "b", Ticks: 300, Rate: 1},
			{Name: "c", Ticks: 200, Rate: 1},
		},
		Servers: 2, Base: 0.01, Jitter: 0,
	}
	r := Simulate(cfg)
	s0, e0 := r.PhaseWindow(0)
	s1, e1 := r.PhaseWindow(1)
	s2, e2 := r.PhaseWindow(2)
	if s0 != 0 || e0 != 1.0 {
		t.Fatalf("phase 0 window %v..%v", s0, e0)
	}
	if s1 != 1.0 || e1 != 4.0 {
		t.Fatalf("phase 1 window %v..%v", s1, e1)
	}
	if s2 != 4.0 || e2 != 6.0 {
		t.Fatalf("phase 2 window %v..%v", s2, e2)
	}
	if r.Duration != 6.0 {
		t.Fatalf("duration %v want 6.0", r.Duration)
	}
}

func TestPeakRate(t *testing.T) {
	r := Simulate(DemoConfig())
	if r.PeakRate() != 12 {
		t.Fatalf("peak %v want 12", r.PeakRate())
	}
}

func TestSeriesCoversDuration(t *testing.T) {
	r := Simulate(DemoConfig())
	if len(r.Series) != 60 {
		t.Fatalf("series len %d want 60", len(r.Series))
	}
	if r.Series[0].T != 0 {
		t.Fatalf("first bucket T %v want 0", r.Series[0].T)
	}
	last := r.Series[len(r.Series)-1]
	if last.T >= r.Duration {
		t.Fatalf("last bucket T %v not below duration %v", last.T, r.Duration)
	}
	// every p95 must be non-negative and finite
	for i, bk := range r.Series {
		if math.IsNaN(bk.P95) || math.IsInf(bk.P95, 0) || bk.P95 < 0 {
			t.Fatalf("bucket %d bad p95 %v", i, bk.P95)
		}
	}
}
