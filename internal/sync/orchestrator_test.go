package sync

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/manmart/negent/internal/agent"
	"github.com/manmart/negent/internal/backend"
)

// mockBackend implements backend.Backend for testing.
type mockBackend struct {
	stagingDir   string
	pushed       bool
	pulled       bool
	fetched      bool
	lastMsg      string
	fetchedFiles []string // files to return from FetchedFiles()
}

func newMockBackend(t *testing.T) *mockBackend {
	return &mockBackend{stagingDir: t.TempDir()}
}

func (m *mockBackend) Init(_ context.Context, _ backend.BackendConfig) error { return nil }
func (m *mockBackend) Fetch(_ context.Context) error {
	m.fetched = true
	return nil
}
func (m *mockBackend) FetchedFiles(_ context.Context) ([]string, error) {
	return m.fetchedFiles, nil
}
func (m *mockBackend) Push(_ context.Context, msg string) error {
	m.pushed = true
	m.lastMsg = msg
	return nil
}
func (m *mockBackend) Pull(_ context.Context) error {
	m.pulled = true
	return nil
}
func (m *mockBackend) Status(_ context.Context) ([]backend.FileChange, error) { return nil, nil }
func (m *mockBackend) StagingDir() string                                     { return m.stagingDir }

// mockAgent implements agent.Agent for testing.
type mockAgent struct {
	name        string
	sourceDir   string
	files       []agent.SyncFile
	placedFiles []agent.SyncFile // recorded by the most recent Place() call
}

const (
	mockTypeClaudeMD = agent.SyncType("claude-md")
	mockTypeSessions = agent.SyncType("sessions")
)

func newMockAgent(t *testing.T, name string, files []agent.SyncFile) *mockAgent {
	dir := t.TempDir()
	// Create the actual files so copyFile works
	for _, f := range files {
		path := filepath.Join(dir, f.RelPath)
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, []byte("test content for "+f.RelPath), 0o644)
	}
	return &mockAgent{name: name, sourceDir: dir, files: files}
}

func (m *mockAgent) Name() string      { return m.name }
func (m *mockAgent) SourceDir() string { return m.sourceDir }
func (m *mockAgent) SupportedSyncTypes() []agent.SyncTypeSpec {
	return []agent.SyncTypeSpec{
		{ID: mockTypeClaudeMD, Default: true, Mode: agent.SyncModeReplace},
		{ID: mockTypeSessions, Mode: agent.SyncModeAppendOnly},
	}
}
func (m *mockAgent) DefaultSyncTypes() []agent.SyncType {
	return []agent.SyncType{mockTypeClaudeMD}
}
func (m *mockAgent) NormalizeSyncTypes(selected []string) ([]agent.SyncType, error) {
	var syncTypes []agent.SyncType
	for _, selectedType := range selected {
		syncTypes = append(syncTypes, agent.SyncType(selectedType))
	}
	return syncTypes, nil
}
func (m *mockAgent) Collect(syncTypes []agent.SyncType) ([]agent.SyncFile, error) {
	return m.files, nil
}
func (m *mockAgent) Place(stagingDir string, files []agent.SyncFile) (*agent.PlaceResult, error) {
	m.placedFiles = files
	return &agent.PlaceResult{Placed: len(files)}, nil
}
func (m *mockAgent) Diff(stagingDir string, syncTypes []agent.SyncType) ([]backend.FileChange, error) {
	// Walk staging to find files not in the collected set (deletions).
	collected := make(map[string]bool)
	for _, f := range m.files {
		collected[f.StagingPath] = true
	}

	var changes []backend.FileChange
	filepath.WalkDir(stagingDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(stagingDir, path)
		rel = filepath.ToSlash(rel)
		if !collected[rel] {
			changes = append(changes, backend.FileChange{Path: rel, Kind: backend.ChangeDeleted})
		}
		return nil
	})
	return changes, nil
}
func (m *mockAgent) SyncTypeForPath(relPath string) agent.SyncType {
	switch {
	case relPath == "CLAUDE.md":
		return mockTypeClaudeMD
	case strings.HasSuffix(relPath, ".jsonl"):
		return mockTypeSessions
	default:
		return ""
	}
}

