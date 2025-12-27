package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.etcd.io/bbolt"
)

const (
	BucketRuns = "runs"
)

type Store struct {
	db       *bbolt.DB
	filePath string
}

func NewStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	dir := filepath.Join(home, ".steadyq", "sessions")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	// Create a unique file for this session
	filename := fmt.Sprintf("session_%d.db", time.Now().UnixNano())
	path := filepath.Join(dir, filename)

	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}

	// Initialize Buckets
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(BucketRuns))
		return err
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	return &Store{
		db:       db,
		filePath: path,
	}, nil
}

func (s *Store) Close() error {
	if s.db != nil {
		s.db.Close()
	}
	// Cleanup the file for "ephemeral" session storage
	if s.filePath != "" {
		return os.Remove(s.filePath)
	}
	return nil
}

func (s *Store) Save(item HistoryItem) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(BucketRuns))

		id := []byte(item.ID)
		data, err := json.Marshal(item)
		if err != nil {
			return err
		}

		return b.Put(id, data)
	})
}

// List returns items without the full Results payload to save memory/time if needed.
// However, since we want full export capabilities from history, we load everything.
// Optimisation: We could create a potentially lighter struct for List if needed.
func (s *Store) List() []HistoryItem {
	var items []HistoryItem

	s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(BucketRuns))
		c := b.Cursor()

		for k, v := c.Last(); k != nil; k, v = c.Prev() {
			var item HistoryItem
			if err := json.Unmarshal(v, &item); err == nil {
				items = append(items, item)
			}
		}
		return nil
	})

	return items
}

func (s *Store) Get(id string) (*HistoryItem, error) {
	var item HistoryItem
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(BucketRuns))
		v := b.Get([]byte(id))
		if v == nil {
			return fmt.Errorf("item not found")
		}
		return json.Unmarshal(v, &item)
	})
	if err != nil {
		return nil, err
	}
	return &item, nil
}
