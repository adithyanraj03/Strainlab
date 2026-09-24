// Real HTTP load runs: wall-clock, measured, NOT seeded. The deterministic
// simulation (sim.go) is the instrument; this is the probe you wave at a
// real endpoint. JSONL output keeps every sample for later inspection.
package run

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"strainlab/internal/stats"
)

// RunSample is one measured request, as written to the JSONL file.
type RunSample struct {
	Seq    int     `json:"seq"`
	T      float64 `json:"t_s"`
	Ms     float64 `json:"ms"`
	Status int     `json:"status"`
	Err    string  `json:"error,omitempty"`
}

// RunResult summarises a real HTTP run.
type RunResult struct {
	URL      string
	Total    int
	Errors   int
	Duration float64
	Stats    stats.Stats
	Samples  []RunSample
}

type job struct {
	seq    int
	startT float64
}

// RunHTTP drives a measured load of `rps` requests/second against `url`
// for `secs` seconds through a pool of `workers` goroutines. It is
// real-time by design; nothing here is deterministic, and it says so.
func RunHTTP(url string, rps, secs float64, workers int) (*RunResult, error) {
	if rps <= 0 || secs <= 0 || workers <= 0 {
		return nil, fmt.Errorf("rps, secs and workers must all be > 0")
	}
	client := &http.Client{Timeout: 30 * time.Second}
	ch := make(chan job, workers*4)

	var mu sync.Mutex
	var samples []RunSample
	res := &RunResult{URL: url}

	start := time.Now()
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				t0 := time.Now()
				req, err := http.NewRequest(http.MethodGet, url, nil)
				if err != nil {
					record(res, &mu, &samples, RunSample{Seq: j.seq, T: j.startT, Err: err.Error()})
					continue
				}
				resp, err := client.Do(req)
				ms := float64(time.Since(t0)) / 1e6
				if err != nil {
					record(res, &mu, &samples, RunSample{Seq: j.seq, T: j.startT, Ms: ms, Err: err.Error()})
					continue
				}
				body := make([]byte, 512)
				_, _ = resp.Body.Read(body)
				_ = resp.Body.Close()
				record(res, &mu, &samples, RunSample{Seq: j.seq, T: j.startT, Ms: ms, Status: resp.StatusCode})
			}
		}()
	}

	// dispatcher: one request every 1/rps seconds
	total := int(rps * secs)
	interval := time.Duration(1e9 / rps)
	for i := 0; i < total; i++ {
		deadline := start.Add(time.Duration(i) * interval)
		if now := time.Now(); now.Before(deadline) {
			time.Sleep(deadline.Sub(now))
		}
		ch <- job{seq: i, startT: float64(time.Since(start)) / 1e9}
	}
	close(ch)
	wg.Wait()
	res.Duration = float64(time.Since(start)) / 1e9
	res.Total = total

	mu.Lock()
	samples = append([]RunSample(nil), samples...)
	mu.Unlock()
	sortSamples(samples)
	res.Samples = samples
	var lat []float64
	errs := 0
	for _, s := range samples {
		if s.Err != "" || s.Status >= 500 {
			errs++
			continue
		}
		lat = append(lat, s.Ms/1000)
	}
	res.Errors = errs
	res.Stats = stats.ComputeStats(lat, errs)
	return res, nil
}

func record(res *RunResult, mu *sync.Mutex, samples *[]RunSample, s RunSample) {
	mu.Lock()
	*samples = append(*samples, s)
	mu.Unlock()
}

func sortSamples(s []RunSample) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j].Seq < s[j-1].Seq; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// WriteJSONL appends the samples to a JSONL file (one sample per line).
func (r *RunResult) WriteJSONL(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, s := range r.Samples {
		if err := enc.Encode(s); err != nil {
			return err
		}
	}
	return nil
}
