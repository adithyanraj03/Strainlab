// The offline HTTP target: a real server with deterministic behaviour.
// Response time is a pure function of the request's 1-based index on a
// seeded stream, so a real run against it is reproducible per-index even
// though wall-clock timing obviously varies.
package target

import (
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"strainlab/internal/prng"
)

// TargetServer serves deterministic workloads on 127.0.0.1.
type TargetServer struct {
	Seed       int64
	Base       time.Duration
	Jitter     float64
	SpikeEvery int
	SpikeMul   time.Duration
	FailMod    int // every Nth request 500s (0 = never)
	Count      atomic.Uint64
}

// NewTargetServer builds the standard offline target: ~120 ms base work,
// 50% jitter, a 10× spike every 40th request, a 500 every 97th.
func NewTargetServer(seed int64) *TargetServer {
	return &TargetServer{
		Seed: seed, Base: 120 * time.Millisecond, Jitter: 0.5,
		SpikeEvery: 40, SpikeMul: 10 * time.Second, FailMod: 97,
	}
}

// Handler wires the routes onto a mux.
func (t *TargetServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", t.root)
	mux.HandleFunc("/load", t.load)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "ok")
	})
	return mux
}

func (t *TargetServer) root(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprint(w, "strainlab target · /load does the work · /healthz pings")
}

// load simulates work: sleep a seeded duration, then answer.
func (t *TargetServer) load(w http.ResponseWriter, _ *http.Request) {
	i := t.Count.Add(1)
	rng := prng.NewRng(uint64(t.Seed) + i*0x9E3779B97F4A7C15)
	d := time.Duration(float64(t.Base) * (1 + t.Jitter*rng.NextFloat()))
	if t.SpikeEvery > 0 && i%uint64(t.SpikeEvery) == 0 {
		d += t.SpikeMul
	}
	time.Sleep(d)
	if t.FailMod > 0 && i%uint64(t.FailMod) == 0 {
		http.Error(w, "injected failure", http.StatusInternalServerError)
		return
	}
	fmt.Fprintf(w, "work done (index %d)", i)
}

// Serve blocks serving until the listener fails.
func (t *TargetServer) Serve(addr string) error {
	return http.ListenAndServe(addr, t.Handler())
}

// ParseAddr parses a listen address or returns a fixed default.
func ParseAddr(s string) string {
	if s == "" {
		return "127.0.0.1:8765"
	}
	return s
}

// ParseDuration parses a duration flag value ("120ms", "1s", "0.12").
func ParseDuration(s string) time.Duration {
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	if f, err := parseFloat(s); err == nil {
		return time.Duration(f * 1e9)
	}
	return 120 * time.Millisecond
}

func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%g", &f)
	return f, err
}

// StrInt parses an int flag value with a fallback.
func StrInt(s string, fallback int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return fallback
}

// StrFloat parses a float flag value with a fallback.
func StrFloat(s string, fallback float64) float64 {
	var f float64
	if _, err := fmt.Sscanf(s, "%g", &f); err == nil {
		return f
	}
	return fallback
}
