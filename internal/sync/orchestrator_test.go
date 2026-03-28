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
	stagingDir string
	pushed     bool
	pulled     bool
	lastMsg    string
}

func newMockBackend(t *testing.T) *mockBackend {
	return &mockBackend{stagingDir: t.TempDir()}
}

func (m *mockBackend) Init(_ context.Context, _ backend.BackendConfig) error { return nil }
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
	name      string
	sourceDir string
	files     []agent.SyncFile
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

func (m *mockAgent) Name() string        { return m.name }
func (m *mockAgent) SourceDir() string    { return m.sourceDir }
func (m *mockAgent) Collect(categories []agent.Category) ([]agent.SyncFile, error) {
	return m.files, nil
}
func (m *mockAgent) Place(stagingDir string, files []agent.SyncFile) (*agent.PlaceResult, error) {
	return &agent.PlaceResult{Placed: len(files)}, nil
}
func (m *mockAgent) Diff(stagingDir string) ([]backend.FileChange, error) { return nil, nil }
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
