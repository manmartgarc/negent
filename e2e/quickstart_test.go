package e2e_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/manmart/negent/internal/config"
)

func TestQuickstartHappyPath(t *testing.T) {
	remote := newBareRemote(t)
	machineA := newMachine(t, "machine-a")
	machineB := newMachine(t, "machine-b")

	const claudeContents = "# Shared instructions\n"
	const commandContents = "Review carefully.\n"

	machineA.SeedAgentFile(t, "claude", "CLAUDE.md", claudeContents)
	machineA.SeedAgentFile(t, "claude", "commands/review.md", commandContents)

	requireCommandSuccess(t, "machine A init", runNegent(t, machineA, "init",
		"--non-interactive",
		"--backend", "git",
		"--repo", remote,
		"--machine", machineA.Name,
		"--agent", "claude",
	))
	assertConfigForMachine(t, machineA, remote)
	assertStagingRepoExists(t, machineA)

	requireCommandSuccess(t, "machine A push", runNegent(t, machineA, "push", "--quiet"))

	remoteFiles := gitOutput(t, testHarness.repoRoot, "--git-dir", remote, "ls-tree", "-r", "--name-only", "HEAD")
	requireContains(t, remoteFiles, "claude/CLAUDE.md")
	requireContains(t, remoteFiles, "claude/commands/review.md")
	requireFileContents(t, filepath.Join(machineA.StagingDir(), "claude", "CLAUDE.md"), claudeContents)
	requireFileContents(t, filepath.Join(machineA.StagingDir(), "claude", "commands", "review.md"), commandContents)

	requireCommandSuccess(t, "machine B init", runNegent(t, machineB, "init",
		"--non-interactive",
		"--backend", "git",
		"--repo", remote,
		"--machine", machineB.Name,
		"--agent", "claude",
	))
	assertConfigForMachine(t, machineB, remote)
	assertStagingRepoExists(t, machineB)

	requireCommandSuccess(t, "machine B pull", runNegent(t, machineB, "pull", "--quiet"))

	requireFileContents(t, filepath.Join(machineB.AgentHome("claude"), "CLAUDE.md"), claudeContents)
	requireFileContents(t, filepath.Join(machineB.AgentHome("claude"), "commands", "review.md"), commandContents)
	requireFileContents(t, filepath.Join(machineB.StagingDir(), "claude", "CLAUDE.md"), claudeContents)
	requireFileContents(t, filepath.Join(machineB.StagingDir(), "claude", "commands", "review.md"), commandContents)
}

