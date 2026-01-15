package app

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"steadyq/internal/runner"
)

type SummaryReport struct {
	TotalRequests uint64         `json:"total_requests"`
	TotalSuccess  uint64         `json:"total_success"`
	TotalFail     uint64         `json:"total_fail"`
	TotalBytes    int64          `json:"total_bytes"`
	P50           float64        `json:"p50_ms"`
	P90           float64        `json:"p90_ms"`
	P95           float64        `json:"p95_ms"`
	P99           float64        `json:"p99_ms"`
	Mean          float64        `json:"mean_ms"`
	Max           float64        `json:"max_ms"`
	Min           float64        `json:"min_ms"`
	StatusCodes   map[int]int    `json:"status_codes"`
	Errors        map[string]int `json:"errors"`
	Duration      time.Duration  `json:"duration"`
	AverageRPS    float64        `json:"avg_rps"`
}

// ExportCSV exports results to a JMeter-compatible CSV file.
// Schema: timeStamp,elapsed,label,responseCode,responseMessage,threadName,dataType,success,failureMessage,bytes,sentBytes,grpThreads,allThreads,URL,Latency,IdleTime,Connect
func ExportCSV(results []runner.ExperimentResult, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	// Header
	header := []string{
		"timeStamp", "elapsed", "label", "responseCode", "responseMessage",
		"threadName", "dataType", "success", "failureMessage", "bytes",
		"sentBytes", "grpThreads", "allThreads", "URL", "Latency", "IdleTime", "Connect",
	}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, res := range results {
		// timeStamp: Unix ms
		ts := fmt.Sprintf("%d", res.TimeStamp.UnixMilli())
		// elapsed: Total latency in ms
		elapsed := fmt.Sprintf("%d", res.Latency.Milliseconds())

		successStr := "true"
		if !res.Success {
			successStr = "false"
		}

		errMsg := ""
		if res.Err != nil {
			errMsg = res.Err.Error()
		}

		// Simplified mapping
		record := []string{
			ts,
			elapsed,
			"SteadyQ Request", // Label
			strconv.Itoa(res.Status),
			httpStatusText(res.Status),
			"User-" + res.UserID, // Thread Name
			"text",               // DataType
			successStr,
			errMsg,
			strconv.FormatInt(res.Bytes, 10),
			"0", // Sent bytes (not tracked currently)
			"1", // grpThreads (mock)
			"1", // allThreads (mock)
			"",  // URL (not in Result struct, could be added later)
			fmt.Sprintf("%d", res.Latency.Milliseconds()),   // Latency
			fmt.Sprintf("%d", res.QueueWait.Milliseconds()), // IdleTime (QueueWait)
			"0", // Connect time (part of ServiceTime, not separated)
		}

		if err := w.Write(record); err != nil {
			return err
		}
	}

	return nil
}

// ExportJSON exports results to a JSON file.
func ExportJSON(results []runner.ExperimentResult, filename string) error {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

func ExportSummary(results []runner.ExperimentResult, baseFilename string) error {
	if len(results) == 0 {
		return fmt.Errorf("no results to summarize")
	}

	report := CalculateSummary(results)

	// JSON Summary
	jsonData, _ := json.MarshalIndent(report, "", "  ")
	os.WriteFile(baseFilename+"_summary.json", jsonData, 0644)

	// CSV Summary (Simple key-value)
	f, _ := os.Create(baseFilename + "_summary.csv")
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()

	w.Write([]string{"Metric", "Value"})
	w.Write([]string{"Total Requests", strconv.FormatUint(report.TotalRequests, 10)})
	w.Write([]string{"Success", strconv.FormatUint(report.TotalSuccess, 10)})
	w.Write([]string{"Fail", strconv.FormatUint(report.TotalFail, 10)})
	w.Write([]string{"P50 ms", fmt.Sprintf("%.2f", report.P50)})
	w.Write([]string{"P90 ms", fmt.Sprintf("%.2f", report.P90)})
	w.Write([]string{"P95 ms", fmt.Sprintf("%.2f", report.P95)})
	w.Write([]string{"P99 ms", fmt.Sprintf("%.2f", report.P99)})
	w.Write([]string{"Mean ms", fmt.Sprintf("%.2f", report.Mean)})
	w.Write([]string{"Max ms", fmt.Sprintf("%.2f", report.Max)})
	w.Write([]string{"Min ms", fmt.Sprintf("%.2f", report.Min)})
	w.Write([]string{"Avg RPS", fmt.Sprintf("%.2f", report.AverageRPS)})

	return nil
}

func CalculateSummary(results []runner.ExperimentResult) SummaryReport {
	var totalBytes int64
	var totalSuccess uint64
	var latencies []float64
	statusCodes := make(map[int]int)
	errors := make(map[string]int)

	minTime := results[0].TimeStamp
	maxTime := results[0].TimeStamp

	for _, r := range results {
		if r.TimeStamp.Before(minTime) {
			minTime = r.TimeStamp
		}
		if r.TimeStamp.After(maxTime) {
			maxTime = r.TimeStamp
		}

		if r.Success {
			totalSuccess++
		}
		totalBytes += r.Bytes
		lat := float64(r.Latency.Microseconds()) / 1000.0
		latencies = append(latencies, lat)
		statusCodes[r.Status]++
		if r.Err != nil {
			errors[r.Err.Error()]++
		}
	}

	sort.Float64s(latencies)
	count := len(latencies)

	getQuantile := func(q float64) float64 {
		if count == 0 {
			return 0
		}
		idx := int(q * float64(count-1))
		return latencies[idx]
	}

	sum := 0.0
	for _, l := range latencies {
		sum += l
	}

	dur := maxTime.Sub(minTime)
	avgRPS := 0.0
	if dur.Seconds() > 0 {
		avgRPS = float64(count) / dur.Seconds()
	}

	return SummaryReport{
		TotalRequests: uint64(count),
		TotalSuccess:  totalSuccess,
		TotalFail:     uint64(count) - totalSuccess,
		TotalBytes:    totalBytes,
		P50:           getQuantile(0.50),
		P90:           getQuantile(0.90),
		P95:           getQuantile(0.95),
		P99:           getQuantile(0.99),
		Mean:          sum / float64(count),
		Max:           latencies[count-1],
		Min:           latencies[0],
		StatusCodes:   statusCodes,
		Errors:        errors,
		Duration:      dur,
		AverageRPS:    avgRPS,
	}
}

func httpStatusText(code int) string {
	// Minimal fallback
	switch code {
	case 200:
		return "OK"
	case 201:
		return "Created"
	case 400:
		return "Bad Request"
	case 401:
		return "Unauthorized"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	case 500:
		return "Internal Server Error"
	case 502:
		return "Bad Gateway"
	case 503:
		return "Service Unavailable"
	case 504:
		return "Gateway Timeout"
	default:
		return ""
	}
}
