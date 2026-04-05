package copilot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/manmart/negent/internal/agent"
	"github.com/manmart/negent/internal/backend"
)

const Name = "copilot"

const (
	SyncTypeConfig agent.SyncType = "config"
	SyncTypeMCP    agent.SyncType = "mcp"
	SyncTypeAgents agent.SyncType = "agents"
	SyncTypeSkills agent.SyncType = "skills"
	SyncTypeHooks  agent.SyncType = "hooks"
)

type collectionRule struct {
	Dir  string
	Glob string
}

var syncTypeSpecs = []agent.SyncTypeSpec{
	{ID: SyncTypeConfig, Label: "Config", Description: "Global Copilot CLI config.json.", Group: "config", Default: true, Mode: agent.SyncModeReplace, Reference: "https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-config-dir-reference"},
	{ID: SyncTypeMCP, Label: "MCP", Description: "MCP server config in mcp-config.json.", Group: "config", Default: true, Mode: agent.SyncModeReplace, Reference: "https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-config-dir-reference"},
	{ID: SyncTypeAgents, Label: "Agents", Description: "Custom agents under agents/.", Group: "prompts", Default: true, Mode: agent.SyncModeReplace, Reference: "https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-config-dir-reference"},
	{ID: SyncTypeSkills, Label: "Skills", Description: "Custom skills under skills/.", Group: "prompts", Default: true, Mode: agent.SyncModeReplace, Reference: "https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-config-dir-reference"},
	{ID: SyncTypeHooks, Label: "Hooks", Description: "Hook scripts and config under hooks/.", Group: "automation", Default: true, Mode: agent.SyncModeReplace, Reference: "https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-config-dir-reference"},
}

var syncTypeRules = map[agent.SyncType][]collectionRule{
	SyncTypeConfig: {{Glob: "config.json"}},
	SyncTypeMCP:    {{Glob: "mcp-config.json"}},
	SyncTypeAgents: {{Dir: "agents"}},
	SyncTypeSkills: {{Dir: "skills"}},
	SyncTypeHooks:  {{Dir: "hooks"}},
}

var excludedFiles = map[string]bool{
	"permissions-config.json": true,
	"session-store.db":        true,
}

var excludedDirs = map[string]bool{
	"session-state":     true,
	"logs":              true,
	"installed-plugins": true,
	"ide":               true,
}

// DefaultSourceDir returns the default GitHub Copilot CLI config directory.
func DefaultSourceDir() string {
	if envDir := os.Getenv("COPILOT_HOME"); envDir != "" {
		return envDir
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = os.Getenv("HOME")
	}
	if home == "" {
		home = os.TempDir()
	}
	return filepath.Join(home, ".copilot")
}

type Copilot struct {
	sourceDir string
}

func New(sourceDir string) *Copilot {
	if sourceDir == "" {
		sourceDir = DefaultSourceDir()
	}
	return &Copilot{sourceDir: sourceDir}
}

func (c *Copilot) Name() string {
	return Name
}

func (c *Copilot) SourceDir() string {
	return c.sourceDir
}

func (c *Copilot) SupportedSyncTypes() []agent.SyncTypeSpec {
	return syncTypeSpecs
}

func (c *Copilot) DefaultSyncTypes() []agent.SyncType {
	var defaults []agent.SyncType
	for _, spec := range syncTypeSpecs {
		if spec.Default {
			defaults = append(defaults, spec.ID)
		}
	}
	return defaults
}

func (c *Copilot) NormalizeSyncTypes(selected []string) ([]agent.SyncType, error) {
	if len(selected) == 0 {
		return nil, nil
	}

	supported := agent.SyncTypeMap(c)
	seen := make(map[agent.SyncType]bool)
	var resolved []agent.SyncType
	for _, raw := range selected {
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

func (c *Copilot) Collect(syncTypes []agent.SyncType) ([]agent.SyncFile, error) {
	typeSet := agent.SyncTypeSet(syncTypes)
	var files []agent.SyncFile

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
				relPath, err := filepath.Rel(c.sourceDir, absPath)
				if err != nil {
					return nil, fmt.Errorf("computing relative path for %s: %w", absPath, err)
				}
				relPath = filepath.ToSlash(relPath)
				files = append(files, agent.SyncFile{
					RelPath:     relPath,
					StagingPath: relPath,
					Type:        syncType,
				})
			}
		}
	}

	return files, nil
}

