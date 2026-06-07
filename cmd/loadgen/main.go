// Command loadgen drives load against logpipe in two modes: concurrent HTTP
// traffic against crud-api, and synthetic log lines written to the tailed file.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/vakharwalad23/logpipe/internal/loadgen"
)

func main() {
	var cfg loadgen.Config
	flag.StringVar(&cfg.Mode, "mode", "traffic", "traffic | synthetic")
	flag.StringVar(&cfg.URL, "url", "http://localhost:8080", "target base URL (traffic)")
	flag.IntVar(&cfg.Rate, "rate", 100, "requests/sec (traffic) or lines/sec cap (synthetic, 0 = unbounded)")
	flag.IntVar(&cfg.Concurrency, "concurrency", 50, "max in-flight requests (traffic)")
	flag.DurationVar(&cfg.Duration, "duration", 30*time.Second, "run duration (0 = until count or signal)")
	flag.IntVar(&cfg.Count, "count", 0, "synthetic line count (0 = until duration or signal)")
	flag.StringVar(&cfg.File, "file", "/var/log/logpipe/synthetic.log", "synthetic target file")
	flag.StringVar(&cfg.Stream, "stream", "app", "synthetic stream: app | http")
	flag.Parse()

	explicit := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	if cfg.Mode == "synthetic" && cfg.Count > 0 {
		if !explicit["rate"] {
			cfg.Rate = 0
		}
		if !explicit["duration"] {
			cfg.Duration = 0
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if cfg.Duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.Duration)
		defer cancel()
	}

	var (
		stats loadgen.Stats
		err   error
	)
	switch cfg.Mode {
	case "traffic":
		stats, err = loadgen.RunTraffic(ctx, cfg)
	case "synthetic":
		stats, err = loadgen.RunSynthetic(ctx, cfg)
	default:
		log.Fatalf("unknown mode %q (want traffic | synthetic)", cfg.Mode)
	}
	if err != nil {
		log.Fatalf("loadgen: %v", err)
	}

	printStats(cfg, stats)
}

func printStats(cfg loadgen.Config, s loadgen.Stats) {
	var rate float64
	if s.Elapsed > 0 {
		rate = float64(s.Sent) / s.Elapsed.Seconds()
	}
	fmt.Printf("mode=%s sent=%d errors=%d elapsed=%s rate=%.0f/s\n",
		cfg.Mode, s.Sent, s.Errors, s.Elapsed.Round(time.Millisecond), rate)
	if len(s.Status) > 0 {
		codes := make([]int, 0, len(s.Status))
		for code := range s.Status {
			codes = append(codes, code)
		}
		sort.Ints(codes)
		for _, code := range codes {
			fmt.Printf("  %d: %d\n", code, s.Status[code])
		}
	}
}
