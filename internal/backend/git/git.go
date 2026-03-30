package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/manmart/negent/internal/backend"
)

const BackendName = "git"

// ConflictError indicates git reported a content conflict while rebasing.
type ConflictError struct {
	Command string
	Files   []string
	Err     error
}

func (e *ConflictError) Error() string {
	if len(e.Files) == 0 {
		return fmt.Sprintf("git %s: conflict", e.Command)
	}
	return fmt.Sprintf("git %s: conflict in %s", e.Command, strings.Join(e.Files, ", "))
}

func (e *ConflictError) Unwrap() error { return e.Err }

// runnerFn executes a git subcommand in dir and returns combined output.
// Injected into Git so tests can verify outgoing commands without running git.
type runnerFn func(ctx context.Context, dir string, args ...string) ([]byte, error)

// Git implements backend.Backend using a local git clone.
type Git struct {
	remote     string
	stagingDir string
	runner     runnerFn
}

// New creates a new Git backend. The stagingDir is where the repo will be
// cloned to (typically ~/.local/share/negent/repo).
func New(remote, stagingDir string) *Git {
	return &Git{
		remote:     remote,
		stagingDir: stagingDir,
		runner:     gitExec,
	}
}

// gitExec is the production runner that shells out to the git CLI.
func gitExec(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return out, nil
}

// DefaultStagingDir returns the default location for the git working copy.
// On Linux: $XDG_DATA_HOME/negent/repo or ~/.local/share/negent/repo
// On macOS: ~/Library/Application Support/negent/repo
func DefaultStagingDir() string {
	// Prefer XDG_DATA_HOME if set (common on Linux, works on macOS too)
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "negent", "repo")
	}
	// Use os.UserConfigDir for platform-appropriate fallback
	if dir, err := os.UserConfigDir(); err == nil {
		// On macOS: ~/Library/Application Support
		// On Linux: ~/.config (but we prefer .local/share for data)
		if strings.Contains(dir, "Library") {
			return filepath.Join(dir, "negent", "repo")
		}
	}
	// Default: ~/.local/share/negent/repo (Linux standard)
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "negent", "repo")
}

func (g *Git) Init(ctx context.Context, cfg backend.BackendConfig) error {
	if remote, ok := cfg["remote"]; ok {
		g.remote = remote
	}
	if g.remote == "" {
		return fmt.Errorf("git backend requires a remote URL")
	}

	// Clone if staging dir doesn't exist yet
	if _, err := os.Stat(filepath.Join(g.stagingDir, ".git")); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(g.stagingDir), 0o755); err != nil {
			return fmt.Errorf("creating staging dir parent: %w", err)
		}
		return g.run(ctx, "", "clone", g.remote, g.stagingDir)
	}

	// Already cloned — pull latest
	return g.Pull(ctx)
}

func (g *Git) Push(ctx context.Context, msg string) error {
	// Safety-first flow: always rebase on top of latest remote before mutating.
	if err := g.Pull(ctx); err != nil {
		return fmt.Errorf("pre-push pull failed: %w", err)
	}

	if err := g.run(ctx, g.stagingDir, "add", "-A"); err != nil {
		return err
	}

	// Check if there's anything to commit
	if err := g.run(ctx, g.stagingDir, "diff", "--cached", "--quiet"); err != nil {
		if err := g.run(ctx, g.stagingDir, "commit", "-m", msg); err != nil {
			return err
		}
	}

	// Always attempt push, including when there was nothing new to commit
	// but previous local commits are still pending.
	if err := g.run(ctx, g.stagingDir, "push"); err != nil {
		// Remote may have advanced after pre-pull; retry once.
		if pullErr := g.Pull(ctx); pullErr != nil {
			return fmt.Errorf("push rejected and retry pull failed: %w", pullErr)
		}
		if retryErr := g.run(ctx, g.stagingDir, "push"); retryErr != nil {
			return fmt.Errorf("push failed after retry: %w", retryErr)
		}
	}
	return nil
}

func (g *Git) Fetch(ctx context.Context) error {
	// If the repo has no commits yet there is nothing to fetch.
	if err := g.run(ctx, g.stagingDir, "rev-parse", "HEAD"); err != nil {
		return nil
	}
	return g.run(ctx, g.stagingDir, "fetch", "origin")
}

