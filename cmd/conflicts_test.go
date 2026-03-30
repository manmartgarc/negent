package cmd

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	syncpkg "github.com/manmart/negent/internal/sync"
)

func TestApplyConflictActionKeepLocalUpdatesStaging(t *testing.T) {
	tmp := t.TempDir()
	localPath := filepath.Join(tmp, "local.md")
	stagingPath := filepath.Join(tmp, "staging.md")

	if err := os.WriteFile(localPath, []byte("local value"), 0o644); err != nil {
		t.Fatalf("write local: %v", err)
	}
	if err := os.WriteFile(stagingPath, []byte("staging value"), 0o644); err != nil {
		t.Fatalf("write staging: %v", err)
	}

	conflict := syncpkg.ConflictInfo{
		Agent:       "claude",
		RelPath:     "CLAUDE.md",
		LocalPath:   localPath,
		StagingPath: stagingPath,
	}
	stagingChanged, err := applyConflictAction(conflict, conflictActionKeepLocal)
	if err != nil {
		t.Fatalf("applyConflictAction: %v", err)
	}
	if !stagingChanged {
		t.Fatal("expected keep-local to mark staging as changed")
	}

	got, err := os.ReadFile(stagingPath)
	if err != nil {
		t.Fatalf("read staging: %v", err)
	}
	if string(got) != "local value" {
		t.Fatalf("staging content = %q, want %q", string(got), "local value")
	}
}

func TestApplyConflictActionTakeRemoteUpdatesLocalOnly(t *testing.T) {
	tmp := t.TempDir()
	localPath := filepath.Join(tmp, "local.md")
	stagingPath := filepath.Join(tmp, "staging.md")

	if err := os.WriteFile(localPath, []byte("local value"), 0o644); err != nil {
		t.Fatalf("write local: %v", err)
	}
	if err := os.WriteFile(stagingPath, []byte("remote value"), 0o644); err != nil {
		t.Fatalf("write staging: %v", err)
	}

	conflict := syncpkg.ConflictInfo{
		Agent:       "claude",
		RelPath:     "CLAUDE.md",
		LocalPath:   localPath,
		StagingPath: stagingPath,
	}
	stagingChanged, err := applyConflictAction(conflict, conflictActionTakeRemote)
	if err != nil {
		t.Fatalf("applyConflictAction: %v", err)
	}
	if stagingChanged {
		t.Fatal("expected take-remote to leave staging unchanged")
	}

	local, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("read local: %v", err)
	}
	if string(local) != "remote value" {
		t.Fatalf("local content = %q, want %q", string(local), "remote value")
	}
}

func TestUniqueRelativePathsDeduplicatesAndSorts(t *testing.T) {
	stagingDir := t.TempDir()
	in := []string{
		filepath.Join(stagingDir, "claude", "z.txt"),
		filepath.Join(stagingDir, "claude", "a.txt"),
		"claude/a.txt",
	}

	got, err := uniqueRelativePaths(stagingDir, in)
	if err != nil {
		t.Fatalf("uniqueRelativePaths: %v", err)
	}

	want := []string{"claude/a.txt", "claude/z.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("paths = %#v, want %#v", got, want)
	}
}

func TestUniqueRelativePathsRejectsOutsideStagingDir(t *testing.T) {
	stagingDir := t.TempDir()
	outside := filepath.Join(filepath.Dir(stagingDir), "outside.txt")

	_, err := uniqueRelativePaths(stagingDir, []string{outside})
	if err == nil {
		t.Fatal("expected error for path outside staging dir")
	}
}

func TestCommitConflictResolutionsNoPathsSkipsGit(t *testing.T) {
	oldRunGit := runGit
	t.Cleanup(func() { runGit = oldRunGit })

	called := false
	runGit = func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		called = true
		return nil, nil
	}

	committed, err := commitConflictResolutions(context.Background(), t.TempDir(), nil)
	if err != nil {
		t.Fatalf("commitConflictResolutions: %v", err)
	}
	if committed {
		t.Fatal("expected no commit when no paths are provided")
	}
	if called {
		t.Fatal("git should not be called when no paths are provided")
	}
}

func TestCommitConflictResolutionsSkipsCommitWhenNoStagedDiff(t *testing.T) {
	oldRunGit := runGit
	t.Cleanup(func() { runGit = oldRunGit })

	var calls [][]string
	runGit = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		copied := append([]string(nil), args...)
		calls = append(calls, copied)
		if len(args) > 0 && args[0] == "diff" {
			return []byte(""), nil
		}
		return []byte(""), nil
	}

	committed, err := commitConflictResolutions(context.Background(), t.TempDir(), []string{"claude/CLAUDE.md"})
	if err != nil {
		t.Fatalf("commitConflictResolutions: %v", err)
	}
	if committed {
		t.Fatal("expected no commit when staged diff is empty")
	}

	if len(calls) != 2 {
		t.Fatalf("expected 2 git calls (add + diff), got %d", len(calls))
	}
	if calls[0][0] != "add" {
		t.Fatalf("first git call = %v, want add", calls[0])
	}
	if calls[1][0] != "diff" {
		t.Fatalf("second git call = %v, want diff", calls[1])
	}
}

func TestCommitConflictResolutionsCommitsWhenStagedDiffExists(t *testing.T) {
	oldRunGit := runGit
	t.Cleanup(func() { runGit = oldRunGit })

	var calls [][]string
	runGit = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		copied := append([]string(nil), args...)
		calls = append(calls, copied)
		if len(args) > 0 && args[0] == "diff" {
			return []byte("claude/CLAUDE.md\n"), nil
		}
		return []byte(""), nil
	}

	committed, err := commitConflictResolutions(context.Background(), t.TempDir(), []string{"claude/CLAUDE.md"})
	if err != nil {
		t.Fatalf("commitConflictResolutions: %v", err)
	}
	if !committed {
		t.Fatal("expected commit when staged diff exists")
	}

	if len(calls) != 3 {
		t.Fatalf("expected 3 git calls (add + diff + commit), got %d", len(calls))
	}
	last := calls[2]
	if len(last) < 3 || last[0] != "commit" || last[1] != "-m" || last[2] != conflictsCommitMessage {
		t.Fatalf("unexpected commit call: %v", last)
	}
}

