package sync

import (
	"context"
	"os"
	"path/filepath"
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

func (m *mockAgent) Name() string     { return m.name }
func (m *mockAgent) SourceDir() string { return m.sourceDir }
func (m *mockAgent) Collect(categories []agent.Category) ([]agent.SyncFile, error) {
	return m.files, nil
}
func (m *mockAgent) Place(stagingDir string, files []agent.SyncFile) (*agent.PlaceResult, error) {
	m.placedFiles = files
	return &agent.PlaceResult{Placed: len(files)}, nil
}
func (m *mockAgent) Diff(stagingDir string, categories []agent.Category) ([]backend.FileChange, error) {
	return nil, nil
}
func (m *mockAgent) DefaultCategories() []agent.Category {
	return []agent.Category{agent.CategoryConfig}
}

func TestPush(t *testing.T) {
	be := newMockBackend(t)
	files := []agent.SyncFile{
		{RelPath: "CLAUDE.md", StagingPath: "CLAUDE.md", Category: agent.CategoryConfig},
		{RelPath: "settings.json", StagingPath: "settings.json", Category: agent.CategoryConfig},
	}
	ag := newMockAgent(t, "claude", files)

	agents := map[string]agent.Agent{"claude": ag}
	categories := map[string][]agent.Category{"claude": {agent.CategoryConfig}}

	orch := NewOrchestrator(be, agents)
	if err := orch.Push(context.Background(), categories); err != nil {
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
	// Empty categories = nothing to sync
	categories := map[string][]agent.Category{}

	orch := NewOrchestrator(be, agents)
	if err := orch.Push(context.Background(), categories); err != nil {
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
	categories := map[string][]agent.Category{"claude": {agent.CategoryConfig}}

	orch := NewOrchestrator(be, agents)
	if err := orch.Pull(context.Background(), categories); err != nil {
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
	categories := map[string][]agent.Category{"claude": {agent.CategoryConfig}}

	orch := NewOrchestrator(be, agents)
	if err := orch.Pull(context.Background(), categories); err != nil {
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
	// settings.json matches base → safe → must be placed.
	if !placed["settings.json"] {
		t.Error("settings.json should be placed (local matches base)")
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
	categories := map[string][]agent.Category{"claude": {agent.CategoryConfig}}

	orch := NewOrchestrator(be, agents)
	if err := orch.Pull(context.Background(), categories); err != nil {
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
	categories := map[string][]agent.Category{"claude": {agent.CategoryConfig}}

	orch := NewOrchestrator(be, agents)
	if err := orch.Pull(context.Background(), categories); err != nil {
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
