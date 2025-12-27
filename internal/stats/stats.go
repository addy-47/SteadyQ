package stats

import (
	"sync/atomic"
)

// Stats holds real-time aggregated metrics
type Stats struct {
	Requests uint64
	Success  uint64
	Fail     uint64
	Bytes    uint64

	// Latency histograms (microseconds)
	ServiceTime *SafeHistogram
	TotalTime   *SafeHistogram

	// Queue wait is important for lag detection
	QueueWait *SafeHistogram
}

func NewStats() *Stats {
	return &Stats{
		ServiceTime: NewSafeHistogram(),
		TotalTime:   NewSafeHistogram(),
		QueueWait:   NewSafeHistogram(),
	}
}

func (s *Stats) AddRequest(success bool, bytes int64, serviceTimeUs, queueWaitUs, totalTimeUs int64) {
	atomic.AddUint64(&s.Requests, 1)
	if success {
		atomic.AddUint64(&s.Success, 1)
	} else {
		atomic.AddUint64(&s.Fail, 1)
	}
	atomic.AddUint64(&s.Bytes, uint64(bytes))

	s.ServiceTime.RecordValue(serviceTimeUs)
	s.QueueWait.RecordValue(queueWaitUs)
	s.TotalTime.RecordValue(totalTimeUs)
}

func (s *Stats) ErrorRate() float64 {
	reqs := atomic.LoadUint64(&s.Requests)
	if reqs == 0 {
		return 0
	}
	fails := atomic.LoadUint64(&s.Fail)
	return (float64(fails) / float64(reqs)) * 100
}

func (s *Stats) GetP99Service() float64 {
	return float64(s.ServiceTime.ValueAtQuantile(99)) / 1000.0 // ms
}

func (s *Stats) GetP99Total() float64 {
	return float64(s.TotalTime.ValueAtQuantile(99)) / 1000.0 // ms
}

// QueueWaitAvgMs returns average queue wait in milliseconds
func (s *Stats) QueueWaitAvgMs() float64 {
	return s.QueueWait.Mean() / 1000.0
}
