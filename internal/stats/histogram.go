package stats

import (
	"sync"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
)

// SafeHistogram is a thread-safe wrapper around hdrhistogram
type SafeHistogram struct {
	hist *hdrhistogram.Histogram
	mu   sync.Mutex
}

func NewSafeHistogram() *SafeHistogram {
	// 1us to 10min, 3 significant figures
	h := hdrhistogram.New(1, int64(10*time.Minute/time.Microsecond), 3)
	return &SafeHistogram{hist: h}
}

// RecordValue records a latency in microseconds
func (h *SafeHistogram) RecordValue(v int64) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.hist.RecordValue(v)
}

func (h *SafeHistogram) ValueAtQuantile(q float64) int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.hist.ValueAtQuantile(q)
}

func (h *SafeHistogram) Mean() float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.hist.Mean()
}

func (h *SafeHistogram) Max() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.hist.Max()
}

func (h *SafeHistogram) TotalCount() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.hist.TotalCount()
}
