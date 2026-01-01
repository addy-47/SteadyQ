package cli

import (
	"context"
	"fmt"
	"time"

	"steadyq/internal/runner"
)

func Start(cfg runner.Config) {
	fmt.Printf("🚀 Starting Headless Load Test\n")
	fmt.Printf("Target: %s [%s]\n", cfg.URL, cfg.Method)
	fmt.Printf("Mode: %s\n", cfg.Mode)
	fmt.Printf("Duration: %ds\n\n", cfg.SteadyDur)

	updates := make(runner.StatsUpdateChan, 100)
	r := runner.NewRunner(cfg, updates)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start Runner
	go r.Run(ctx)

	// Start Monitor Loop
	startTime := time.Now()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// total duration = rampUp + steady + rampDown
	// For CLI, we assume simple steady for now or use cfg values?
	// Config struct has all 3. Flags only set SteadyDur. Let's assume simple run.
	totalDuration := time.Duration(cfg.SteadyDur) * time.Second

	for {
		select {
		case <-updates:
			// Drain updates without blocking
		case <-ticker.C:
		case <-ticker.C:
			elapsed := time.Since(startTime)

			// Print Stats Line
			stats := r.Stats
			rps := 0.0
			if elapsed.Seconds() > 0 {
				rps = float64(stats.Requests) / elapsed.Seconds()
			}

			fmt.Printf("\r[%s] Reqs: %d | RPS: %.1f | P99: %.1fms | Err: %d",
				elapsed.Round(time.Second),
				stats.Requests,
				rps,
				stats.GetP99Service(),
				stats.Fail,
			)

			if elapsed >= totalDuration {
				cancel() // Stop the runner context
				fmt.Printf("\n\n✅ Test Completed.\n")
				return
			}
		}
	}
}
