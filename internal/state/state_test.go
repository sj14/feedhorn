package state

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingFile(t *testing.T) {
	state, err := Load(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}

	if len(state) != 0 {
		t.Fatalf("got %v, want empty state", state)
	}
}

func TestSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)

	saved := State{"https://example.com/feed": {{ID: "a", LastSeen: now}}}
	if err := Save(path, saved); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	entries := loaded["https://example.com/feed"]
	if len(entries) != 1 || entries[0].ID != "a" || !entries[0].LastSeen.Equal(now) {
		t.Fatalf("got %v, want %v", loaded, saved)
	}
}

func TestRefresh(t *testing.T) {
	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	retention := 24 * time.Hour

	entries := []Entry{
		{ID: "listed", LastSeen: now.Add(-48 * time.Hour)},
		{ID: "recently-gone", LastSeen: now.Add(-time.Hour)},
		{ID: "long-gone", LastSeen: now.Add(-48 * time.Hour)},
	}
	listed := map[string]bool{"listed": true, "never-posted": true}

	got := Refresh(entries, listed, now, retention)

	want := []Entry{
		{ID: "listed", LastSeen: now},
		{ID: "recently-gone", LastSeen: now.Add(-time.Hour)},
	}

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	for i := range want {
		if got[i].ID != want[i].ID || !got[i].LastSeen.Equal(want[i].LastSeen) {
			t.Fatalf("entry %d is %v, want %v", i, got[i], want[i])
		}
	}
}
