package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/manmart/negent/internal/agent"
	"github.com/manmart/negent/internal/backend"
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

func TestDefaultSyncTypes(t *testing.T) {
	c := New("")
	syncTypes := c.DefaultSyncTypes()

	expected := map[agent.SyncType]bool{
		SyncTypeClaudeMD:    true,
		SyncTypeRules:       true,
		SyncTypeCommands:    true,
		SyncTypeSkills:      true,
		SyncTypeAgents:      true,
		SyncTypePlugins:     true,
		SyncTypeOutputStyle: true,
		SyncTypeAgentMemory: true,
		SyncTypeAutoMemory:  true,
	}

	if len(syncTypes) != len(expected) {
		t.Fatalf("DefaultSyncTypes() returned %d, want %d", len(syncTypes), len(expected))
	}
	for _, syncType := range syncTypes {
		if !expected[syncType] {
			t.Errorf("unexpected default sync type: %q", syncType)
		}
	}
}

func TestSyncTypeRules(t *testing.T) {
	rules := CollectionRules()
	if len(rules) == 0 {
		t.Fatal("CollectionRules() returned empty map")
	}
	if _, ok := rules[SyncTypeClaudeMD]; !ok {
		t.Error("missing claude-md rules")
	}
	if _, ok := rules[SyncTypeAutoMemory]; !ok {
		t.Error("missing auto-memory rules")
	}
}

func TestNormalizeSyncTypesLegacyAliases(t *testing.T) {
	c := New("")

	got, err := c.NormalizeSyncTypes([]string{"config", "custom-code", "memory", "sessions", "history"})
	if err != nil {
		t.Fatalf("NormalizeSyncTypes() returned error: %v", err)
	}

	want := []agent.SyncType{
		SyncTypeClaudeMD,
		SyncTypeCommands,
		SyncTypeSkills,
		SyncTypeAgents,
		SyncTypeRules,
		SyncTypeOutputStyle,
		SyncTypeAgentMemory,
		SyncTypeAutoMemory,
		SyncTypeSessions,
		SyncTypeHistory,
	}
	if len(got) != len(want) {
		t.Fatalf("NormalizeSyncTypes() returned %d items, want %d", len(got), len(want))
	}
	for i, syncType := range want {
		if got[i] != syncType {
			t.Fatalf("NormalizeSyncTypes()[%d] = %q, want %q", i, got[i], syncType)
		}
	}
}