func TestPush(t *testing.T) {
	be := newMockBackend(t)
	files := []agent.SyncFile{
		{RelPath: "CLAUDE.md", StagingPath: "CLAUDE.md", Type: mockTypeClaudeMD},
		{RelPath: "settings.json", StagingPath: "settings.json", Type: mockTypeClaudeMD},
	}
	ag := newMockAgent(t, "claude", files)

	agents := map[string]agent.Agent{"claude": ag}
	syncTypes := map[string][]agent.SyncType{"claude": {mockTypeClaudeMD}}

	orch := NewOrchestrator(be, agents)
	if err := orch.Push(context.Background(), syncTypes); err != nil {
		t.Fatalf("Push: %v", err)
	}

	if !be.pushed {
		t.Error("backend.Push was not called")
	}

	// Verify files were copied to staging dir
	for _, f := range files {
		staged := filepath.Join(be.StagingDir(), "claude", f.StagingPath)
		if _, err := os.Stat(staged); os.IsNotExist(err) {
			t.Errorf("file not staged: %s", staged)
		}
	}
}

func TestPushSkipsUnconfiguredAgents(t *testing.T) {
	be := newMockBackend(t)
	ag := newMockAgent(t, "claude", nil)

	agents := map[string]agent.Agent{"claude": ag}
	// Empty sync types = nothing to sync
	syncTypes := map[string][]agent.SyncType{}

	orch := NewOrchestrator(be, agents)
	if err := orch.Push(context.Background(), syncTypes); err != nil {
		t.Fatalf("Push: %v", err)
	}

	if !be.pushed {
		t.Error("backend.Push should still be called (with empty commit)")
	}
}

func TestPull(t *testing.T) {
	be := newMockBackend(t)

	// Pre-populate staging dir with files
	agentDir := filepath.Join(be.StagingDir(), "claude")
	os.MkdirAll(agentDir, 0o755)
	os.WriteFile(filepath.Join(agentDir, "CLAUDE.md"), []byte("# Test"), 0o644)

	ag := newMockAgent(t, "claude", nil)
	agents := map[string]agent.Agent{"claude": ag}
	syncTypes := map[string][]agent.SyncType{"claude": {mockTypeClaudeMD}}

	orch := NewOrchestrator(be, agents)
	if err := orch.Pull(context.Background(), syncTypes); err != nil {
		t.Fatalf("Pull: %v", err)
	}

	if !be.pulled {
		t.Error("backend.Pull was not called")
	}
}

func TestPullConflict(t *testing.T) {
	be := newMockBackend(t)

	// Populate staging with base content (the last synced remote state).
	agentDir := filepath.Join(be.StagingDir(), "claude")
	os.MkdirAll(agentDir, 0o755)
	os.WriteFile(filepath.Join(agentDir, "CLAUDE.md"), []byte("base content"), 0o644)
	os.WriteFile(filepath.Join(agentDir, "settings.json"), []byte(`{"key":"val"}`), 0o644)

	// Local: CLAUDE.md edited since last sync, settings.json unchanged.
	ag := newMockAgent(t, "claude", nil)
	os.WriteFile(filepath.Join(ag.SourceDir(), "CLAUDE.md"), []byte("local changes"), 0o644)
	os.WriteFile(filepath.Join(ag.SourceDir(), "settings.json"), []byte(`{"key":"val"}`), 0o644)

	agents := map[string]agent.Agent{"claude": ag}
	syncTypes := map[string][]agent.SyncType{"claude": {mockTypeClaudeMD}}

	orch := NewOrchestrator(be, agents)
	if err := orch.Pull(context.Background(), syncTypes); err != nil {
		t.Fatalf("Pull: %v", err)
	}

	placed := make(map[string]bool)
	for _, f := range ag.placedFiles {
		placed[f.StagingPath] = true
	}

	// CLAUDE.md differs from base → conflict → must NOT be placed.
	if placed["CLAUDE.md"] {
		t.Error("CLAUDE.md should not be placed (local diverged from base)")
	}
}

func TestPullNoConflict(t *testing.T) {
	be := newMockBackend(t)

	agentDir := filepath.Join(be.StagingDir(), "claude")
	os.MkdirAll(agentDir, 0o755)
	os.WriteFile(filepath.Join(agentDir, "CLAUDE.md"), []byte("base content"), 0o644)

	// Local matches base — no local edits.
	ag := newMockAgent(t, "claude", nil)
	os.WriteFile(filepath.Join(ag.SourceDir(), "CLAUDE.md"), []byte("base content"), 0o644)

	agents := map[string]agent.Agent{"claude": ag}
	syncTypes := map[string][]agent.SyncType{"claude": {mockTypeClaudeMD}}

	orch := NewOrchestrator(be, agents)
	if err := orch.Pull(context.Background(), syncTypes); err != nil {
		t.Fatalf("Pull: %v", err)
	}

	placed := make(map[string]bool)
	for _, f := range ag.placedFiles {
		placed[f.StagingPath] = true
	}

	if !placed["CLAUDE.md"] {
		t.Error("CLAUDE.md should be placed (local matches base, no conflict)")
	}
}

