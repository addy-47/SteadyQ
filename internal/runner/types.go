package runner

import (
	"time"
)

type Config struct {
	URL        string
	TargetRPS  int
	SteadyDur  int
	RampUp     int
	RampDown   int
	TimeoutSec int

	// Open-Loop (RPS) vs Closed-Loop (Users)
	Mode      string        // "rps" or "users"
	NumUsers  int           // For "users" mode
	ThinkTime time.Duration // For "users" mode
}

type ExperimentResult struct {
	TimeStamp    time.Time
	Latency      time.Duration // Total Time
	ServiceTime  time.Duration // Network/Server Time
	QueueWait    time.Duration // Schedule Lag
	Status       int
	Success      bool
	Bytes        int64
	UserID       string
	Query        string
	Err          error
	ResponseBody string
}
