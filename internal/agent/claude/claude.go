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

// syncTypeRules maps each sync type to collection rules. Rules with Dir set
// collect all files recursively under that directory. Rules with Glob set
// use filepath.Glob for exact matching.
type collectionRule struct {
	Dir  string // collect all files recursively under this dir (relative to source)
	Glob string // filepath.Glob pattern (relative to source)
}

const (
	SyncTypeClaudeMD    agent.SyncType = "claude-md"
	SyncTypeRules       agent.SyncType = "rules"
	SyncTypeCommands    agent.SyncType = "commands"
	SyncTypeSkills      agent.SyncType = "skills"
	SyncTypeAgents      agent.SyncType = "agents"
	SyncTypePlugins     agent.SyncType = "plugins"
	SyncTypeOutputStyle agent.SyncType = "output-styles"
	SyncTypeAgentMemory agent.SyncType = "agent-memory"
	SyncTypeAutoMemory  agent.SyncType = "auto-memory"
	SyncTypeSessions    agent.SyncType = "sessions"
	SyncTypeHistory     agent.SyncType = "history"
	SyncTypeKeybindings agent.SyncType = "keybindings"
)

var syncTypeSpecs = []agent.SyncTypeSpec{
	{ID: SyncTypeClaudeMD, Label: "CLAUDE.md", Description: "Global Claude instructions.", Group: "instructions", Default: true, Mode: agent.SyncModeReplace, Reference: "https://code.claude.com/docs/en/claude-directory"},
	{ID: SyncTypeRules, Label: "Rules", Description: "Topic-scoped instructions in rules/.", Group: "instructions", Default: true, Mode: agent.SyncModeReplace, Reference: "https://code.claude.com/docs/en/claude-directory"},
	{ID: SyncTypeCommands, Label: "Commands", Description: "Reusable prompts in commands/.", Group: "prompts", Default: true, Mode: agent.SyncModeReplace, Reference: "https://code.claude.com/docs/en/claude-directory"},
	{ID: SyncTypeSkills, Label: "Skills", Description: "Skills in skills/<name>/.", Group: "prompts", Default: true, Mode: agent.SyncModeReplace, Reference: "https://code.claude.com/docs/en/claude-directory"},
	{ID: SyncTypeAgents, Label: "Subagents", Description: "Subagent definitions in agents/.", Group: "prompts", Default: true, Mode: agent.SyncModeReplace, Reference: "https://code.claude.com/docs/en/claude-directory"},
	{ID: SyncTypePlugins, Label: "Plugins", Description: "Claude plugins in plugins/.", Group: "prompts", Default: true, Mode: agent.SyncModeReplace, Reference: "https://code.claude.com/docs/en/claude-directory"},
	{ID: SyncTypeOutputStyle, Label: "Output styles", Description: "Custom output styles in output-styles/.", Group: "prompts", Default: true, Mode: agent.SyncModeReplace, Reference: "https://code.claude.com/docs/en/claude-directory"},
	{ID: SyncTypeAgentMemory, Label: "Agent memory", Description: "Persistent subagent memory in agent-memory/.", Group: "memory", Default: true, Mode: agent.SyncModeReplace, Reference: "https://code.claude.com/docs/en/claude-directory"},
	{ID: SyncTypeAutoMemory, Label: "Auto memory", Description: "Project auto-memory in projects/*/memory/.", Group: "memory", Default: true, Mode: agent.SyncModeReplace, Reference: "https://code.claude.com/docs/en/claude-directory"},
	{ID: SyncTypeSessions, Label: "Sessions", Description: "Project session JSONL logs.", Group: "history", Default: false, Mode: agent.SyncModeAppendOnly, Reference: "https://code.claude.com/docs/en/claude-directory"},
	{ID: SyncTypeHistory, Label: "History", Description: "Global history.jsonl.", Group: "history", Default: false, Mode: agent.SyncModeReplace, Reference: "https://code.claude.com/docs/en/claude-directory"},
	{ID: SyncTypeKeybindings, Label: "Keybindings", Description: "Global keybindings.json.", Group: "config", Default: false, Mode: agent.SyncModeReplace, Reference: "https://code.claude.com/docs/en/claude-directory"},
}

