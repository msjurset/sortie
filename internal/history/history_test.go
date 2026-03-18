package history

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAppendAndList(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "history.json"))

	records := []Record{
		{ID: "aaa", Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), RuleName: "rule-a", Action: "move", Src: "/a", Dest: "/b"},
		{ID: "bbb", Timestamp: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), RuleName: "rule-b", Action: "copy", Src: "/c", Dest: "/d"},
		{ID: "ccc", Timestamp: time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC), RuleName: "rule-c", Action: "move", Src: "/e", Dest: "/f"},
	}

	for _, rec := range records {
		if err := store.Append(rec); err != nil {
			t.Fatalf("Append() error: %v", err)
		}
	}

	got, err := store.List(0)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("List() returned %d records, want 3", len(got))
	}

	// Should be newest first
	if got[0].ID != "ccc" {
		t.Errorf("first record ID = %q, want %q", got[0].ID, "ccc")
	}
	if got[2].ID != "aaa" {
		t.Errorf("last record ID = %q, want %q", got[2].ID, "aaa")
	}
}

func TestListWithLimit(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "history.json"))

	for i := 0; i < 10; i++ {
		rec := Record{
			RuleName: "rule",
			Action:   "move",
			Src:      "/src",
			Dest:     "/dest",
		}
		if err := store.Append(rec); err != nil {
			t.Fatalf("Append() error: %v", err)
		}
	}

	got, err := store.List(3)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	if len(got) != 3 {
		t.Errorf("List(3) returned %d records, want 3", len(got))
	}
}

func TestListEmpty(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "history.json"))

	got, err := store.List(0)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if got != nil {
		t.Errorf("List() on empty store returned %d records, want nil", len(got))
	}
}
