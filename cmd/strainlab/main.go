// strainlab — a seeded, reproducible HTTP load tester.
//
// Two instruments, one rule:
//
//   - the SIMULATION (demo/simulate/report) runs on a virtual clock with a
//     seeded request stream — the result is a pure function of the config,
//     so the report is byte-identical on every run;
//   - the PROBE (serve/run) is a real HTTP target and a real load client,
//     clearly labelled as measured rather than seeded.
//
// stdlib only: no modules, no goroutine-scheduling tricks in the results,
// no network in the test suite.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"strainlab/internal/report"
	"strainlab/internal/run"
	"strainlab/internal/sim"
	"strainlab/internal/target"
)

const version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	switch cmd {
	case "demo":
		runDemo(args, false)
	case "simulate":
		runSimulate(args)
	case "report":
		runReport(args)
	case "serve":
		runServe(args)
	case "version", "--version", "-v":
		fmt.Printf("strainlab v%s\n", version)
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", cmd)
		usage()
		os.Exit(2)
	}
}

func usageText() string {
	return `strainlab v` + version + ` — seeded, reproducible HTTP load tester

usage:
  strainlab demo      the standard simulation, printed as a table
  strainlab simulate  the standard simulation (same as demo)
  strainlab report    render the standard simulation as an HTML report
  strainlab serve     run the deterministic offline HTTP target
  strainlab help      this text
  strainlab version

deterministic simulation flags (simulate), standard defaults shown:
  --seed 7          PRNG seed
  --tick 0.01       virtual seconds per tick
  --servers 2       identical service workers
  --base 0.10       base service time (s)
  --jitter 0.5      multiplicative jitter on service time
  --err 0.01        error rate 0..1
  --spike-every 40  every Nth request is a slow spike (0 = off)
  --spike-mul 10    spike service-time multiplier
  --ramp-ticks 300 / --ramp-rate 12
  --sustain-ticks 1500 / --sustain-rate 12
  --soak-ticks 1000 / --soak-rate 3

offline target (serve):
  --listen string   listen address (default 127.0.0.1:8765)
  --seed int        target seed (default 7)
  --base string     base service duration (default 120ms)
`
}

func usage() {
	fmt.Print(usageText())
}

// ---- deterministic simulation commands ---------------------------------

type simFlags struct {
	seed       int64
	tick       float64
	servers    int
	base       float64
	jitter     float64
	err        float64
	spikeEvery int
	spikeMul   float64
	rampTicks  int
	rampRate   float64
	susTicks   int
	susRate    float64
	soakTicks  int
	soakRate   float64
	out        string
}

func simFlagSet(fs *flag.FlagSet) *simFlags {
	// DemoConfig is the single source of truth for the standard run.
	d := sim.DemoConfig()
	f := &simFlags{}
	fs.Int64Var(&f.seed, "seed", d.Seed, "PRNG seed")
	fs.Float64Var(&f.tick, "tick", d.Tick, "virtual seconds per tick")
	fs.IntVar(&f.servers, "servers", d.Servers, "identical service workers")
	fs.Float64Var(&f.base, "base", d.Base, "base service time (s)")
	fs.Float64Var(&f.jitter, "jitter", d.Jitter, "multiplicative jitter")
	fs.Float64Var(&f.err, "err", d.ErrorRate, "error rate 0..1")
	fs.IntVar(&f.spikeEvery, "spike-every", d.SpikeEvery, "spike every Nth request (0=off)")
	fs.Float64Var(&f.spikeMul, "spike-mul", d.SpikeMul, "spike service-time multiplier")
	fs.IntVar(&f.rampTicks, "ramp-ticks", d.Phases[0].Ticks, "ramp phase ticks")
	fs.Float64Var(&f.rampRate, "ramp-rate", d.Phases[0].Rate, "ramp peak rate (req/s)")
	fs.IntVar(&f.susTicks, "sustain-ticks", d.Phases[1].Ticks, "sustain phase ticks")
	fs.Float64Var(&f.susRate, "sustain-rate", d.Phases[1].Rate, "sustain rate (req/s)")
	fs.IntVar(&f.soakTicks, "soak-ticks", d.Phases[2].Ticks, "soak phase ticks")
	fs.Float64Var(&f.soakRate, "soak-rate", d.Phases[2].Rate, "soak rate (req/s)")
	fs.StringVar(&f.out, "out", "report.html", "report output path")
	return f
}

