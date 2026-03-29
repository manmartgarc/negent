package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/manmart/negent/internal/agent"
	"github.com/manmart/negent/internal/backend"
)

const Name = "claude"

// DefaultSourceDir returns the default Claude Code config directory.
func DefaultSourceDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}

// categoryRules maps each category to collection rules. Rules with Dir set
// collect all files recursively under that directory. Rules with Glob set
// use filepath.Glob for exact matching.
type collectionRule struct {
	Dir  string // collect all files recursively under this dir (relative to source)
	Glob string // filepath.Glob pattern (relative to source)
}

var categoryRules = map[agent.Category][]collectionRule{
	agent.CategoryConfig: {
		{Glob: "CLAUDE.md"},
	},
	agent.CategoryCustomCode: {
		{Dir: "commands"},
		{Dir: "skills"},
		{Dir: "agents"},
		{Dir: "rules"},
		{Dir: "output-styles"},
	},
	agent.CategoryMemory: {
		// Handled specially: we walk projects/*/memory/
		{Glob: "projects/*/memory/*"},
	},
	agent.CategorySessions: {
		{Glob: "projects/*/*.jsonl"},
	},
	agent.CategoryHistory: {
		{Glob: "history.jsonl"},
	},
}

// excludePatterns are always excluded from sync.
var excludePatterns = []string{
	".credentials.json",
	"credentials.json",
	"auth.json",
	"stats-cache.json",
	"*.tmp",
	".lock",
}

// excludeDirs are directories always excluded from sync.
var excludeDirs = map[string]bool{
	"telemetry":     true,
	"cache":         true,
	"backups":       true,
	"debug":         true,
	"downloads":     true,
	"file-history":  true,
	"ide":           true,
	"paste-cache":   true,
	"plans":         true,
	"plugins":       true,
	"session-env":   true,
	"sessions":      true,
	"shell-snapshots": true,
	"tasks":         true,
	"teams":         true,
	"todos":         true,
}

// SidecarMeta is the metadata written alongside each project directory
// to enable cross-machine matching.
type SidecarMeta struct {
	AbsolutePath string   `json:"absolute_path"`
	Segments     []string `json:"segments"`
	GitRemote    string   `json:"git_remote,omitempty"`
	OS           string   `json:"os"`
	IsHome       bool     `json:"is_home,omitempty"`
}

// Claude implements agent.Agent for Claude Code.
type Claude struct {
	sourceDir string
	links     map[string]string // remote project dir -> local absolute path (manual overrides)
}

// New creates a new Claude agent with the given source directory.
// If sourceDir is empty, the default (~/.claude) is used.
// Links are optional manual project mappings from config.
func New(sourceDir string, links ...map[string]string) *Claude {
	if sourceDir == "" {
		sourceDir = DefaultSourceDir()
	}
	c := &Claude{sourceDir: sourceDir}
	if len(links) > 0 && links[0] != nil {
		c.links = links[0]
	}
	return c
}

func (c *Claude) Name() string {
	return Name
}

func (c *Claude) SourceDir() string {
	return c.sourceDir
}

func (c *Claude) DefaultCategories() []agent.Category {
	return []agent.Category{
		agent.CategoryConfig,
		agent.CategoryCustomCode,
		agent.CategoryMemory,
	}
}

func (c *Claude) Collect(categories []agent.Category) ([]agent.SyncFile, error) {
	var files []agent.SyncFile

	catSet := make(map[agent.Category]bool)
	for _, cat := range categories {
		catSet[cat] = true
	}

	for cat, rules := range categoryRules {
		if !catSet[cat] {
			continue
		}

		for _, rule := range rules {
			var collected []string
			var err error

			if rule.Dir != "" {
				collected, err = c.walkDir(rule.Dir)
			} else {
				collected, err = c.globFiles(rule.Glob)
			}
			if err != nil {
				return nil, err
			}

			for _, absPath := range collected {
				if c.isExcluded(absPath) {
					continue
				}
				relPath, _ := filepath.Rel(c.sourceDir, absPath)
				files = append(files, agent.SyncFile{
					RelPath:     relPath,
					StagingPath: filepath.ToSlash(relPath),
					Category:    cat,
				})
			}
		}
	}

	// Generate sidecar metadata for project directories
	if catSet[agent.CategoryMemory] || catSet[agent.CategorySessions] {
		sidecars, err := c.generateSidecars()
		if err != nil {
			return nil, fmt.Errorf("generating sidecars: %w", err)
		}
		files = append(files, sidecars...)
	}

	return files, nil
}

