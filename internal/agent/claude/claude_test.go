package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/manmart/negent/internal/agent"
)

func TestNew(t *testing.T) {
	c := New("/tmp/test-claude")
	if c.Name() != "claude" {
		t.Errorf("Name() = %q, want %q", c.Name(), "claude")
	}
	if c.SourceDir() != "/tmp/test-claude" {
		t.Errorf("SourceDir() = %q, want %q", c.SourceDir(), "/tmp/test-claude")
	}
}

func TestNewDefault(t *testing.T) {
	c := New("")
	if c.SourceDir() == "" {
		t.Fatal("SourceDir() should not be empty with default")
	}
}

func TestDefaultCategories(t *testing.T) {
	c := New("")
	cats := c.DefaultCategories()

	expected := map[agent.Category]bool{
		agent.CategoryConfig:     true,
		agent.CategoryCustomCode: true,
		agent.CategoryMemory:     true,
	}

	if len(cats) != len(expected) {
		t.Fatalf("DefaultCategories() returned %d, want %d", len(cats), len(expected))
	}
	for _, cat := range cats {
		if !expected[cat] {
			t.Errorf("unexpected default category: %q", cat)
		}
	}
}

func TestCategoryRules(t *testing.T) {
	rules := CategoryRules()
	if len(rules) == 0 {
		t.Fatal("CategoryRules() returned empty map")
	}
	if _, ok := rules[agent.CategoryConfig]; !ok {
		t.Error("missing config category rules")
	}
	if _, ok := rules[agent.CategoryMemory]; !ok {
		t.Error("missing memory category rules")
	}
}

func TestExcludePatterns(t *testing.T) {
	patterns := ExcludePatterns()
	if len(patterns) == 0 {
		t.Fatal("ExcludePatterns() returned empty")
	}
}

// setupTestDir creates a temporary directory structure mimicking ~/.claude
func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Config files
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), "# My Instructions")
	writeFile(t, filepath.Join(dir, "settings.json"), `{"key": "value"}`)
	writeFile(t, filepath.Join(dir, "settings.local.json"), `{}`)

	// Custom code
	os.MkdirAll(filepath.Join(dir, "skills", "my-skill"), 0o755)
	writeFile(t, filepath.Join(dir, "skills", "my-skill", "skill.md"), "# Skill")

	// Memory (project dirs)
	projDir := filepath.Join(dir, "projects", "-home-user-repos-myproject")
	os.MkdirAll(filepath.Join(projDir, "memory"), 0o755)
	writeFile(t, filepath.Join(projDir, "memory", "MEMORY.md"), "# Memory")
	writeFile(t, filepath.Join(projDir, "memory", "context.md"), "context")

	// Session file
	writeFile(t, filepath.Join(projDir, "abc-123.jsonl"), `{"type":"session"}`)

	// Excluded stuff
	writeFile(t, filepath.Join(dir, ".credentials.json"), "secret")
	os.MkdirAll(filepath.Join(dir, "telemetry"), 0o755)
	writeFile(t, filepath.Join(dir, "telemetry", "data.json"), "telemetry")
	os.MkdirAll(filepath.Join(dir, "cache"), 0o755)
	writeFile(t, filepath.Join(dir, "cache", "cached.json"), "cache")

	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCollectConfig(t *testing.T) {
	dir := setupTestDir(t)
	c := New(dir)

	files, err := c.Collect([]agent.Category{agent.CategoryConfig})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	paths := make(map[string]bool)
	for _, f := range files {
		paths[f.RelPath] = true
		if f.Category != agent.CategoryConfig {
			t.Errorf("file %q has category %q, want config", f.RelPath, f.Category)
		}
	}

	for _, expected := range []string{"CLAUDE.md", "settings.json", "settings.local.json"} {
		if !paths[expected] {
			t.Errorf("missing config file: %s", expected)
		}
	}

	// Should NOT include credentials
	if paths[".credentials.json"] {
		t.Error("collected excluded file .credentials.json")
	}
}

func TestCollectCustomCode(t *testing.T) {
	dir := setupTestDir(t)
	c := New(dir)

	files, err := c.Collect([]agent.Category{agent.CategoryCustomCode})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("expected at least one custom code file")
	}

	for _, f := range files {
		if f.Category != agent.CategoryCustomCode {
			t.Errorf("file %q has category %q, want custom-code", f.RelPath, f.Category)
		}
	}
}

func TestCollectMemory(t *testing.T) {
	dir := setupTestDir(t)
	c := New(dir)

	files, err := c.Collect([]agent.Category{agent.CategoryMemory})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	// Should include memory files + sidecar
	var memoryFiles, sidecarFiles int
	for _, f := range files {
		if filepath.Ext(f.RelPath) == ".json" && filepath.Base(f.RelPath) != "MEMORY.md" {
			sidecarFiles++
		} else {
			memoryFiles++
		}
	}

	if memoryFiles == 0 {
		t.Error("expected at least one memory file")
	}
	if sidecarFiles == 0 {
		t.Error("expected at least one sidecar file")
	}
}

