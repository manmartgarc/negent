package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/manmart/negent/internal/config"
	syncpkg "github.com/manmart/negent/internal/sync"
)

var conflictsCmd = &cobra.Command{
	Use:   "conflicts",
	Short: "List and resolve sync conflicts",
	RunE:  runConflicts,
}

var (
	conflictsKeepRemote bool
	conflictsList       bool
)

type conflictAction string

const (
	conflictActionKeepLocal  conflictAction = "keep-local"
	conflictActionTakeRemote conflictAction = "take-remote"
	conflictActionSkip       conflictAction = "skip"

	conflictsCommitMessage = "negent conflicts: keep local resolutions"
)

func init() {
	conflictsCmd.Flags().BoolVar(&conflictsKeepRemote, "keep-remote", false, "overwrite all local files with remote versions")
	conflictsCmd.Flags().BoolVar(&conflictsList, "list", false, "list conflicts without resolving")
	rootCmd.AddCommand(conflictsCmd)
}

func runConflicts(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(resolveConfigPath())
	if err != nil {
		return fmt.Errorf("loading config (run 'negent init' first): %w", err)
	}

	be, err := newBackend(cfg)
	if err != nil {
		return err
	}

	agents, syncTypes, err := buildAgents(cfg)
	if err != nil {
		return err
	}

	orch := syncpkg.NewOrchestrator(be, agents)
	conflicts, err := orch.Conflicts(syncTypes)
	if err != nil {
		return fmt.Errorf("detecting conflicts: %w", err)
	}

	if len(conflicts) == 0 {
		fmt.Println("No conflicts.")
		return nil
	}

	if conflictsList {
		for _, c := range conflicts {
			fmt.Printf("  CONFLICT %s:%s\n", c.Agent, c.RelPath)
		}
		return nil
	}

	if conflictsKeepRemote {
		var stagingPaths []string
		for _, c := range conflicts {
			stagingChanged, err := applyConflictAction(c, conflictActionTakeRemote)
			if err != nil {
				return err
			}
			if stagingChanged {
				stagingPaths = append(stagingPaths, c.StagingPath)
			}
		}
		return finalizeConflictResolutions(context.Background(), be.StagingDir(), stagingPaths)
	}

	// Interactive mode.
	stagingPaths, err := resolveInteractive(conflicts)
	if err != nil {
		return err
	}
	return finalizeConflictResolutions(context.Background(), be.StagingDir(), stagingPaths)
}

func resolveInteractive(conflicts []syncpkg.ConflictInfo) ([]string, error) {
	var stagingPaths []string
	for i, c := range conflicts {
		fmt.Printf("\n[%d/%d] %s:%s\n", i+1, len(conflicts), c.Agent, c.RelPath)
		for {
			var choice string
			err := huh.NewSelect[string]().
				Title("Resolve conflict").
				Options(
					huh.NewOption("Keep local", "l"),
					huh.NewOption("Take remote", "r"),
					huh.NewOption("Show diff", "d"),
					huh.NewOption("Skip", "s"),
					huh.NewOption("Quit", "q"),
				).
				Value(&choice).
				Run()
			if err != nil {
				return nil, err
			}
			switch choice {
			case "l":
				stagingChanged, err := applyConflictAction(c, conflictActionKeepLocal)
				if err != nil {
					return nil, err
				}
				if stagingChanged {
					stagingPaths = append(stagingPaths, c.StagingPath)
				}
			case "r":
				stagingChanged, err := applyConflictAction(c, conflictActionTakeRemote)
				if err != nil {
					return nil, err
				}
				if stagingChanged {
					stagingPaths = append(stagingPaths, c.StagingPath)
				}
			case "d":
				printDiff(c.LocalPath, c.StagingPath)
				continue
			case "s":
				stagingChanged, err := applyConflictAction(c, conflictActionSkip)
				if err != nil {
					return nil, err
				}
				if stagingChanged {
					stagingPaths = append(stagingPaths, c.StagingPath)
				}
			case "q":
				return stagingPaths, nil
			}
			break
		}
	}
	return stagingPaths, nil
}

