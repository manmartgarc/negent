package copilot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/manmart/negent/internal/agent"
	"github.com/manmart/negent/internal/backend"
)

func mustNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	mustNoErr(t, os.MkdirAll(filepath.Dir(path), 0o755))
	mustNoErr(t, os.WriteFile(path, []byte(content), 0o644))
}

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "config.json"), `{"theme":"dark"}`)
	writeFile(t, filepath.Join(dir, "mcp-config.json"), `{"servers":[]}`)
	writeFile(t, filepath.Join(dir, "agents", "reviewer.md"), "# reviewer")
	writeFile(t, filepath.Join(dir, "skills", "git", "SKILL.md"), "# skill")
	writeFile(t, filepath.Join(dir, "skills", "git", "bin", "run.sh"), "#!/bin/sh")
	writeFile(t, filepath.Join(dir, "skills", "git", "logs", "example.txt"), "keep me")
	writeFile(t, filepath.Join(dir, "hooks", "pre-run.sh"), "#!/bin/sh")
	writeFile(t, filepath.Join(dir, "hooks", "ide", "bootstrap.sh"), "#!/bin/sh")
	writeFile(t, filepath.Join(dir, "session-state", "resume.json"), `{"id":"123"}`)
	writeFile(t, filepath.Join(dir, "session-state", "s1", "events.jsonl"), `{"event":"start"}`)
	writeFile(t, filepath.Join(dir, "session-state", "s1", "plan.md"), "# plan")

	writeFile(t, filepath.Join(dir, "permissions-config.json"), `{"allow":[]}`)
	writeFile(t, filepath.Join(dir, "session-store.db"), "sqlite")
	writeFile(t, filepath.Join(dir, "logs", "copilot.log"), "log")
	writeFile(t, filepath.Join(dir, "installed-plugins", "plugin.json"), `{"name":"x"}`)
	writeFile(t, filepath.Join(dir, "ide", "state.json"), `{"window":1}`)

	return dir
}

func TestNew(t *testing.T) {
	c := New("/tmp/test-copilot")
	if c.Name() != Name {
		t.Fatalf("Name() = %q, want %q", c.Name(), Name)
	}
	if c.SourceDir() != "/tmp/test-copilot" {
		t.Fatalf("SourceDir() = %q, want %q", c.SourceDir(), "/tmp/test-copilot")
	}
}

func TestDefaultSourceDirUsesCopilotHome(t *testing.T) {
	t.Setenv("COPILOT_HOME", "/tmp/copilot-home")
	if got := DefaultSourceDir(); got != "/tmp/copilot-home" {
		t.Fatalf("DefaultSourceDir() = %q, want %q", got, "/tmp/copilot-home")
	}
}

