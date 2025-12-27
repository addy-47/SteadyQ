package main

import (
	"fmt"
	"net/http"
	"time"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Simulate processing time
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, SteadyQ!"))
	})

	fmt.Println("Dummy server listening on :8080")
	http.ListenAndServe(":8080", nil)
}