// walkDir recursively collects all files under a directory.
func (c *Claude) walkDir(dir string) ([]string, error) {
	root := filepath.Join(c.sourceDir, dir)
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil, nil
	}

	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// globFiles matches files using filepath.Glob.
func (c *Claude) globFiles(pattern string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(c.sourceDir, pattern))
	if err != nil {
		return nil, fmt.Errorf("globbing %q: %w", pattern, err)
	}
	// Filter out directories
	var files []string
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil || info.IsDir() {
			continue
		}
		files = append(files, m)
	}
	return files, nil
}

func (c *Claude) Place(stagingDir string, files []agent.SyncFile) (*agent.PlaceResult, error) {
	result := &agent.PlaceResult{}

	// Normalize RelPath separators for cross-platform consistency (git always uses /).
	for i := range files {
		files[i].RelPath = filepath.ToSlash(files[i].RelPath)
	}

	// Separate files into project files and non-project files
	var nonProjectFiles []agent.SyncFile
	projectFiles := make(map[string][]agent.SyncFile) // encoded dir name -> files
	sidecarFiles := make(map[string]SidecarMeta)       // encoded dir name -> metadata

	for _, f := range files {
		parts := strings.SplitN(f.RelPath, "/", 3)
		if len(parts) >= 2 && parts[0] == "projects" {
			encodedDir := parts[1]

			// Check if this is a sidecar file
			if strings.HasSuffix(encodedDir, ".meta.json") {
				encodedDir = strings.TrimSuffix(encodedDir, ".meta.json")
				data, err := os.ReadFile(filepath.Join(stagingDir, f.RelPath))
				if err == nil {
					var meta SidecarMeta
					if json.Unmarshal(data, &meta) == nil {
						sidecarFiles[encodedDir] = meta
					}
				}
				continue // don't place sidecar files themselves
			}

			projectFiles[encodedDir] = append(projectFiles[encodedDir], f)
		} else {
			nonProjectFiles = append(nonProjectFiles, f)
		}
	}

	// Place non-project files directly
	for _, f := range nonProjectFiles {
		src := filepath.Join(stagingDir, f.RelPath)
		dst := filepath.Join(c.sourceDir, f.RelPath)
		if err := copyFileForPlace(src, dst); err != nil {
			return nil, fmt.Errorf("placing %s: %w", f.RelPath, err)
		}
		result.Placed++
	}

	// Match and place project files
	localProjects, err := c.listLocalProjects()
	if err != nil {
		return nil, fmt.Errorf("listing local projects: %w", err)
	}

	for encodedDir, pFiles := range projectFiles {
		meta := sidecarFiles[encodedDir]
		localDir, matched := c.matchProject(encodedDir, meta, localProjects)

		if !matched {
			result.Unmatched = append(result.Unmatched, encodedDir)
			result.Skipped += len(pFiles)
			continue
		}

		for _, f := range pFiles {
			// Rewrite the path from remote project dir to local project dir
			parts := strings.SplitN(f.RelPath, "/", 3)
			var localRelPath string
			if len(parts) == 3 {
				localRelPath = filepath.Join("projects", localDir, parts[2])
			} else {
				localRelPath = filepath.Join("projects", localDir)
			}

			src := filepath.Join(stagingDir, f.RelPath)
			dst := filepath.Join(c.sourceDir, localRelPath)
			if err := copyFileForPlace(src, dst); err != nil {
				return nil, fmt.Errorf("placing %s: %w", f.RelPath, err)
			}
			result.Placed++
		}
	}

	return result, nil
}

// matchProject implements the 4-tier matching algorithm.
// Returns the local encoded directory name and whether a match was found.
func (c *Claude) matchProject(remoteDir string, meta SidecarMeta, localProjects map[string]string) (string, bool) {
	// Tier 1: Exact match — same encoded directory name
	if _, ok := localProjects[remoteDir]; ok {
		return remoteDir, true
	}

	// Tier 2: Git remote match
	if meta.GitRemote != "" {
		for localDir, localPath := range localProjects {
			localRemote := gitRemoteFor(localPath)
			if localRemote != "" && localRemote == meta.GitRemote {
				return localDir, true
			}
		}
	}

	// Tier 3: Suffix match on path segments
	if len(meta.Segments) > 0 {
		bestMatch := ""
		bestScore := 0

		for localDir, localPath := range localProjects {
			localSegments := pathSegments(localPath)
			score := suffixMatchScore(meta.Segments, localSegments)
			if score > bestScore && score >= 2 { // require at least 2 matching segments
				bestScore = score
				bestMatch = localDir
			}
		}

		if bestMatch != "" {
			return bestMatch, true
		}
	}

	// Tier 4: Home directory match — if the remote project is the other
	// machine's home dir, match it to the local home dir project.
	if meta.IsHome || looksLikeHomeDir(meta) {
		homeDir, _ := os.UserHomeDir()
		if homeDir != "" {
			for localDir, localPath := range localProjects {
				if filepath.Clean(localPath) == filepath.Clean(homeDir) {
					return localDir, true
				}
			}
		}
	}

	// Tier 5: Manual link from config
	if c.links != nil {
		if localPath, ok := c.links[remoteDir]; ok {
			// Find or create the local encoded dir for this path
			for localDir, lp := range localProjects {
				if lp == localPath {
					return localDir, true
				}
			}
			// The linked path might not have a local project dir yet — skip for now
		}
	}

	// Tier 6: No match — stage for later
	return "", false
}

