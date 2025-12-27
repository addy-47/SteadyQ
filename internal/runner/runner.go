package runner

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"steadyq/internal/stats"

	"github.com/google/uuid"
)

// StatsSnapshot is sent over the channel
type StatsSnapshot struct {
	Requests uint64
	Success  uint64
	Fail     uint64
	Bytes    uint64
	Inflight int64

	// Pre-calculated percentiles for the UI (cheap copy)
	P50ServiceMs float64
	P90ServiceMs float64
	P99ServiceMs float64
	MaxServiceMs int64

	AvgQueueWaitMs float64
}

// StatsUpdateChan is the channel type
type StatsUpdateChan chan StatsSnapshot

type Runner struct {
	Cfg     Config
	Stats   *stats.Stats
	Client  *http.Client
	Results []ExperimentResult
	mu      sync.Mutex

	inflight int64

	// Event Channel
	Updates StatsUpdateChan
}

func NewRunner(cfg Config, updates StatsUpdateChan) *Runner {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 2000
	t.MaxConnsPerHost = 2000
	t.MaxIdleConnsPerHost = 2000
	t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	client := &http.Client{
		Timeout:   time.Duration(cfg.TimeoutSec) * time.Second,
		Transport: t,
	}

	if updates == nil {
		// Avoid nil panics if not provided
		updates = make(StatsUpdateChan, 10)
	}

	return &Runner{
		Cfg:     cfg,
		Stats:   stats.NewStats(),
		Client:  client,
		Updates: updates,
	}
}

// StartTickLoop starts a goroutine that pushes stats updates
func (r *Runner) StartTickLoop(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.sendUpdate()
			}
		}
	}()
}

func (r *Runner) sendUpdate() {
	// Create snapshot
	s := StatsSnapshot{
		Requests:       atomic.LoadUint64(&r.Stats.Requests),
		Success:        atomic.LoadUint64(&r.Stats.Success),
		Fail:           atomic.LoadUint64(&r.Stats.Fail),
		Bytes:          atomic.LoadUint64(&r.Stats.Bytes),
		Inflight:       atomic.LoadInt64(&r.inflight),
		P50ServiceMs:   r.Stats.GetP50Service(),
		P90ServiceMs:   r.Stats.GetP90Service(),
		P99ServiceMs:   r.Stats.GetP99Service(),
		MaxServiceMs:   r.Stats.ServiceTime.Max() / 1000,
		AvgQueueWaitMs: r.Stats.QueueWaitAvgMs(),
	}

	// Non-blocking send
	select {
	case r.Updates <- s:
	default:
		// Drop update if channel full, UI acts as backpressure
	}
}

func (r *Runner) Run(ctx context.Context) {
	// Start Tick Loop for UI
	r.StartTickLoop(ctx, 200*time.Millisecond)

	if r.Cfg.Mode == "users" {
		r.runUsers(ctx)
	} else {
		r.runRPS(ctx)
	}
}

// ... rest of the runUsers/runRPS logic ...
// (We reuse the existing logic, but I need to include it here to compile)

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
				wg.Wait()
				return
			}

			targetRPS := r.getCurrentRPS(elapsed)
			if targetRPS <= 0.1 {
				time.Sleep(100 * time.Millisecond)
				nextRequestTime = time.Now()
				continue
			}

			period := time.Duration(float64(time.Second) / targetRPS)

			if nextRequestTime.After(now) {
				time.Sleep(nextRequestTime.Sub(now))
			}

			wg.Add(1)
			scheduledTime := nextRequestTime

			go func() {
				defer wg.Done()
				r.executeRequest(scheduledTime)
			}()

			nextRequestTime = nextRequestTime.Add(period)

			if time.Since(nextRequestTime) > 1*time.Second {
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
	q := "Why is the sky blue?" // Optimization: Pre-allocate or reuse
	bodyBytes, _ := json.Marshal(map[string]string{"query": q})

	req, _ := http.NewRequest(
		"POST",
		fmt.Sprintf("%s?chatID=%s&userID=%s", r.Cfg.URL, chatID, userID),
		bytes.NewBuffer(bodyBytes),
	)
	req.Header.Set("Content-Type", "application/json")

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

	r.Stats.Add(
		res.Success,
		uint64(res.Bytes),
		res.ServiceTime,
		res.QueueWait,
		res.Latency,
	)

	r.mu.Lock()
	r.Results = append(r.Results, res)
	r.mu.Unlock()
}

func (r *Runner) getCurrentRPS(elapsedSec float64) float64 {
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
