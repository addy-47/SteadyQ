package stats

import (
	"sync"
	"sync/atomic"
	"time"
)

type Stats struct {
	Requests uint64
	Success  uint64
	Fail     uint64
	Bytes    uint64

	// Lags
	TotalQueueWaitMicro int64

	// Histograms
	ServiceTime *SafeHistogram
	TotalTime   *SafeHistogram

	// Status Codes (Protected by Mutex for map, or simple Atomic counters)
	// For high throughput, atomic counters for common codes is better,
	// or a sharded map. For TUI app, a Mutex map is probably fine if infrequent updates,
	// but Add is on hot path.
	// Let's use simple sync.Map or Mutex.
	// Given single threaded runner loop for non-async parts, typically we want low contention.
	// Let's use a Mutex for now, simplistic.
	muCodes     sync.Mutex
	StatusCodes map[int]int
}

func NewStats() *Stats {
	return &Stats{
		ServiceTime: NewSafeHistogram(),
		TotalTime:   NewSafeHistogram(),
		StatusCodes: make(map[int]int),
	}
}

func (s *Stats) Reset() {
	atomic.StoreUint64(&s.Requests, 0)
	atomic.StoreUint64(&s.Success, 0)
	atomic.StoreUint64(&s.Fail, 0)
	atomic.StoreUint64(&s.Bytes, 0)
	atomic.StoreInt64(&s.TotalQueueWaitMicro, 0)

	s.ServiceTime = NewSafeHistogram()
	s.TotalTime = NewSafeHistogram()

	s.muCodes.Lock()
	s.StatusCodes = make(map[int]int)
	s.muCodes.Unlock()
}

func (s *Stats) Add(res bool, bytes uint64, service, queue, total time.Duration, code int) {
	atomic.AddUint64(&s.Requests, 1)
	if res {
		atomic.AddUint64(&s.Success, 1)
	} else {
		atomic.AddUint64(&s.Fail, 1)
	}
	atomic.AddUint64(&s.Bytes, bytes)

	atomic.AddInt64(&s.TotalQueueWaitMicro, int64(queue.Microseconds()))

	s.ServiceTime.RecordValue(service.Microseconds())
	s.TotalTime.RecordValue(total.Microseconds())

	// Update Codes
	s.muCodes.Lock()
	s.StatusCodes[code]++
	s.muCodes.Unlock()
}

func (s *Stats) QueueWaitAvgMs() float64 {
	reqs := atomic.LoadUint64(&s.Requests)
	if reqs == 0 {
		return 0
	}
	totalMicro := atomic.LoadInt64(&s.TotalQueueWaitMicro)
	return float64(totalMicro) / float64(reqs) / 1000.0
}

func (s *Stats) GetStatusCodes() map[int]int {
	s.muCodes.Lock()
	defer s.muCodes.Unlock()
	// Copy to avoid race
	copy := make(map[int]int)
	for k, v := range s.StatusCodes {
		copy[k] = v
	}
	return copy
}

// ... Getters ...
func (s *Stats) GetP99Service() float64 {
	return float64(s.ServiceTime.ValueAtQuantile(99)) / 1000.0
}
func (s *Stats) GetP50Service() float64 {
	return float64(s.ServiceTime.ValueAtQuantile(50)) / 1000.0
}
func (s *Stats) GetP90Service() float64 {
	return float64(s.ServiceTime.ValueAtQuantile(90)) / 1000.0
}
func (s *Stats) GetP95Service() float64 {
	return float64(s.ServiceTime.ValueAtQuantile(95)) / 1000.0
}
