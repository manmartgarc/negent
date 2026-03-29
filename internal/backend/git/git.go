package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/manmart/negent/internal/backend"
)

const BackendName = "git"

// Git implements backend.Backend using a local git clone.
type Git struct {
	remote     string
	stagingDir string
}

// New creates a new Git backend. The stagingDir is where the repo will be
// cloned to (typically ~/.local/share/negent/repo).
func New(remote, stagingDir string) *Git {
	return &Git{
		remote:     remote,
		stagingDir: stagingDir,
	}
}

// DefaultStagingDir returns the default location for the git working copy.
// On Linux: $XDG_DATA_HOME/negent/repo or ~/.local/share/negent/repo
// On Windows: %LOCALAPPDATA%\negent\repo
// On macOS: ~/Library/Application Support/negent/repo
func DefaultStagingDir() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "negent", "repo")
	}
	switch runtime.GOOS {
	case "windows":
		if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
			return filepath.Join(dir, "negent", "repo")
		}
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "Library", "Application Support", "negent", "repo")
		}
	}
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
	if err := g.run(ctx, g.stagingDir, "add", "-A"); err != nil {
		return err
	}

	// Check if there's anything to commit
	if err := g.run(ctx, g.stagingDir, "diff", "--cached", "--quiet"); err == nil {
		return nil // nothing to commit
	}

	if err := g.run(ctx, g.stagingDir, "commit", "-m", msg); err != nil {
		return err
	}
	return g.run(ctx, g.stagingDir, "push")
}

func (g *Git) Fetch(ctx context.Context) error {
	// If the repo has no commits yet there is nothing to fetch.
	if err := g.run(ctx, g.stagingDir, "rev-parse", "HEAD"); err != nil {
		return nil
	}
	return g.run(ctx, g.stagingDir, "fetch", "origin")
}

func (g *Git) FetchedFiles(ctx context.Context) ([]string, error) {
	// If no fetch has been done, FETCH_HEAD won't exist.
	fetchHeadPath := filepath.Join(g.stagingDir, ".git", "FETCH_HEAD")
	if _, err := os.Stat(fetchHeadPath); os.IsNotExist(err) {
		return nil, nil
	}
	cmd := exec.CommandContext(ctx, "git", "diff", "--name-only", "HEAD", "FETCH_HEAD")
	cmd.Dir = g.stagingDir
	out, err := cmd.Output()
	if err != nil {
		// Gracefully handle empty repo or identical refs.
		return nil, nil
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

func (g *Git) Pull(ctx context.Context) error {
	// Check if there are any commits in the remote; if not, skip pull.
	if err := g.run(ctx, g.stagingDir, "rev-parse", "HEAD"); err != nil {
		return nil // empty repo, nothing to pull
	}
	return g.run(ctx, g.stagingDir, "pull", "--rebase")
}

func (g *Git) Status(ctx context.Context) ([]backend.FileChange, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = g.stagingDir
	out, err := cmd.Output()
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

// run executes a git command with the given arguments.
func (g *Git) run(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return nil
}
