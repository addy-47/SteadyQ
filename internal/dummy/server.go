package dummy

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"
)

type ServerConfig struct {
	Port int
}

func Start(cfg ServerConfig) {
	mux := http.NewServeMux()

	// 1. Fast Endpoint (10-50ms)
	mux.HandleFunc("/fast", func(w http.ResponseWriter, r *http.Request) {
		jitter := time.Duration(rand.Intn(40)+10) * time.Millisecond
		time.Sleep(jitter)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Fast response"))
	})

	// 2. Medium Endpoint (100-300ms)
	mux.HandleFunc("/medium", func(w http.ResponseWriter, r *http.Request) {
		jitter := time.Duration(rand.Intn(200)+100) * time.Millisecond
		time.Sleep(jitter)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Medium response"))
	})

	// 3. Slow Endpoint (1s-2s) - Good for testing timeouts and queuing
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		jitter := time.Duration(rand.Intn(1000)+1000) * time.Millisecond
		time.Sleep(jitter)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Slow response"))
	})

	// 4. Spike Endpoint (Usually fast, randomly very slow)
	// P99 will be terrible, P50 will be fine.
	mux.HandleFunc("/spike", func(w http.ResponseWriter, r *http.Request) {
		if rand.Float32() < 0.05 { // 5% chance of spike
			time.Sleep(2 * time.Second)
		} else {
			time.Sleep(20 * time.Millisecond)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Spikey response"))
	})

	// 5. Error Endpoint (Random failures)
	mux.HandleFunc("/error", func(w http.ResponseWriter, r *http.Request) {
		rnd := rand.Float32()
		if rnd < 0.2 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 Internal Server Error"))
		} else if rnd < 0.4 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("429 Too Many Requests"))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}
	})

	addr := fmt.Sprintf(":%d", cfg.Port)
	fmt.Printf("👻 Dummy Server running on http://localhost%s\n", addr)
	fmt.Println("   Endpoints: /fast, /medium, /slow, /spike, /error")

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Server failed: %v\n", err)
		}
	}()
}
