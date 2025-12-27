package runner

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"steadyq/internal/stats"

	"github.com/google/uuid"
)

var questions = []string{
	"What is rainwater harvesting?",
	"Explain POSH act",
	"How does BigQuery work?",
	"What is RAG in AI?",
	"Explain vector search",
	"What is Karmayogi Bharat?",
	"How does Gemini LLM work?",
	"Explain cosine similarity",
	"What is cloud storage?",
	"What is an embedding model?",
}

type Runner struct {
	Cfg     Config
	Stats   *stats.Stats
	Client  *http.Client
	Results []ExperimentResult
	mu      sync.Mutex

	inflight int64
}

func NewRunner(cfg Config) *Runner {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 2000
	t.MaxConnsPerHost = 2000
	t.MaxIdleConnsPerHost = 2000
	t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	client := &http.Client{
		Timeout:   time.Duration(cfg.TimeoutSec) * time.Second,
		Transport: t,
	}

	return &Runner{
		Cfg:    cfg,
		Stats:  stats.NewStats(),
		Client: client,
	}
}

func (r *Runner) Run(ctx context.Context) {
	if r.Cfg.Mode == "users" {
		r.runUsers(ctx)
	} else {
		r.runRPS(ctx)
	}
}

// runUsers implements Closed-Loop model (Fixed Concurrency)
func (r *Runner) runUsers(ctx context.Context) {
	var wg sync.WaitGroup
	start := time.Now()
	totalDur := time.Duration(r.Cfg.RampUp+r.Cfg.SteadyDur+r.Cfg.RampDown) * time.Second

	for i := 0; i < r.Cfg.NumUsers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					if time.Since(start) > totalDur {
						return
					}
					// In closed loop, scheduled time is "now" (whenever worker is free)
					// So QueueWait is effectively 0 unless we measure something deeper.
					// We'll pass time.Now() as scheduled time.
					r.executeRequest(time.Now())
					if r.Cfg.ThinkTime > 0 {
						time.Sleep(r.Cfg.ThinkTime)
					}
				}
			}
		}()
	}
	wg.Wait()
}

// runRPS implements Open-Loop model (Constant Arrival Rate)
func (r *Runner) runRPS(ctx context.Context) {
	start := time.Now()
	totalDur := time.Duration(r.Cfg.RampUp+r.Cfg.SteadyDur+r.Cfg.RampDown) * time.Second

	var wg sync.WaitGroup
	nextRequestTime := start

	for {
		select {
		case <-ctx.Done():
			return
		default:
			now := time.Now()
			elapsed := now.Sub(start).Seconds()

			if elapsed >= totalDur.Seconds() {
				// Wait for inflight
				wg.Wait()
				return
			}

			targetRPS := r.getCurrentRPS(elapsed)
			if targetRPS <= 0.1 {
				time.Sleep(100 * time.Millisecond)
				nextRequestTime = time.Now() // Reset schedule if paused
				continue
			}

			period := time.Duration(float64(time.Second) / targetRPS)

			// Schedule Check
			if nextRequestTime.After(now) {
				time.Sleep(nextRequestTime.Sub(now))
			}

			// If we are way behind (> 10ms ???), we should perhaps skip or just log it?
			// SteadyQ philosophy: Constant Throughput. Try to catch up, but warn if Coordinated Omission.

			// Launch Request
			wg.Add(1)

			// Capture loop variables
			scheduledTime := nextRequestTime

			go func() {
				defer wg.Done()
				r.executeRequest(scheduledTime)
			}()

			nextRequestTime = nextRequestTime.Add(period)

			// Prevent massive catch-up bursts if we paused for GC or something
			// If nextRequestTime is > 1s behind Now, reset it.
			if time.Since(nextRequestTime) > 1*time.Second {
				// Warn?
				nextRequestTime = time.Now()
			}
		}
	}
}

func (r *Runner) executeRequest(scheduledTime time.Time) {
	actualStart := time.Now()
	queueWait := actualStart.Sub(scheduledTime)
	if queueWait < 0 {
		queueWait = 0
	}

	atomic.AddInt64(&r.inflight, 1)
	defer atomic.AddInt64(&r.inflight, -1)

	userID := uuid.New().String()
	chatID := uuid.New().String()
	q := questions[rand.Intn(len(questions))]
	bodyBytes, _ := json.Marshal(map[string]string{"query": q})

	req, _ := http.NewRequest(
		"POST",
		fmt.Sprintf("%s?chatID=%s&userID=%s", r.Cfg.URL, chatID, userID),
		bytes.NewBuffer(bodyBytes),
	)
	req.Header.Set("Content-Type", "application/json")

	// Network Call
	resp, err := r.Client.Do(req)

	endTime := time.Now()
	serviceTime := endTime.Sub(actualStart)
	totalLatency := endTime.Sub(scheduledTime)

	res := ExperimentResult{
		TimeStamp:   scheduledTime,
		Latency:     totalLatency,
		ServiceTime: serviceTime,
		QueueWait:   queueWait,
		Err:         err,
		UserID:      userID,
		Query:       q,
	}

	if err == nil {
		res.Status = resp.StatusCode
		res.Bytes = resp.ContentLength

		if resp.StatusCode >= 300 {
			b, _ := io.ReadAll(resp.Body)
			res.ResponseBody = string(b)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			res.Success = true
		}
	}

	// Update Stats (microseconds)
	r.Stats.AddRequest(
		res.Success,
		res.Bytes,
		int64(res.ServiceTime.Microseconds()),
		int64(res.QueueWait.Microseconds()),
		int64(res.Latency.Microseconds()),
	)

	r.mu.Lock()
	r.Results = append(r.Results, res)
	r.mu.Unlock()
}

func (r *Runner) getCurrentRPS(elapsedSec float64) float64 {
	// Re-implementing the simpler linear ramp logic
	cfg := r.Cfg
	if elapsedSec < float64(cfg.RampUp) {
		if cfg.RampUp == 0 {
			return float64(cfg.TargetRPS)
		}
		return float64(cfg.TargetRPS) * (elapsedSec / float64(cfg.RampUp))
	}
	steadyEnd := float64(cfg.RampUp + cfg.SteadyDur)
	if elapsedSec < steadyEnd {
		return float64(cfg.TargetRPS)
	}
	totalDur := float64(cfg.RampUp + cfg.SteadyDur + cfg.RampDown)
	if elapsedSec < totalDur {
		if cfg.RampDown == 0 {
			return 0
		}
		remaining := totalDur - elapsedSec
		return float64(cfg.TargetRPS) * (remaining / float64(cfg.RampDown))
	}
	return 0
}

func (r *Runner) GetInflight() int64 {
	return atomic.LoadInt64(&r.inflight)
}