func (g *Git) FetchedFiles(ctx context.Context) ([]backend.FileChange, error) {
	// If no fetch has been done, FETCH_HEAD won't exist.
	fetchHeadPath := filepath.Join(g.stagingDir, ".git", "FETCH_HEAD")
	if _, err := os.Stat(fetchHeadPath); os.IsNotExist(err) {
		return nil, nil
	}
	out, err := g.runner(ctx, g.stagingDir, "diff", "--name-status", "HEAD", "FETCH_HEAD")
	if err != nil {
		// Gracefully handle empty repo or identical refs.
		return nil, nil
	}
	var changes []backend.FileChange
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		kind := backend.ChangeModified
		switch fields[0] {
		case "A":
			kind = backend.ChangeAdded
		case "D":
			kind = backend.ChangeDeleted
		}
		changes = append(changes, backend.FileChange{
			Path: fields[len(fields)-1],
			Kind: kind,
		})
	}
	return changes, nil
}

func (g *Git) Pull(ctx context.Context) error {
	// Check if there are any commits in the remote; if not, skip pull.
	if err := g.run(ctx, g.stagingDir, "rev-parse", "HEAD"); err != nil {
		return nil // empty repo, nothing to pull
	}
	// If a previous rebase was interrupted (e.g. crash, signal), complete or
	// abort it before attempting a new pull.
	rebaseMerge := filepath.Join(g.stagingDir, ".git", "rebase-merge")
	rebaseApply := filepath.Join(g.stagingDir, ".git", "rebase-apply")
	_, mergeInProgress := os.Stat(rebaseMerge)
	_, applyInProgress := os.Stat(rebaseApply)
	if mergeInProgress == nil || applyInProgress == nil {
		// Try to continue first; if that fails, abort so we're on a clean branch.
		if err := g.run(ctx, g.stagingDir, "rebase", "--continue"); err != nil {
			_ = g.run(ctx, g.stagingDir, "rebase", "--abort")
		}
	}
	// Keep linear history and rely on git's default conflict behavior.
	out, err := g.runner(ctx, g.stagingDir, "pull", "--rebase")
	if err != nil {
		conflicts, conflictsErr := g.conflictedFiles(ctx)
		// Leave staging clean on conflict so future commands can proceed.
		_ = g.run(ctx, g.stagingDir, "rebase", "--abort")
		if (conflictsErr == nil && len(conflicts) > 0) || looksLikeConflict(out, err) {
			return &ConflictError{
				Command: "pull --rebase",
				Files:   conflicts,
				Err:     err,
			}
		}
		return err
	}
	return nil
}

func (g *Git) Status(ctx context.Context) ([]backend.FileChange, error) {
	out, err := g.runner(ctx, g.stagingDir, "status", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("git status: %w", err)
	}

	var changes []backend.FileChange
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 4 {
			continue
		}
		status := line[:2]
		path := strings.TrimSpace(line[3:])

		var kind backend.ChangeKind
		switch {
		case strings.Contains(status, "D"):
			kind = backend.ChangeDeleted
		case strings.Contains(status, "?"):
			kind = backend.ChangeAdded
		case strings.Contains(status, "A"):
			kind = backend.ChangeAdded
		default:
			kind = backend.ChangeModified
		}
		changes = append(changes, backend.FileChange{Path: path, Kind: kind})
	}

	return changes, nil
}

func (g *Git) StagingDir() string {
	return g.stagingDir
}

// run executes a git subcommand, discarding output bytes.
func (g *Git) run(ctx context.Context, dir string, args ...string) error {
	_, err := g.runner(ctx, dir, args...)
	return err
}

func (g *Git) conflictedFiles(ctx context.Context) ([]string, error) {
	out, err := g.runner(ctx, g.stagingDir, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(out)) == "" {
		return nil, nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	files := make([]string, 0, len(lines))
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			files = append(files, trimmed)
		}
	}
	sort.Strings(files)
	return files, nil
}

func looksLikeConflict(out []byte, err error) bool {
	msg := strings.ToLower(string(out))
	if err != nil {
		msg += "\n" + strings.ToLower(err.Error())
	}
	return strings.Contains(msg, "conflict") || strings.Contains(msg, "could not apply")
}
