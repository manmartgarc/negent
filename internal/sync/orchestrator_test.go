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
	fetchedFiles []backend.FileChange
}

func newMockBackend(t *testing.T) *mockBackend {
	return &mockBackend{stagingDir: t.TempDir()}
}

func (m *mockBackend) Init(_ context.Context, _ backend.BackendConfig) error { return nil }
func (m *mockBackend) Fetch(_ context.Context) error {
	m.fetched = true
	return nil
}
func (m *mockBackend) FetchedFiles(_ context.Context) ([]backend.FileChange, error) {
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
	case relPath == "CLAUDE.md", strings.HasSuffix(relPath, ".md"):
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

func TestPlanCombinesLocalAndRemoteChanges(t *testing.T) {
	be := newMockBackend(t)
	be.fetchedFiles = []backend.FileChange{
		{Path: "claude/CLAUDE.md", Kind: backend.ChangeModified},
		{Path: "claude/new.md", Kind: backend.ChangeAdded},
		{Path: "claude/gone.md", Kind: backend.ChangeDeleted},
	}

	agentDir := filepath.Join(be.StagingDir(), "claude")
	os.MkdirAll(agentDir, 0o755)
	os.WriteFile(filepath.Join(agentDir, "CLAUDE.md"), []byte("base"), 0o644)
	os.WriteFile(filepath.Join(agentDir, "old.md"), []byte("staged"), 0o644)

	files := []agent.SyncFile{
		{RelPath: "CLAUDE.md", StagingPath: "CLAUDE.md", Type: mockTypeClaudeMD},
	}
	ag := newMockAgent(t, "claude", files)
	os.WriteFile(filepath.Join(ag.SourceDir(), "CLAUDE.md"), []byte("local edits"), 0o644)

	orch := NewOrchestrator(be, map[string]agent.Agent{"claude": ag})
	plan, err := orch.Plan(context.Background(), map[string][]agent.SyncType{"claude": {mockTypeClaudeMD}})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	if be.fetched {
		t.Fatal("Plan should not fetch remote state; callers own freshness")
	}

	if len(plan.Local) != 1 {
		t.Fatalf("local changes = %d, want 1", len(plan.Local))
	}
	if plan.Local[0].Action != ActionDeleteRemote || plan.Local[0].Path != "old.md" {
		t.Fatalf("local change = %+v, want delete-remote old.md", plan.Local[0])
	}

	if len(plan.Remote) != 2 {
		t.Fatalf("remote changes = %d, want 2", len(plan.Remote))
	}

	got := map[string]PlanAction{}
	for _, ch := range plan.Remote {
		got[ch.Path] = ch.Action
	}
	if got["CLAUDE.md"] != ActionConflict {
		t.Errorf("CLAUDE.md action = %q, want %q", got["CLAUDE.md"], ActionConflict)
	}
	if got["new.md"] != ActionDownload {
		t.Errorf("new.md action = %q, want %q", got["new.md"], ActionDownload)
	}
	if _, ok := got["gone.md"]; ok {
		t.Errorf("gone.md should be omitted from preview because pull does not delete local files")
	}
}

func TestPlanTreatsProjectFilesAsDownloads(t *testing.T) {
	be := newMockBackend(t)
	be.fetchedFiles = []backend.FileChange{
		{Path: "claude/projects/myapp/memory/MEMORY.md", Kind: backend.ChangeModified},
	}

	agentDir := filepath.Join(be.StagingDir(), "claude", "projects", "myapp", "memory")
	os.MkdirAll(agentDir, 0o755)
	os.WriteFile(filepath.Join(agentDir, "MEMORY.md"), []byte("remote"), 0o644)

	ag := newMockAgent(t, "claude", nil)
	localProjectDir := filepath.Join(ag.SourceDir(), "projects", "myapp", "memory")
	os.MkdirAll(localProjectDir, 0o755)
	os.WriteFile(filepath.Join(localProjectDir, "MEMORY.md"), []byte("local edits"), 0o644)

	orch := NewOrchestrator(be, map[string]agent.Agent{"claude": ag})
	plan, err := orch.Plan(context.Background(), map[string][]agent.SyncType{"claude": {mockTypeClaudeMD, mockTypeSessions}})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	if len(plan.Remote) != 1 {
		t.Fatalf("remote changes = %d, want 1", len(plan.Remote))
	}
	if plan.Remote[0].Path != "projects/myapp/memory/MEMORY.md" {
		t.Fatalf("remote path = %q, want project memory path", plan.Remote[0].Path)
	}
	if plan.Remote[0].Action != ActionDownload {
		t.Fatalf("remote action = %q, want %q", plan.Remote[0].Action, ActionDownload)
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

func TestPullMergesAppendOnlyFiles(t *testing.T) {
	be := newMockBackend(t)

	// Pre-populate staging with an append-only file (history from another machine).
	agentDir := filepath.Join(be.StagingDir(), "claude")
	os.MkdirAll(agentDir, 0o755)
	stagedContent := strings.Join([]string{
		`{"timestamp":100,"sessionId":"sA","display":"A"}`,
		`{"timestamp":200,"sessionId":"sB","display":"B"}`,
		`{"timestamp":300,"sessionId":"sC","display":"C"}`,
		"",
	}, "\n")
	os.WriteFile(filepath.Join(agentDir, "history.jsonl"), []byte(stagedContent), 0o644)

	// Local history has overlapping lines (B,C) plus a local-only line (D).
	ag := newMockAgent(t, "claude", nil)
	localContent := strings.Join([]string{
		`{"timestamp":200,"sessionId":"sB","display":"B"}`,
		`{"timestamp":300,"sessionId":"sC","display":"C"}`,
		`{"timestamp":400,"sessionId":"sD","display":"D"}`,
		"",
	}, "\n")
	os.WriteFile(filepath.Join(ag.SourceDir(), "history.jsonl"), []byte(localContent), 0o644)

	agents := map[string]agent.Agent{"claude": ag}
	syncTypes := map[string][]agent.SyncType{"claude": {mockTypeSessions}}

	orch := NewOrchestrator(be, agents)
	if err := orch.Pull(context.Background(), syncTypes); err != nil {
		t.Fatalf("Pull: %v", err)
	}

	// Append-only files should NOT be passed to Place (they are merged directly).
	for _, f := range ag.placedFiles {
		if f.StagingPath == "history.jsonl" {
			t.Error("history.jsonl should not be placed via Place(); it should be merged directly")
		}
	}

	// Verify merged result: staged lines + local-only lines.
	merged, err := os.ReadFile(filepath.Join(ag.SourceDir(), "history.jsonl"))
	if err != nil {
		t.Fatalf("reading merged file: %v", err)
	}

	entries, err := readHistoryFile(filepath.Join(ag.SourceDir(), "history.jsonl"))
	if err != nil {
		t.Fatalf("readHistoryFile: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("got %d entries, want 4:\n%s", len(entries), string(merged))
	}
	if entries[0].Timestamp != 100 || entries[1].Timestamp != 200 || entries[2].Timestamp != 300 || entries[3].Timestamp != 400 {
		t.Fatalf("entries not sorted/merged as expected: %+v", entries)
	}
}

func TestPullCanonicalizesHistoryJSONL(t *testing.T) {
	be := newMockBackend(t)

	agentDir := filepath.Join(be.StagingDir(), "claude")
	os.MkdirAll(agentDir, 0o755)
	stagedContent := strings.Join([]string{
		"<<<<<<< HEAD",
		`{"timestamp":200,"sessionId":"s2","display":"b"}`,
		"=======",
		`{"timestamp":100,"sessionId":"s1","display":"a"}`,
		">>>>>>> branch",
		"",
	}, "\n")
	os.WriteFile(filepath.Join(agentDir, "history.jsonl"), []byte(stagedContent), 0o644)

	ag := newMockAgent(t, "claude", nil)
	localContent := `{"timestamp":300,"sessionId":"s3","display":"c"}` + "\n"
	os.WriteFile(filepath.Join(ag.SourceDir(), "history.jsonl"), []byte(localContent), 0o644)

	agents := map[string]agent.Agent{"claude": ag}
	syncTypes := map[string][]agent.SyncType{"claude": {mockTypeSessions}}

	orch := NewOrchestrator(be, agents)
	if err := orch.Pull(context.Background(), syncTypes); err != nil {
		t.Fatalf("Pull: %v", err)
	}

	mergedPath := filepath.Join(ag.SourceDir(), "history.jsonl")
	merged, err := os.ReadFile(mergedPath)
	if err != nil {
		t.Fatalf("reading merged file: %v", err)
	}
	if strings.Contains(string(merged), "<<<<<<<") || strings.Contains(string(merged), "=======") || strings.Contains(string(merged), ">>>>>>>") {
		t.Fatalf("merged history should not contain conflict markers:\n%s", string(merged))
	}

	entries, err := readHistoryFile(mergedPath)
	if err != nil {
		t.Fatalf("readHistoryFile: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(entries))
	}
	if entries[0].Timestamp != 100 || entries[1].Timestamp != 200 || entries[2].Timestamp != 300 {
		t.Fatalf("entries not sorted by timestamp: %+v", entries)
	}
}

func TestPushCanonicalizesLocalAndStagingHistoryJSONL(t *testing.T) {
	be := newMockBackend(t)

	ag := newMockAgent(t, "claude", []agent.SyncFile{
		{RelPath: "history.jsonl", StagingPath: "history.jsonl", Type: mockTypeSessions},
	})
	localContent := strings.Join([]string{
		"<<<<<<< HEAD",
		`{"timestamp":300,"sessionId":"s3","display":"c"}`,
		"=======",
		`{"timestamp":100,"sessionId":"s1","display":"a"}`,
		">>>>>>> branch",
		`{"timestamp":200,"sessionId":"s2","display":"b"}`,
		"",
	}, "\n")
	os.WriteFile(filepath.Join(ag.SourceDir(), "history.jsonl"), []byte(localContent), 0o644)

	agents := map[string]agent.Agent{"claude": ag}
	syncTypes := map[string][]agent.SyncType{"claude": {mockTypeSessions}}

	orch := NewOrchestrator(be, agents)
	if err := orch.Push(context.Background(), syncTypes); err != nil {
		t.Fatalf("Push: %v", err)
	}

	localMerged, err := os.ReadFile(filepath.Join(ag.SourceDir(), "history.jsonl"))
	if err != nil {
		t.Fatalf("reading local merged history: %v", err)
	}
	stagedMerged, err := os.ReadFile(filepath.Join(be.StagingDir(), "claude", "history.jsonl"))
	if err != nil {
		t.Fatalf("reading staged merged history: %v", err)
	}

	if string(localMerged) != string(stagedMerged) {
		t.Fatalf("local and staged history should match after push canonicalization")
	}
	if strings.Contains(string(localMerged), "<<<<<<<") || strings.Contains(string(localMerged), "=======") || strings.Contains(string(localMerged), ">>>>>>>") {
		t.Fatalf("canonicalized history should not contain conflict markers:\n%s", string(localMerged))
	}

	entries, err := readHistoryFile(filepath.Join(ag.SourceDir(), "history.jsonl"))
	if err != nil {
		t.Fatalf("readHistoryFile: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(entries))
	}
	if entries[0].Timestamp != 100 || entries[1].Timestamp != 200 || entries[2].Timestamp != 300 {
		t.Fatalf("entries not sorted: %+v", entries)
	}
}

func TestPullAppendOnlyNeverConflicts(t *testing.T) {
	be := newMockBackend(t)

	// Staging has base content for the append-only file.
	agentDir := filepath.Join(be.StagingDir(), "claude")
	os.MkdirAll(agentDir, 0o755)
	os.WriteFile(filepath.Join(agentDir, "history.jsonl"), []byte(`{"timestamp":100,"sessionId":"sA","display":"A"}`+"\n"), 0o644)

	// Local has completely different content — would be a conflict for replace mode.
	ag := newMockAgent(t, "claude", nil)
	os.WriteFile(filepath.Join(ag.SourceDir(), "history.jsonl"), []byte(`{"timestamp":200,"sessionId":"sZ","display":"Z"}`+"\n"), 0o644)

	agents := map[string]agent.Agent{"claude": ag}
	syncTypes := map[string][]agent.SyncType{"claude": {mockTypeSessions}}

	orch := NewOrchestrator(be, agents)
	if err := orch.Pull(context.Background(), syncTypes); err != nil {
		t.Fatalf("Pull: %v", err)
	}

	// Verify it was merged, not skipped as a conflict.
	merged, err := os.ReadFile(filepath.Join(ag.SourceDir(), "history.jsonl"))
	if err != nil {
		t.Fatalf("reading merged file: %v", err)
	}

	content := string(merged)
	if !strings.Contains(content, `"sessionId":"sA"`) {
		t.Error("merged file should contain remote session sA")
	}
	if !strings.Contains(content, `"sessionId":"sZ"`) {
		t.Error("merged file should contain local session sZ")
	}
}

func TestPlanAppendOnlyNeverConflicts(t *testing.T) {
	be := newMockBackend(t)
	be.fetchedFiles = []backend.FileChange{
		{Path: "claude/history.jsonl", Kind: backend.ChangeModified},
	}

	// Staging has base content.
	agentDir := filepath.Join(be.StagingDir(), "claude")
	os.MkdirAll(agentDir, 0o755)
	os.WriteFile(filepath.Join(agentDir, "history.jsonl"), []byte("line-A\n"), 0o644)

	// Local has different content — would be conflict for replace mode.
	ag := newMockAgent(t, "claude", nil)
	os.WriteFile(filepath.Join(ag.SourceDir(), "history.jsonl"), []byte("line-Z\n"), 0o644)

	orch := NewOrchestrator(be, map[string]agent.Agent{"claude": ag})
	plan, err := orch.Plan(context.Background(), map[string][]agent.SyncType{"claude": {mockTypeSessions}})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	if len(plan.Remote) != 1 {
		t.Fatalf("remote changes = %d, want 1", len(plan.Remote))
	}
	if plan.Remote[0].Action != ActionDownload {
		t.Errorf("history.jsonl action = %q, want %q (append-only files should never conflict)", plan.Remote[0].Action, ActionDownload)
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
