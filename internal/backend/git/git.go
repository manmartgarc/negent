package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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
func DefaultStagingDir() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "negent", "repo")
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

func (g *Git) Pull(ctx context.Context) error {
	return g.run(ctx, g.stagingDir, "pull", "--rebase")
}

func (g *Git) Status(ctx context.Context) ([]backend.FileChange, error) {
	// TODO: parse git status --porcelain for structured output
	return nil, nil
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
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
