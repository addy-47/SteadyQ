package stats

import (
	"sync/atomic"
	"time"
)

type Stats struct {
	Requests uint64
	Success  uint64
	Fail     uint64
	Bytes    uint64

	// Lags
	TotalQueueWaitMicro int64 // Sum to calculate avg

	// Histograms
	ServiceTime *SafeHistogram
	TotalTime   *SafeHistogram
}

func NewStats() *Stats {
	return &Stats{
		ServiceTime: NewSafeHistogram(),
		TotalTime:   NewSafeHistogram(),
	}
}

func (s *Stats) Reset() {
	atomic.StoreUint64(&s.Requests, 0)
	atomic.StoreUint64(&s.Success, 0)
	atomic.StoreUint64(&s.Fail, 0)
	atomic.StoreUint64(&s.Bytes, 0)
	atomic.StoreInt64(&s.TotalQueueWaitMicro, 0)

	// Re-create histograms
	s.ServiceTime = NewSafeHistogram()
	s.TotalTime = NewSafeHistogram()
}

func (s *Stats) Add(res bool, bytes uint64, service, queue, total time.Duration) {
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
}

func (s *Stats) QueueWaitAvgMs() float64 {
	reqs := atomic.LoadUint64(&s.Requests)
	if reqs == 0 {
		return 0
	}
	totalMicro := atomic.LoadInt64(&s.TotalQueueWaitMicro)
	return float64(totalMicro) / float64(reqs) / 1000.0
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
