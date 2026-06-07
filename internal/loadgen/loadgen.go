// Package loadgen drives load against the system in two modes: concurrent HTTP
// traffic aimed at crud-api at a configurable rate, and synthetic log lines
// written straight to the tailed log file to reach high volume quickly.
package loadgen

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Config struct {
	Mode        string
	URL         string
	Rate        int
	Concurrency int
	Duration    time.Duration
	Count       int
	File        string
	Stream      string
}

type Stats struct {
	Sent    int64
	Errors  int64
	Status  map[int]int64
	Elapsed time.Duration
}

func RunTraffic(ctx context.Context, c Config) (Stats, error) {
	if c.Rate <= 0 {
		return Stats{}, fmt.Errorf("rate must be > 0, got %d", c.Rate)
	}
	if c.Concurrency <= 0 {
		c.Concurrency = 1
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        c.Concurrency * 2,
			MaxIdleConnsPerHost: c.Concurrency * 2,
		},
	}

	var (
		sent   int64
		errs   int64
		mu     sync.Mutex
		status = make(map[int]int64)
	)
	record := func(code int, err error) {
		atomic.AddInt64(&sent, 1)
		if err != nil {
			atomic.AddInt64(&errs, 1)
			return
		}
		mu.Lock()
		status[code]++
		mu.Unlock()
	}

	d := newDriver(strings.TrimRight(c.URL, "/"), client, record)

	interval := time.Second / time.Duration(c.Rate)
	if interval <= 0 {
		interval = time.Nanosecond
	}
	tick := time.NewTicker(interval)
	defer tick.Stop()

	sem := make(chan struct{}, c.Concurrency)
	var wg sync.WaitGroup
	start := time.Now()

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return Stats{
				Sent:    atomic.LoadInt64(&sent),
				Errors:  atomic.LoadInt64(&errs),
				Status:  status,
				Elapsed: time.Since(start),
			}, nil
		case <-tick.C:
			select {
			case sem <- struct{}{}:
			default:
				continue
			}
			wg.Go(func() {
				defer func() { <-sem }()
				d.do(ctx)
			})
		}
	}
}

type driver struct {
	base   string
	client *http.Client
	record func(int, error)

	mu  sync.Mutex
	ids []string
}

func newDriver(base string, client *http.Client, record func(int, error)) *driver {
	return &driver{base: base, client: client, record: record}
}

func (d *driver) do(ctx context.Context) {
	switch n := rand.Intn(100); {
	case n < 50:
		d.create(ctx)
	case n < 80:
		d.list(ctx)
	case n < 90:
		d.update(ctx)
	default:
		d.remove(ctx)
	}
}

func (d *driver) send(req *http.Request) (int, []byte, error) {
	resp, err := d.client.Do(req)
	if err != nil {
		d.record(0, err)
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	d.record(resp.StatusCode, nil)
	return resp.StatusCode, data, nil
}

func (d *driver) create(ctx context.Context) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.base+"/tasks", strings.NewReader(`{"title":"loadgen task"}`))
	if err != nil {
		d.record(0, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	code, data, err := d.send(req)
	if err != nil || code != http.StatusCreated {
		return
	}
	var t struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(data, &t) == nil && t.ID != "" {
		d.mu.Lock()
		d.ids = append(d.ids, t.ID)
		d.mu.Unlock()
	}
}

func (d *driver) list(ctx context.Context) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.base+"/tasks", nil)
	if err != nil {
		d.record(0, err)
		return
	}
	d.send(req)
}

func (d *driver) update(ctx context.Context) {
	id := d.randID()
	if id == "" {
		d.create(ctx)
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, d.base+"/tasks/"+id, strings.NewReader(`{"done":true}`))
	if err != nil {
		d.record(0, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	d.send(req)
}

func (d *driver) remove(ctx context.Context) {
	id := d.randID()
	if id == "" {
		d.create(ctx)
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, d.base+"/tasks/"+id, nil)
	if err != nil {
		d.record(0, err)
		return
	}
	d.send(req)
}

func (d *driver) randID() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.ids) == 0 {
		return ""
	}
	return d.ids[rand.Intn(len(d.ids))]
}

func RunSynthetic(ctx context.Context, c Config) (Stats, error) {
	if c.Stream != "app" && c.Stream != "http" {
		return Stats{}, fmt.Errorf("stream must be \"app\" or \"http\", got %q", c.Stream)
	}
	if c.File == "" {
		return Stats{}, fmt.Errorf("synthetic mode requires a file")
	}

	f, err := os.OpenFile(c.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return Stats{}, fmt.Errorf("open synthetic file %q: %w", c.File, err)
	}
	defer f.Close()

	w := bufio.NewWriterSize(f, 1<<20)
	enc := json.NewEncoder(w)

	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "loadgen"
	}

	var limiter *time.Ticker
	if c.Rate > 0 {
		interval := time.Second / time.Duration(c.Rate)
		if interval <= 0 {
			interval = time.Nanosecond
		}
		limiter = time.NewTicker(interval)
		defer limiter.Stop()
	}

	var sent, errs int64
	start := time.Now()

loop:
	for i := 0; c.Count == 0 || i < c.Count; i++ {
		if limiter != nil {
			select {
			case <-ctx.Done():
				break loop
			case <-limiter.C:
			}
		} else {
			select {
			case <-ctx.Done():
				break loop
			default:
			}
		}
		if err := enc.Encode(buildLine(c.Stream, host, i)); err != nil {
			errs++
			continue
		}
		sent++
	}

	if err := w.Flush(); err != nil {
		return Stats{Sent: sent, Errors: errs, Elapsed: time.Since(start)}, fmt.Errorf("flush synthetic file: %w", err)
	}
	if err := f.Sync(); err != nil {
		return Stats{Sent: sent, Errors: errs, Elapsed: time.Since(start)}, fmt.Errorf("sync synthetic file: %w", err)
	}
	return Stats{Sent: sent, Errors: errs, Elapsed: time.Since(start)}, nil
}

func buildLine(stream, host string, i int) map[string]any {
	now := time.Now().UTC()
	if stream == "http" {
		methods := []string{"GET", "POST", "PUT", "DELETE"}
		statuses := []int{200, 201, 204, 404}
		return map[string]any{
			"timestamp":   now.Format(time.RFC3339),
			"log_type":    "http",
			"host":        host,
			"method":      methods[rand.Intn(len(methods))],
			"path":        "/tasks",
			"status":      statuses[rand.Intn(len(statuses))],
			"duration_ms": rand.Float64() * 0.05,
			"bytes":       rand.Intn(512),
			"client_ip":   "127.0.0.1",
			"user_agent":  "loadgen",
		}
	}
	levels := []string{"INFO", "INFO", "INFO", "WARN"}
	return map[string]any{
		"timestamp": now.Format(time.RFC3339Nano),
		"level":     levels[rand.Intn(len(levels))],
		"log_type":  "app",
		"service":   "crud-api",
		"host":      host,
		"message":   "synthetic event",
		"task_id":   fmt.Sprintf("%d", i),
		"title":     "synthetic task",
	}
}