var syncTypeRules = map[agent.SyncType][]collectionRule{
	SyncTypeClaudeMD: {
		{Glob: "CLAUDE.md"},
	},
	SyncTypeRules: {
		{Dir: "rules"},
	},
	SyncTypeCommands: {
		{Dir: "commands"},
	},
	SyncTypeSkills: {
		{Dir: "skills"},
	},
	SyncTypeAgents: {
		{Dir: "agents"},
	},
	SyncTypePlugins: {
		{Dir: "plugins"},
	},
	SyncTypeOutputStyle: {
		{Dir: "output-styles"},
	},
	SyncTypeAgentMemory: {
		{Dir: "agent-memory"},
	},
	SyncTypeAutoMemory: {
		{Glob: "projects/*/memory/*"},
	},
	SyncTypeSessions: {
		{Glob: "projects/*/*.jsonl"},
	},
	SyncTypeHistory: {
		{Glob: "history.jsonl"},
	},
	SyncTypeKeybindings: {
		{Glob: "keybindings.json"},
	},
}

var legacySyncTypeAliases = map[string][]agent.SyncType{
	"config":      {SyncTypeClaudeMD},
	"custom-code": {SyncTypeCommands, SyncTypeSkills, SyncTypeAgents, SyncTypeRules, SyncTypeOutputStyle},
	"memory":      {SyncTypeAgentMemory, SyncTypeAutoMemory},
	"sessions":    {SyncTypeSessions},
	"history":     {SyncTypeHistory},
	"plugins":     {SyncTypePlugins},
}

// excludePatterns are always excluded from sync.
var excludePatterns = []string{
	".credentials.json",
	"credentials.json",
	"auth.json",
	"stats-cache.json",
	"install-counts-cache.json",
	"*.tmp",
	".lock",
}