func TestDefaultSyncTypes(t *testing.T) {
	c := New("")
	got := c.DefaultSyncTypes()
	want := []agent.SyncType{
		SyncTypeConfig,
		SyncTypeMCP,
		SyncTypeAgents,
		SyncTypeSkills,
		SyncTypeHooks,
	}
	if len(got) != len(want) {
		t.Fatalf("DefaultSyncTypes() returned %d items, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DefaultSyncTypes()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNormalizeSyncTypes(t *testing.T) {
	c := New("")

	got, err := c.NormalizeSyncTypes([]string{"config", "skills", "config"})
	if err != nil {
		t.Fatalf("NormalizeSyncTypes() error = %v", err)
	}

	want := []agent.SyncType{SyncTypeConfig, SyncTypeSkills}
	if len(got) != len(want) {
		t.Fatalf("NormalizeSyncTypes() returned %d items, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("NormalizeSyncTypes()[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	got, err = c.NormalizeSyncTypes([]string{"config", "sessions", "config"})
	if err != nil {
		t.Fatalf("NormalizeSyncTypes() with sessions error = %v", err)
	}
	want = []agent.SyncType{SyncTypeConfig, SyncTypeSessions}
	if len(got) != len(want) {
		t.Fatalf("NormalizeSyncTypes() with sessions returned %d items, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("NormalizeSyncTypes() with sessions[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	if _, err := c.NormalizeSyncTypes([]string{"unknown"}); err == nil {
		t.Fatal("NormalizeSyncTypes() should reject unsupported sync types")
	}
}

func TestSyncTypeForPath(t *testing.T) {
	tests := map[string]agent.SyncType{
		"config.json":            SyncTypeConfig,
		"mcp-config.json":        SyncTypeMCP,
		"agents/reviewer.md":     SyncTypeAgents,
		"skills/git/SKILL.md":    SyncTypeSkills,
		"hooks/pre-run.sh":       SyncTypeHooks,
		"session-state/run.json": SyncTypeSessions,
	}

	for relPath, want := range tests {
		if got := syncTypeForPath(relPath); got != want {
			t.Fatalf("syncTypeForPath(%q) = %q, want %q", relPath, got, want)
		}
	}
}

func TestCollectIncludesOnlySupportedPaths(t *testing.T) {
	dir := setupTestDir(t)
	c := New(dir)

	files, err := c.Collect(c.DefaultSyncTypes())
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	got := make(map[string]agent.SyncType, len(files))
	for _, f := range files {
		got[f.RelPath] = f.Type
	}

	want := map[string]agent.SyncType{
		"config.json":                 SyncTypeConfig,
		"mcp-config.json":             SyncTypeMCP,
		"agents/reviewer.md":          SyncTypeAgents,
		"skills/git/SKILL.md":         SyncTypeSkills,
		"skills/git/bin/run.sh":       SyncTypeSkills,
		"skills/git/logs/example.txt": SyncTypeSkills,
		"hooks/pre-run.sh":            SyncTypeHooks,
		"hooks/ide/bootstrap.sh":      SyncTypeHooks,
	}

	if len(got) != len(want) {
		t.Fatalf("Collect() returned %d files, want %d", len(got), len(want))
	}
	for relPath, wantType := range want {
		if got[relPath] != wantType {
			t.Fatalf("Collect() missing or wrong type for %q: got %q want %q", relPath, got[relPath], wantType)
		}
	}
}

func TestCollectExcludesManagedState(t *testing.T) {
	dir := setupTestDir(t)
	c := New(dir)

	files, err := c.Collect(c.DefaultSyncTypes())
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	for _, f := range files {
		switch f.RelPath {
		case "permissions-config.json", "session-store.db", "logs/copilot.log", "installed-plugins/plugin.json", "ide/state.json":
			t.Fatalf("Collect() unexpectedly included excluded path %q", f.RelPath)
		}
	}
}

func TestCollectIncludesSessionsWhenEnabled(t *testing.T) {
	dir := setupTestDir(t)
	c := New(dir)

	files, err := c.Collect([]agent.SyncType{SyncTypeSessions})
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	got := make(map[string]agent.SyncType, len(files))
	for _, f := range files {
		got[f.RelPath] = f.Type
	}

	want := map[string]agent.SyncType{
		"session-state/resume.json":     SyncTypeSessions,
		"session-state/s1/events.jsonl": SyncTypeSessions,
		"session-state/s1/plan.md":      SyncTypeSessions,
	}

	if len(got) != len(want) {
		t.Fatalf("Collect() with sessions returned %d files, want %d", len(got), len(want))
	}
	for relPath, wantType := range want {
		if got[relPath] != wantType {
			t.Fatalf("Collect() with sessions missing or wrong type for %q: got %q want %q", relPath, got[relPath], wantType)
		}
	}
}

func TestCollectDoesNotExcludeNestedPathsInsideSupportedDirs(t *testing.T) {
	dir := setupTestDir(t)
	c := New(dir)

	files, err := c.Collect(c.DefaultSyncTypes())
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	got := make(map[string]bool, len(files))
	for _, f := range files {
		got[f.RelPath] = true
	}

	for _, relPath := range []string{
		"skills/git/logs/example.txt",
		"hooks/ide/bootstrap.sh",
	} {
		if !got[relPath] {
			t.Fatalf("Collect() should include nested supported path %q", relPath)
		}
	}
}

func TestPlaceCopiesFiles(t *testing.T) {
	sourceDir := t.TempDir()
	stagingDir := t.TempDir()
	writeFile(t, filepath.Join(stagingDir, "skills", "git", "SKILL.md"), "# synced skill")
	writeFile(t, filepath.Join(stagingDir, "config.json"), `{"ok":true}`)

	c := New(sourceDir)
	result, err := c.Place(stagingDir, []agent.SyncFile{
		{RelPath: "skills/git/SKILL.md", StagingPath: "skills/git/SKILL.md", Type: SyncTypeSkills},
		{RelPath: "config.json", StagingPath: "config.json", Type: SyncTypeConfig},
	})
	if err != nil {
		t.Fatalf("Place() error = %v", err)
	}
	if result.Placed != 2 {
		t.Fatalf("Place() placed %d files, want 2", result.Placed)
	}

	data, err := os.ReadFile(filepath.Join(sourceDir, "skills", "git", "SKILL.md"))
	if err != nil {
		t.Fatalf("reading placed file: %v", err)
	}
	if string(data) != "# synced skill" {
		t.Fatalf("placed content = %q, want %q", string(data), "# synced skill")
	}
}

func TestDiffReportsReplaceModeChanges(t *testing.T) {
	sourceDir := t.TempDir()
	stagingDir := t.TempDir()

	writeFile(t, filepath.Join(sourceDir, "config.json"), `{"local":true}`)
	writeFile(t, filepath.Join(sourceDir, "skills", "git", "SKILL.md"), "# local")

	writeFile(t, filepath.Join(stagingDir, "config.json"), `{"remote":true}`)
	writeFile(t, filepath.Join(stagingDir, "agents", "obsolete.md"), "# old")
	writeFile(t, filepath.Join(stagingDir, "session-state", "resume.json"), `{"ignore":true}`)

	c := New(sourceDir)
	changes, err := c.Diff(stagingDir, c.DefaultSyncTypes())
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}

	got := make(map[string]backend.ChangeKind, len(changes))
	for _, ch := range changes {
		got[ch.Path] = ch.Kind
	}

	want := map[string]backend.ChangeKind{
		"config.json":         backend.ChangeModified,
		"skills/git/SKILL.md": backend.ChangeAdded,
		"agents/obsolete.md":  backend.ChangeDeleted,
	}

	if len(got) != len(want) {
		t.Fatalf("Diff() returned %d changes, want %d", len(got), len(want))
	}
	for path, kind := range want {
		if got[path] != kind {
			t.Fatalf("Diff() for %q = %q, want %q", path, got[path], kind)
		}
	}
}

func TestDiffSkipsDeletingRemoteOnlySessionStateFiles(t *testing.T) {
	sourceDir := t.TempDir()
	stagingDir := t.TempDir()
	writeFile(t, filepath.Join(stagingDir, "session-state", "remote", "events.jsonl"), `{"event":"remote"}`)

	c := New(sourceDir)
	changes, err := c.Diff(stagingDir, []agent.SyncType{SyncTypeSessions})
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	if len(changes) != 0 {
		t.Fatalf("Diff() returned %d changes, want 0", len(changes))
	}
}

func TestCollectSkipsSymlinkedDirectories(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()

	writeFile(t, filepath.Join(target, "SKILL.md"), "# symlinked skill")
	mustNoErr(t, os.Symlink(target, filepath.Join(dir, "skills")))
	writeFile(t, filepath.Join(dir, "config.json"), `{}`)

	c := New(dir)
	files, err := c.Collect([]agent.SyncType{SyncTypeSkills, SyncTypeConfig})
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	for _, f := range files {
		if f.Type == SyncTypeSkills {
			t.Fatalf("Collect() should skip symlinked skills dir, got %q", f.RelPath)
		}
	}
}

func TestCollectSkipsSymlinkedFiles(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()

	writeFile(t, filepath.Join(target, "real.json"), `{"real":true}`)
	mustNoErr(t, os.Symlink(
		filepath.Join(target, "real.json"),
		filepath.Join(dir, "config.json"),
	))

	c := New(dir)
	files, err := c.Collect([]agent.SyncType{SyncTypeConfig})
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	for _, f := range files {
		if f.RelPath == "config.json" {
			t.Fatalf("Collect() should skip symlinked file config.json")
		}
	}
}
