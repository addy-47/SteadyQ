package storage

import (
	"time"

	"steadyq/internal/runner"
)

type HistoryItem struct {
	ID        string                    `json:"id"`
	Timestamp time.Time                 `json:"timestamp"`
	Config    runner.Config             `json:"config"`
	Summary   RunSummary                `json:"summary"`
	Results   []runner.ExperimentResult `json:"results"`
}

type RunSummary struct {
	TotalRequests uint64  `json:"total_requests"`
	Success       uint64  `json:"success"`
	Fail          uint64  `json:"fail"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	P99LatencyMs  float64 `json:"p99_latency_ms"`
}
