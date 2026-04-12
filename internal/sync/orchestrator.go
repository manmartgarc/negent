package sync

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/manmart/negent/internal/agent"
	"github.com/manmart/negent/internal/backend"
)

// Orchestrator coordinates sync operations across agents and the backend.
type Orchestrator struct {
	backend backend.Backend
	agents  map[string]agent.Agent
}

// PlanAction describes a user-visible sync operation.
type PlanAction string

const (
	ActionUpload       PlanAction = "upload"
	ActionDeleteRemote PlanAction = "delete-remote"
	ActionDownload     PlanAction = "download"
	ActionOverwrite    PlanAction = "overwrite"
	ActionConflict     PlanAction = "conflict"
)

// PlannedChange is one user-visible sync action.
type PlannedChange struct {
	Agent  string
	Path   string
	Action PlanAction
}

// SyncPlan is a remote-aware preview of pending sync work.
type SyncPlan struct {
	Local  []PlannedChange
	Remote []PlannedChange
}

// NewOrchestrator creates a new sync orchestrator.
func NewOrchestrator(be backend.Backend, agents map[string]agent.Agent) *Orchestrator {
	return &Orchestrator{
		backend: be,
		agents:  agents,
	}
}

// Plan returns a remote-aware sync preview for all configured agents.
func (o *Orchestrator) Plan(ctx context.Context, syncTypes map[string][]agent.SyncType) (*SyncPlan, error) {
	plan := &SyncPlan{}
	stagingDir := o.backend.StagingDir()

	remoteChanges, err := o.backend.FetchedFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("reading fetched files: %w", err)
	}

	base, err := snapshotBase(stagingDir, o.agents, syncTypes)
	if err != nil {
		return nil, fmt.Errorf("snapshotting base state: %w", err)
	}

	// Group remote changes by agent to avoid O(agents × remoteChanges).
	remoteByAgent := make(map[string][]backend.FileChange)
	for _, ch := range remoteChanges {
		agentName, _, ok := splitAgentPath(ch.Path)
		if ok {
			remoteByAgent[agentName] = append(remoteByAgent[agentName], ch)
		}
	}

	for name, ag := range o.agents {
		types, ok := syncTypes[name]
		if !ok {
			continue
		}

		agentDir := filepath.Join(stagingDir, name)
		localChanges, err := ag.Diff(agentDir, types)
		if err != nil {
			return nil, fmt.Errorf("diffing %s: %w", name, err)
		}
		for _, ch := range localChanges {
			action := ActionUpload
			if ch.Kind == backend.ChangeDeleted {
				action = ActionDeleteRemote
			}
			plan.Local = append(plan.Local, PlannedChange{
				Agent:  name,
				Path:   ch.Path,
				Action: action,
			})
		}

		typeSet := agent.SyncTypeSet(types)
		specMap := agent.SyncTypeMap(ag)
		for _, ch := range remoteByAgent[name] {
			_, relPath, _ := splitAgentPath(ch.Path)
			syncType := ag.SyncTypeForPath(relPath)
			if syncType == "" || !typeSet[syncType] {
				continue
			}
			if ch.Kind == backend.ChangeDeleted {
				continue
			}

			if strings.HasPrefix(relPath, "projects/") {
				plan.Remote = append(plan.Remote, PlannedChange{
					Agent:  name,
					Path:   relPath,
					Action: ActionDownload,
				})
				continue
			}

			// Append-only files are always merged, never conflicted.
			if agent.LookupMode(specMap, syncType) == agent.SyncModeAppendOnly {
				plan.Remote = append(plan.Remote, PlannedChange{
					Agent:  name,
					Path:   relPath,
					Action: ActionDownload,
				})
				continue
			}

			action := ActionDownload
			localPath := filepath.Join(ag.SourceDir(), relPath)
			localContent, localErr := os.ReadFile(localPath)
			baseContent, hasBase := base[name][relPath]
			switch {
			case localErr == nil && !hasBase:
				action = ActionConflict
			case localErr == nil && hasBase && !bytes.Equal(localContent, baseContent):
				action = ActionConflict
			case localErr == nil:
				action = ActionOverwrite
			}

			plan.Remote = append(plan.Remote, PlannedChange{
				Agent:  name,
				Path:   relPath,
				Action: action,
			})
		}
	}

	return plan, nil
}

