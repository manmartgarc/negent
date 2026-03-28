package sync

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// historyEntry represents a single line in a history.jsonl file.
// We only parse the fields needed for deduplication and sorting.
type historyEntry struct {
	Timestamp int64  `json:"timestamp"`
	SessionID string `json:"sessionId"`
	raw       string // original JSON line, preserved verbatim
}

// dedupKey returns a unique key for deduplication.
func (e historyEntry) dedupKey() string {
	return fmt.Sprintf("%d:%s", e.Timestamp, e.SessionID)
}

// MergeHistoryFiles performs a union merge of multiple history.jsonl files.
// It deduplicates by (timestamp, sessionId) and sorts chronologically.
// The result is written to dst.
func MergeHistoryFiles(dst string, srcs ...string) error {
	seen := make(map[string]bool)
	var entries []historyEntry

	for _, src := range srcs {
		parsed, err := readHistoryFile(src)
		if err != nil {
			return fmt.Errorf("reading %s: %w", src, err)
		}
		for _, e := range parsed {
			key := e.dedupKey()
			if !seen[key] {
				seen[key] = true
				entries = append(entries, e)
			}
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp < entries[j].Timestamp
	})

	return writeHistoryFile(dst, entries)
}

// readHistoryFile parses a JSONL history file, preserving original lines.
func readHistoryFile(path string) ([]historyEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var entries []historyEntry
	scanner := bufio.NewScanner(f)
	// Increase buffer for potentially large lines
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var e historyEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			// Skip malformed lines rather than failing
			continue
		}
		e.raw = line
		entries = append(entries, e)
	}

	return entries, scanner.Err()
}

// writeHistoryFile writes entries as JSONL, using the original raw lines.
func writeHistoryFile(path string, entries []historyEntry) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, e := range entries {
		if _, err := w.WriteString(e.raw + "\n"); err != nil {
			return err
		}
	}
	return w.Flush()
}
