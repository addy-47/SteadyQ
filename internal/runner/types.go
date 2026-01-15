package runner

import (
	"time"
)

type Config struct {
	URL        string
	Method     string // HTTP Method
	Body       string // Request Body
	Headers    map[string]string
	TargetRPS  int
	SteadyDur  int
	RampUp     int
	RampDown   int
	TimeoutSec int

	// Open-Loop (RPS) vs Closed-Loop (Users)
	// Open-Loop (RPS) vs Closed-Loop (Users)
	Mode      string        // "rps", "users", "script"
	NumUsers  int           // For "users" mode
	ThinkTime time.Duration // For "users" mode

	// Custom Scripting
	Command string // Shell command to execute per request (overrides URL/Method)

	// Reporting
	OutPrefix string // Prefix for auto-report generation
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