func TestCollectSessions(t *testing.T) {
	dir := setupTestDir(t)
	c := New(dir)

	files, err := c.Collect([]agent.Category{agent.CategorySessions})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	found := false
	for _, f := range files {
		if filepath.Ext(f.RelPath) == ".jsonl" && f.Category == agent.CategorySessions {
			found = true
		}
	}
	if !found {
		t.Error("expected at least one session .jsonl file")
	}
}

func TestCollectExcludes(t *testing.T) {
	dir := setupTestDir(t)
	c := New(dir)

	// Collect everything
	allCats := []agent.Category{
		agent.CategoryConfig,
		agent.CategoryCustomCode,
		agent.CategoryMemory,
		agent.CategorySessions,
		agent.CategoryHistory,
	}
	files, err := c.Collect(allCats)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	for _, f := range files {
		base := filepath.Base(f.RelPath)
		if base == ".credentials.json" || base == "credentials.json" {
			t.Errorf("collected excluded file: %s", f.RelPath)
		}
		// Check no telemetry or cache dirs
		for _, excluded := range []string{"telemetry", "cache"} {
			if filepath.Dir(f.RelPath) == excluded {
				t.Errorf("collected file from excluded dir: %s", f.RelPath)
			}
		}
	}
}

func TestDecodeProjectPath(t *testing.T) {
	tests := []struct {
		encoded string
		want    string
	}{
		{"-home-user-repos-myproject", "/home/user/repos/myproject"},
		// On Linux, Windows-encoded paths are decoded as Unix paths (best-effort).
		// On Windows, decodeProjectPath would return "C:\Users\user\repos\myproject".
		{"-C-Users-user-repos-myproject", "/C/Users/user/repos/myproject"},
		{"-home-user", "/home/user"},
	}
	for _, tt := range tests {
		got := decodeProjectPath(tt.encoded)
		if got != tt.want {
			t.Errorf("decodeProjectPath(%q) = %q, want %q", tt.encoded, got, tt.want)
		}
	}
}

func TestPathSegments(t *testing.T) {
	tests := []struct {
		path string
		want []string
	}{
		{"/home/user/repos/myproject", []string{"home", "user", "repos", "myproject"}},
		{"/C/Users/user", []string{"C", "Users", "user"}},
	}
	for _, tt := range tests {
		got := pathSegments(tt.path)
		if len(got) != len(tt.want) {
			t.Errorf("pathSegments(%q) = %v, want %v", tt.path, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("pathSegments(%q)[%d] = %q, want %q", tt.path, i, got[i], tt.want[i])
			}
		}
	}
}

func TestGenerateSidecars(t *testing.T) {
	dir := setupTestDir(t)
	c := New(dir)

	sidecars, err := c.generateSidecars()
	if err != nil {
		t.Fatalf("generateSidecars: %v", err)
	}

	if len(sidecars) == 0 {
		t.Fatal("expected at least one sidecar")
	}

	// Verify sidecar file was written
	sidecarPath := filepath.Join(dir, "projects", "-home-user-repos-myproject.meta.json")
	data, err := os.ReadFile(sidecarPath)
	if err != nil {
		t.Fatalf("reading sidecar: %v", err)
	}

	var meta SidecarMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("unmarshaling sidecar: %v", err)
	}

	if meta.AbsolutePath != "/home/user/repos/myproject" {
		t.Errorf("sidecar AbsolutePath = %q, want %q", meta.AbsolutePath, "/home/user/repos/myproject")
	}
	if len(meta.Segments) != 4 {
		t.Errorf("sidecar Segments = %v, want 4 segments", meta.Segments)
	}
	if meta.OS == "" {
		t.Error("sidecar OS is empty")
	}
}

// --- Place() tests ---

// setupPlaceTest creates a local Claude dir with project dirs and a staging
// dir with files to place.
func setupPlaceTest(t *testing.T) (localDir, stagingDir string) {
	t.Helper()
	localDir = t.TempDir()
	stagingDir = t.TempDir()

	// Create a local project dir (simulating user already has this project)
	projDir := filepath.Join(localDir, "projects", "-home-user-repos-myproject")
	os.MkdirAll(filepath.Join(projDir, "memory"), 0o755)

	return localDir, stagingDir
}

