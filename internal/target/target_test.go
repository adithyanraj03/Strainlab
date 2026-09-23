package target

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func get(t *testing.T, srv *httptest.Server, path string) (int, string) {
	t.Helper()
	resp, err := http.Get(srv.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func TestHealthz(t *testing.T) {
	tg := NewTargetServer(7)
	srv := httptest.NewServer(tg.Handler())
	defer srv.Close()
	code, body := get(t, srv, "/healthz")
	if code != 200 || body != "ok" {
		t.Fatalf("healthz: %d %q", code, body)
	}
}

func TestLoadReturnsWork(t *testing.T) {
	tg := NewTargetServer(7)
	tg.Base = time.Millisecond // keep the test fast
	srv := httptest.NewServer(tg.Handler())
	defer srv.Close()
	code, body := get(t, srv, "/load")
	if code != 200 {
		t.Fatalf("load: status %d", code)
	}
	if body != "work done (index 1)" {
		t.Fatalf("load body %q", body)
	}
	if got := tg.Count.Load(); got != 1 {
		t.Fatalf("count %d want 1", got)
	}
}

func TestLoadFailureInjection(t *testing.T) {
	tg := NewTargetServer(7)
	tg.Base = time.Millisecond
	tg.FailMod = 3 // every 3rd request 500s
	srv := httptest.NewServer(tg.Handler())
	defer srv.Close()
	for i := 1; i <= 6; i++ {
		code, _ := get(t, srv, "/load")
		want := 200
		if i%3 == 0 {
			want = 500
		}
		if code != want {
			t.Fatalf("request %d: status %d want %d", i, code, want)
		}
	}
}

func TestTargetBehaviourIsDeterministicPerIndex(t *testing.T) {
	// Two identically-seeded targets must agree on the status of every
	// request index — behaviour is a pure function of (seed, index).
	run := func() []int {
		tg := NewTargetServer(42)
		tg.Base = time.Millisecond
		tg.FailMod = 5
		tg.SpikeEvery = 0 // keep it fast
		srv := httptest.NewServer(tg.Handler())
		defer srv.Close()
		codes := make([]int, 12)
		for i := range codes {
			codes[i], _ = get(t, srv, "/load")
		}
		return codes
	}
	a, b := run(), run()
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("index %d: %d vs %d", i, a[i], b[i])
		}
	}
	// index 5 and 10 must be the injected 500s
	if a[4] != 500 || a[9] != 500 {
		t.Fatalf("expected 500s at indices 5,10: %v", a)
	}
}

func TestParseDuration(t *testing.T) {
	if got := ParseDuration("120ms"); got != 120*time.Millisecond {
		t.Fatalf("120ms: %v", got)
	}
	if got := ParseDuration("1s"); got != time.Second {
		t.Fatalf("1s: %v", got)
	}
	if got := ParseDuration("0.12"); got != 120*time.Millisecond {
		t.Fatalf("0.12: %v", got)
	}
	if got := ParseDuration("junk"); got != 120*time.Millisecond {
		t.Fatalf("junk: %v", got)
	}
}

func TestStrIntStrFloatFallbacks(t *testing.T) {
	if got := StrInt("42", 3); got != 42 {
		t.Fatalf("StrInt 42: %d", got)
	}
	if got := StrInt("junk", 3); got != 3 {
		t.Fatalf("StrInt fallback: %d", got)
	}
	if got := StrFloat("2.5", 1); got != 2.5 {
		t.Fatalf("StrFloat 2.5: %v", got)
	}
	if got := StrFloat("junk", 1); got != 1 {
		t.Fatalf("StrFloat fallback: %v", got)
	}
}

func TestParseAddr(t *testing.T) {
	if got := ParseAddr(""); got != "127.0.0.1:8765" {
		t.Fatalf("default addr: %q", got)
	}
	if got := ParseAddr("127.0.0.1:9999"); got != "127.0.0.1:9999" {
		t.Fatalf("explicit addr: %q", got)
	}
}
