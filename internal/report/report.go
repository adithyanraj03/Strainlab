// The deterministic HTML report. Same Atelier contract as the rest of the
// lab: one self-contained file, no external assets, no script tags,
// byte-identical on re-render for the same (config, seed).
package report

import (
	"fmt"
	"strings"

	"strainlab/internal/sim"
)

const (
	repPaper  = "#F7F6F1"
	repInk    = "#252524"
	repMuted  = "#676662"
	repHair   = "#DCDAD1"
	repGreen  = "#22AC80"
	repIndigo = "#5B51C7"
	repBrick  = "#A74221"
	repOrange = "#E8833A"
	repSerif  = `Georgia, "Times New Roman", serif`
	repSans   = `"Segoe UI", system-ui, -apple-system, sans-serif`
	repMono   = `"Cascadia Code", Consolas, "Courier New", monospace`
)

// RenderReport renders the full load report for one deterministic run.
func RenderReport(r *sim.SimResult) string {
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8">
<title>Strainlab — Load Report</title>
<style>
  body { margin: 0; background: ` + repPaper + `; color: ` + repInk + `; font-family: ` + repSans + `; }
  .wrap { max-width: 980px; margin: 0 auto; padding: 44px 36px 30px; }
  h1 { font-family: ` + repSerif + `; font-size: 34px; margin: 0 0 6px; letter-spacing: 0.2px; }
  .meta { font-size: 11.5px; letter-spacing: 2.4px; text-transform: uppercase; color: ` + repMuted + `; margin-bottom: 30px; }
  .cards { display: flex; gap: 14px; margin: 0 0 34px; }
  .card { flex: 1; background: #fff; border: 1px solid ` + repHair + `; border-radius: 10px; padding: 14px 16px; }
  .card .k { font-size: 10.5px; letter-spacing: 1.8px; text-transform: uppercase; color: ` + repMuted + `; }
  .card .v { font-family: ` + repMono + `; font-size: 21px; margin-top: 6px; }
  h2 { font-family: ` + repSerif + `; font-size: 21px; margin: 34px 0 4px; }
  .sub { font-size: 12.5px; color: ` + repMuted + `; margin: 0 0 12px; }
  table { width: 100%; border-collapse: collapse; background: #fff; border: 1px solid ` + repHair + `; border-radius: 10px; overflow: hidden; }
  th { font-size: 10.5px; letter-spacing: 1.6px; text-transform: uppercase; color: ` + repMuted + `; text-align: left; padding: 9px 12px; border-bottom: 1px solid ` + repHair + `; }
  td { font-family: ` + repMono + `; font-size: 12.5px; padding: 8px 12px; border-bottom: 1px solid ` + repHair + `; }
  tr:last-child td { border-bottom: none; }
  .num { text-align: right; }
  th.num { text-align: right; }
  .good { color: ` + repGreen + `; font-weight: 600; }
  .warn { color: ` + repOrange + `; font-weight: 600; }
  .hot  { color: ` + repBrick + `; font-weight: 600; }
  .phase { display: inline-block; width: 9px; height: 9px; border-radius: 2px; margin-right: 8px; vertical-align: 1px; }
  ul { font-size: 13px; line-height: 1.65; padding-left: 20px; color: ` + repInk + `; }
  li { margin-bottom: 7px; }
  .foot { margin-top: 40px; padding-top: 14px; border-top: 1px solid ` + repHair + `; font-size: 11.5px; color: ` + repMuted + `; letter-spacing: 1px; }
  svg text { font-family: ` + repMono + `; }
</style></head><body><div class="wrap">
`)

	// ---- header ----------------------------------------------------------
	b.WriteString(`<h1>Strainlab — Load Report</h1>`)
	phaseNames := make([]string, len(r.Config.Phases))
	for i, ph := range r.Config.Phases {
		phaseNames[i] = strings.ToUpper(ph.Name)
	}
	b.WriteString(`<div class="meta">` + strings.Join(phaseNames, " + ") +
		` · ` + fmt.Sprintf("%d SERVERS", r.Config.Servers) +
		` · SEED ` + fmt.Sprintf("%d", r.Config.Seed) +
		` · VIRTUAL CLOCK · 100% DETERMINISTIC</div>`)

	// ---- summary cards ---------------------------------------------------
	errPct := 0.0
	if r.Overall.Count > 0 {
		errPct = 100 * float64(r.Overall.Errors) / float64(r.Overall.Count)
	}
	b.WriteString(`<div class="cards">`)
	writeCard(&b, "Requests", fmt.Sprintf("%d", r.Overall.Count))
	writeCard(&b, "Mean latency", fmtMs(r.Overall.Mean))
	writeCard(&b, "p95", fmtMs(r.Overall.P95))
	writeCard(&b, "p99", fmtMs(r.Overall.P99))
	writeCard(&b, "Errors", fmtPct(errPct))
	writeCard(&b, "Peak rate", fmt.Sprintf("%.0f req/s", r.PeakRate()))
	b.WriteString(`</div>`)

	// ---- per-phase table ---------------------------------------------------
	b.WriteString(`<h2>1 · Phases</h2>`)
	b.WriteString(`<p class="sub">one row per phase — percentiles over every request that arrived in the phase</p>`)
	b.WriteString(`<table><tr><th>Phase</th><th class="num">Req</th><th class="num">Err</th><th class="num">Mean</th><th class="num">p50</th><th class="num">p95</th><th class="num">p99</th><th class="num">Max</th><th class="num">Rate</th></tr>`)
	phaseColors := []string{repIndigo, repGreen, repOrange}
	for i, ph := range r.Config.Phases {
		st := r.Stats[i]
		c := phaseColors[i%len(phaseColors)]
		var rateStr string
		if ph.Ramp {
			rateStr = fmt.Sprintf("0→%.0f", ph.Rate)
		} else {
			rateStr = fmt.Sprintf("%.0f", ph.Rate)
		}
		b.WriteString(fmt.Sprintf(
			`<tr><td><span class="phase" style="background:%s"></span>%s</td><td class="num">%d</td><td class="num">%d</td><td class="num">%s</td><td class="num">%s</td><td class="num %s">%s</td><td class="num %s">%s</td><td class="num">%s</td><td class="num">%s</td></tr>`,
			c, ph.Name, st.Count, st.Errors,
			fmtMs(st.Mean), fmtMs(st.P50),
			latClass(st.P95, st.P50), fmtMs(st.P95),
			latClass(st.P99, st.P50), fmtMs(st.P99),
			fmtMs(st.Max), rateStr))
	}
	b.WriteString(fmt.Sprintf(
		`<tr><td><b>overall</b></td><td class="num"><b>%d</b></td><td class="num"><b>%d</b></td><td class="num"><b>%s</b></td><td class="num"><b>%s</b></td><td class="num"><b>%s</b></td><td class="num"><b>%s</b></td><td class="num"><b>%s</b></td><td class="num">—</td></tr>`,
		r.Overall.Count, r.Overall.Errors,
		fmtMs(r.Overall.Mean), fmtMs(r.Overall.P50), fmtMs(r.Overall.P95), fmtMs(r.Overall.P99), fmtMs(r.Overall.Max)))
	b.WriteString(`</table>`)

	// ---- latency over time ---------------------------------------------
	b.WriteString(`<h2>2 · Latency over time</h2>`)
	b.WriteString(`<p class="sub">p95 of the requests arriving in each time slice; shaded bands are the phase windows</p>`)
	b.WriteString(renderTimeSeries(r))

	// ---- methodology -------------------------------------------------------
	b.WriteString(`<h2>3 · Methodology</h2>`)
	b.WriteString(`<ul>`)
	b.WriteString(fmt.Sprintf(
		`<li><b>Deterministic by construction.</b> Arrivals come from a seeded SplitMix64 thinning of a per-tick rate; service times are seeded draws from a fixed distribution; there is no wall clock anywhere in this run. Same config + seed ⇒ byte-identical report.</li>
<li><b>Discrete-event server model.</b> %d identical workers with a FIFO queue: a request's latency is queue-wait + service. Completions start the oldest queued request, in arrival order.</li>
<li><b>Tail pressure is injected, not hoped for.</b> Every %dth request is a %g× service-time spike (the slow path), and %.0f%% of requests end in 500. The p99/p95 gap is therefore a *measured* property of the system under these conditions, not an artifact of a lucky run.</li>
<li><b>Percentiles are type-7</b> (linear interpolation, the numpy default) over every request of the phase; slices with no arrivals carry the previous p95 so the series stays continuous.</li>
<li><b>Zero modules.</b> stdlib only — no router, no template engine, no plotting package. The whole document is one formatted string build, self-contained and byte-identical on re-render.</li></ul>`,
		r.Config.Servers,
		spikeEveryOr(r.Config.SpikeEvery),
		r.Config.SpikeMul, 100*r.Config.ErrorRate))
	b.WriteString(`</ul>`)

	// ---- footer ------------------------------------------------------------
	b.WriteString(`<div class="foot">strainlab v1.0.0 · generated deterministically · MIT © 2026 adithyanraj03</div>`)
	b.WriteString(`</div></body></html>`)
	return b.String()
}

func spikeEveryOr(n int) int {
	if n > 0 {
		return n
	}
	return 1
}

func writeCard(b *strings.Builder, k, v string) {
	b.WriteString(fmt.Sprintf(`<div class="card"><div class="k">%s</div><div class="v">%s</div></div>`, k, v))
}

// latClass grades a percentile against the phase's own p50: within 2× is
// fine, within 6× is warm, beyond is hot.
func latClass(v, p50 float64) string {
	if p50 <= 0 {
		return ""
	}
	ratio := v / p50
	switch {
	case ratio < 2:
		return "good"
	case ratio < 6:
		return "warn"
	default:
		return "hot"
	}
}

func fmtMs(s float64) string {
	if s <= 0 {
		return "0 ms"
	}
	if s < 0.001 {
		return fmt.Sprintf("%.2f µs", s*1e6)
	}
	if s < 1 {
		return fmt.Sprintf("%.0f ms", s*1000)
	}
	return fmt.Sprintf("%.2f s", s)
}

func fmtPct(p float64) string {
	if p == 0 {
		return "0.0%"
	}
	return fmt.Sprintf("%.1f%%", p)
}

// renderTimeSeries draws the p95-over-time SVG with phase shading.
func renderTimeSeries(r *sim.SimResult) string {
	const (
		W    = 908
		H    = 260
		padL = 58
		padR = 18
		padT = 18
		padB = 34
	)
	plotW := float64(W - padL - padR)
	plotH := float64(H - padT - padB)
	dur := r.Duration
	if dur <= 0 {
		return `<p class="sub">(no samples)</p>`
	}
	ymax := 0.0
	for _, bk := range r.Series {
		if bk.P95 > ymax {
			ymax = bk.P95
		}
	}
	if ymax <= 0 {
		ymax = 1.0
	}
	ymax *= 1.15

	x := func(t float64) float64 { return padL + t/dur*plotW }
	y := func(v float64) float64 { return padT + plotH - v/ymax*plotH }

	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<svg width="%d" height="%d" viewBox="0 0 %d %d" style="background:#fff;border:1px solid %s;border-radius:10px">`, W, H, W, H, repHair))

	// phase bands
	phaseColors := []string{repIndigo, repGreen, repOrange}
	for i := range r.Config.Phases {
		s, e := r.PhaseWindow(i)
		c := phaseColors[i%len(phaseColors)]
		b.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%d" fill="%s" opacity="0.055"/>`,
			int(x(s)), padT, int(x(e)-x(s)), int(plotH), c))
	}

	// horizontal gridlines (4)
	for g := 1; g <= 4; g++ {
		v := ymax * float64(g) / 4
		yy := y(v)
		b.WriteString(fmt.Sprintf(`<line x1="%d" y1="%.1f" x2="%d" y2="%.1f" stroke="%s" stroke-dasharray="3 4"/>`,
			padL, yy, W-padR, yy, repHair))
		b.WriteString(fmt.Sprintf(`<text x="%d" y="%.1f" font-size="10" fill="%s" text-anchor="end">%s</text>`,
			padL-8, yy+3, repMuted, fmtMs(v)))
	}

	// p95 polyline
	var pts []string
	for _, bk := range r.Series {
		pts = append(pts, fmt.Sprintf("%.1f,%.1f", x(bk.T), y(bk.P95)))
	}
	b.WriteString(fmt.Sprintf(`<polyline points="%s" fill="none" stroke="%s" stroke-width="2"/>`,
		strings.Join(pts, " "), repBrick))

	// phase boundary labels
	for i := range r.Config.Phases {
		s, e := r.PhaseWindow(i)
		mid := (x(s) + x(e)) / 2
		b.WriteString(fmt.Sprintf(`<text x="%.1f" y="%d" font-size="10" fill="%s" text-anchor="middle" letter-spacing="1.5">%s</text>`,
			mid, H-10, repMuted, strings.ToUpper(r.Config.Phases[i].Name)))
	}
	_ = y // (x, y used above)
	b.WriteString(`</svg>`)
	return b.String()
}
