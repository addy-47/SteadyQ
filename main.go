package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"steadyq/internal/runner"
	"steadyq/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// CLI Flags
	url := flag.String("url", "", "Target URL")
	rps := flag.Int("rps", 10, "Target RPS (for Open Loop)")
	users := flag.Int("users", 0, "Number of Users (for Closed Loop)")
	thinkTime := flag.Duration("think-time", 0, "Think time per user (e.g. 100ms)")

	duration := flag.Int("duration", 60, "Steady state duration (s)")
	rampUp := flag.Int("ramp-up", 0, "Ramp up duration (s)")
	rampDown := flag.Int("ramp-down", 0, "Ramp down duration (s)")
	out := flag.String("out", "loadtest_report", "Output filename prefix")
	timeout := flag.Int("timeout", 30, "HTTP Timeout (s)")
	flag.Parse()

	if *url == "" {
		fmt.Println("❌ --url required")
		os.Exit(1)
	}

	// Determine Mode
	mode := "rps"
	if *users > 0 {
		mode = "users"
	}

	cfg := runner.Config{
		URL:        *url,
		TargetRPS:  *rps,
		SteadyDur:  *duration,
		RampUp:     *rampUp,
		RampDown:   *rampDown,
		TimeoutSec: *timeout,
		Mode:       mode,
		NumUsers:   *users,
		ThinkTime:  *thinkTime,
	}

	run := runner.NewRunner(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start Runner in Background
	go func() {
		run.Run(ctx)
	}()

	// Start TUI
	totalDur := time.Duration(*rampUp+*duration+*rampDown) * time.Second
	m := tui.NewModel(run, totalDur)
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}

	// Stop runner if TUI quits early
	cancel()

	fmt.Println("\n💾 Generating Reports...")

	// Reporting Logic
	saveCSV(*out, run.Results)
	saveJSON(*out, run)
	saveTimeline(*out, run)

	fmt.Println("✅ Done!")
}

func saveCSV(prefix string, results []runner.ExperimentResult) {
	if prefix == "" {
		return
	}
	file, err := os.Create(prefix + ".csv")
	if err != nil {
		fmt.Printf("Error creating CSV: %v\n", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{
		"timeStamp", "elapsed", "label", "responseCode", "responseMessage",
		"threadName", "success", "bytes", "url",
		"Latency", "ServiceTime", "QueueWait", "Error",
	}) // JMeter compatible-ish

	for _, r := range results {
		msg := "OK"
		if r.Err != nil {
			msg = r.Err.Error()
		}

		writer.Write([]string{
			fmt.Sprintf("%d", r.TimeStamp.UnixMilli()),
			fmt.Sprintf("%d", r.Latency.Milliseconds()),
			"HTTP Request",
			strconv.Itoa(r.Status),
			msg,
			r.UserID,
			strconv.FormatBool(r.Success),
			fmt.Sprintf("%d", r.Bytes),
			"URL", // Placeholder
			fmt.Sprintf("%d", r.Latency.Microseconds()),
			fmt.Sprintf("%d", r.ServiceTime.Microseconds()),
			fmt.Sprintf("%d", r.QueueWait.Microseconds()),
			msg,
		})
	}
}

func saveJSON(prefix string, r *runner.Runner) {
	summary := map[string]interface{}{
		"total_requests": r.Stats.Requests,
		"success":        r.Stats.Success,
		"fail":           r.Stats.Fail,
		"rps_target":     r.Cfg.TargetRPS,
		"p50_service":    r.Stats.ServiceTime.ValueAtQuantile(50),
		"p99_service":    r.Stats.ServiceTime.ValueAtQuantile(99),
		"p99_total":      r.Stats.TotalTime.ValueAtQuantile(99),
	}

	b, _ := json.MarshalIndent(summary, "", "  ")
	os.WriteFile(prefix+"_summary.json", b, 0644)
}

func saveTimeline(prefix string, r *runner.Runner) {
	type TimeBucket struct {
		Timestamp int64 `json:"timestamp"`
		Requests  int   `json:"requests"`
		Errors    int   `json:"errors"`
	}

	buckets := make(map[int64]*TimeBucket)

	for _, res := range r.Results {
		ts := res.TimeStamp.Unix()
		if _, ok := buckets[ts]; !ok {
			buckets[ts] = &TimeBucket{Timestamp: ts}
		}
		b := buckets[ts]
		b.Requests++
		if !res.Success {
			b.Errors++
		}
	}

	var timeline []TimeBucket
	for _, b := range buckets {
		timeline = append(timeline, *b)
	}

	sort.Slice(timeline, func(i, j int) bool {
		return timeline[i].Timestamp < timeline[j].Timestamp
	})

	b, _ := json.MarshalIndent(timeline, "", "  ")
	os.WriteFile(prefix+"_timeline.json", b, 0644)
}
