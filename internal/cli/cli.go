package cli

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"steadyq/internal/runner"
	"steadyq/internal/tui/app"
)

func Start(cfg runner.Config) {
	printHeader(cfg)

	updates := make(runner.StatsUpdateChan, 100)
	r := runner.NewRunner(cfg, updates)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start Runner
	go r.Run(ctx)

	// Start Monitor Loop
	startTime := time.Now()
	ticker := time.NewTicker(200 * time.Millisecond) // Faster updates for progress bar
	defer ticker.Stop()

	totalDuration := time.Duration(cfg.RampUp+cfg.SteadyDur+cfg.RampDown) * time.Second

	for {
		select {
		case <-updates:
			// Drain updates
		case <-ticker.C:
			elapsed := time.Since(startTime)
			stats := r.Stats
			inflight := atomic.LoadInt64(&r.Inflight)
			rps := 0.0
			if elapsed.Seconds() > 0 {
				rps = float64(stats.Requests) / elapsed.Seconds()
			}

			pct := elapsed.Seconds() / totalDuration.Seconds()
			if pct > 1.0 {
				pct = 1.0
			}

			fmt.Printf("\r%s %3.0f%% | %s/%s | Inf: %3d | RPS: %.1f | OK: %d | Err: %d",
				progressBar(pct, 20), pct*100,
				elapsed.Round(time.Second), totalDuration,
				inflight,
				rps,
				atomic.LoadUint64(&stats.Success),
				atomic.LoadUint64(&stats.Fail),
			)

			if elapsed >= totalDuration {
				if inflight > 0 {
					fmt.Printf("\r%s %3.0f%% | %s/%s | Draining: %d requests...                ",
						progressBar(1.0, 20), 100.0,
						elapsed.Round(time.Second), totalDuration,
						inflight)
					continue
				}
				cancel()
				printSummary(r, elapsed)
				handleAutoReport(r, cfg)
				return
			}
		}
	}
}

func printHeader(cfg runner.Config) {
	fmt.Printf("\n🚀 STARTING STEADYQ LOAD TEST\n")
	fmt.Printf("======================================================================\n")
	fmt.Printf("Target URL : %s\n", cfg.URL)
	fmt.Printf("Method     : %s\n", cfg.Method)
	fmt.Printf("RPS / Users: %d / %d\n", cfg.TargetRPS, cfg.NumUsers)
	fmt.Printf("Duration   : %ds (Steady) + %ds (RampUp) + %ds (RampDown)\n", cfg.SteadyDur, cfg.RampUp, cfg.RampDown)
	fmt.Printf("Timeout    : %ds\n", cfg.TimeoutSec)
	fmt.Printf("======================================================================\n\n")
}

func progressBar(pct float64, width int) string {
	filled := int(pct * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return "[" + strings.Repeat("█", filled) + strings.Repeat("-", width-filled) + "]"
}

func printSummary(r *runner.Runner, totalTime time.Duration) {
	stats := r.Stats
	rps := float64(stats.Requests) / totalTime.Seconds()

	fmt.Printf("\n\n📊 LOAD TEST RESULTS\n")
	fmt.Printf("======================================================================\n")
	fmt.Printf("Total Duration : %s\n", totalTime.Round(time.Second))
	fmt.Printf("Requests Sent  : %d\n", stats.Requests)
	fmt.Printf("Success        : %d\n", stats.Success)
	fmt.Printf("Failures       : %d\n", stats.Fail)
	fmt.Printf("Actual RPS     : %.2f\n", rps)
	fmt.Printf("\n⏱️  RESPONSE TIMES (ms) [Success Only]\n")
	fmt.Printf("   P50 : %.2f\n", stats.GetP50Service())
	fmt.Printf("   P90 : %.2f\n", stats.GetP90Service())
	fmt.Printf("   P95 : %.2f\n", stats.GetP95Service())
	fmt.Printf("   P99 : %.2f\n", stats.GetP99Service())
	fmt.Printf("   Max : %d\n", stats.ServiceTime.Max()/1000)

	errCounts := stats.GetErrorCounts()
	if len(errCounts) > 0 {
		fmt.Printf("\n❌ FAILURE SUMMARY\n")
		for errStr, count := range errCounts {
			fmt.Printf("   %d x %s\n", count, errStr)
		}
	}
	fmt.Printf("======================================================================\n")
}

func handleAutoReport(r *runner.Runner, cfg runner.Config) {
	if cfg.OutPrefix == "" || len(r.Results) == 0 {
		return
	}

	fmt.Printf("\n💾 Generating reports with prefix: %s\n", cfg.OutPrefix)
	app.ExportCSV(r.Results, cfg.OutPrefix+".csv")
	app.ExportJSON(r.Results, cfg.OutPrefix+".json")
	app.ExportSummary(r.Results, cfg.OutPrefix)
	fmt.Printf("✅ Reports saved to %s.{csv,json,_summary.json}\n", cfg.OutPrefix)
}