func TestPlaceNonProjectFiles(t *testing.T) {
	localDir, stagingDir := setupPlaceTest(t)

	// Stage a config file
	writeFile(t, filepath.Join(stagingDir, "CLAUDE.md"), "# Remote Instructions")
	writeFile(t, filepath.Join(stagingDir, "settings.json"), `{"remote": true}`)

	files := []agent.SyncFile{
		{RelPath: "CLAUDE.md", StagingPath: "CLAUDE.md"},
		{RelPath: "settings.json", StagingPath: "settings.json"},
	}

	c := New(localDir)
	result, err := c.Place(stagingDir, files)
	if err != nil {
		t.Fatalf("Place: %v", err)
	}

	if result.Placed != 2 {
		t.Errorf("Placed = %d, want 2", result.Placed)
	}

	// Verify files were placed
	data, err := os.ReadFile(filepath.Join(localDir, "CLAUDE.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# Remote Instructions" {
		t.Errorf("CLAUDE.md content = %q, want %q", string(data), "# Remote Instructions")
	}
}

func TestPlaceTier1ExactMatch(t *testing.T) {
	localDir, stagingDir := setupPlaceTest(t)

	// Stage memory for a project with the same encoded name
	projPath := filepath.Join(stagingDir, "projects", "-home-user-repos-myproject", "memory", "ctx.md")
	writeFile(t, projPath, "remote memory")

	files := []agent.SyncFile{
		{RelPath: filepath.Join("projects", "-home-user-repos-myproject", "memory", "ctx.md")},
	}

	c := New(localDir)
	result, err := c.Place(stagingDir, files)
	if err != nil {
		t.Fatalf("Place: %v", err)
	}

	if result.Placed != 1 {
		t.Errorf("Placed = %d, want 1", result.Placed)
	}

	placed := filepath.Join(localDir, "projects", "-home-user-repos-myproject", "memory", "ctx.md")
	data, err := os.ReadFile(placed)
	if err != nil {
		t.Fatalf("file not placed: %v", err)
	}
	if string(data) != "remote memory" {
		t.Errorf("content = %q, want %q", string(data), "remote memory")
	}
}

func TestPlaceTier3SuffixMatch(t *testing.T) {
	localDir, stagingDir := setupPlaceTest(t)

	// Remote project from Windows with different path prefix
	remoteDir := "-C-Users-user-repos-myproject"

	// Stage memory + sidecar
	writeFile(t, filepath.Join(stagingDir, "projects", remoteDir, "memory", "notes.md"), "remote notes")
	sidecar := `{"absolute_path":"/C/Users/user/repos/myproject","segments":["C","Users","user","repos","myproject"],"os":"windows"}`
	writeFile(t, filepath.Join(stagingDir, "projects", remoteDir+".meta.json"), sidecar)

	files := []agent.SyncFile{
		{RelPath: filepath.Join("projects", remoteDir, "memory", "notes.md")},
		{RelPath: filepath.Join("projects", remoteDir+".meta.json")},
	}

	c := New(localDir)
	result, err := c.Place(stagingDir, files)
	if err != nil {
		t.Fatalf("Place: %v", err)
	}

	if result.Placed != 1 {
		t.Errorf("Placed = %d, want 1", result.Placed)
	}
	if len(result.Unmatched) != 0 {
		t.Errorf("Unmatched = %v, want empty", result.Unmatched)
	}

	// Verify it was placed in the LOCAL project dir, not the remote one
	placed := filepath.Join(localDir, "projects", "-home-user-repos-myproject", "memory", "notes.md")
	if _, err := os.Stat(placed); os.IsNotExist(err) {
		t.Error("file not placed in local project dir via suffix match")
	}
}

func TestPlaceTier4Unmatched(t *testing.T) {
	localDir, stagingDir := setupPlaceTest(t)

	// Remote project with no local equivalent
	remoteDir := "-home-otheruser-repos-unknown"
	writeFile(t, filepath.Join(stagingDir, "projects", remoteDir, "memory", "data.md"), "remote data")

	files := []agent.SyncFile{
		{RelPath: filepath.Join("projects", remoteDir, "memory", "data.md")},
	}

	c := New(localDir)
	result, err := c.Place(stagingDir, files)
	if err != nil {
		t.Fatalf("Place: %v", err)
	}

	if result.Placed != 0 {
		t.Errorf("Placed = %d, want 0", result.Placed)
	}
	if result.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", result.Skipped)
	}
	if len(result.Unmatched) != 1 {
		t.Errorf("Unmatched = %d, want 1", len(result.Unmatched))
	}
}

func TestSuffixMatchScore(t *testing.T) {
	tests := []struct {
		a, b []string
		want int
	}{
		{
			[]string{"home", "user", "repos", "myproject"},
			[]string{"C", "Users", "user", "repos", "myproject"},
			3, // user/repos/myproject
		},
		{
			[]string{"home", "user", "repos", "myproject"},
			[]string{"home", "user", "repos", "myproject"},
			4, // full match
		},
		{
			[]string{"a", "b", "c"},
			[]string{"x", "y", "z"},
			0, // no match
		},
		{
			[]string{"repos", "myproject"},
			[]string{"other", "repos", "myproject"},
			2,
		},
	}
	for _, tt := range tests {
		got := suffixMatchScore(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("suffixMatchScore(%v, %v) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
