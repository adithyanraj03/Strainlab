package main

import (
	"flag"
	"reflect"
	"strings"
	"testing"

	"strainlab/internal/sim"
)

func TestUsageListsCommands(t *testing.T) {
	want := []string{"demo", "simulate", "report", "serve", "run", "version", "help"}
	for _, w := range want {
		if !strings.Contains(usageText(), w) {
			t.Fatalf("usage text missing command %q", w)
		}
	}
}

func TestVersionConstant(t *testing.T) {
	if version != "1.0.0" {
		t.Fatalf("version %q", version)
	}
}

func TestSimFlagDefaultsMatchDemoConfig(t *testing.T) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	f := simFlagSet(fs)
	d := sim.DemoConfig()
	if f.seed != d.Seed {
		t.Fatalf("seed %d want %d", f.seed, d.Seed)
	}
	if f.tick != d.Tick {
		t.Fatalf("tick %v want %v", f.tick, d.Tick)
	}
	if f.servers != d.Servers {
		t.Fatalf("servers %d want %d", f.servers, d.Servers)
	}
	if f.base != d.Base {
		t.Fatalf("base %v want %v", f.base, d.Base)
	}
	if f.spikeEvery != d.SpikeEvery {
		t.Fatalf("spikeEvery %d want %d", f.spikeEvery, d.SpikeEvery)
	}
	if f.rampTicks != d.Phases[0].Ticks || f.rampRate != d.Phases[0].Rate {
		t.Fatalf("ramp %d/%v want %d/%v", f.rampTicks, f.rampRate, d.Phases[0].Ticks, d.Phases[0].Rate)
	}
	if f.susTicks != d.Phases[1].Ticks || f.susRate != d.Phases[1].Rate {
		t.Fatalf("sustain %d/%v want %d/%v", f.susTicks, f.susRate, d.Phases[1].Ticks, d.Phases[1].Rate)
	}
	if f.soakTicks != d.Phases[2].Ticks || f.soakRate != d.Phases[2].Rate {
		t.Fatalf("soak %d/%v want %d/%v", f.soakTicks, f.soakRate, d.Phases[2].Ticks, d.Phases[2].Rate)
	}
	// and the assembled config must equal the demo config
	if got := f.Config(); !reflect.DeepEqual(got, d) {
		t.Fatalf("assembled config differs:\n got %+v\nwant %+v", got, d)
	}
}
