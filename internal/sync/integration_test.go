package sync

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/manmart/negent/internal/agent"
	agentclaude "github.com/manmart/negent/internal/agent/claude"
	gitbackend "github.com/manmart/negent/internal/backend/git"
)

// initBareRepo creates a bare git repo for use as a test remote.
func initBareRepo(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "remote.git")
	cmd := exec.Command("git", "init", "--bare", dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	return dir
}

// setupClaudeDir creates a temp Claude source dir with realistic structure.
func setupClaudeDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Global config files
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("# My rules"), 0o644)
	os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"key":"val"}`), 0o644)

	// Custom code
	os.MkdirAll(filepath.Join(dir, "commands"), 0o755)
	os.WriteFile(filepath.Join(dir, "commands", "deploy.md"), []byte("deploy steps"), 0o644)

	// Project with memory
	projDir := filepath.Join(dir, "projects", "-home-user-repos-myapp")
	os.MkdirAll(filepath.Join(projDir, "memory"), 0o755)
	os.WriteFile(filepath.Join(projDir, "memory", "MEMORY.md"), []byte("- remember X"), 0o644)

	return dir
}

func TestIntegrationPushPull(t *testing.T) {
	remote := initBareRepo(t)
	staging1 := filepath.Join(t.TempDir(), "staging1")
	staging2 := filepath.Join(t.TempDir(), "staging2")
	ctx := context.Background()

	// Machine A: set up source dir and push
	srcA := setupClaudeDir(t)
	beA := gitbackend.New(remote, staging1)
	if err := beA.Init(ctx, nil); err != nil {
		t.Fatalf("backend init A: %v", err)
	}

	agA := agentclaude.New(srcA)
	orchA := NewOrchestrator(beA, map[string]agent.Agent{"claude": agA})

	cats := map[string][]agent.Category{
		"claude": {agent.CategoryConfig, agent.CategoryCustomCode, agent.CategoryMemory},
	}

	if err := orchA.Push(ctx, cats); err != nil {
		t.Fatalf("Push A: %v", err)
	}

	// Verify files in staging
	if _, err := os.Stat(filepath.Join(staging1, "claude", "CLAUDE.md")); err != nil {
		t.Error("CLAUDE.md not staged after push")
	}
	if _, err := os.Stat(filepath.Join(staging1, "claude", "commands", "deploy.md")); err != nil {
		t.Error("commands/deploy.md not staged after push")
	}

	// Machine B: pull into a fresh Claude dir
	srcB := t.TempDir()
	// Create the same project dir so Place() can match
	os.MkdirAll(filepath.Join(srcB, "projects", "-home-user-repos-myapp"), 0o755)

	beB := gitbackend.New(remote, staging2)
	if err := beB.Init(ctx, nil); err != nil {
		t.Fatalf("backend init B: %v", err)
	}

	agB := agentclaude.New(srcB)
	orchB := NewOrchestrator(beB, map[string]agent.Agent{"claude": agB})

	if err := orchB.Pull(ctx, cats); err != nil {
		t.Fatalf("Pull B: %v", err)
	}

	// Verify non-project files were placed
	data, err := os.ReadFile(filepath.Join(srcB, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("reading CLAUDE.md on machine B: %v", err)
	}
	if string(data) != "# My rules" {
		t.Errorf("CLAUDE.md = %q, want %q", data, "# My rules")
	}

	data, err = os.ReadFile(filepath.Join(srcB, "commands", "deploy.md"))
	if err != nil {
		t.Fatalf("reading deploy.md on machine B: %v", err)
	}
	if string(data) != "deploy steps" {
		t.Errorf("deploy.md = %q, want %q", data, "deploy steps")
	}

	// Verify project memory was placed (exact match on dir name)
	data, err = os.ReadFile(filepath.Join(srcB, "projects", "-home-user-repos-myapp", "memory", "MEMORY.md"))
	if err != nil {
		t.Fatalf("reading project memory on machine B: %v", err)
	}
	if string(data) != "- remember X" {
		t.Errorf("memory = %q, want %q", data, "- remember X")
	}
}

func TestIntegrationPushIdempotent(t *testing.T) {
	remote := initBareRepo(t)
	staging := filepath.Join(t.TempDir(), "staging")
	ctx := context.Background()

	src := setupClaudeDir(t)
	be := gitbackend.New(remote, staging)
	if err := be.Init(ctx, nil); err != nil {
		t.Fatalf("backend init: %v", err)
	}

	ag := agentclaude.New(src)
	orch := NewOrchestrator(be, map[string]agent.Agent{"claude": ag})
	cats := map[string][]agent.Category{
		"claude": {agent.CategoryConfig, agent.CategoryCustomCode, agent.CategoryMemory},
	}

	// Push twice — second should be a no-op (no new commit)
	if err := orch.Push(ctx, cats); err != nil {
		t.Fatalf("Push 1: %v", err)
	}
	if err := orch.Push(ctx, cats); err != nil {
		t.Fatalf("Push 2: %v", err)
	}
}

func TestIntegrationConflictDetection(t *testing.T) {
	remote := initBareRepo(t)
	staging1 := filepath.Join(t.TempDir(), "staging1")
	staging2 := filepath.Join(t.TempDir(), "staging2")
	ctx := context.Background()

	cats := map[string][]agent.Category{
		"claude": {agent.CategoryConfig},
	}

	// Machine A: push initial content.
	srcA := t.TempDir()
	os.WriteFile(filepath.Join(srcA, "CLAUDE.md"), []byte("initial"), 0o644)
	beA := gitbackend.New(remote, staging1)
	if err := beA.Init(ctx, nil); err != nil {
		t.Fatalf("beA Init: %v", err)
	}
	agA := agentclaude.New(srcA)
	orchA := NewOrchestrator(beA, map[string]agent.Agent{"claude": agA})
	if err := orchA.Push(ctx, cats); err != nil {
		t.Fatalf("Push A initial: %v", err)
	}

	// Machine B: pull the initial content.
	srcB := t.TempDir()
	beB := gitbackend.New(remote, staging2)
	if err := beB.Init(ctx, nil); err != nil {
		t.Fatalf("beB Init: %v", err)
	}
	agB := agentclaude.New(srcB)
	orchB := NewOrchestrator(beB, map[string]agent.Agent{"claude": agB})
	if err := orchB.Pull(ctx, cats); err != nil {
		t.Fatalf("Pull B initial: %v", err)
	}
	// Verify initial placement.
	data, err := os.ReadFile(filepath.Join(srcB, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("reading CLAUDE.md on B after initial pull: %v", err)
	}
	if string(data) != "initial" {
		t.Fatalf("expected 'initial', got %q", data)
	}

	// Machine B: edit CLAUDE.md locally (diverges from staging base = "initial").
	os.WriteFile(filepath.Join(srcB, "CLAUDE.md"), []byte("B's local changes"), 0o644)

	// Machine A: push a different update to CLAUDE.md.
	os.WriteFile(filepath.Join(srcA, "CLAUDE.md"), []byte("A's update"), 0o644)
	if err := orchA.Push(ctx, cats); err != nil {
		t.Fatalf("Push A update: %v", err)
	}

	// Machine B: pull — must detect the conflict and preserve local content.
	if err := orchB.Pull(ctx, cats); err != nil {
		t.Fatalf("Pull B conflict: %v", err)
	}

	data, err = os.ReadFile(filepath.Join(srcB, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("reading CLAUDE.md on B after conflict pull: %v", err)
	}
	if string(data) != "B's local changes" {
		t.Errorf("CLAUDE.md = %q, want %q (local must be preserved on conflict)", data, "B's local changes")
	}
}

func TestIntegrationPullEmptyRemote(t *testing.T) {
	remote := initBareRepo(t)
	staging := filepath.Join(t.TempDir(), "staging")
	ctx := context.Background()

	src := t.TempDir()
	be := gitbackend.New(remote, staging)
	if err := be.Init(ctx, nil); err != nil {
		t.Fatalf("backend init: %v", err)
	}

	ag := agentclaude.New(src)
	orch := NewOrchestrator(be, map[string]agent.Agent{"claude": ag})
	cats := map[string][]agent.Category{
		"claude": {agent.CategoryConfig},
	}

	// Pull from empty remote should succeed
	if err := orch.Pull(ctx, cats); err != nil {
		t.Fatalf("Pull empty: %v", err)
	}
}
