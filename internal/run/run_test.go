package run

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHTTPAgainstLocalTarget(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		io.WriteString(w, "ok")
	}))
	defer srv.Close()

	res, err := RunHTTP(srv.URL, 10, 2, 4)
	if err != nil {
		t.Fatalf("RunHTTP: %v", err)
	}
	if res.Total != 20 {
		t.Fatalf("total %d want 20", res.Total)
	}
	if res.Errors != 0 {
		t.Fatalf("errors %d want 0", res.Errors)
	}
	if len(res.Samples) != 20 {
		t.Fatalf("samples %d want 20", len(res.Samples))
	}
	for i, s := range res.Samples {
		if s.Seq != i {
			t.Fatalf("sample order broken at %d: seq %d", i, s.Seq)
		}
		if s.Ms < 0 {
			t.Fatalf("sample %d negative latency", i)
		}
	}
}

func TestRunHTTPValidatesArgs(t *testing.T) {
	if _, err := RunHTTP("http://example.invalid", 0, 1, 1); err == nil {
		t.Fatal("rps=0 should error")
	}
	if _, err := RunHTTP("http://example.invalid", 1, 0, 1); err == nil {
		t.Fatal("secs=0 should error")
	}
	if _, err := RunHTTP("http://example.invalid", 1, 1, 0); err == nil {
		t.Fatal("workers=0 should error")
	}
}

func TestRunHTTPJSONL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "ok")
	}))
	defer srv.Close()

	res, err := RunHTTP(srv.URL, 5, 2, 2)
	if err != nil {
		t.Fatalf("RunHTTP: %v", err)
	}
	path := filepath.Join(t.TempDir(), "out.jsonl")
	if err := res.WriteJSONL(path); err != nil {
		t.Fatalf("WriteJSONL: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := 0
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		lines++
		var s RunSample
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			t.Fatalf("line %d not JSON: %v", lines, err)
		}
		if s.Seq < 0 {
			t.Fatalf("line %d bad seq", lines)
		}
	}
	if lines != res.Total {
		t.Fatalf("jsonl lines %d want %d", lines, res.Total)
	}
}

func TestRunHTTPCountsServerErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	res, err := RunHTTP(srv.URL, 5, 1, 2)
	if err != nil {
		t.Fatalf("RunHTTP: %v", err)
	}
	if res.Errors != res.Total || res.Total != 5 {
		t.Fatalf("errors %d of %d want all", res.Errors, res.Total)
	}
}

func TestSortSamplesAscending(t *testing.T) {
	s := []RunSample{{Seq: 3}, {Seq: 1}, {Seq: 2}, {Seq: 0}}
	sortSamples(s)
	for i := 1; i < len(s); i++ {
		if s[i].Seq < s[i-1].Seq {
			t.Fatalf("not sorted at %d: %v", i, s)
		}
	}
}
