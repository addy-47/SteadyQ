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
	"text/template"
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

	Inflight int64

	// Event Channel
	Updates StatsUpdateChan

	// Template Engine
	TmplEngine *TemplateEngine
	TmplURL    *template.Template
	TmplBody   *template.Template
	TmplCmd    *template.Template
	TmplHeader map[string]*template.Template
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

// StartTickLoop starts a goroutine that pushes stats updates until stop channel is closed
func (r *Runner) StartTickLoop(stop chan struct{}, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				r.sendUpdate() // One final update
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
		Inflight:        atomic.LoadInt64(&r.Inflight),
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
	// Initialize Template Engine
	r.TmplEngine = NewTemplateEngine()
	var err error

	// Parse URL
	r.TmplURL, err = r.TmplEngine.Parse("url", r.Cfg.URL)
	if err != nil {
		fmt.Printf("Error parsing URL template: %v\n", err) // Should probably log better
	}

	// Parse Body
	if r.Cfg.Body != "" {
		bodyText := r.Cfg.Body
		if strings.HasPrefix(bodyText, "@") {
			fname := strings.TrimPrefix(bodyText, "@")
			bodyText = fmt.Sprintf("{{readFile %q}}", fname)
		}
		r.TmplBody, err = r.TmplEngine.Parse("body", bodyText)
		if err != nil {
			fmt.Printf("Error parsing Body template: %v\n", err)
		}
	}

	// Parse Command
	if r.Cfg.Command != "" {
		r.TmplCmd, err = r.TmplEngine.Parse("cmd", r.Cfg.Command)
		if err != nil {
			fmt.Printf("Error parsing Command template: %v\n", err)
		}
	}

	// Parse Headers
	r.TmplHeader = make(map[string]*template.Template)
	for k, v := range r.Cfg.Headers {
		t, err := r.TmplEngine.Parse("header-"+k, v)
		if err != nil {
			fmt.Printf("Error parsing Header '%s' template: %v\n", k, err)
		} else {
			r.TmplHeader[k] = t
		}
	}

	// Start Tick Loop for UI
	stopTicker := make(chan struct{})
	r.StartTickLoop(stopTicker, 100*time.Millisecond)
	defer close(stopTicker)

	if r.Cfg.Mode == "users" {
		r.runUsers(ctx)
	} else {
		r.runRPS(ctx)
	}
}

// applyTemplates executes the pre-parsed templates
// Note: We changed signature to take *Template, not string input
func (r *Runner) applyTemplates(t *template.Template, userID, requestUUID string) string {
	if t == nil {
		return ""
	}
	out, err := r.TmplEngine.Execute(t, TemplateData{
		UserID: userID,
		UUID:   requestUUID,
	})
	if err != nil {
		return "" // Fail gracefully?
	}
	return out
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
			// Generate STABLE userID for this virtual user
			vUser := uuid.New().String()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					if time.Since(start) > totalDur {
						return
					}
					r.executeRequest(time.Now(), vUser)
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
			wg.Wait()
			return
		default:
			now := time.Now()
			elapsed := now.Sub(start).Seconds()

			if elapsed >= totalDur.Seconds() {
				wg.Wait()
				return
			}

			targetRPS := r.getCurrentRPS(elapsed)
			if targetRPS <= 0.001 {
				time.Sleep(100 * time.Millisecond)
				nextRequestTime = time.Now()
				continue
			}

			period := time.Duration(float64(time.Second) / targetRPS)

			// If we are way behind (more than 1s), reset nextRequestTime to avoid a massive burst
			// But if we are only slightly behind, spawn immediately to catch up.
			if now.Sub(nextRequestTime) > 1*time.Second {
				nextRequestTime = now
			}

			// While we are behind the schedule, spawn requests
			for nextRequestTime.Before(now) || nextRequestTime.Equal(now) {
				wg.Add(1)
				scheduledTime := nextRequestTime
				go func() {
					defer wg.Done()
					// RPS mode = independent events, fresh userID by default
					r.executeRequest(scheduledTime, uuid.New().String())
				}()
				nextRequestTime = nextRequestTime.Add(period)
			}

			// If next one is in the future, sleep until then
			if nextRequestTime.After(now) {
				time.Sleep(nextRequestTime.Sub(now))
			}
		}
	}
}

func (r *Runner) executeRequest(scheduledTime time.Time, userID string) {
	actualStart := time.Now()
	queueWait := actualStart.Sub(scheduledTime)
	if queueWait < 0 {
		queueWait = 0
	}

	atomic.AddInt64(&r.Inflight, 1)
	defer atomic.AddInt64(&r.Inflight, -1)

	reqID := uuid.New().String()

	var err error
	var status int
	var bytesLen int64
	var respBody string

	if r.Cfg.Command != "" {
		// Custom Script Execution
		// We use TmplCmd if available, otherwise fallback to raw string (shouldn't happen if parsed)
		cmdStr := ""
		if r.TmplCmd != nil {
			cmdStr = r.applyTemplates(r.TmplCmd, userID, reqID)
		} else {
			cmdStr = r.Cfg.Command
		}

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

		url := ""
		if r.TmplURL != nil {
			url = r.applyTemplates(r.TmplURL, userID, reqID)
		} else {
			url = r.Cfg.URL
		}

		var body io.Reader
		if r.Cfg.Body != "" && r.TmplBody != nil {
			bodyStr := r.applyTemplates(r.TmplBody, userID, reqID)
			body = strings.NewReader(bodyStr)
		} else if r.Cfg.Body != "" {
			body = strings.NewReader(r.Cfg.Body)
		}

		req, _ := http.NewRequest(method, url, body)

		// Set Headers with templating
		hasContentType := false
		for k, v := range r.Cfg.Headers {
			val := v
			if t, ok := r.TmplHeader[k]; ok {
				val = r.applyTemplates(t, userID, reqID)
			}
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
	return atomic.LoadInt64(&r.Inflight)
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