func TestPreviewStatusAndDiff(t *testing.T) {
	remote := newBareRemote(t)
	machineA := newMachine(t, "machine-a")
	machineB := newMachine(t, "machine-b")

	machineA.SeedAgentFile(t, "claude", "CLAUDE.md", "# Base instructions\n")

	requireCommandSuccess(t, "machine A init", runNegent(t, machineA, "init",
		"--non-interactive",
		"--backend", "git",
		"--repo", remote,
		"--machine", machineA.Name,
		"--agent", "claude",
	))
	requireCommandSuccess(t, "machine A initial push", runNegent(t, machineA, "push", "--quiet"))

	requireCommandSuccess(t, "machine B init", runNegent(t, machineB, "init",
		"--non-interactive",
		"--backend", "git",
		"--repo", remote,
		"--machine", machineB.Name,
		"--agent", "claude",
	))
	requireCommandSuccess(t, "machine B initial pull", runNegent(t, machineB, "pull", "--quiet"))

	localOnlyPath := filepath.Join(machineB.AgentHome("claude"), "commands", "local.md")
	writeFile(t, localOnlyPath, "Local preview only.\n")

	pushPreview := runNegent(t, machineB, "push", "--dry-run")
	requireCommandSuccess(t, "push --dry-run", pushPreview)
	requireContains(t, pushPreview.Stdout, "Pending sync changes:")
	requireContains(t, pushPreview.Stdout, "Push preview:")
	requireContains(t, pushPreview.Stdout, "upload")
	requireContains(t, pushPreview.Stdout, "claude:commands/local.md")
	requireNoFile(t, filepath.Join(machineB.StagingDir(), "claude", "commands", "local.md"))

	commitCountBeforePreview := gitOutput(t, testHarness.repoRoot, "--git-dir", remote, "rev-list", "--count", "--all")
	if commitCountBeforePreview != "1" {
		t.Fatalf("remote commit count after push preview = %q, want 1", commitCountBeforePreview)
	}

	machineA.SeedAgentFile(t, "claude", "rules/remote.md", "Remote preview only.\n")
	requireCommandSuccess(t, "machine A second push", runNegent(t, machineA, "push", "--quiet"))

	commitCountAfterRemotePush := gitOutput(t, testHarness.repoRoot, "--git-dir", remote, "rev-list", "--count", "--all")
	if commitCountAfterRemotePush != "2" {
		t.Fatalf("remote commit count after second push = %q, want 2", commitCountAfterRemotePush)
	}

	pullPreview := runNegent(t, machineB, "pull", "--dry-run")
	requireCommandSuccess(t, "pull --dry-run", pullPreview)
	requireContains(t, pullPreview.Stdout, "Pending sync changes:")
	requireContains(t, pullPreview.Stdout, "Pull preview:")
	requireContains(t, pullPreview.Stdout, "download")
	requireContains(t, pullPreview.Stdout, "claude:rules/remote.md")
	requireNoFile(t, filepath.Join(machineB.AgentHome("claude"), "rules", "remote.md"))
	requireNoFile(t, filepath.Join(machineB.StagingDir(), "claude", "rules", "remote.md"))

	statusResult := runNegent(t, machineB, "status")
	requireCommandSuccess(t, "status", statusResult)
	requireContains(t, statusResult.Stdout, "Pending sync changes:")
	requireContains(t, statusResult.Stdout, "local upload")
	requireContains(t, statusResult.Stdout, "remote download")

	diffResult := runNegent(t, machineB, "diff")
	requireCommandSuccess(t, "diff", diffResult)
	requireContains(t, diffResult.Stdout, "Local changes (push):")
	requireContains(t, diffResult.Stdout, "claude:commands/local.md")
	requireContains(t, diffResult.Stdout, "Remote changes (pull):")
	requireContains(t, diffResult.Stdout, "claude:rules/remote.md")
}

func requireCommandSuccess(t *testing.T, name string, result CommandResult) {
	t.Helper()

	if result.Err != nil {
		t.Fatalf("%s: %v\nstdout:\n%s\nstderr:\n%s", name, result.Err, result.Stdout, result.Stderr)
	}
}

func assertConfigForMachine(t *testing.T, machine *Machine, remote string) {
	t.Helper()

	cfg, err := config.Load(machine.ConfigPath())
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Backend != "git" {
		t.Fatalf("config backend = %q, want git", cfg.Backend)
	}
	if cfg.Repo != remote {
		t.Fatalf("config repo = %q, want %q", cfg.Repo, remote)
	}
	if cfg.Machine != machine.Name {
		t.Fatalf("config machine = %q, want %q", cfg.Machine, machine.Name)
	}
	claudeCfg, ok := cfg.Agents["claude"]
	if !ok {
		t.Fatalf("config missing claude agent: %#v", cfg.Agents)
	}
	if len(claudeCfg.Sync) == 0 {
		t.Fatal("claude sync types should not be empty")
	}
}

func assertStagingRepoExists(t *testing.T, machine *Machine) {
	t.Helper()

	if _, err := os.Stat(filepath.Join(machine.StagingDir(), ".git")); err != nil {
		t.Fatalf("staging repo not created: %v", err)
	}
}

func requireFileContents(t *testing.T, path, want string) {
	t.Helper()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("%s contents = %q, want %q", path, string(got), want)
	}
}

func requireNoFile(t *testing.T, path string) {
	t.Helper()

	_, err := os.Stat(path)
	if err == nil {
		t.Fatalf("expected %s to be absent", path)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func requireContains(t *testing.T, text, want string) {
	t.Helper()

	if !strings.Contains(text, want) {
		t.Fatalf("output missing %q:\n%s", want, text)
	}
}