func TestNormalizeSyncTypesEmptyMeansNothing(t *testing.T) {
	c := New("")

	got, err := c.NormalizeSyncTypes([]string{})
	if err != nil {
		t.Fatalf("NormalizeSyncTypes() returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("NormalizeSyncTypes(empty) returned %d types, want 0", len(got))
	}
}

func TestNormalizeSyncTypesPluginsLegacy(t *testing.T) {
	c := New("")

	got, err := c.NormalizeSyncTypes([]string{"plugins"})
	if err != nil {
		t.Fatalf("NormalizeSyncTypes(plugins) returned error: %v", err)
	}
	if len(got) != 1 || got[0] != SyncTypePlugins {
		t.Fatalf("NormalizeSyncTypes(plugins) = %v, want [%q]", got, SyncTypePlugins)
	}
}

func TestSyncTypeForPathIncludesPlugins(t *testing.T) {
	if got := syncTypeForPath("plugins/my-plugin/plugin.json"); got != SyncTypePlugins {
		t.Fatalf("syncTypeForPath(plugins/...) = %q, want %q", got, SyncTypePlugins)
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
	os.MkdirAll(filepath.Join(dir, "plugins", "my-plugin"), 0o755)
	writeFile(t, filepath.Join(dir, "plugins", "my-plugin", "plugin.json"), `{"name":"my-plugin"}`)
	os.MkdirAll(filepath.Join(dir, "plugins", "marketplaces", "claude-plugins-official"), 0o755)
	writeFile(t, filepath.Join(dir, "plugins", "marketplaces", "claude-plugins-official", "README.md"), "# Catalog")
	writeFile(t, filepath.Join(dir, "plugins", "install-counts-cache.json"), `{"version":1}`)

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

func TestCollectClaudeMD(t *testing.T) {
	dir := setupTestDir(t)
	c := New(dir)

	files, err := c.Collect([]agent.SyncType{SyncTypeClaudeMD})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	paths := make(map[string]bool)
	for _, f := range files {
		paths[f.RelPath] = true
		if f.Type != SyncTypeClaudeMD {
			t.Errorf("file %q has type %q, want %q", f.RelPath, f.Type, SyncTypeClaudeMD)
		}
	}

	if !paths["CLAUDE.md"] {
		t.Error("missing config file: CLAUDE.md")
	}
	if paths["settings.json"] {
		t.Error("settings.json should not be collected (machine-specific)")
	}
	if paths["settings.local.json"] {
		t.Error("settings.local.json should not be collected")
	}

	// Should NOT include credentials
	if paths[".credentials.json"] {
		t.Error("collected excluded file .credentials.json")
	}
}

func TestCollectCustomTypes(t *testing.T) {
	dir := setupTestDir(t)
	c := New(dir)

	files, err := c.Collect([]agent.SyncType{SyncTypeSkills})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("expected at least one skill file")
	}

	for _, f := range files {
		if f.Type != SyncTypeSkills {
			t.Errorf("file %q has type %q, want %q", f.RelPath, f.Type, SyncTypeSkills)
		}
	}
}

func TestCollectAutoMemory(t *testing.T) {
	dir := setupTestDir(t)
	c := New(dir)

	files, err := c.Collect([]agent.SyncType{SyncTypeAutoMemory})
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

	files, err := c.Collect([]agent.SyncType{SyncTypeSessions})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	found := false
	for _, f := range files {
		if filepath.Ext(f.RelPath) == ".jsonl" && f.Type == SyncTypeSessions {
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
	allSyncTypes := []agent.SyncType{
		SyncTypeClaudeMD,
		SyncTypeRules,
		SyncTypeCommands,
		SyncTypeSkills,
		SyncTypeAgents,
		SyncTypePlugins,
		SyncTypeOutputStyle,
		SyncTypeAgentMemory,
		SyncTypeAutoMemory,
		SyncTypeSessions,
		SyncTypeHistory,
	}
	files, err := c.Collect(allSyncTypes)
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

func TestCollectPluginsExcludesMarketplaceCatalog(t *testing.T) {
	dir := setupTestDir(t)
	c := New(dir)

	files, err := c.Collect([]agent.SyncType{SyncTypePlugins})
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}

	var gotPlugin bool
	for _, f := range files {
		if f.RelPath == filepath.Join("plugins", "my-plugin", "plugin.json") {
			gotPlugin = true
		}
		rel := filepath.ToSlash(f.RelPath)
		if rel == "plugins/marketplaces/claude-plugins-official/README.md" {
			t.Fatalf("collected marketplace catalog file: %s", f.RelPath)
		}
		if rel == "plugins/install-counts-cache.json" {
			t.Fatalf("collected install-counts cache file: %s", f.RelPath)
		}
	}

	if !gotPlugin {
		t.Fatal("expected user plugin file to be collected")
	}
}

func TestDecodeProjectPath(t *testing.T) {
	tests := []struct {
		encoded string
		want    string
	}{
		{"-home-user-repos-myproject", "/home/user/repos/myproject"},
		{"-home-user", "/home/user"},
		{"-C-Users-user-repos-myproject", "/C/Users/user/repos/myproject"},
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

	// Remote project from another machine (e.g., macOS) with different path prefix
	remoteDir := "-Users-otheruser-repos-myproject"

	// Stage memory + sidecar
	writeFile(t, filepath.Join(stagingDir, "projects", remoteDir, "memory", "notes.md"), "remote notes")
	sidecar := `{"absolute_path":"/Users/otheruser/repos/myproject","segments":["Users","otheruser","repos","myproject"],"os":"darwin"}`
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

// --- Cross-machine mapping tests ---

// setupCrossMachineTest creates a local dir with a Linux project and a
// staging dir with a macOS project + sidecar that share the same suffix.
func setupCrossMachineTest(t *testing.T) (localDir, stagingDir string) {
	t.Helper()
	localDir = t.TempDir()
	stagingDir = t.TempDir()

	// Local project (Linux encoding)
	localProj := filepath.Join(localDir, "projects", "-home-user-repos-myproject")
	os.MkdirAll(filepath.Join(localProj, "memory"), 0o755)
	writeFile(t, filepath.Join(localProj, "memory", "MEMORY.md"), "local memory")

	// Staging project (simulates macOS encoding with sidecar)
	stagingProj := filepath.Join(stagingDir, "projects", "-Users-otheruser-repos-myproject")
	os.MkdirAll(filepath.Join(stagingProj, "memory"), 0o755)
	writeFile(t, filepath.Join(stagingProj, "memory", "MEMORY.md"), "remote memory")
	writeFile(t, filepath.Join(stagingProj, "memory", "remote-only.md"), "remote only file")

	sidecar := SidecarMeta{
		AbsolutePath: "/Users/otheruser/repos/myproject",
		Segments:     []string{"Users", "otheruser", "repos", "myproject"},
		OS:           "darwin",
	}
	data, _ := json.Marshal(sidecar)
	writeFile(t, filepath.Join(stagingDir, "projects", "-Users-otheruser-repos-myproject.meta.json"), string(data))

	return localDir, stagingDir
}

func TestListStagingProjects(t *testing.T) {
	stagingDir := t.TempDir()

	sidecar := SidecarMeta{
		AbsolutePath: "/home/user/repos/myproject",
		Segments:     []string{"home", "user", "repos", "myproject"},
		GitRemote:    "https://github.com/user/myproject.git",
		OS:           "linux",
	}
	data, _ := json.Marshal(sidecar)
	writeFile(t, filepath.Join(stagingDir, "projects", "-home-user-repos-myproject.meta.json"), string(data))

	projects, err := listStagingProjects(stagingDir)
	if err != nil {
		t.Fatalf("listStagingProjects: %v", err)
	}
	if len(projects) != 1 {
		t.Fatalf("got %d projects, want 1", len(projects))
	}
	meta, ok := projects["-home-user-repos-myproject"]
	if !ok {
		t.Fatal("missing -home-user-repos-myproject")
	}
	if meta.GitRemote != "https://github.com/user/myproject.git" {
		t.Errorf("GitRemote = %q, want %q", meta.GitRemote, "https://github.com/user/myproject.git")
	}
}

func TestListStagingProjectsEmpty(t *testing.T) {
	stagingDir := t.TempDir()
	projects, err := listStagingProjects(stagingDir)
	if err != nil {
		t.Fatalf("listStagingProjects: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("got %d projects, want 0", len(projects))
	}
}

func TestBuildProjectMapping(t *testing.T) {
	localDir, stagingDir := setupCrossMachineTest(t)
	c := New(localDir)

	mapping, _, err := c.buildProjectMapping(stagingDir)
	if err != nil {
		t.Fatalf("buildProjectMapping: %v", err)
	}

	// "-home-user-repos-myproject" (local) should map to "-Users-otheruser-repos-myproject" (staging)
	// via suffix match (repos/myproject = 2 matching segments)
	want := "-Users-otheruser-repos-myproject"
	got, ok := mapping["-home-user-repos-myproject"]
	if !ok {
		t.Fatalf("no mapping for -home-user-repos-myproject; mapping = %v", mapping)
	}
	if got != want {
		t.Errorf("mapping = %q, want %q", got, want)
	}
}

func TestRemapStagingPath(t *testing.T) {
	mapping := map[string]string{
		"-home-user-repos-myproject": "-Users-otheruser-repos-myproject",
	}

	tests := []struct {
		input string
		want  string
	}{
		// Project files get remapped
		{"projects/-home-user-repos-myproject/memory/MEMORY.md", "projects/-Users-otheruser-repos-myproject/memory/MEMORY.md"},
		// Sidecar files are NOT remapped
		{"projects/-home-user-repos-myproject.meta.json", "projects/-home-user-repos-myproject.meta.json"},
		// Non-project files are NOT remapped
		{"settings.json", "settings.json"},
		{"CLAUDE.md", "CLAUDE.md"},
		// Unmapped projects keep their encoding
		{"projects/-other-project/memory/foo.md", "projects/-other-project/memory/foo.md"},
	}

	for _, tt := range tests {
		got := remapStagingPath(tt.input, mapping)
		if got != tt.want {
			t.Errorf("remapStagingPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMapStagingPaths(t *testing.T) {
	localDir, stagingDir := setupCrossMachineTest(t)
	c := New(localDir)

	files := []agent.SyncFile{
		{RelPath: "CLAUDE.md", StagingPath: "CLAUDE.md"},
		{RelPath: "projects/-home-user-repos-myproject/memory/MEMORY.md", StagingPath: "projects/-home-user-repos-myproject/memory/MEMORY.md"},
		{RelPath: "projects/-home-user-repos-myproject.meta.json", StagingPath: "projects/-home-user-repos-myproject.meta.json"},
	}

	result, err := c.MapStagingPaths(stagingDir, files)
	if err != nil {
		t.Fatalf("MapStagingPaths: %v", err)
	}

	if result[0].StagingPath != "CLAUDE.md" {
		t.Errorf("non-project file remapped: %q", result[0].StagingPath)
	}
	if result[1].StagingPath != "projects/-Users-otheruser-repos-myproject/memory/MEMORY.md" {
		t.Errorf("project file not remapped: %q", result[1].StagingPath)
	}
	if result[2].StagingPath != "projects/-home-user-repos-myproject.meta.json" {
		t.Errorf("sidecar file was remapped: %q", result[2].StagingPath)
	}
}

func TestDiffCrossMachine(t *testing.T) {
	localDir, stagingDir := setupCrossMachineTest(t)
	c := New(localDir)

	// Also add a config file that exists in both
	writeFile(t, filepath.Join(localDir, "CLAUDE.md"), "local instructions")
	writeFile(t, filepath.Join(stagingDir, "CLAUDE.md"), "local instructions")

	changes, err := c.Diff(stagingDir, []agent.SyncType{SyncTypeClaudeMD, SyncTypeAutoMemory})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	for _, ch := range changes {
		// Should NOT see the macOS-encoded project dir as "Deleted"
		if ch.Kind == backend.ChangeDeleted && ch.Path == "projects/-Users-otheruser-repos-myproject/memory/remote-only.md" {
			t.Error("cross-machine file incorrectly reported as deleted")
		}
		// Should NOT see the Linux-encoded project dir as "Added"
		if ch.Kind == backend.ChangeAdded && ch.Path == "projects/-home-user-repos-myproject/memory/MEMORY.md" {
			t.Error("local file incorrectly reported as new (should map to existing staging path)")
		}
	}

	// The memory file exists on both sides — check if it's reported as modified
	// (content differs: "local memory" vs "remote memory")
	foundModified := false
	for _, ch := range changes {
		if ch.Kind == backend.ChangeModified && ch.Path == "projects/-Users-otheruser-repos-myproject/memory/MEMORY.md" {
			foundModified = true
		}
	}
	if !foundModified {
		t.Error("expected MEMORY.md to be reported as modified (local vs remote content differs)")
	}
}

// TestDiffSidecarOnlyNoPhantomChanges verifies that when staging has content
// under the local encoding plus sidecars from another machine (but no dirs),
// Diff does not report phantom New/Deleted entries.
func TestDiffSidecarOnlyNoPhantomChanges(t *testing.T) {
	localDir := t.TempDir()
	stagingDir := t.TempDir()

	// Local project (Linux encoding — same machine that pushed)
	localProj := filepath.Join(localDir, "projects", "-home-user-repos-myproject")
	os.MkdirAll(filepath.Join(localProj, "memory"), 0o755)
	writeFile(t, filepath.Join(localProj, "memory", "MEMORY.md"), "my memory")

	// Staging has content under the SAME Linux encoding (pushed from this machine)
	stagingProj := filepath.Join(stagingDir, "projects", "-home-user-repos-myproject")
	os.MkdirAll(filepath.Join(stagingProj, "memory"), 0o755)
	writeFile(t, filepath.Join(stagingProj, "memory", "MEMORY.md"), "my memory")

	// Linux sidecar
	linuxSidecar := SidecarMeta{
		AbsolutePath: "/home/user/repos/myproject",
		Segments:     []string{"home", "user", "repos", "myproject"},
		OS:           "linux",
	}
	data, _ := json.MarshalIndent(linuxSidecar, "", "  ")
	writeFile(t, filepath.Join(stagingDir, "projects", "-home-user-repos-myproject.meta.json"), string(data))

	// macOS sidecar exists but NO corresponding macOS directory in staging
	macSidecar := SidecarMeta{
		AbsolutePath: "/Users/otheruser/repos/myproject",
		Segments:     []string{"Users", "otheruser", "repos", "myproject"},
		OS:           "darwin",
	}
	data, _ = json.MarshalIndent(macSidecar, "", "  ")
	writeFile(t, filepath.Join(stagingDir, "projects", "-Users-otheruser-repos-myproject.meta.json"), string(data))

	c := New(localDir)
	changes, err := c.Diff(stagingDir, []agent.SyncType{SyncTypeAutoMemory})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	if len(changes) != 0 {
		t.Errorf("expected no changes, got %d:", len(changes))
		for _, ch := range changes {
			t.Errorf("  %s: %s", ch.Kind, ch.Path)
		}
	}
}

func TestDiffAppendOnlySupersetIsClean(t *testing.T) {
	localDir := t.TempDir()
	stagingDir := t.TempDir()
	c := New(localDir)

	writeFile(t, filepath.Join(localDir, "history.jsonl"), "line-A\nline-B\n")
	writeFile(t, filepath.Join(stagingDir, "history.jsonl"), "line-B\nline-A\nline-C\n")

	changes, err := c.Diff(stagingDir, []agent.SyncType{SyncTypeHistory})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if len(changes) != 0 {
		t.Fatalf("expected no changes, got %v", changes)
	}
}

func TestDiffAppendOnlyLocalOnlyLinesArePending(t *testing.T) {
	localDir := t.TempDir()
	stagingDir := t.TempDir()
	c := New(localDir)

	writeFile(t, filepath.Join(localDir, "history.jsonl"), "line-A\nline-B\nline-LOCAL\n")
	writeFile(t, filepath.Join(stagingDir, "history.jsonl"), "line-A\nline-B\n")

	changes, err := c.Diff(stagingDir, []agent.SyncType{SyncTypeHistory})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %v", len(changes), changes)
	}
	if changes[0].Path != "history.jsonl" || changes[0].Kind != backend.ChangeModified {
		t.Fatalf("unexpected change: %+v", changes[0])
	}
}

func TestDiffAppendOnlyStagedOnlyIsNotDeletion(t *testing.T) {
	localDir := t.TempDir()
	stagingDir := t.TempDir()
	c := New(localDir)

	writeFile(t, filepath.Join(stagingDir, "history.jsonl"), "line-A\n")

	changes, err := c.Diff(stagingDir, []agent.SyncType{SyncTypeHistory})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if len(changes) != 0 {
		t.Fatalf("expected no changes, got %v", changes)
	}
}
