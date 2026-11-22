// Package state remembers which feed items are already known, so a restart
// does not post them again.
package state

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// State maps a feed URL to the items known from it. A feed missing from the
// map has never been fetched successfully.
type State map[string][]Entry

// Entry is one known item. LastSeen is when the feed listed it the last time;
// an item that drops out of the feed stays known until it has been gone longer
// than the retention, so coming back does not make it look new.
type Entry struct {
	ID       string    `json:"id"`
	LastSeen time.Time `json:"lastSeen"`
}

// Load reads the stored state. A missing file is the first run, anything else
// is a real problem and must not be mistaken for "nothing known yet", which
// would skip every new item.
func Load(path string) (State, error) {
	loaded, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return State{}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}

	var state State
	if err := json.Unmarshal(loaded, &state); err != nil {
		return nil, fmt.Errorf("decode state: %w", err)
	}

	if state == nil {
		state = State{}
	}

	return state, nil
}

// Save replaces the stored state. The new content goes to a temporary file
// that is renamed into place, so a process killed mid write cannot leave a
// truncated file behind that the next start would refuse to decode.
func Save(path string, state State) error {
	// the default encoder escapes &, < and > for embedding in html, which only
	// makes the file harder to read
	var buf bytes.Buffer

	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")

	if err := enc.Encode(state); err != nil {
		return fmt.Errorf("encode state: %w", err)
	}

	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create state: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(buf.Bytes()); err != nil {
		tmp.Close()
		return fmt.Errorf("write state: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close state: %w", err)
	}

	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("replace state: %w", err)
	}

	return nil
}

// Refresh marks the listed IDs as seen now and drops entries the feed stopped
// listing more than retention ago. Listed IDs without an entry are not added:
// an item only becomes known once it was posted (or on the first run).
func Refresh(entries []Entry, listed map[string]bool, now time.Time, retention time.Duration) []Entry {
	kept := make([]Entry, 0, len(entries))

	for _, entry := range entries {
		if listed[entry.ID] {
			entry.LastSeen = now
		} else if now.Sub(entry.LastSeen) > retention {
			continue
		}

		kept = append(kept, entry)
	}

	return kept
}
