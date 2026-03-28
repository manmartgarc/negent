package sync

import (
	"bufio"
	"os"
	"path/filepath"
	"testing"
)

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if line := scanner.Text(); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func TestMergeHistoryFiles_Dedup(t *testing.T) {
	dir := t.TempDir()

	// Two files with overlapping entries
	f1 := writeTestFile(t, dir, "a.jsonl", `{"timestamp":100,"sessionId":"s1","display":"hello"}
{"timestamp":200,"sessionId":"s1","display":"world"}
`)
	f2 := writeTestFile(t, dir, "b.jsonl", `{"timestamp":200,"sessionId":"s1","display":"world"}
{"timestamp":300,"sessionId":"s2","display":"new"}
`)

	dst := filepath.Join(dir, "merged.jsonl")
	if err := MergeHistoryFiles(dst, f1, f2); err != nil {
		t.Fatalf("MergeHistoryFiles: %v", err)
	}

	lines := readLines(t, dst)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
}

func TestMergeHistoryFiles_Sorted(t *testing.T) {
	dir := t.TempDir()

	// Out-of-order entries
	f1 := writeTestFile(t, dir, "a.jsonl", `{"timestamp":300,"sessionId":"s1","display":"c"}
{"timestamp":100,"sessionId":"s1","display":"a"}
`)
	f2 := writeTestFile(t, dir, "b.jsonl", `{"timestamp":200,"sessionId":"s2","display":"b"}
`)

	dst := filepath.Join(dir, "merged.jsonl")
	if err := MergeHistoryFiles(dst, f1, f2); err != nil {
		t.Fatalf("MergeHistoryFiles: %v", err)
	}

	entries, err := readHistoryFile(dst)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	for i := 1; i < len(entries); i++ {
		if entries[i].Timestamp < entries[i-1].Timestamp {
			t.Errorf("entries not sorted: %d < %d", entries[i].Timestamp, entries[i-1].Timestamp)
		}
	}
}

func TestMergeHistoryFiles_PreservesRawJSON(t *testing.T) {
	dir := t.TempDir()

	original := `{"timestamp":100,"sessionId":"s1","display":"hello","extra":"preserved"}`
	f1 := writeTestFile(t, dir, "a.jsonl", original+"\n")

	dst := filepath.Join(dir, "merged.jsonl")
	if err := MergeHistoryFiles(dst, f1); err != nil {
		t.Fatalf("MergeHistoryFiles: %v", err)
	}

	lines := readLines(t, dst)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0] != original {
		t.Errorf("raw JSON not preserved:\ngot:  %s\nwant: %s", lines[0], original)
	}
}

func TestMergeHistoryFiles_EmptyInputs(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "merged.jsonl")

	// No source files
	if err := MergeHistoryFiles(dst); err != nil {
		t.Fatalf("MergeHistoryFiles: %v", err)
	}

	lines := readLines(t, dst)
	if len(lines) != 0 {
		t.Errorf("expected 0 lines, got %d", len(lines))
	}
}

func TestMergeHistoryFiles_MissingFile(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "merged.jsonl")

	// One real file, one missing
	f1 := writeTestFile(t, dir, "a.jsonl", `{"timestamp":100,"sessionId":"s1","display":"hello"}
`)
	missing := filepath.Join(dir, "nonexistent.jsonl")

	if err := MergeHistoryFiles(dst, f1, missing); err != nil {
		t.Fatalf("MergeHistoryFiles should handle missing files: %v", err)
	}

	lines := readLines(t, dst)
	if len(lines) != 1 {
		t.Errorf("expected 1 line, got %d", len(lines))
	}
}

func TestMergeHistoryFiles_SkipsMalformed(t *testing.T) {
	dir := t.TempDir()

	f1 := writeTestFile(t, dir, "a.jsonl", `{"timestamp":100,"sessionId":"s1","display":"good"}
not json at all
{"timestamp":200,"sessionId":"s2","display":"also good"}
`)

	dst := filepath.Join(dir, "merged.jsonl")
	if err := MergeHistoryFiles(dst, f1); err != nil {
		t.Fatalf("MergeHistoryFiles: %v", err)
	}

	lines := readLines(t, dst)
	if len(lines) != 2 {
		t.Errorf("expected 2 lines (skipping malformed), got %d", len(lines))
	}
}
