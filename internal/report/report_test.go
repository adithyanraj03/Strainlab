package report

import (
	"strings"
	"testing"

	"strainlab/internal/sim"
)

func TestReportStructure(t *testing.T) {
	r := sim.Simulate(sim.DemoConfig())
	html := RenderReport(r)
	for _, want := range []string{
		"Strainlab — Load Report",
		"100% DETERMINISTIC",
		"Phases",
		"Latency over time",
		"Methodology",
		"<svg",
		"<polyline",
		"MIT © 2026 adithyanraj03",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("report missing %q", want)
		}
	}
	// the three phase names, upper-cased in the meta line
	for _, ph := range []string{"RAMP", "SUSTAIN", "SOAK"} {
		if !strings.Contains(html, ph) {
			t.Fatalf("report missing phase %q", ph)
		}
	}
}

func TestReportByteIdentical(t *testing.T) {
	a := RenderReport(sim.Simulate(sim.DemoConfig()))
	b := RenderReport(sim.Simulate(sim.DemoConfig()))
	if a != b {
		t.Fatalf("reports differ: %d vs %d bytes", len(a), len(b))
	}
}

func TestReportDifferentConfigsDiffer(t *testing.T) {
	a := RenderReport(sim.Simulate(sim.DemoConfig()))
	cfg := sim.DemoConfig()
	cfg.Seed = 99
	b := RenderReport(sim.Simulate(cfg))
	if a == b {
		t.Fatal("different configs produced identical reports")
	}
}

func TestReportPhaseNumbersVerbatim(t *testing.T) {
	r := sim.Simulate(sim.DemoConfig())
	html := RenderReport(r)
	// Every reported percentile for the sustain phase must appear verbatim.
	for _, v := range []float64{r.Stats[1].P50, r.Stats[1].P95, r.Stats[1].P99} {
		if !strings.Contains(html, fmtMs(v)) {
			t.Fatalf("sustain value %q missing from report", fmtMs(v))
		}
	}
}

func TestReportNoExternalRefs(t *testing.T) {
	html := RenderReport(sim.Simulate(sim.DemoConfig()))
	for _, bad := range []string{`src="http`, `href="http`, `<script`, `<link`, "url("} {
		if strings.Contains(html, bad) {
			t.Fatalf("report contains external ref %q", bad)
		}
	}
}

func TestReportEmptySamples(t *testing.T) {
	cfg := sim.SimConfig{
		Seed: 7, Tick: 0.01,
		Phases:  []sim.Phase{{Name: "void", Ticks: 0, Rate: 1}},
		Servers: 1, Base: 0.01,
	}
	html := RenderReport(sim.Simulate(cfg))
	if !strings.Contains(html, "(no samples)") {
		t.Fatal("empty run missing '(no samples)' guard")
	}
}

func TestFmtMsFormats(t *testing.T) {
	cases := []struct {
		s    float64
		want string
	}{
		{0, "0 ms"},
		{0.0005, "500.00 µs"},
		{0.1542, "154 ms"},
		{1.39, "1.39 s"},
	}
	for _, c := range cases {
		if got := fmtMs(c.s); got != c.want {
			t.Fatalf("fmtMs(%v) = %q want %q", c.s, got, c.want)
		}
	}
}

func TestFmtPctFormats(t *testing.T) {
	if got := fmtPct(0); got != "0.0%" {
		t.Fatalf("fmtPct(0) = %q", got)
	}
	if got := fmtPct(1.3); got != "1.3%" {
		t.Fatalf("fmtPct(1.3) = %q", got)
	}
}

func TestLatClassGrading(t *testing.T) {
	if got := latClass(0.15, 0.1); got != "good" {
		t.Fatalf("1.5x ratio: got %q want good", got)
	}
	if got := latClass(0.5, 0.1); got != "warn" {
		t.Fatalf("5x ratio: got %q want warn", got)
	}
	if got := latClass(1.5, 0.1); got != "hot" {
		t.Fatalf("15x ratio: got %q want hot", got)
	}
	if got := latClass(0.2, 0); got != "" {
		t.Fatalf("zero p50: got %q want empty", got)
	}
}