// suffixMatchScore returns the number of matching segments from the right.
func suffixMatchScore(a, b []string) int {
	score := 0
	ai, bi := len(a)-1, len(b)-1
	for ai >= 0 && bi >= 0 {
		if a[ai] != b[bi] {
			break
		}
		score++
		ai--
		bi--
	}
	return score
}

// listLocalProjects returns a map of encoded dir name -> absolute path
// for all project directories in the local Claude config.
func (c *Claude) listLocalProjects() (map[string]string, error) {
	projectsDir := filepath.Join(c.sourceDir, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	projects := make(map[string]string)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirName := entry.Name()
		absPath := decodeProjectPath(dirName)
		projects[dirName] = absPath
	}
	return projects, nil
}

// listStagingProjects reads sidecar .meta.json files from the staging
// directory and returns a map of encoded dir name -> SidecarMeta.
func listStagingProjects(stagingDir string) (map[string]SidecarMeta, error) {
	projectsDir := filepath.Join(stagingDir, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	projects := make(map[string]SidecarMeta)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".meta.json") {
			continue
		}
		encodedDir := strings.TrimSuffix(name, ".meta.json")
		data, err := os.ReadFile(filepath.Join(projectsDir, name))
		if err != nil {
			continue
		}
		var meta SidecarMeta
		if json.Unmarshal(data, &meta) == nil {
			projects[encodedDir] = meta
		}
	}
	return projects, nil
}

// buildProjectMapping returns a map of localEncodedDir -> stagingEncodedDir
// for projects that exist in staging under a different path encoding (cross-machine).
// Projects with the same encoding on both sides are omitted (no remapping needed).
func (c *Claude) buildProjectMapping(stagingDir string) (map[string]string, error) {
	localProjects, err := c.listLocalProjects()
	if err != nil {
		return nil, err
	}
	stagingProjects, err := listStagingProjects(stagingDir)
	if err != nil {
		return nil, err
	}
	if len(localProjects) == 0 || len(stagingProjects) == 0 {
		return nil, nil
	}

	mapping := make(map[string]string)
	for stagingName, meta := range stagingProjects {
		localDir, matched := c.matchProject(stagingName, meta, localProjects)
		if matched && localDir != stagingName {
			// Only remap if the staging directory actually exists with content.
			// A sidecar alone (no directory) means this machine's encoding is
			// the canonical one — no remapping needed.
			stagingProjDir := filepath.Join(stagingDir, "projects", stagingName)
			if info, err := os.Stat(stagingProjDir); err == nil && info.IsDir() {
				mapping[localDir] = stagingName
			}
		}
	}
	return mapping, nil
}

// remapStagingPath rewrites the project directory segment of a staging path
// using the given mapping. Non-project paths and sidecar files are returned as-is.
func remapStagingPath(stagingPath string, mapping map[string]string) string {
	if len(mapping) == 0 {
		return stagingPath
	}
	parts := strings.SplitN(stagingPath, "/", 3)
	if len(parts) < 2 || parts[0] != "projects" {
		return stagingPath
	}
	if strings.HasSuffix(parts[1], ".meta.json") {
		return stagingPath
	}
	if remapped, ok := mapping[parts[1]]; ok {
		if len(parts) == 3 {
			return "projects/" + remapped + "/" + parts[2]
		}
		return "projects/" + remapped
	}
	return stagingPath
}

