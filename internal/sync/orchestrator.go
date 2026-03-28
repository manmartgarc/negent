package sync

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
		if err := os.MkdirAll(agentDir, 0o755); err != nil {
			return fmt.Errorf("creating agent dir %s: %w", agentDir, err)
		}

		for _, f := range files {
			src := filepath.Join(ag.SourceDir(), f.RelPath)
			dst := filepath.Join(agentDir, f.StagingPath)

			if err := copyFile(src, dst); err != nil {
				return fmt.Errorf("copying %s: %w", f.RelPath, err)
			}
		}

		fmt.Printf("  %s: %d files collected\n", name, len(files))
	}

	msg := fmt.Sprintf("negent push %s", time.Now().Format(time.RFC3339))
	return o.backend.Push(ctx, msg)
}

// Pull fetches from the backend and places files into agent source directories.
func (o *Orchestrator) Pull(ctx context.Context, categories map[string][]agent.Category) error {
	if err := o.backend.Pull(ctx); err != nil {
		return fmt.Errorf("pulling from backend: %w", err)
	}

	stagingDir := o.backend.StagingDir()

	for name, ag := range o.agents {
		if _, ok := categories[name]; !ok {
			continue
		}

		agentDir := filepath.Join(stagingDir, name)
		if _, err := os.Stat(agentDir); os.IsNotExist(err) {
			fmt.Printf("  %s: no data in remote\n", name)
			continue
		}

		// Collect all files from the agent's staging namespace
		var files []agent.SyncFile
		err := filepath.WalkDir(agentDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			relPath, _ := filepath.Rel(agentDir, path)
			files = append(files, agent.SyncFile{
				RelPath:     relPath,
				StagingPath: relPath,
			})
			return nil
		})
		if err != nil {
			return fmt.Errorf("walking staging dir for %s: %w", name, err)
		}

		result, err := ag.Place(agentDir, files)
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