// Push collects files from all configured agents and pushes them to the backend.
func (o *Orchestrator) Push(ctx context.Context, syncTypes map[string][]agent.SyncType) error {
	stagingDir := o.backend.StagingDir()

	for name, ag := range o.agents {
		types, ok := syncTypes[name]
		if !ok {
			continue
		}

		files, err := ag.Collect(types)
		if err != nil {
			return fmt.Errorf("collecting from %s: %w", name, err)
		}

		agentDir := filepath.Join(stagingDir, name)

		// Remap staging paths for cross-machine project matching.
		if mapper, ok := ag.(agent.StagingMapper); ok {
			files, err = mapper.MapStagingPaths(agentDir, files)
			if err != nil {
				return fmt.Errorf("mapping staging paths for %s: %w", name, err)
			}
		}
		if err := os.MkdirAll(agentDir, 0o755); err != nil {
			return fmt.Errorf("creating agent dir %s: %w", agentDir, err)
		}

		specMap := agent.SyncTypeMap(ag)

		for _, f := range files {
			src := filepath.Join(ag.SourceDir(), f.RelPath)
			dst := filepath.Join(agentDir, f.StagingPath)

			if agent.LookupMode(specMap, f.Type) == agent.SyncModeAppendOnly {
				if err := mergeAppendOnly(src, dst); err != nil {
					return fmt.Errorf("merging session %s: %w", f.RelPath, err)
				}
			} else {
				if err := copyFile(src, dst); err != nil {
					return fmt.Errorf("copying %s: %w", f.RelPath, err)
				}
			}
		}

		// Remove staged files that are no longer collected (deletions).
		// We compute this inline rather than calling ag.Diff (which would
		// re-collect and re-build project mappings redundantly).
		collectedPaths := make(map[string]bool, len(files))
		localProjDirs := make(map[string]bool)
		for _, f := range files {
			collectedPaths[f.StagingPath] = true
			if strings.HasPrefix(f.StagingPath, "projects/") {
				parts := strings.SplitN(f.StagingPath, "/", 3)
				if len(parts) >= 2 {
					localProjDirs[strings.TrimSuffix(parts[1], ".meta.json")] = true
				}
			}
		}

		typeSet := agent.SyncTypeSet(types)

		deleted := 0
		if err := filepath.WalkDir(agentDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			relPath, err := filepath.Rel(agentDir, path)
			if err != nil {
				return fmt.Errorf("computing path relative to agent dir: %w", err)
			}
			relPath = filepath.ToSlash(relPath)
			if collectedPaths[relPath] {
				return nil
			}
			syncType := ag.SyncTypeForPath(relPath)
			if syncType != "" && !typeSet[syncType] {
				return nil
			}
			if agent.LookupMode(specMap, syncType) == agent.SyncModeAppendOnly {
				return nil // append-only across machines
			}
			if strings.HasPrefix(relPath, "projects/") {
				parts := strings.SplitN(relPath, "/", 3)
				if len(parts) >= 2 {
					dirName := strings.TrimSuffix(parts[1], ".meta.json")
					if !localProjDirs[dirName] {
						return nil
					}
				}
			}
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("removing %s: %w", relPath, err)
			}
			deleted++
			return nil
		}); err != nil {
			return fmt.Errorf("walking staged files for %s: %w", name, err)
		}

		fmt.Printf("  %s: %d files collected", name, len(files))
		if deleted > 0 {
			fmt.Printf(", %d deleted", deleted)
		}
		fmt.Println()
	}

	msg := fmt.Sprintf("negent push %s", time.Now().Format(time.RFC3339))
	return o.backend.Push(ctx, msg)
}

// PullResult summarises the outcome of a Pull operation.
type PullResult struct {
	Conflicts []ConflictInfo
	Placed    int
	Merged    int
	Skipped   int
}

// ConflictInfo describes a single file that could not be placed due to a conflict.
// LocalPath and StagingPath are absolute paths.
type ConflictInfo struct {
	Agent       string
	RelPath     string
	LocalPath   string
	StagingPath string
}

// Conflicts returns all files that are currently in conflict: present in staging,
// present locally, and differing — without performing a pull.
func (o *Orchestrator) Conflicts(syncTypes map[string][]agent.SyncType) ([]ConflictInfo, error) {
	stagingDir := o.backend.StagingDir()
	base, err := snapshotBase(stagingDir, o.agents, syncTypes)
	if err != nil {
		return nil, fmt.Errorf("snapshotting base state: %w", err)
	}
	var all []ConflictInfo

	for name, ag := range o.agents {
		types, ok := syncTypes[name]
		if !ok {
			continue
		}
		agentDir := filepath.Join(stagingDir, name)
		conflicts, err := detectConflicts(name, ag, agentDir, types, base[name])
		if err != nil {
			return nil, err
		}
		all = append(all, conflicts...)
	}
	return all, nil
}