func applyConflictAction(c syncpkg.ConflictInfo, action conflictAction) (bool, error) {
	switch action {
	case conflictActionKeepLocal:
		if err := copyFileFromTo(c.LocalPath, c.StagingPath); err != nil {
			return false, fmt.Errorf("recording local for %s: %w", c.RelPath, err)
		}
		fmt.Printf("  kept local: %s:%s\n", c.Agent, c.RelPath)
		return true, nil
	case conflictActionTakeRemote:
		if err := copyFileFromTo(c.StagingPath, c.LocalPath); err != nil {
			return false, fmt.Errorf("overwriting %s: %w", c.RelPath, err)
		}
		fmt.Printf("  took remote: %s:%s\n", c.Agent, c.RelPath)
		return false, nil
	case conflictActionSkip:
		fmt.Printf("  skipped: %s:%s\n", c.Agent, c.RelPath)
		return false, nil
	default:
		return false, fmt.Errorf("unknown conflict action: %q", action)
	}
}

func finalizeConflictResolutions(ctx context.Context, stagingDir string, stagingPaths []string) error {
	committed, err := commitConflictResolutions(ctx, stagingDir, stagingPaths)
	if err != nil {
		return err
	}
	if committed {
		fmt.Println("✓ Recorded keep-local resolution(s) in staging repo")
	}
	return nil
}

func commitConflictResolutions(ctx context.Context, stagingDir string, stagingPaths []string) (bool, error) {
	relPaths, err := uniqueRelativePaths(stagingDir, stagingPaths)
	if err != nil {
		return false, err
	}
	if len(relPaths) == 0 {
		return false, nil
	}

	addArgs := append([]string{"add", "--"}, relPaths...)
	if _, err := runGit(ctx, stagingDir, addArgs...); err != nil {
		return false, fmt.Errorf("staging conflict resolutions: %w", err)
	}

	diffArgs := append([]string{"diff", "--cached", "--name-only", "--"}, relPaths...)
	out, err := runGit(ctx, stagingDir, diffArgs...)
	if err != nil {
		return false, fmt.Errorf("checking staged conflict resolutions: %w", err)
	}
	if strings.TrimSpace(string(out)) == "" {
		return false, nil
	}

	if _, err := runGit(ctx, stagingDir, "commit", "-m", conflictsCommitMessage); err != nil {
		return false, fmt.Errorf("committing conflict resolutions: %w", err)
	}
	return true, nil
}

func uniqueRelativePaths(stagingDir string, paths []string) ([]string, error) {
	set := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		if strings.TrimSpace(p) == "" {
			continue
		}
		rel := p
		if filepath.IsAbs(p) {
			candidate, err := filepath.Rel(stagingDir, p)
			if err != nil {
				return nil, fmt.Errorf("computing path relative to staging dir: %w", err)
			}
			rel = candidate
		}
		rel = filepath.ToSlash(rel)
		if rel == ".." || strings.HasPrefix(rel, "../") {
			return nil, fmt.Errorf("path %q is outside staging dir %q", p, stagingDir)
		}
		set[rel] = struct{}{}
	}

	relPaths := make([]string, 0, len(set))
	for rel := range set {
		relPaths = append(relPaths, rel)
	}
	sort.Strings(relPaths)
	return relPaths, nil
}

var runGit = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return out, nil
}

func printDiff(localPath, stagingPath string) {
	// Use unified diff via the system diff command for readable output.
	cmd := exec.Command("diff", "-u", "--label", "local", "--label", "remote", localPath, stagingPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			fmt.Fprintf(os.Stderr, "running diff: %v\n", err)
		}
	}
}

func copyFileFromTo(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode())
}
