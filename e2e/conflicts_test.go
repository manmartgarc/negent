package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPullConflictListAndKeepRemote(t *testing.T) {
	remote := newBareRemote(t)
	machineA := newMachine(t, "machine-a")
	machineB := newMachine(t, "machine-b")

	const (
		relPath       = "CLAUDE.md"
		baseline      = "# baseline\n"
		localChange   = "# machine-b local edit\n"
		remoteChange  = "# machine-a remote edit\n"
		conflictLabel = "claude:" + relPath
	)

	writeFile(t, filepath.Join(machineA.AgentHome("claude"), relPath), baseline)

	runOK(t, "machine A init", runNegent(t, machineA, "init",
		"--non-interactive",
		"--backend", "git",
		"--repo", remote,
		"--machine", machineA.Name,
		"--agent", "claude",
	))
	runOK(t, "machine A push baseline", runNegent(t, machineA, "push", "--quiet"))

	runOK(t, "machine B init", runNegent(t, machineB, "init",
		"--non-interactive",
		"--backend", "git",
		"--repo", remote,
		"--machine", machineB.Name,
		"--agent", "claude",
	))
	runOK(t, "machine B pull baseline", runNegent(t, machineB, "pull", "--quiet"))

	localFile := filepath.Join(machineB.AgentHome("claude"), relPath)
	if got := readFile(t, localFile); got != baseline {
		t.Fatalf("machine B baseline file = %q, want %q", got, baseline)
	}

	writeFile(t, localFile, localChange)
	writeFile(t, filepath.Join(machineA.AgentHome("claude"), relPath), remoteChange)
	runOK(t, "machine A push remote change", runNegent(t, machineA, "push", "--quiet"))

	pullResult := runNegent(t, machineB, "pull")
	runOK(t, "machine B pull conflict", pullResult)
	assertContains(t, pullResult.Stdout,
		"✓ Pull complete",
		"claude: 1 conflict(s) — run 'negent conflicts' to resolve:",
		"CONFLICT: "+relPath,
		"1 conflict(s) — run 'negent conflicts' to resolve",
	)
	if got := readFile(t, localFile); got != localChange {
		t.Fatalf("machine B file after conflicting pull = %q, want local edit %q", got, localChange)
	}

	listResult := runNegent(t, machineB, "conflicts", "--list")
	runOK(t, "machine B list conflicts", listResult)
	assertContains(t, listResult.Stdout, "CONFLICT "+conflictLabel)

	resolveResult := runNegent(t, machineB, "conflicts", "--keep-remote")
	runOK(t, "machine B keep remote", resolveResult)
	assertContains(t, resolveResult.Stdout, "took remote: "+conflictLabel)
	if got := readFile(t, localFile); got != remoteChange {
		t.Fatalf("machine B file after keep-remote = %q, want remote edit %q", got, remoteChange)
	}

	postListResult := runNegent(t, machineB, "conflicts", "--list")
	runOK(t, "machine B list conflicts after resolution", postListResult)
	assertContains(t, postListResult.Stdout, "No conflicts.")
}

func runOK(t *testing.T, label string, result CommandResult) {
	t.Helper()

	if result.Err != nil {
		t.Fatalf("%s: %v\nstdout:\n%s\nstderr:\n%s", label, result.Err, result.Stdout, result.Stderr)
	}
}

func assertContains(t *testing.T, output string, want ...string) {
	t.Helper()

	for _, needle := range want {
		if !strings.Contains(output, needle) {
			t.Fatalf("output missing %q:\n%s", needle, output)
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
