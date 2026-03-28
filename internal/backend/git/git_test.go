package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/manmart/negent/internal/backend"
)

// initBareRepo creates a bare git repo to act as a "remote" for tests.
func initBareRepo(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "remote.git")
	run(t, "", "git", "init", "--bare", dir)
	return dir
}

// run is a test helper to execute a command.
func run(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
	return string(out)
}

func TestInitClonesRepo(t *testing.T) {
	remote := initBareRepo(t)
	staging := filepath.Join(t.TempDir(), "staging")

	g := New(remote, staging)
	err := g.Init(context.Background(), backend.BackendConfig{"remote": remote})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Verify .git directory was created
	if _, err := os.Stat(filepath.Join(staging, ".git")); os.IsNotExist(err) {
		t.Error("staging dir was not cloned")
	}
}

func TestInitPullsIfAlreadyCloned(t *testing.T) {
	remote := initBareRepo(t)
	staging := filepath.Join(t.TempDir(), "staging")

	g := New(remote, staging)
	err := g.Init(context.Background(), backend.BackendConfig{"remote": remote})
	if err != nil {
		t.Fatalf("first Init: %v", err)
	}

	// Second init should pull (not re-clone)
	err = g.Init(context.Background(), backend.BackendConfig{"remote": remote})
	if err != nil {
		t.Fatalf("second Init: %v", err)
	}
}

func TestInitRequiresRemote(t *testing.T) {
	staging := filepath.Join(t.TempDir(), "staging")
	g := New("", staging)
	err := g.Init(context.Background(), backend.BackendConfig{})
	if err == nil {
		t.Fatal("expected error for empty remote")
	}
}

func TestStagingDir(t *testing.T) {
	g := New("some-remote", "/tmp/test-staging")
	if g.StagingDir() != "/tmp/test-staging" {
		t.Errorf("StagingDir = %q, want /tmp/test-staging", g.StagingDir())
	}
}

func TestPushAndPull(t *testing.T) {
	remote := initBareRepo(t)
	staging := filepath.Join(t.TempDir(), "staging")

	g := New(remote, staging)
	ctx := context.Background()

	if err := g.Init(ctx, backend.BackendConfig{"remote": remote}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Create a file in staging
	os.WriteFile(filepath.Join(staging, "test.txt"), []byte("hello"), 0o644)

	// Push
	if err := g.Push(ctx, "test commit"); err != nil {
		t.Fatalf("Push: %v", err)
	}

	// Clone into a second staging dir to verify the push landed
	staging2 := filepath.Join(t.TempDir(), "staging2")
	g2 := New(remote, staging2)
	if err := g2.Init(ctx, backend.BackendConfig{"remote": remote}); err != nil {
		t.Fatalf("Init staging2: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(staging2, "test.txt"))
	if err != nil {
		t.Fatalf("reading pushed file: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("file content = %q, want %q", data, "hello")
	}
}

func TestPushNoChanges(t *testing.T) {
	remote := initBareRepo(t)
	staging := filepath.Join(t.TempDir(), "staging")

	g := New(remote, staging)
	ctx := context.Background()

	if err := g.Init(ctx, backend.BackendConfig{"remote": remote}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Push with nothing to commit should succeed silently
	if err := g.Push(ctx, "empty"); err != nil {
		t.Fatalf("Push with no changes: %v", err)
	}
}

func TestStatusDetectsChanges(t *testing.T) {
	remote := initBareRepo(t)
	staging := filepath.Join(t.TempDir(), "staging")

	g := New(remote, staging)
	ctx := context.Background()

	if err := g.Init(ctx, backend.BackendConfig{"remote": remote}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Push an initial file
	os.WriteFile(filepath.Join(staging, "existing.txt"), []byte("v1"), 0o644)
	if err := g.Push(ctx, "initial"); err != nil {
		t.Fatalf("Push: %v", err)
	}

	// Create changes: modify existing, add new
	os.WriteFile(filepath.Join(staging, "existing.txt"), []byte("v2"), 0o644)
	os.WriteFile(filepath.Join(staging, "new.txt"), []byte("new"), 0o644)

	changes, err := g.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	if len(changes) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(changes))
	}

	found := make(map[string]backend.ChangeKind)
	for _, c := range changes {
		found[c.Path] = c.Kind
	}

	if found["existing.txt"] != backend.ChangeModified {
		t.Errorf("existing.txt kind = %q, want modified", found["existing.txt"])
	}
	if found["new.txt"] != backend.ChangeAdded {
		t.Errorf("new.txt kind = %q, want added", found["new.txt"])
	}
}

func TestDefaultStagingDir(t *testing.T) {
	dir := DefaultStagingDir()
	if dir == "" {
		t.Error("DefaultStagingDir returned empty")
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("DefaultStagingDir = %q, expected absolute path", dir)
	}
}
