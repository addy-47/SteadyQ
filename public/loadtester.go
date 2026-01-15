package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// ========= CONFIGURATION =========

var SAMPLE_QUERIES = []string{
	"suggest best power point courses",
}

// Request Payload Structure
type AsyncRequestPayload struct {
	Query  string `json:"query"`
	UserID string `json:"user_id"`
	ChatID string `json:"chat_id"`
}

// Response Structure for Validation
type AsyncResponsePayload struct {
	QueryID string `json:"query_id"`
}

type Result struct {
	TimeStamp    time.Time
	Latency      time.Duration
	Status       int
	Success      bool
	Bytes        int64
	Err          error
	ResponseBody string
}

type Config struct {
	BaseURL    string
	TargetRPS  int
	SteadyDur  int
	OutPrefix  string
	TimeoutSec int
}

type FailureSignature struct {
	Status int
	Body   string
	Err    string
}

// ========= HELPER FUNCTIONS =========

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	i := int(math.Ceil((p/100)*float64(len(sorted)))) - 1
	if i < 0 {
		i = 0
	}
	return sorted[i]
}

func progressBar(pct float64, width int) string {
	filled := int(pct * float64(width))
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("█", filled) + strings.Repeat("-", width-filled) + "]"
}

func main() {
	// Seed Random Number Generator
	rand.Seed(time.Now().UnixNano())

	// ========= CLI FLAGS =========
	baseURL := flag.String("url", "", "Base URL (e.g., ")
	rps := flag.Int("rps", 50, "Target RPS")
	duration := flag.Int("duration", 180, "Test duration (seconds)")
	out := flag.String("out", "async_test", "Output filename prefix")
	timeout := flag.Int("timeout", 3000, "HTTP Timeout (s)")
	flag.Parse()

	if *baseURL == "" {
		fmt.Println("❌ --url required")
		os.Exit(1)
	}

	// Clean URL ensures no double slashes
	targetURL := strings.TrimRight(*baseURL, "/") + "/api/v1/embedding/search-async"

	cfg := Config{
		BaseURL:    targetURL,
		TargetRPS:  *rps,
		SteadyDur:  *duration,
		OutPrefix:  *out,
		TimeoutSec: *timeout,
	}

	// HTTP Client Optimization
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 2000
	t.MaxConnsPerHost = 2000
	t.MaxIdleConnsPerHost = 2000
	t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	client := &http.Client{
		Timeout:   time.Duration(cfg.TimeoutSec) * time.Second,
		Transport: t,
	}

	var (
		results  []Result
		mu       sync.Mutex
		inflight int64
		sent     uint64
		success  uint64
		fail     uint64
	)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	start := time.Now()
	totalTestTime := time.Duration(cfg.SteadyDur) * time.Second

	fmt.Printf("\n🚀 STARTING ASYNC LOAD TEST (Go Version)\n")
	fmt.Printf("======================================================================\n")
	fmt.Printf("Endpoint   : %s\n", cfg.BaseURL)
	fmt.Printf("Target RPS : %d\n", cfg.TargetRPS)
	fmt.Printf("Duration   : %ds\n", cfg.SteadyDur)
	fmt.Printf("Timeout    : %ds\n", cfg.TimeoutSec)
	fmt.Printf("======================================================================\n\n")

	// ========= PROGRESS BAR =========
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case t := <-ticker.C:
				elapsed := t.Sub(start).Seconds()
				if elapsed > totalTestTime.Seconds() && atomic.LoadInt64(&inflight) == 0 {
					return
				}
				pct := elapsed / totalTestTime.Seconds()
				if pct > 1.0 {
					pct = 1.0
				}

				fmt.Printf(
					"\r%s %3.0f%% | %s/%s | Inflight: %3d | OK: %d | Err: %d",
					progressBar(pct, 20), pct*100,
					time.Duration(elapsed)*time.Second, totalTestTime,
					atomic.LoadInt64(&inflight),
					atomic.LoadUint64(&success),
					atomic.LoadUint64(&fail),
				)
			}
		}
	}()

	// ========= LOAD GENERATOR =========
	// Using Ticker for precise RPS control
	interval := time.Second / time.Duration(cfg.TargetRPS)
	ticker := time.NewTicker(interval)
	wg := sync.WaitGroup{}

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if time.Since(start) >= totalTestTime {
					return
				}

				wg.Add(1)
				atomic.AddUint64(&sent, 1)
				atomic.AddInt64(&inflight, 1)

				go func() {
					defer wg.Done()
					defer atomic.AddInt64(&inflight, -1)

					// 1. Generate Data (Matching Python Logic)
					reqCount := atomic.LoadUint64(&sent)
					payload := AsyncRequestPayload{
						Query:  SAMPLE_QUERIES[rand.Intn(len(SAMPLE_QUERIES))],
						UserID: fmt.Sprintf("load_test_user_%d", reqCount%1000),
						ChatID: fmt.Sprintf("load_test_chat_%d", reqCount),
					}

					bodyBytes, _ := json.Marshal(payload)

					req, _ := http.NewRequest("POST", cfg.BaseURL, bytes.NewBuffer(bodyBytes))
					req.Header.Set("Content-Type", "application/json")

					// 2. Execute Request
					startReq := time.Now()
					resp, err := client.Do(req)
					lat := time.Since(startReq)

					res := Result{
						TimeStamp: startReq,
						Latency:   lat,
						Err:       err,
					}

					// 3. Validation Logic
					if err == nil {
						res.Status = resp.StatusCode
						res.Bytes = resp.ContentLength

						// Read Body
						respBody, _ := io.ReadAll(resp.Body)
						resp.Body.Close()

						// Only store body on failure or for debugging (truncated)
						if len(respBody) > 0 {
							limit := 500
							if len(respBody) < limit {
								limit = len(respBody)
							}
							res.ResponseBody = string(respBody[:limit])
						}

						// Logic: Status 200 or 202 is OK
						if resp.StatusCode == 200 || resp.StatusCode == 202 {
							// Check JSON for query_id
							var jsonResp AsyncResponsePayload
							if jsonErr := json.Unmarshal(respBody, &jsonResp); jsonErr == nil {
								if jsonResp.QueryID != "" {
									res.Success = true
									atomic.AddUint64(&success, 1)
								} else {
									res.Success = false
									res.Err = fmt.Errorf("Missing query_id in response")
									atomic.AddUint64(&fail, 1)
								}
							} else {
								res.Success = false
								res.Err = fmt.Errorf("Invalid JSON")
								atomic.AddUint64(&fail, 1)
							}
						} else {
							res.Success = false
							atomic.AddUint64(&fail, 1)
						}
					} else {
						atomic.AddUint64(&fail, 1)
					}

					mu.Lock()
					results = append(results, res)
					mu.Unlock()
				}()
			}
		}
	}()

	// Wait for duration + drain inflight
	time.Sleep(totalTestTime + 500*time.Millisecond)
	if atomic.LoadInt64(&inflight) > 0 {
		fmt.Printf("\n\n⚠️  Waiting for %d inflight requests...", atomic.LoadInt64(&inflight))
	}
	wg.Wait()

	totalRealTime := time.Since(start)

	// ========= REPORT GENERATION =========
	generateReports(results, cfg, totalRealTime, success, fail)
}

