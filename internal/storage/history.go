package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"steadyq/internal/runner"
)

type HistoryItem struct {
	ID        string        `json:"id"`
	Timestamp time.Time     `json:"timestamp"`
	Config    runner.Config `json:"config"`
	Summary   RunSummary    `json:"summary"`
}

type RunSummary struct {
	TotalRequests uint64  `json:"total_requests"`
	Success       uint64  `json:"success"`
	Fail          uint64  `json:"fail"`
	AvgLatencyMs  float64 `json:"avg_latency_ms"`
	P99LatencyMs  float64 `json:"p99_latency_ms"`
}

type Store struct {
	mu       sync.RWMutex
	filePath string
	items    []HistoryItem
}

func NewStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	dir := filepath.Join(home, ".steadyq")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	path := filepath.Join(dir, "history.json")

	s := &Store{
		filePath: path,
	}

	s.load()
	return s, nil
}

func (s *Store) load() {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return // Ignore errors (file might not exist)
	}

	json.Unmarshal(data, &s.items)
}

func (s *Store) Save(item HistoryItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Add to beginning
	s.items = append([]HistoryItem{item}, s.items...)

	// Keep max 100 items
	if len(s.items) > 100 {
		s.items = s.items[:100]
	}

	data, err := json.MarshalIndent(s.items, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0644)
}

func (s *Store) List() []HistoryItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return copy
	res := make([]HistoryItem, len(s.items))
	copy(res, s.items)
	return res
}

func (s *Store) Get(id string) *HistoryItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, item := range s.items {
		if item.ID == id {
			return &item
		}
	}
	return nil
}