// detectConflicts walks an agent's staging directory and returns files where
// the local version differs from both the base snapshot and the staging content.
// Project files and append-only files are excluded.
func detectConflicts(agentName string, ag agent.Agent, agentDir string, types []agent.SyncType, agentBase map[string][]byte) ([]ConflictInfo, error) {
	typeSet := agent.SyncTypeSet(types)
	specMap := agent.SyncTypeMap(ag)
	var conflicts []ConflictInfo

	err := filepath.WalkDir(agentDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		relPath, err := filepath.Rel(agentDir, path)
		if err != nil {
			return fmt.Errorf("computing path relative to agent dir: %w", err)
		}
		relPath = filepath.ToSlash(relPath)
		if strings.HasPrefix(relPath, "projects/") {
			return nil
		}
		syncType := ag.SyncTypeForPath(relPath)
		if syncType == "" || !typeSet[syncType] {
			return nil
		}
		if agent.LookupMode(specMap, syncType) == agent.SyncModeAppendOnly {
			return nil
		}
		localContent, err := os.ReadFile(filepath.Join(ag.SourceDir(), relPath))
		if err != nil {
			return nil // local absent — not a conflict
		}
		baseContent, hasBase := agentBase[relPath]
		stagingContent, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading staged file %s: %w", relPath, err)
		}
		isConflict := !hasBase || (!bytes.Equal(localContent, baseContent) && !bytes.Equal(localContent, stagingContent))
		if isConflict {
			conflicts = append(conflicts, ConflictInfo{
				Agent:       agentName,
				RelPath:     relPath,
				LocalPath:   filepath.Join(ag.SourceDir(), relPath),
				StagingPath: path,
			})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking staging for %s: %w", agentName, err)
	}
	return conflicts, nil
}

// Pull fetches from the backend and places files into agent source directories.
// Conflict detection: any non-project file where local differs from the staged
// base (the last synced remote state) is skipped and reported. This protects
// local edits regardless of whether the remote also changed the file.
func (o *Orchestrator) Pull(ctx context.Context, syncTypes map[string][]agent.SyncType) (*PullResult, error) {
	stagingDir := o.backend.StagingDir()

	// Snapshot staged content for non-project files before the working tree is
	// updated. This is the base: the last remote state this machine synced to.
	// Project dirs use path-encoded names; Place() handles matching for those.
	base, err := snapshotBase(stagingDir, o.agents, syncTypes)
	if err != nil {
		return nil, fmt.Errorf("snapshotting base state: %w", err)
	}

	// Integrate remote changes into the working tree.
	if err := o.backend.Pull(ctx); err != nil {
		return nil, fmt.Errorf("pulling from backend: %w", err)
	}

	var pullResult PullResult

	for name, ag := range o.agents {
		types, ok := syncTypes[name]
		if !ok {
			continue
		}

		agentDir := filepath.Join(stagingDir, name)
		if _, err := os.Stat(agentDir); os.IsNotExist(err) {
			fmt.Printf("  %s: no data in remote\n", name)
			continue
		}

		typeSet := agent.SyncTypeSet(types)

		// Walk staging (now at remote HEAD) to build the file list,
		// filtering to only files belonging to enabled sync types.
		var files []agent.SyncFile
		err := filepath.WalkDir(agentDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			relPath, err := filepath.Rel(agentDir, path)
			if err != nil {
				return fmt.Errorf("computing path relative to agent dir: %w", err)
			}
			relPath = filepath.ToSlash(relPath)
			syncType := ag.SyncTypeForPath(relPath)
			if syncType == "" {
				return nil
			}
			if !typeSet[syncType] {
				return nil
			}
			files = append(files, agent.SyncFile{
				RelPath:     relPath,
				StagingPath: relPath,
			})
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walking staging dir for %s: %w", name, err)
		}

		specMap := agent.SyncTypeMap(ag)

		// Detect conflicts using the shared helper.
		conflicts, err := detectConflicts(name, ag, agentDir, types, base[name])
		if err != nil {
			return nil, err
		}
		conflictSet := make(map[string]bool, len(conflicts))
		for _, c := range conflicts {
			conflictSet[c.RelPath] = true
		}

		var safeFiles []agent.SyncFile
		var appendOnlyFiles []agent.SyncFile
		for _, f := range files {
			if strings.HasPrefix(f.StagingPath, "projects/") {
				safeFiles = append(safeFiles, f)
				continue
			}
			syncType := ag.SyncTypeForPath(f.RelPath)
			if agent.LookupMode(specMap, syncType) == agent.SyncModeAppendOnly {
				appendOnlyFiles = append(appendOnlyFiles, f)
				continue
			}
			if conflictSet[f.RelPath] {
				continue
			}
			safeFiles = append(safeFiles, f)
		}

		// Merge append-only files from staging into local.
		for _, f := range appendOnlyFiles {
			src := filepath.Join(agentDir, f.StagingPath)
			dst := filepath.Join(ag.SourceDir(), f.RelPath)
			if err := mergeAppendOnly(src, dst); err != nil {
				return nil, fmt.Errorf("merging append-only %s: %w", f.RelPath, err)
			}
		}

		if len(conflicts) > 0 {
			fmt.Printf("  %s: %d conflict(s) — run 'negent conflicts' to resolve:\n", name, len(conflicts))
			for _, c := range conflicts {
				fmt.Printf("    CONFLICT: %s\n", c.RelPath)
			}
		}

		result, err := ag.Place(agentDir, safeFiles)
		if err != nil {
			return nil, fmt.Errorf("placing files for %s: %w", name, err)
		}

		pullResult.Placed += result.Placed
		pullResult.Merged += len(appendOnlyFiles)
		pullResult.Skipped += result.Skipped
		pullResult.Conflicts = append(pullResult.Conflicts, conflicts...)

		fmt.Printf("  %s: %d placed, %d merged, %d skipped", name, result.Placed, len(appendOnlyFiles), result.Skipped)
		if len(result.Unmatched) > 0 {
			fmt.Printf(", %d unmatched", len(result.Unmatched))
		}
		fmt.Println()
	}

	return &pullResult, nil
}