func (f *simFlags) Config() sim.SimConfig {
	return sim.SimConfig{
		Seed: f.seed, Tick: f.tick,
		Phases: []sim.Phase{
			{Name: "ramp", Ticks: f.rampTicks, Rate: f.rampRate, Ramp: true},
			{Name: "sustain", Ticks: f.susTicks, Rate: f.susRate},
			{Name: "soak", Ticks: f.soakTicks, Rate: f.soakRate},
		},
		Servers:    f.servers,
		Base:       f.base,
		Jitter:     f.jitter,
		ErrorRate:  f.err,
		SpikeEvery: f.spikeEvery,
		SpikeMul:   f.spikeMul,
	}
}

func runDemo(args []string, quiet bool) {
	fs := flag.NewFlagSet("demo", flag.ExitOnError)
	f := simFlagSet(fs)
	fs.Parse(args)
	res := sim.Simulate(f.Config())
	if !quiet {
		printSimTable(res)
	}
}

func runSimulate(args []string) {
	runDemo(args, false)
}

func runReport(args []string) {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	f := simFlagSet(fs)
	fs.Parse(args)
	res := sim.Simulate(f.Config())
	html := report.RenderReport(res)
	if err := os.WriteFile(f.out, []byte(html), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write:", err)
		os.Exit(1)
	}
	fmt.Printf("report written to %s (%d bytes)\n", f.out, len(html))
}

// ---- offline target ------------------------------------------------------

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	listen := fs.String("listen", "127.0.0.1:8765", "listen address")
	seed := fs.Int64("seed", 7, "target seed")
	base := fs.String("base", "120ms", "base service duration")
	fs.Parse(args)
	t := target.NewTargetServer(*seed)
	t.Base = target.ParseDuration(*base)
	fmt.Printf("strainlab v%s target listening on http://%s\n", version, target.ParseAddr(*listen))
	fmt.Printf("  GET /load     deterministic work (seed %d)\n", *seed)
	fmt.Printf("  GET /healthz  liveness\n")
	if err := t.Serve(*listen); err != nil {
		fmt.Fprintln(os.Stderr, "serve:", err)
		os.Exit(1)
	}
}

// ---- output ----------------------------------------------------------------

func printSimTable(r *sim.SimResult) {
	fmt.Printf("strainlab v%s · seeded load simulation · seed %d\n", version, r.Config.Seed)
	fmt.Println(strings.Repeat("─", 78))
	fmt.Printf("%-9s %5s %5s %9s %9s %9s %9s %9s %6s\n",
		"phase", "req", "err", "mean", "p50", "p95", "p99", "max", "rate")
	for i, ph := range r.Config.Phases {
		st := r.Stats[i]
		var rate string
		if ph.Ramp {
			rate = fmt.Sprintf("0→%.0f", ph.Rate)
		} else {
			rate = fmt.Sprintf("%.0f", ph.Rate)
		}
		fmt.Printf("%-9s %5d %5d %9s %9s %9s %9s %9s %6s\n",
			ph.Name, st.Count, st.Errors,
			fmtLat(st.Mean), fmtLat(st.P50), fmtLat(st.P95), fmtLat(st.P99), fmtLat(st.Max), rate)
	}
	o := r.Overall
	fmt.Println(strings.Repeat("─", 78))
	fmt.Printf("overall: %d req · %d err · mean %s · p50 %s · p95 %s · p99 %s · max %s\n",
		o.Count, o.Errors, fmtLat(o.Mean), fmtLat(o.P50), fmtLat(o.P95), fmtLat(o.P99), fmtLat(o.Max))
}

// fmtLat renders seconds for the fixed-width terminal table.
func fmtLat(s float64) string {
	if s <= 0 {
		return "0.0 ms"
	}
	if s < 0.001 {
		return fmt.Sprintf("%.3f µs", s*1e6)
	}
	if s < 1 {
		return fmt.Sprintf("%.1f ms", s*1000)
	}
	return fmt.Sprintf("%.2f s", s)
}