func TestPullNewRemoteFile(t *testing.T) {
	be := newMockBackend(t)

	// Staging has only settings.json; CLAUDE.md is newly added by remote.
	agentDir := filepath.Join(be.StagingDir(), "claude")
	os.MkdirAll(agentDir, 0o755)
	os.WriteFile(filepath.Join(agentDir, "settings.json"), []byte(`{}`), 0o644)
	os.WriteFile(filepath.Join(agentDir, "CLAUDE.md"), []byte("brand new"), 0o644)

	ag := newMockAgent(t, "claude", nil)
	// No local CLAUDE.md — remote added it fresh.

	agents := map[string]agent.Agent{"claude": ag}
	syncTypes := map[string][]agent.SyncType{"claude": {mockTypeClaudeMD}}

	orch := NewOrchestrator(be, agents)
	if err := orch.Pull(context.Background(), syncTypes); err != nil {
		t.Fatalf("Pull: %v", err)
	}

	placed := make(map[string]bool)
	for _, f := range ag.placedFiles {
		placed[f.StagingPath] = true
	}

	// No local file → no conflict → safe to place.
	if !placed["CLAUDE.md"] {
		t.Error("CLAUDE.md should be placed (newly added by remote, no local conflict)")
	}
}

func TestPushDeletesStaleStagedFiles(t *testing.T) {
	be := newMockBackend(t)

	// Pre-populate staging with a file that will no longer be collected.
	agentDir := filepath.Join(be.StagingDir(), "claude")
	os.MkdirAll(agentDir, 0o755)
	os.WriteFile(filepath.Join(agentDir, "settings.json"), []byte(`{"old":"data"}`), 0o644)
	os.WriteFile(filepath.Join(agentDir, "CLAUDE.md"), []byte("old"), 0o644)

	// Agent now only collects CLAUDE.md — settings.json was removed from collection.
	files := []agent.SyncFile{
		{RelPath: "CLAUDE.md", StagingPath: "CLAUDE.md", Type: mockTypeClaudeMD},
	}
	ag := newMockAgent(t, "claude", files)

	agents := map[string]agent.Agent{"claude": ag}
	syncTypes := map[string][]agent.SyncType{"claude": {mockTypeClaudeMD}}

	orch := NewOrchestrator(be, agents)
	if err := orch.Push(context.Background(), syncTypes); err != nil {
		t.Fatalf("Push: %v", err)
	}

	// settings.json should have been removed from staging.
	staleFile := filepath.Join(agentDir, "settings.json")
	if _, err := os.Stat(staleFile); !os.IsNotExist(err) {
		t.Errorf("settings.json should have been deleted from staging, but still exists")
	}

	// CLAUDE.md should still be present.
	keptFile := filepath.Join(agentDir, "CLAUDE.md")
	if _, err := os.Stat(keptFile); os.IsNotExist(err) {
		t.Error("CLAUDE.md should still exist in staging")
	}
}

