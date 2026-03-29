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

// NewOrchestrator creates a new sync orchestrator.
func NewOrchestrator(be backend.Backend, agents map[string]agent.Agent) *Orchestrator {
	return &Orchestrator{
		backend: be,
		agents:  agents,
	}
}

// Push collects files from all configured agents and pushes them to the backend.
func (o *Orchestrator) Push(ctx context.Context, categories map[string][]agent.Category) error {
	stagingDir := o.backend.StagingDir()

	for name, ag := range o.agents {
		cats, ok := categories[name]
		if !ok {
			continue
		}

		files, err := ag.Collect(cats)
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

		for _, f := range files {
			src := filepath.Join(ag.SourceDir(), f.RelPath)
			dst := filepath.Join(agentDir, f.StagingPath)

			if f.Category == agent.CategorySessions {
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

		catSet := make(map[agent.Category]bool, len(cats))
		for _, c := range cats {
			catSet[c] = true
		}

		deleted := 0
		filepath.WalkDir(agentDir, func(path string, d os.DirEntry, err error) error { //nolint:errcheck
			if err != nil || d.IsDir() {
				return err
			}
			relPath, _ := filepath.Rel(agentDir, path)
			relPath = filepath.ToSlash(relPath)
			if collectedPaths[relPath] {
				return nil
			}
			cat := ag.CategorizePath(relPath)
			if cat != "" && !catSet[cat] {
				return nil
			}
			if cat == agent.CategorySessions {
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
		})

		fmt.Printf("  %s: %d files collected", name, len(files))
		if deleted > 0 {
			fmt.Printf(", %d deleted", deleted)
		}
		fmt.Println()
	}

	msg := fmt.Sprintf("negent push %s", time.Now().Format(time.RFC3339))
	return o.backend.Push(ctx, msg)
}

// Pull fetches from the backend and places files into agent source directories.
// Conflict detection: any non-project file where local differs from the staged
// base (the last synced remote state) is skipped and reported. This protects
// local edits regardless of whether the remote also changed the file.
func (o *Orchestrator) Pull(ctx context.Context, categories map[string][]agent.Category) error {
	stagingDir := o.backend.StagingDir()

	// Snapshot staged content for non-project files before the working tree is
	// updated. This is the base: the last remote state this machine synced to.
	// Project dirs use path-encoded names; Place() handles matching for those.
	base := make(map[string]map[string][]byte) // agent -> stagingRelPath -> bytes
	for name := range o.agents {
		if _, ok := categories[name]; !ok {
			continue
		}
		agentDir := filepath.Join(stagingDir, name)
		base[name] = make(map[string][]byte)
		filepath.WalkDir(agentDir, func(path string, d os.DirEntry, err error) error { //nolint:errcheck
			if err != nil || d.IsDir() {
				return err
			}
			relPath, _ := filepath.Rel(agentDir, path)
			relPath = filepath.ToSlash(relPath)
			if strings.HasPrefix(relPath, "projects/") {
				return nil
			}
			if content, err := os.ReadFile(path); err == nil {
				base[name][relPath] = content
			}
			return nil
		})
	}

	// Integrate remote changes into the working tree.
	if err := o.backend.Pull(ctx); err != nil {
		return fmt.Errorf("pulling from backend: %w", err)
	}

	for name, ag := range o.agents {
		cats, ok := categories[name]
		if !ok {
			continue
		}

		agentDir := filepath.Join(stagingDir, name)
		if _, err := os.Stat(agentDir); os.IsNotExist(err) {
			fmt.Printf("  %s: no data in remote\n", name)
			continue
		}

		catSet := make(map[agent.Category]bool, len(cats))
		for _, c := range cats {
			catSet[c] = true
		}

		// Walk staging (now at remote HEAD) to build the file list,
		// filtering to only files belonging to enabled categories.
		var files []agent.SyncFile
		err := filepath.WalkDir(agentDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			relPath, _ := filepath.Rel(agentDir, path)
			relPath = filepath.ToSlash(relPath)
			if cat := ag.CategorizePath(relPath); cat != "" && !catSet[cat] {
				return nil
			}
			files = append(files, agent.SyncFile{
				RelPath:     relPath,
				StagingPath: relPath,
			})
			return nil
		})
		if err != nil {
			return fmt.Errorf("walking staging dir for %s: %w", name, err)
		}

		agentBase := base[name]
		var safeFiles []agent.SyncFile
		var conflicts []string

		for _, f := range files {
			// Project files: pass through to Place() for path-encoded matching.
			if strings.HasPrefix(f.StagingPath, "projects/") {
				safeFiles = append(safeFiles, f)
				continue
			}

			localContent, err := os.ReadFile(filepath.Join(ag.SourceDir(), f.RelPath))
			if err != nil {
				// Local file absent — new file from remote; safe to place.
				safeFiles = append(safeFiles, f)
				continue
			}

			baseContent, hasBase := agentBase[f.StagingPath]
			if !hasBase {
				// No prior base but local exists — remote added a file that
				// collides with a local file. Protect local to avoid silent loss.
				conflicts = append(conflicts, f.StagingPath)
				continue
			}

			if bytes.Equal(localContent, baseContent) {
				// Local matches base — no local edits; safe to accept remote version.
				safeFiles = append(safeFiles, f)
			} else {
				// Local differs from base — user has unsaved local changes; protect.
				conflicts = append(conflicts, f.StagingPath)
			}
		}

		if len(conflicts) > 0 {
			fmt.Printf("  %s: %d conflict(s) — keeping local version:\n", name, len(conflicts))
			for _, c := range conflicts {
				fmt.Printf("    CONFLICT: %s\n", c)
			}
		}

		result, err := ag.Place(agentDir, safeFiles)
		if err != nil {
			return fmt.Errorf("placing files for %s: %w", name, err)
		}

		fmt.Printf("  %s: %d placed, %d skipped", name, result.Placed, result.Skipped)
		if len(result.Unmatched) > 0 {
			fmt.Printf(", %d unmatched", len(result.Unmatched))
		}
		fmt.Println()
	}

	return nil
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

	dstData, _ := os.ReadFile(dst) // ignore error: dst may not exist yet

	if len(dstData) == 0 {
		return os.WriteFile(dst, srcData, 0o644)
	}

	dstLines := strings.Split(strings.TrimRight(string(dstData), "\n"), "\n")
	existing := make(map[string]struct{}, len(dstLines))
	for _, line := range dstLines {
		existing[line] = struct{}{}
	}

	srcLines := strings.Split(strings.TrimRight(string(srcData), "\n"), "\n")
	var newLines []string
	for _, line := range srcLines {
		if _, ok := existing[line]; !ok {
			newLines = append(newLines, line)
		}
	}

	if len(newLines) == 0 {
		return nil
	}

	f, err := os.OpenFile(dst, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(strings.Join(newLines, "\n") + "\n")
	return err
}

// copyFile copies a file from src to dst, creating parent directories as needed.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
