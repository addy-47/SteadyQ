package app

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"steadyq/internal/runner"
)

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