// excludeDirs are directories always excluded from sync.
var excludeDirs = map[string]bool{
	"telemetry":       true,
	"cache":           true,
	"backups":         true,
	"debug":           true,
	"downloads":       true,
	"file-history":    true,
	"ide":             true,
	"marketplaces":    true,
	"paste-cache":     true,
	"plans":           true,
	"session-env":     true,
	"sessions":        true,
	"shell-snapshots": true,
	"tasks":           true,
	"teams":           true,
	"todos":           true,
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

func (c *Claude) SupportedSyncTypes() []agent.SyncTypeSpec {
	return syncTypeSpecs
}

func (c *Claude) DefaultSyncTypes() []agent.SyncType {
	var defaults []agent.SyncType
	for _, spec := range syncTypeSpecs {
		if spec.Default {
			defaults = append(defaults, spec.ID)
		}
	}
	return defaults
}

func (c *Claude) NormalizeSyncTypes(selected []string) ([]agent.SyncType, error) {
	if len(selected) == 0 {
		return nil, nil
	}

	supported := agent.SyncTypeMap(c)
	seen := make(map[agent.SyncType]bool)
	var resolved []agent.SyncType

	for _, raw := range selected {
		if aliases, ok := legacySyncTypeAliases[raw]; ok {
			for _, syncType := range aliases {
				if !seen[syncType] {
					resolved = append(resolved, syncType)
					seen[syncType] = true
				}
			}
			continue
		}

		syncType := agent.SyncType(raw)
		if _, ok := supported[syncType]; !ok {
			return nil, fmt.Errorf("unsupported sync type %q", raw)
		}
		if !seen[syncType] {
			resolved = append(resolved, syncType)
			seen[syncType] = true
		}
	}

	return resolved, nil
}

func (c *Claude) Collect(syncTypes []agent.SyncType) ([]agent.SyncFile, error) {
	var files []agent.SyncFile

	typeSet := agent.SyncTypeSet(syncTypes)

	for syncType, rules := range syncTypeRules {
		if !typeSet[syncType] {
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
					Type:        syncType,
				})
			}
		}
	}

	// Generate sidecar metadata for project directories
	if typeSet[SyncTypeAutoMemory] || typeSet[SyncTypeSessions] {
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
	sidecarFiles, err := listStagingProjects(stagingDir)
	if err != nil {
		return nil, fmt.Errorf("reading project sidecars: %w", err)
	}
	if sidecarFiles == nil {
		sidecarFiles = make(map[string]SidecarMeta)
	}

	for _, f := range files {
		parts := strings.SplitN(f.RelPath, "/", 3)
		if len(parts) >= 2 && parts[0] == "projects" {
			encodedDir := parts[1]

			// Check if this is a sidecar file
			if strings.HasSuffix(encodedDir, ".meta.json") {
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
// for projects that exist in staging under a different path encoding (cross-machine),
// along with the local projects map. Projects with the same encoding on both
// sides are omitted from the mapping (no remapping needed).
func (c *Claude) buildProjectMapping(stagingDir string) (map[string]string, map[string]string, error) {
	localProjects, err := c.listLocalProjects()
	if err != nil {
		return nil, nil, err
	}
	stagingProjects, err := listStagingProjects(stagingDir)
	if err != nil {
		return nil, nil, err
	}
	if len(localProjects) == 0 || len(stagingProjects) == 0 {
		return nil, localProjects, nil
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
	return mapping, localProjects, nil
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
	mapping, _, err := c.buildProjectMapping(stagingDir)
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

func (c *Claude) Diff(stagingDir string, syncTypes []agent.SyncType) ([]backend.FileChange, error) {
	files, err := c.Collect(syncTypes)
	if err != nil {
		return nil, fmt.Errorf("collecting files: %w", err)
	}

	// Build cross-machine project mapping so we compare against the right
	// staging paths and don't report other machines' project dirs as deletions.
	mapping, localProjects, _ := c.buildProjectMapping(stagingDir)

	localProjectDirs := make(map[string]bool)
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

	typeSet := agent.SyncTypeSet(syncTypes)

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
			// Only report deletions for files that belong to an enabled sync type.
			if syncType := syncTypeForPath(relPath); syncType != "" && !typeSet[syncType] {
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
				if len(parts) == 3 && syncTypeForPath(relPath) == SyncTypeSessions {
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
			Type:        SyncTypeAutoMemory,
		})
	}

	return files, nil
}

// SyncTypeForPath implements the Agent interface.
func (c *Claude) SyncTypeForPath(relPath string) agent.SyncType {
	return syncTypeForPath(relPath)
}

// syncTypeForPath returns the sync type a staging-relative path belongs to,
// or empty string if it cannot be determined.
func syncTypeForPath(relPath string) agent.SyncType {
	switch {
	case relPath == "history.jsonl":
		return SyncTypeHistory
	case relPath == "CLAUDE.md":
		return SyncTypeClaudeMD
	case relPath == "keybindings.json":
		return SyncTypeKeybindings
	case strings.HasPrefix(relPath, "commands/"):
		return SyncTypeCommands
	case strings.HasPrefix(relPath, "skills/"):
		return SyncTypeSkills
	case strings.HasPrefix(relPath, "agents/"):
		return SyncTypeAgents
	case strings.HasPrefix(relPath, "plugins/"):
		return SyncTypePlugins
	case strings.HasPrefix(relPath, "rules/"):
		return SyncTypeRules
	case strings.HasPrefix(relPath, "output-styles/"):
		return SyncTypeOutputStyle
	case strings.HasPrefix(relPath, "agent-memory/"):
		return SyncTypeAgentMemory
	case strings.HasPrefix(relPath, "projects/"):
		parts := strings.SplitN(relPath, "/", 3)
		if len(parts) < 3 {
			return ""
		}
		if strings.HasSuffix(parts[1], ".meta.json") {
			return SyncTypeAutoMemory
		}
		sub := parts[2]
		if strings.HasPrefix(sub, "memory/") {
			return SyncTypeAutoMemory
		}
		if strings.HasSuffix(sub, ".jsonl") {
			return SyncTypeSessions
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

// CollectionRules returns the collection rules for each sync type.
func CollectionRules() map[agent.SyncType][]collectionRule {
	return syncTypeRules
}

// ExcludePatterns returns the patterns that are always excluded.
func ExcludePatterns() []string {
	return excludePatterns
}