// MapStagingPaths implements agent.StagingMapper. It rewrites StagingPath
// fields for project files that have cross-machine equivalents in staging.
// Sidecar files are not remapped — both machines' sidecars coexist in staging.
func (c *Claude) MapStagingPaths(stagingDir string, files []agent.SyncFile) ([]agent.SyncFile, error) {
	mapping, err := c.buildProjectMapping(stagingDir)
	if err != nil {
		return nil, fmt.Errorf("building project mapping: %w", err)
	}
	if len(mapping) == 0 {
		return files, nil
	}

	result := make([]agent.SyncFile, len(files))
	for i, f := range files {
		result[i] = f
		result[i].StagingPath = remapStagingPath(f.StagingPath, mapping)
	}
	return result, nil
}

// copyFileForPlace copies a file from src to dst, creating parent dirs as needed.
func copyFileForPlace(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func (c *Claude) Diff(stagingDir string, categories []agent.Category) ([]backend.FileChange, error) {
	files, err := c.Collect(categories)
	if err != nil {
		return nil, fmt.Errorf("collecting files: %w", err)
	}

	// Build cross-machine project mapping so we compare against the right
	// staging paths and don't report other machines' project dirs as deletions.
	mapping, _ := c.buildProjectMapping(stagingDir)

	// Collect local project dir names so we know which staging project dirs
	// belong to this machine.
	localProjectDirs := make(map[string]bool)
	localProjects, _ := c.listLocalProjects()
	for dirName := range localProjects {
		localProjectDirs[dirName] = true
	}
	// Cross-machine staging dirs (mapped from local projects) should also
	// be skipped — files under these belong to both machines and are handled
	// by the local-vs-staging comparison above.
	crossMachineDirs := make(map[string]bool)
	for _, stagingName := range mapping {
		crossMachineDirs[stagingName] = true
	}

	var changes []backend.FileChange
	localPaths := make(map[string]bool, len(files))

	for _, f := range files {
		mapped := remapStagingPath(f.StagingPath, mapping)
		localPaths[mapped] = true
		localData, err := os.ReadFile(filepath.Join(c.sourceDir, f.RelPath))
		if err != nil {
			continue
		}
		stagedPath := filepath.Join(stagingDir, mapped)
		stagedData, err := os.ReadFile(stagedPath)
		if os.IsNotExist(err) {
			changes = append(changes, backend.FileChange{Path: mapped, Kind: backend.ChangeAdded})
		} else if err == nil && string(localData) != string(stagedData) {
			changes = append(changes, backend.FileChange{Path: mapped, Kind: backend.ChangeModified})
		}
	}

	// Build set of enabled categories for filtering the deletion scan.
	catSet := make(map[agent.Category]bool, len(categories))
	for _, cat := range categories {
		catSet[cat] = true
	}

	// Find staged files not present locally (deletions)
	if _, err := os.Stat(stagingDir); err == nil {
		filepath.WalkDir(stagingDir, func(path string, d os.DirEntry, err error) error { //nolint:errcheck
			if err != nil || d.IsDir() {
				return err
			}
			relPath, _ := filepath.Rel(stagingDir, path)
			relPath = filepath.ToSlash(relPath)
			if localPaths[relPath] {
				return nil
			}
			// Only report deletions for files that belong to an enabled category.
			if cat := categorizePath(relPath); cat != "" && !catSet[cat] {
				return nil
			}
			// Skip project files that don't belong to this machine:
			// - cross-machine dirs (matched to a local project under a different encoding)
			// - remote-only dirs (no local project at all)
			// - session files (.jsonl) are append-only across machines; a session
			//   pushed from another machine should not appear as a local deletion.
			parts := strings.SplitN(relPath, "/", 3)
			if len(parts) >= 2 && parts[0] == "projects" {
				dirName := strings.TrimSuffix(parts[1], ".meta.json")
				if crossMachineDirs[dirName] || !localProjectDirs[dirName] {
					return nil
				}
				if len(parts) == 3 && categorizePath(relPath) == agent.CategorySessions {
					return nil
				}
			}
			changes = append(changes, backend.FileChange{Path: relPath, Kind: backend.ChangeDeleted})
			return nil
		})
	}

	return changes, nil
}

// isExcluded checks if a file path matches any exclude pattern.
func (c *Claude) isExcluded(path string) bool {
	base := filepath.Base(path)

	// Check file patterns
	for _, pattern := range excludePatterns {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}

	// Check if any parent directory is excluded
	relPath, err := filepath.Rel(c.sourceDir, path)
	if err != nil {
		return false
	}
	parts := strings.Split(relPath, string(filepath.Separator))
	for _, part := range parts {
		if excludeDirs[part] {
			return true
		}
	}

	return false
}

// generateSidecars creates SidecarMeta files for each project directory.
func (c *Claude) generateSidecars() ([]agent.SyncFile, error) {
	projectsDir := filepath.Join(c.sourceDir, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	homeDir, _ := os.UserHomeDir()

	var files []agent.SyncFile
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dirName := entry.Name()
		absPath := decodeProjectPath(dirName)
		segments := pathSegments(absPath)

		meta := SidecarMeta{
			AbsolutePath: absPath,
			Segments:     segments,
			GitRemote:    gitRemoteFor(absPath),
			OS:           runtime.GOOS,
			IsHome:       homeDir != "" && filepath.Clean(absPath) == filepath.Clean(homeDir),
		}

		data, err := json.MarshalIndent(meta, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshaling sidecar for %s: %w", dirName, err)
		}

		// Write sidecar to source dir so it gets picked up by the collector
		sidecarPath := filepath.Join(projectsDir, dirName+".meta.json")
		if err := os.WriteFile(sidecarPath, data, 0o644); err != nil {
			return nil, fmt.Errorf("writing sidecar for %s: %w", dirName, err)
		}

		relPath := filepath.Join("projects", dirName+".meta.json")
		files = append(files, agent.SyncFile{
			RelPath:     relPath,
			StagingPath: "projects/" + dirName + ".meta.json",
			Category:    agent.CategoryMemory,
		})
	}

	return files, nil
}

// CategorizePath implements the Agent interface.
func (c *Claude) CategorizePath(relPath string) agent.Category {
	return categorizePath(relPath)
}

// categorizePath returns the category a staging-relative path belongs to,
// or empty string if it cannot be determined.
func categorizePath(relPath string) agent.Category {
	switch {
	case relPath == "history.jsonl":
		return agent.CategoryHistory
	case relPath == "CLAUDE.md" || relPath == "settings.json" || relPath == "settings.local.json":
		return agent.CategoryConfig
	case strings.HasPrefix(relPath, "commands/") ||
		strings.HasPrefix(relPath, "skills/") ||
		strings.HasPrefix(relPath, "agents/") ||
		strings.HasPrefix(relPath, "rules/") ||
		strings.HasPrefix(relPath, "output-styles/"):
		return agent.CategoryCustomCode
	case strings.HasPrefix(relPath, "projects/"):
		parts := strings.SplitN(relPath, "/", 3)
		if len(parts) < 3 {
			return ""
		}
		if strings.HasSuffix(parts[1], ".meta.json") {
			return agent.CategoryMemory // sidecars are generated alongside memory/sessions
		}
		sub := parts[2]
		if strings.HasPrefix(sub, "memory/") {
			return agent.CategoryMemory
		}
		if strings.HasSuffix(sub, ".jsonl") {
			return agent.CategorySessions
		}
	}
	return ""
}

// looksLikeHomeDir heuristically detects whether a sidecar's absolute path
// is a home directory based on the OS and path pattern. This allows matching
// even when the sidecar was generated before the IsHome field was added.
func looksLikeHomeDir(meta SidecarMeta) bool {
	if len(meta.Segments) == 0 {
		return false
	}
	switch meta.OS {
	case "linux":
		// /home/<username>
		return len(meta.Segments) == 2 && meta.Segments[0] == "home"
	case "darwin":
		// /Users/<username>
		return len(meta.Segments) == 2 && meta.Segments[0] == "Users"
	}
	return false
}

// decodeProjectPath converts a Claude-encoded directory name back to an
// absolute path. The encoding replaces path separators with dashes.
// This is inherently ambiguous (dash is also a valid path char), but we
// use best-effort decoding — the sidecar stores the real path anyway.
func decodeProjectPath(encoded string) string {
	encoded = strings.TrimPrefix(encoded, "-")
	return "/" + strings.ReplaceAll(encoded, "-", "/")
}

// pathSegments splits an absolute path into its component segments.
func pathSegments(absPath string) []string {
	cleaned := filepath.Clean(absPath)
	parts := strings.Split(cleaned, string(filepath.Separator))
	var segments []string
	for _, p := range parts {
		if p != "" {
			segments = append(segments, p)
		}
	}
	return segments
}

// gitRemoteFor tries to get the origin remote URL for a path that might
// be a git repository.
func gitRemoteFor(path string) string {
	cmd := exec.Command("git", "-C", path, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// CategoryRules returns the collection rules for each category.
func CategoryRules() map[agent.Category][]collectionRule {
	return categoryRules
}

// ExcludePatterns returns the patterns that are always excluded.
func ExcludePatterns() []string {
	return excludePatterns
}