// mergeAppendOnly merges an append-only file (like session JSONL) from src into
// dst. Lines already present in dst are kept; lines in src not in dst are
// appended. If dst doesn't exist, src is simply copied.
func mergeAppendOnly(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	srcData, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	dstData, err := os.ReadFile(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		dstData = nil
	}

	if len(dstData) == 0 {
		if filepath.Base(dst) == "history.jsonl" || filepath.Base(src) == "history.jsonl" {
			if err := MergeHistoryFiles(dst, src); err != nil {
				return err
			}
			return syncAppendOnlyPair(src, dst)
		}
		return writeBytesAtomic(dst, srcData)
	}

	if filepath.Base(dst) == "history.jsonl" || filepath.Base(src) == "history.jsonl" {
		if err := MergeHistoryFiles(dst, dst, src); err != nil {
			return err
		}
		return syncAppendOnlyPair(src, dst)
	}

	dstLines := splitAppendOnlyLines(dstData)
	existing := make(map[string]struct{}, len(dstLines))
	for _, line := range dstLines {
		existing[line] = struct{}{}
	}

	srcLines := splitAppendOnlyLines(srcData)
	var newLines []string
	for _, line := range srcLines {
		if _, ok := existing[line]; !ok {
			newLines = append(newLines, line)
		}
	}

	if len(newLines) == 0 {
		return nil
	}

	mergedLines := append(dstLines, newLines...)
	return writeBytesAtomic(dst, []byte(strings.Join(mergedLines, "\n")+"\n"))
}

func syncAppendOnlyPair(src, dst string) error {
	merged, err := os.ReadFile(dst)
	if err != nil {
		return err
	}
	if err := writeBytesAtomic(src, merged); err != nil {
		return err
	}
	return nil
}

func splitAppendOnlyLines(data []byte) []string {
	trimmed := strings.TrimRight(string(data), "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

// copyFile copies a file from src to dst, creating parent directories as needed.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	return writeFileAtomic(dst, func(out io.Writer) error {
		return copyToWriter(in, out)
	})
}

func snapshotBase(stagingDir string, agents map[string]agent.Agent, syncTypes map[string][]agent.SyncType) (map[string]map[string][]byte, error) {
	base := make(map[string]map[string][]byte)
	for name := range agents {
		if _, ok := syncTypes[name]; !ok {
			continue
		}
		agentDir := filepath.Join(stagingDir, name)
		if _, err := os.Stat(agentDir); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stating %s staging dir: %w", name, err)
		}
		base[name] = make(map[string][]byte)
		if err := filepath.WalkDir(agentDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			relPath, err := filepath.Rel(agentDir, path)
			if err != nil {
				return fmt.Errorf("computing path relative to agent dir: %w", err)
			}
			relPath = filepath.ToSlash(relPath)
			if strings.HasPrefix(relPath, "projects/") {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("reading staged file %s: %w", relPath, err)
			}
			base[name][relPath] = content
			return nil
		}); err != nil {
			return nil, fmt.Errorf("walking staging dir for %s: %w", name, err)
		}
	}
	return base, nil
}

func splitAgentPath(path string) (agentName string, relPath string, ok bool) {
	parts := strings.SplitN(filepath.ToSlash(path), "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}
