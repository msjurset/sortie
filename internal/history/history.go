package history

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Record represents a single dispatch action.
type Record struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"ts"`
	RuleName  string    `json:"rule"`
	Action    string    `json:"action"`
	Src       string    `json:"src"`
	Dest      string    `json:"dest,omitempty"`
	Undone    bool      `json:"undone,omitempty"`
	Error     string    `json:"error,omitempty"`
	ChainID   string    `json:"chain_id,omitempty"` // links actions in a chain
}

// Store reads and writes dispatch history as JSON Lines.
type Store struct {
	Path string
}

// NewStore creates a store that writes to the given file path.
func NewStore(path string) *Store {
	return &Store{Path: path}
}

// Append adds a record to the history file.
func (s *Store) Append(rec Record) error {
	if rec.ID == "" {
		rec.ID = newID()
	}
	if rec.Timestamp.IsZero() {
		rec.Timestamp = time.Now().UTC()
	}

	f, err := os.OpenFile(s.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening history file: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshaling record: %w", err)
	}

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("writing record: %w", err)
	}
	return nil
}

// List returns the most recent records, newest first.
func (s *Store) List(limit int) ([]Record, error) {
	f, err := os.Open(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("opening history file: %w", err)
	}
	defer f.Close()

	var records []Record
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var rec Record
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			continue
		}
		records = append(records, rec)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading history: %w", err)
	}

	// Reverse for newest first
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	if limit > 0 && len(records) > limit {
		records = records[:limit]
	}

	return records, nil
}

func newID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
