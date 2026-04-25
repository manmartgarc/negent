package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitAndPushWithHarness(t *testing.T) {
	remote := newBareRemote(t)
	machine := newMachine(t, "machine-a")
	machine.SeedAgentFile(t, "claude", "CLAUDE.md", "# machine-a\n")

	initResult := runNegent(t, machine, "init",
		"--non-interactive",
		"--backend", "git",
		"--repo", remote,
		"--machine", machine.Name,
		"--agent", "claude",
	)
	if initResult.Err != nil {
		t.Fatalf("negent init: %v\nstdout:\n%s\nstderr:\n%s", initResult.Err, initResult.Stdout, initResult.Stderr)
	}
	if !strings.Contains(initResult.Stdout, "Config written") {
		t.Fatalf("init output missing config confirmation:\n%s", initResult.Stdout)
	}

	if _, err := os.Stat(machine.ConfigPath()); err != nil {
		t.Fatalf("config not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(machine.StagingDir(), ".git")); err != nil {
		t.Fatalf("staging repo not created: %v", err)
	}

	pushResult := runNegent(t, machine, "push", "--quiet")
	if pushResult.Err != nil {
		t.Fatalf("negent push: %v\nstdout:\n%s\nstderr:\n%s", pushResult.Err, pushResult.Stdout, pushResult.Stderr)
	}

	commitCount := gitOutput(t, testHarness.repoRoot, "--git-dir", remote, "rev-list", "--count", "--all")
	if commitCount != "1" {
		t.Fatalf("expected 1 remote commit, got %q", commitCount)
	}

	trackedFiles := gitOutput(t, testHarness.repoRoot, "--git-dir", remote, "ls-tree", "-r", "--name-only", "HEAD")
	if !strings.Contains(trackedFiles, "CLAUDE.md") {
		t.Fatalf("remote tree missing seeded Claude fixture:\n%s", trackedFiles)
	}
}