func (c *Copilot) walkDir(dir string) ([]string, error) {
	root := filepath.Join(c.sourceDir, dir)
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil, nil
	}

	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			relPath, relErr := filepath.Rel(c.sourceDir, path)
			if relErr == nil {
				parts := strings.Split(filepath.Clean(relPath), string(filepath.Separator))
				if len(parts) == 1 && excludedDirs[parts[0]] {
					return filepath.SkipDir
				}
			}
			return nil
		}
		files = append(files, path)
		return nil
	})
	return files, err
}

func (c *Copilot) globFiles(pattern string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(c.sourceDir, pattern))
	if err != nil {
		return nil, fmt.Errorf("globbing %q: %w", pattern, err)
	}

	var files []string
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil || info.IsDir() {
			continue
		}
		files = append(files, match)
	}
	return files, nil
}

func (c *Copilot) Place(stagingDir string, files []agent.SyncFile) (*agent.PlaceResult, error) {
	result := &agent.PlaceResult{}

	for _, f := range files {
		src := filepath.Join(stagingDir, filepath.FromSlash(f.RelPath))
		dst := filepath.Join(c.sourceDir, filepath.FromSlash(f.RelPath))
		if err := copyFile(src, dst); err != nil {
			return nil, fmt.Errorf("placing %s: %w", f.RelPath, err)
		}
		result.Placed++
	}

	return result, nil
}

func (c *Copilot) Diff(stagingDir string, syncTypes []agent.SyncType) ([]backend.FileChange, error) {
	files, err := c.Collect(syncTypes)
	if err != nil {
		return nil, fmt.Errorf("collecting files: %w", err)
	}

	var changes []backend.FileChange
	localPaths := make(map[string]bool, len(files))
	typeSet := agent.SyncTypeSet(syncTypes)

	for _, f := range files {
		localPaths[f.StagingPath] = true

		localData, err := os.ReadFile(filepath.Join(c.sourceDir, filepath.FromSlash(f.RelPath)))
		if err != nil {
			continue
		}

		stagedPath := filepath.Join(stagingDir, filepath.FromSlash(f.StagingPath))
		stagedData, err := os.ReadFile(stagedPath)
		if os.IsNotExist(err) {
			changes = append(changes, backend.FileChange{Path: f.StagingPath, Kind: backend.ChangeAdded})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("reading staged file %s: %w", f.StagingPath, err)
		}
		if string(localData) != string(stagedData) {
			changes = append(changes, backend.FileChange{Path: f.StagingPath, Kind: backend.ChangeModified})
		}
	}

	if _, err := os.Stat(stagingDir); err == nil {
		err := filepath.WalkDir(stagingDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			relPath, err := filepath.Rel(stagingDir, path)
			if err != nil {
				return fmt.Errorf("computing path relative to staging: %w", err)
			}
			relPath = filepath.ToSlash(relPath)
			if localPaths[relPath] {
				return nil
			}

			syncType := syncTypeForPath(relPath)
			if syncType == "" {
				return nil
			}
			if !typeSet[syncType] {
				return nil
			}

			changes = append(changes, backend.FileChange{Path: relPath, Kind: backend.ChangeDeleted})
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walking staging dir: %w", err)
		}
	}

	return changes, nil
}

func (c *Copilot) SyncTypeForPath(relPath string) agent.SyncType {
	return syncTypeForPath(relPath)
}

func syncTypeForPath(relPath string) agent.SyncType {
	switch {
	case relPath == "config.json":
		return SyncTypeConfig
	case relPath == "mcp-config.json":
		return SyncTypeMCP
	case strings.HasPrefix(relPath, "agents/"):
		return SyncTypeAgents
	case strings.HasPrefix(relPath, "skills/"):
		return SyncTypeSkills
	case strings.HasPrefix(relPath, "hooks/"):
		return SyncTypeHooks
	default:
		return ""
	}
}

func (c *Copilot) isExcluded(path string) bool {
	relPath, err := filepath.Rel(c.sourceDir, path)
	if err != nil {
		return false
	}

	relPath = filepath.Clean(relPath)
	if relPath == "." {
		return false
	}

	if excludedFiles[filepath.Base(relPath)] && !strings.Contains(relPath, string(filepath.Separator)) {
		return true
	}

	parts := strings.Split(relPath, string(filepath.Separator))
	if len(parts) > 0 && excludedDirs[parts[0]] {
		return true
	}
	return false
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