func TestPushMergesSessionFiles(t *testing.T) {
	be := newMockBackend(t)

	// Pre-populate staging with a session file (as if pushed from another machine).
	agentDir := filepath.Join(be.StagingDir(), "claude")
	projDir := filepath.Join(agentDir, "projects", "myapp")
	os.MkdirAll(projDir, 0o755)
	stagedContent := "line-A\nline-B\nline-C\n"
	os.WriteFile(filepath.Join(projDir, "session.jsonl"), []byte(stagedContent), 0o644)

	// Local session has overlapping lines (B,C) plus new lines (D,E).
	localContent := "line-B\nline-C\nline-D\nline-E\n"
	files := []agent.SyncFile{
		{RelPath: "projects/myapp/session.jsonl", StagingPath: "projects/myapp/session.jsonl", Type: mockTypeSessions},
	}
	ag := newMockAgent(t, "claude", files)
	sessionPath := filepath.Join(ag.SourceDir(), "projects", "myapp")
	os.MkdirAll(sessionPath, 0o755)
	os.WriteFile(filepath.Join(sessionPath, "session.jsonl"), []byte(localContent), 0o644)

	agents := map[string]agent.Agent{"claude": ag}
	syncTypes := map[string][]agent.SyncType{"claude": {mockTypeSessions}}

	orch := NewOrchestrator(be, agents)
	if err := orch.Push(context.Background(), syncTypes); err != nil {
		t.Fatalf("Push: %v", err)
	}

	// Verify merged result: staged lines preserved + local-only lines appended.
	merged, err := os.ReadFile(filepath.Join(projDir, "session.jsonl"))
	if err != nil {
		t.Fatalf("reading merged file: %v", err)
	}

	lines := strings.Split(strings.TrimRight(string(merged), "\n"), "\n")
	want := []string{"line-A", "line-B", "line-C", "line-D", "line-E"}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d:\n%s", len(lines), len(want), string(merged))
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("line %d: got %q, want %q", i, lines[i], w)
		}
	}
}

func TestPushNewSessionCopied(t *testing.T) {
	be := newMockBackend(t)

	// No pre-existing staged session.
	localContent := "line-A\nline-B\n"
	files := []agent.SyncFile{
		{RelPath: "projects/myapp/session.jsonl", StagingPath: "projects/myapp/session.jsonl", Type: mockTypeSessions},
	}
	ag := newMockAgent(t, "claude", files)
	sessionPath := filepath.Join(ag.SourceDir(), "projects", "myapp")
	os.MkdirAll(sessionPath, 0o755)
	os.WriteFile(filepath.Join(sessionPath, "session.jsonl"), []byte(localContent), 0o644)

	agents := map[string]agent.Agent{"claude": ag}
	syncTypes := map[string][]agent.SyncType{"claude": {mockTypeSessions}}

	orch := NewOrchestrator(be, agents)
	if err := orch.Push(context.Background(), syncTypes); err != nil {
		t.Fatalf("Push: %v", err)
	}

	// Verify file was copied (not merged, since no prior staging).
	staged, err := os.ReadFile(filepath.Join(be.StagingDir(), "claude", "projects", "myapp", "session.jsonl"))
	if err != nil {
		t.Fatalf("reading staged file: %v", err)
	}
	if string(staged) != localContent {
		t.Errorf("staged content = %q, want %q", string(staged), localContent)
	}
}

func TestPullNoData(t *testing.T) {
	be := newMockBackend(t)
	// agentDir does not exist — simulates a remote with no data for this agent.
	ag := newMockAgent(t, "claude", nil)
	agents := map[string]agent.Agent{"claude": ag}
	syncTypes := map[string][]agent.SyncType{"claude": {mockTypeClaudeMD}}

	orch := NewOrchestrator(be, agents)
	if err := orch.Pull(context.Background(), syncTypes); err != nil {
		t.Fatalf("Pull with no remote data: %v", err)
	}
	if len(ag.placedFiles) != 0 {
		t.Errorf("expected no placed files, got %d", len(ag.placedFiles))
	}
}

func TestPushDeleteCleansEmptyDirs(t *testing.T) {
	be := newMockBackend(t)

	// Pre-populate staging with a file in a subdirectory.
	agentDir := filepath.Join(be.StagingDir(), "claude")
	subDir := filepath.Join(agentDir, "commands")
	os.MkdirAll(subDir, 0o755)
	os.WriteFile(filepath.Join(subDir, "old-command.md"), []byte("old"), 0o644)

	// Agent collects nothing — all files removed.
	ag := newMockAgent(t, "claude", nil)

	agents := map[string]agent.Agent{"claude": ag}
	syncTypes := map[string][]agent.SyncType{"claude": {mockTypeClaudeMD}}

	orch := NewOrchestrator(be, agents)
	if err := orch.Push(context.Background(), syncTypes); err != nil {
		t.Fatalf("Push: %v", err)
	}

	// The file should be removed.
	if _, err := os.Stat(filepath.Join(subDir, "old-command.md")); !os.IsNotExist(err) {
		t.Error("old-command.md should have been deleted from staging")
	}

	// Verify the parent "commands" dir is now empty or removed.
	entries, err := os.ReadDir(subDir)
	if err == nil && len(entries) > 0 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("commands/ should be empty after deletion, but contains: %s", strings.Join(names, ", "))
	}
}