func generateReports(results []Result, cfg Config, duration time.Duration, success, fail uint64) {
	var latencies []float64
	uniqueErrors := make(map[FailureSignature]int)
	statusCodes := make(map[int]int)

	// CSV Export
	var wRaw *csv.Writer
	if cfg.OutPrefix != "" {
		fRaw, _ := os.Create(cfg.OutPrefix + "_raw.csv")
		defer fRaw.Close()
		wRaw = csv.NewWriter(fRaw)
		wRaw.Write([]string{"timeStamp", "elapsed", "label", "responseCode", "success", "bytes", "failureMessage", "debugBody"})
	}

	for _, r := range results {
		statusCodes[r.Status]++
		if r.Success {
			latencies = append(latencies, float64(r.Latency.Milliseconds()))
		} else {
			// Error Analysis
			failMsg := ""
			if r.Err != nil {
				failMsg = r.Err.Error()
			} else {
				failMsg = fmt.Sprintf("HTTP %d", r.Status)
			}
			
			// Normalize Timeout errors
			if strings.Contains(failMsg, "Timeout") || strings.Contains(failMsg, "deadline exceeded") {
				failMsg = "Client Timeout"
			}

			sig := FailureSignature{Status: r.Status, Body: strings.TrimSpace(r.ResponseBody), Err: failMsg}
			uniqueErrors[sig]++
		}

		if wRaw != nil {
			failMsg := ""
			if r.Err != nil {
				failMsg = r.Err.Error()
			}
			cleanBody := strings.ReplaceAll(r.ResponseBody, "\n", " ")
			cleanBody = strings.ReplaceAll(cleanBody, "\"", "'")

			wRaw.Write([]string{
				fmt.Sprintf("%d", r.TimeStamp.UnixMilli()),
				fmt.Sprintf("%d", r.Latency.Milliseconds()),
				"AsyncSearch",
				strconv.Itoa(r.Status),
				strconv.FormatBool(r.Success),
				fmt.Sprintf("%d", r.Bytes),
				failMsg,
				cleanBody,
			})
		}
	}
	if wRaw != nil {
		wRaw.Flush()
	}

	// Stats
	sort.Float64s(latencies)
	avg := 0.0
	if len(latencies) > 0 {
		sum := 0.0
		for _, l := range latencies {
			sum += l
		}
		avg = sum / float64(len(latencies))
	}
	p50 := percentile(latencies, 50)
	p95 := percentile(latencies, 95)
	p99 := percentile(latencies, 99)
	actualRPS := float64(len(results)) / duration.Seconds()

	// Summary CSV
	if cfg.OutPrefix != "" {
		fSum, _ := os.Create(cfg.OutPrefix + "_summary.csv")
		wSum := csv.NewWriter(fSum)
		wSum.Write([]string{"Total_Reqs", "RPS_Achieved", "Success_Count", "Fail_Count", "Avg_Lat", "P50", "P95", "P99"})
		wSum.Write([]string{
			strconv.Itoa(len(results)),
			fmt.Sprintf("%.2f", actualRPS),
			strconv.FormatUint(success, 10),
			strconv.FormatUint(fail, 10),
			fmt.Sprintf("%.0f", avg),
			fmt.Sprintf("%.0f", p50),
			fmt.Sprintf("%.0f", p95),
			fmt.Sprintf("%.0f", p99),
		})
		wSum.Flush()
		fSum.Close()
	}

	// Console Output
	fmt.Printf("\n\n📊 LOAD TEST RESULTS\n")
	fmt.Printf("======================================================================\n")
	fmt.Printf("Total Duration : %s\n", duration)
	fmt.Printf("Requests Sent  : %d\n", len(results))
	fmt.Printf("Success        : %d\n", success)
	fmt.Printf("Failures       : %d\n", fail)
	fmt.Printf("Actual RPS     : %.2f\n", actualRPS)
	fmt.Printf("\n⏱️ RESPONSE TIMES (ms) [Success Only]\n")
	fmt.Printf("   Avg  : %.2f\n", avg)
	fmt.Printf("   P50  : %.0f\n", p50)
	fmt.Printf("   P95  : %.0f\n", p95)
	fmt.Printf("   P99  : %.0f\n", p99)

	if len(uniqueErrors) > 0 {
		fmt.Printf("\n❌ FAILURE SUMMARY\n")
		for sig, count := range uniqueErrors {
			fmt.Printf("   [%d] %d x %s | Body: %s\n", count, sig.Status, sig.Err, sig.Body)
		}
	}

	fmt.Printf("\n💾 Saved reports to %s_raw.csv and %s_summary.csv\n", cfg.OutPrefix, cfg.OutPrefix)
	fmt.Printf("======================================================================\n")
}