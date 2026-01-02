package runner

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
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
	P50ServiceMs  float64
	P90ServiceMs  float64
	P95ServiceMs  float64
	P99ServiceMs  float64
	MaxServiceMs  int64
	MeanServiceMs float64

	AvgQueueWaitMs float64

	StatusCodes     map[int]int
	ErrorCounts     map[string]int
	ResponseSamples map[int]string
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

	timeout := time.Duration(cfg.TimeoutSec) * time.Second
	if cfg.TimeoutSec == 0 {
		timeout = 30 * time.Second
	}

	client := &http.Client{
		Timeout:   timeout,
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
		Requests:        atomic.LoadUint64(&r.Stats.Requests),
		Success:         atomic.LoadUint64(&r.Stats.Success),
		Fail:            atomic.LoadUint64(&r.Stats.Fail),
		Bytes:           atomic.LoadUint64(&r.Stats.Bytes),
		Inflight:        atomic.LoadInt64(&r.inflight),
		P50ServiceMs:    r.Stats.GetP50Service(),
		P90ServiceMs:    r.Stats.GetP90Service(),
		P95ServiceMs:    r.Stats.GetP95Service(),
		P99ServiceMs:    r.Stats.GetP99Service(),
		MaxServiceMs:    r.Stats.ServiceTime.Max() / 1000,
		MeanServiceMs:   r.Stats.ServiceTime.Mean() / 1000,
		AvgQueueWaitMs:  r.Stats.QueueWaitAvgMs(),
		StatusCodes:     r.Stats.GetStatusCodes(),
		ErrorCounts:     r.Stats.GetErrorCounts(),
		ResponseSamples: r.Stats.GetResponseSamples(),
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
	r.StartTickLoop(ctx, 100*time.Millisecond)

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

	// Calculate spawn interval for RampUp
	// If RampUp is 0, we spawn all immediately (interval 0)
	var spawnInterval time.Duration
	if r.Cfg.RampUp > 0 && r.Cfg.NumUsers > 1 {
		// e.g. 10 users over 10s = 1 user per 1s
		spawnInterval = time.Duration(float64(r.Cfg.RampUp) / float64(r.Cfg.NumUsers) * float64(time.Second))
	}

	for i := 0; i < r.Cfg.NumUsers; i++ {
		// Wait before spawning next user if RampUp is active
		if i > 0 && spawnInterval > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(spawnInterval):
			}
		}

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
	reqID := uuid.New().String()

	var err error
	var status int
	var bytesLen int64
	var respBody string

	if r.Cfg.Command != "" {
		// Custom Script Execution
		cmdStr := r.Cfg.Command
		// Simple Templating
		cmdStr = strings.ReplaceAll(cmdStr, "{{userID}}", userID)
		cmdStr = strings.ReplaceAll(cmdStr, "{{reqID}}", reqID)

		// Execute shell
		// Using sh -c to allow complex commands
		cmd := exec.Command("sh", "-c", cmdStr)
		var out bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr

		err = cmd.Run()

		if err == nil {
			status = 200 // Success assumption for zero exit code
			bytesLen = int64(out.Len())
			respBody = out.String()
		} else {
			status = 500
			if exitErr, ok := err.(*exec.ExitError); ok {
				status = exitErr.ExitCode()
			}
			respBody = stderr.String()
		}

	} else {
		// Standard HTTP Request
		method := r.Cfg.Method
		if method == "" {
			method = "GET"
		}

		url := r.Cfg.URL
		url = strings.ReplaceAll(url, "{{userID}}", userID)
		url = strings.ReplaceAll(url, "{{reqID}}", reqID)

		// Cache busting (if not already present)
		if !strings.Contains(url, "reqID=") {
			sep := "?"
			if strings.Contains(url, "?") {
				sep = "&"
			}
			url = fmt.Sprintf("%s%sreqID=%s&userID=%s", url, sep, reqID, userID)
		}

		var body io.Reader
		if r.Cfg.Body != "" {
			bodyStr := r.Cfg.Body
			bodyStr = strings.ReplaceAll(bodyStr, "{{userID}}", userID)
			bodyStr = strings.ReplaceAll(bodyStr, "{{reqID}}", reqID)
			body = strings.NewReader(bodyStr)
		}

		req, _ := http.NewRequest(method, url, body)

		// Set Headers with templating
		hasContentType := false
		for k, v := range r.Cfg.Headers {
			val := strings.ReplaceAll(v, "{{userID}}", userID)
			val = strings.ReplaceAll(val, "{{reqID}}", reqID)
			req.Header.Set(k, val)
			if strings.ToLower(k) == "content-type" {
				hasContentType = true
			}
		}
		if !hasContentType && r.Cfg.Body != "" {
			req.Header.Set("Content-Type", "application/json")
		}

		var resp *http.Response
		resp, err = r.Client.Do(req)

		if err == nil {
			status = resp.StatusCode
			bytesLen = resp.ContentLength

			if resp.StatusCode >= 400 {
				b, _ := io.ReadAll(resp.Body)
				respBody = string(b)
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	}

	endTime := time.Now()
	serviceTime := endTime.Sub(actualStart)
	totalLatency := endTime.Sub(scheduledTime)

	res := ExperimentResult{
		TimeStamp:    scheduledTime,
		Latency:      totalLatency,
		ServiceTime:  serviceTime,
		QueueWait:    queueWait,
		Err:          err,
		UserID:       userID,
		Query:        "custom",
		Status:       status,
		Bytes:        bytesLen,
		ResponseBody: respBody,
	}

	if err == nil {
		if status >= 200 && status < 300 {
			res.Success = true
		}
	}

	errStr := ""
	if err != nil {
		errStr = cleanError(err)
	}

	r.Stats.Add(
		res.Success,
		uint64(res.Bytes),
		res.ServiceTime,
		res.QueueWait,
		res.Latency,
		res.Status,
		errStr,
		respBody,
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

func cleanError(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()

	// Strip common redundant prefixes from net/http errors
	// Example: Get "http://localhost:8080": dial tcp [::1]:8080: connect: connection refused
	// We want to skip the URL part if possible.
	if idx := strings.LastIndex(s, ": "); idx != -1 {
		// Try to find the last part which is usually the actual root cause
		// but only if it's a network-like error.
		if strings.Contains(s, "dial") || strings.Contains(s, "timeout") || strings.Contains(s, "connect") {
			return s[idx+2:]
		}
	}

	return s
}
