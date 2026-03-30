package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/manmart/negent/internal/backend"
)

// --- fake runner infrastructure ---

type call struct {
	dir  string
	args []string
}

type fakeRunner struct {
	calls     []call
	responses []fakeResp
}

type fakeResp struct {
	out []byte
	err error
}

func (f *fakeRunner) run(_ context.Context, dir string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, call{dir: dir, args: args})
	if len(f.responses) > 0 {
		r := f.responses[0]
		f.responses = f.responses[1:]
		return r.out, r.err
	}
	return nil, nil
}

func okResp(out string) fakeResp  { return fakeResp{out: []byte(out)} }
func errResp(msg string) fakeResp { return fakeResp{err: errors.New(msg)} }

func assertCall(t *testing.T, c call, wantDir string, wantArgs ...string) {
	t.Helper()
	if c.dir != wantDir {
		t.Errorf("dir = %q, want %q", c.dir, wantDir)
	}
	if !slices.Equal(c.args, wantArgs) {
		t.Errorf("args = %v, want %v", c.args, wantArgs)
	}
}

// --- tests ---

func TestDefaultStagingDir(t *testing.T) {
	dir := DefaultStagingDir()
	if dir == "" {
		t.Error("DefaultStagingDir returned empty")
	}
	if !filepath.IsAbs(dir) {
		t.Errorf("DefaultStagingDir = %q, expected absolute path", dir)
	}
}

func TestStagingDir(t *testing.T) {
	g := &Git{stagingDir: "/tmp/test-staging"}
	if g.StagingDir() != "/tmp/test-staging" {
		t.Errorf("StagingDir = %q, want /tmp/test-staging", g.StagingDir())
	}
}

func TestInitRequiresRemote(t *testing.T) {
	fr := &fakeRunner{}
	g := &Git{stagingDir: filepath.Join(t.TempDir(), "staging"), runner: fr.run}
	if err := g.Init(context.Background(), backend.BackendConfig{}); err == nil {
		t.Fatal("expected error for empty remote")
	}
	if len(fr.calls) != 0 {
		t.Errorf("expected no git calls, got %d", len(fr.calls))
	}
}

func TestInitClones(t *testing.T) {
	// staging does not exist yet — should trigger clone
	staging := filepath.Join(t.TempDir(), "staging")
	fr := &fakeRunner{}
	g := &Git{remote: "git@github.com:user/repo.git", stagingDir: staging, runner: fr.run}

	if err := g.Init(context.Background(), backend.BackendConfig{}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if len(fr.calls) != 1 {
		t.Fatalf("expected 1 call, got %d: %v", len(fr.calls), fr.calls)
	}
	assertCall(t, fr.calls[0], "", "clone", "git@github.com:user/repo.git", staging)
}

func TestInitConfigRemoteOverridesField(t *testing.T) {
	staging := filepath.Join(t.TempDir(), "staging")
	fr := &fakeRunner{}
	g := &Git{remote: "old-remote", stagingDir: staging, runner: fr.run}

	cfg := backend.BackendConfig{"remote": "new-remote"}
	if err := g.Init(context.Background(), cfg); err != nil {
		t.Fatalf("Init: %v", err)
	}

	assertCall(t, fr.calls[0], "", "clone", "new-remote", staging)
}

func TestInitPullsIfAlreadyCloned(t *testing.T) {
	staging := t.TempDir()
	os.MkdirAll(filepath.Join(staging, ".git"), 0o755)

	fr := &fakeRunner{responses: []fakeResp{
		okResp("abc123\n"), // rev-parse HEAD
		okResp(""),         // pull --rebase
	}}
	g := &Git{remote: "git@github.com:user/repo.git", stagingDir: staging, runner: fr.run}

	if err := g.Init(context.Background(), backend.BackendConfig{}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if len(fr.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d: %v", len(fr.calls), fr.calls)
	}
	assertCall(t, fr.calls[0], staging, "rev-parse", "HEAD")
	assertCall(t, fr.calls[1], staging, "pull", "--rebase")
}

func TestPushSendsCommitAndPush(t *testing.T) {
	fr := &fakeRunner{responses: []fakeResp{
		okResp("abc123\n"),  // rev-parse HEAD (inside pre-push Pull)
		okResp(""),          // pull --rebase
		okResp(""),          // add -A
		errResp("has diff"), // diff --cached --quiet (non-zero = changes exist)
		okResp(""),          // commit -m
		okResp(""),          // push
	}}
	g := &Git{stagingDir: "/staging", runner: fr.run}

	if err := g.Push(context.Background(), "test commit"); err != nil {
		t.Fatalf("Push: %v", err)
	}

	if len(fr.calls) != 6 {
		t.Fatalf("expected 6 calls, got %d", len(fr.calls))
	}
	assertCall(t, fr.calls[0], "/staging", "rev-parse", "HEAD")
	assertCall(t, fr.calls[1], "/staging", "pull", "--rebase")
	assertCall(t, fr.calls[2], "/staging", "add", "-A")
	assertCall(t, fr.calls[3], "/staging", "diff", "--cached", "--quiet")
	assertCall(t, fr.calls[4], "/staging", "commit", "-m", "test commit")
	assertCall(t, fr.calls[5], "/staging", "push")
}

func TestPushRetriesAfterPull(t *testing.T) {
	fr := &fakeRunner{responses: []fakeResp{
		okResp("abc123\n"),  // rev-parse HEAD (inside pre-push Pull)
		okResp(""),          // pull --rebase
		okResp(""),          // add -A
		errResp("has diff"), // diff --cached --quiet
		okResp(""),          // commit -m
		errResp("rejected"), // push (fails — remote diverged after pre-pull)
		okResp("abc123\n"),  // rev-parse HEAD (inside retry Pull)
		okResp(""),          // pull --rebase
		okResp(""),          // push (retry)
	}}
	g := &Git{stagingDir: "/staging", runner: fr.run}

	if err := g.Push(context.Background(), "test commit"); err != nil {
		t.Fatalf("Push: %v", err)
	}

	if len(fr.calls) != 9 {
		t.Fatalf("expected 9 calls, got %d: %v", len(fr.calls), fr.calls)
	}
	assertCall(t, fr.calls[0], "/staging", "rev-parse", "HEAD")
	assertCall(t, fr.calls[1], "/staging", "pull", "--rebase")
	assertCall(t, fr.calls[2], "/staging", "add", "-A")
	assertCall(t, fr.calls[3], "/staging", "diff", "--cached", "--quiet")
	assertCall(t, fr.calls[4], "/staging", "commit", "-m", "test commit")
	assertCall(t, fr.calls[5], "/staging", "push")
	assertCall(t, fr.calls[6], "/staging", "rev-parse", "HEAD")
	assertCall(t, fr.calls[7], "/staging", "pull", "--rebase")
	assertCall(t, fr.calls[8], "/staging", "push")
}

func TestPushSkipsCommitWhenNothingStaged(t *testing.T) {
	fr := &fakeRunner{responses: []fakeResp{
		okResp("abc123\n"), // rev-parse HEAD
		okResp(""),         // pull --rebase
		okResp(""),         // add -A
		okResp(""),         // diff --cached --quiet → exit 0 means nothing staged
		okResp(""),         // push
	}}
	g := &Git{stagingDir: "/staging", runner: fr.run}

	if err := g.Push(context.Background(), "msg"); err != nil {
		t.Fatalf("Push: %v", err)
	}

	if len(fr.calls) != 5 {
		t.Fatalf("expected 5 calls (no commit, still push), got %d: %v", len(fr.calls), fr.calls)
	}
	assertCall(t, fr.calls[0], "/staging", "rev-parse", "HEAD")
	assertCall(t, fr.calls[1], "/staging", "pull", "--rebase")
	assertCall(t, fr.calls[2], "/staging", "add", "-A")
	assertCall(t, fr.calls[3], "/staging", "diff", "--cached", "--quiet")
	assertCall(t, fr.calls[4], "/staging", "push")
}

func TestPullSendsRebaseCommand(t *testing.T) {
	fr := &fakeRunner{responses: []fakeResp{
		okResp("abc123\n"), // rev-parse HEAD
		okResp(""),         // pull --rebase
	}}
	g := &Git{stagingDir: "/staging", runner: fr.run}

	if err := g.Pull(context.Background()); err != nil {
		t.Fatalf("Pull: %v", err)
	}

	if len(fr.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(fr.calls))
	}
	assertCall(t, fr.calls[0], "/staging", "rev-parse", "HEAD")
	assertCall(t, fr.calls[1], "/staging", "pull", "--rebase")
}

func TestPullSkipsEmptyRepo(t *testing.T) {
	fr := &fakeRunner{responses: []fakeResp{
		errResp("no HEAD"), // rev-parse HEAD fails on empty repo
	}}
	g := &Git{stagingDir: "/staging", runner: fr.run}

	if err := g.Pull(context.Background()); err != nil {
		t.Fatalf("Pull on empty repo should succeed, got: %v", err)
	}

	if len(fr.calls) != 1 {
		t.Fatalf("expected 1 call (rev-parse only), got %d", len(fr.calls))
	}
	assertCall(t, fr.calls[0], "/staging", "rev-parse", "HEAD")
}

func TestPullContinuesInterruptedRebase(t *testing.T) {
	staging := t.TempDir()
	if err := os.MkdirAll(filepath.Join(staging, ".git", "rebase-merge"), 0o755); err != nil {
		t.Fatalf("mkdir rebase-merge: %v", err)
	}

	fr := &fakeRunner{responses: []fakeResp{
		okResp("abc123\n"), // rev-parse HEAD
		okResp(""),         // rebase --continue
		okResp(""),         // pull --rebase
	}}
	g := &Git{stagingDir: staging, runner: fr.run}

	if err := g.Pull(context.Background()); err != nil {
		t.Fatalf("Pull: %v", err)
	}

	if len(fr.calls) != 3 {
		t.Fatalf("expected 3 calls, got %d: %v", len(fr.calls), fr.calls)
	}
	assertCall(t, fr.calls[0], staging, "rev-parse", "HEAD")
	assertCall(t, fr.calls[1], staging, "rebase", "--continue")
	assertCall(t, fr.calls[2], staging, "pull", "--rebase")
}

func TestPullAbortsInterruptedRebaseWhenContinueFails(t *testing.T) {
	staging := t.TempDir()
	if err := os.MkdirAll(filepath.Join(staging, ".git", "rebase-apply"), 0o755); err != nil {
		t.Fatalf("mkdir rebase-apply: %v", err)
	}

	fr := &fakeRunner{responses: []fakeResp{
		okResp("abc123\n"),  // rev-parse HEAD
		errResp("continue"), // rebase --continue
		okResp(""),          // rebase --abort
		okResp(""),          // pull --rebase
	}}
	g := &Git{stagingDir: staging, runner: fr.run}

	if err := g.Pull(context.Background()); err != nil {
		t.Fatalf("Pull: %v", err)
	}

	if len(fr.calls) != 4 {
		t.Fatalf("expected 4 calls, got %d: %v", len(fr.calls), fr.calls)
	}
	assertCall(t, fr.calls[0], staging, "rev-parse", "HEAD")
	assertCall(t, fr.calls[1], staging, "rebase", "--continue")
	assertCall(t, fr.calls[2], staging, "rebase", "--abort")
	assertCall(t, fr.calls[3], staging, "pull", "--rebase")
}

func TestPullAbortsWhenRebasePullFails(t *testing.T) {
	fr := &fakeRunner{responses: []fakeResp{
		okResp("abc123\n"),        // rev-parse HEAD
		errResp("merge conflict"), // pull --rebase
		okResp(""),                // diff --name-only --diff-filter=U
		okResp(""),                // rebase --abort
	}}
	g := &Git{stagingDir: "/staging", runner: fr.run}

	err := g.Pull(context.Background())
	if err == nil {
		t.Fatal("expected Pull to fail on rebase conflict")
	}
	var conflictErr *ConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected ConflictError, got %T: %v", err, err)
	}

	if len(fr.calls) != 4 {
		t.Fatalf("expected 4 calls, got %d: %v", len(fr.calls), fr.calls)
	}
	assertCall(t, fr.calls[0], "/staging", "rev-parse", "HEAD")
	assertCall(t, fr.calls[1], "/staging", "pull", "--rebase")
	assertCall(t, fr.calls[2], "/staging", "diff", "--name-only", "--diff-filter=U")
	assertCall(t, fr.calls[3], "/staging", "rebase", "--abort")
}

func TestPullReturnsConflictErrorWhenUnmergedFilesDetected(t *testing.T) {
	fr := &fakeRunner{responses: []fakeResp{
		okResp("abc123\n"),                    // rev-parse HEAD
		errResp("rebase failed"),              // pull --rebase
		okResp("claude/skills/a.md\nb.txt\n"), // diff --name-only --diff-filter=U
		okResp(""),                            // rebase --abort
	}}
	g := &Git{stagingDir: "/staging", runner: fr.run}

	err := g.Pull(context.Background())
	if err == nil {
		t.Fatal("expected Pull to fail")
	}
	var conflictErr *ConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected ConflictError, got %T: %v", err, err)
	}
	if len(conflictErr.Files) != 2 || conflictErr.Files[0] != "b.txt" || conflictErr.Files[1] != "claude/skills/a.md" {
		t.Fatalf("conflict files = %v, want sorted file list", conflictErr.Files)
	}
}

func TestPullReturnsGenericErrorWhenNoConflictDetected(t *testing.T) {
	fr := &fakeRunner{responses: []fakeResp{
		okResp("abc123\n"),     // rev-parse HEAD
		errResp("auth failed"), // pull --rebase
		okResp(""),             // diff --name-only --diff-filter=U
		okResp(""),             // rebase --abort
	}}
	g := &Git{stagingDir: "/staging", runner: fr.run}

	err := g.Pull(context.Background())
	if err == nil {
		t.Fatal("expected Pull to fail")
	}
	var conflictErr *ConflictError
	if errors.As(err, &conflictErr) {
		t.Fatalf("did not expect ConflictError for auth failure, got: %v", err)
	}
}

func TestFetchSendsFetchOrigin(t *testing.T) {
	fr := &fakeRunner{responses: []fakeResp{
		okResp("abc123\n"), // rev-parse HEAD
		okResp(""),         // fetch origin
	}}
	g := &Git{stagingDir: "/staging", runner: fr.run}

	if err := g.Fetch(context.Background()); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if len(fr.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(fr.calls))
	}
	assertCall(t, fr.calls[0], "/staging", "rev-parse", "HEAD")
	assertCall(t, fr.calls[1], "/staging", "fetch", "origin")
}

func TestFetchedFilesParsesOutput(t *testing.T) {
	fr := &fakeRunner{responses: []fakeResp{okResp("A new.txt\nM existing.txt\nD removed.txt\n")}}
	staging := t.TempDir()
	os.MkdirAll(filepath.Join(staging, ".git"), 0o755)
	os.WriteFile(filepath.Join(staging, ".git", "FETCH_HEAD"), []byte("fetched"), 0o644)
	g := &Git{stagingDir: staging, runner: fr.run}

	changes, err := g.FetchedFiles(context.Background())
	if err != nil {
		t.Fatalf("FetchedFiles: %v", err)
	}

	assertCall(t, fr.calls[0], staging, "diff", "--name-status", "HEAD", "FETCH_HEAD")

	byPath := make(map[string]backend.ChangeKind)
	for _, c := range changes {
		byPath[c.Path] = c.Kind
	}

	if byPath["new.txt"] != backend.ChangeAdded {
		t.Errorf("new.txt kind = %v, want added", byPath["new.txt"])
	}
	if byPath["existing.txt"] != backend.ChangeModified {
		t.Errorf("existing.txt kind = %v, want modified", byPath["existing.txt"])
	}
	if byPath["removed.txt"] != backend.ChangeDeleted {
		t.Errorf("removed.txt kind = %v, want deleted", byPath["removed.txt"])
	}
}

func TestFetchSkipsEmptyRepo(t *testing.T) {
	fr := &fakeRunner{responses: []fakeResp{
		errResp("no HEAD"),
	}}
	g := &Git{stagingDir: "/staging", runner: fr.run}

	if err := g.Fetch(context.Background()); err != nil {
		t.Fatalf("Fetch on empty repo should succeed, got: %v", err)
	}

	if len(fr.calls) != 1 {
		t.Fatalf("expected 1 call (rev-parse only), got %d", len(fr.calls))
	}
	assertCall(t, fr.calls[0], "/staging", "rev-parse", "HEAD")
}

func TestStatusParsesOutput(t *testing.T) {
	porcelain := " M existing.txt\n?? new.txt\n D deleted.txt\n"
	fr := &fakeRunner{responses: []fakeResp{okResp(porcelain)}}
	g := &Git{stagingDir: "/staging", runner: fr.run}

	changes, err := g.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	assertCall(t, fr.calls[0], "/staging", "status", "--porcelain")

	byPath := make(map[string]backend.ChangeKind)
	for _, c := range changes {
		byPath[c.Path] = c.Kind
	}

	if byPath["existing.txt"] != backend.ChangeModified {
		t.Errorf("existing.txt: got %v, want modified", byPath["existing.txt"])
	}
	if byPath["new.txt"] != backend.ChangeAdded {
		t.Errorf("new.txt: got %v, want added", byPath["new.txt"])
	}
	if byPath["deleted.txt"] != backend.ChangeDeleted {
		t.Errorf("deleted.txt: got %v, want deleted", byPath["deleted.txt"])
	}
}

func TestStatusEmpty(t *testing.T) {
	fr := &fakeRunner{responses: []fakeResp{okResp("")}}
	g := &Git{stagingDir: "/staging", runner: fr.run}

	changes, err := g.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(changes) != 0 {
		t.Errorf("expected 0 changes, got %d", len(changes))
	}
}

func TestFetchedFilesNoFetchHead(t *testing.T) {
	// No .git/FETCH_HEAD — should return empty without calling git.
	staging := t.TempDir()
	fr := &fakeRunner{}
	g := &Git{stagingDir: staging, runner: fr.run}

	files, err := g.FetchedFiles(context.Background())
	if err != nil {
		t.Fatalf("FetchedFiles: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d", len(files))
	}
	if len(fr.calls) != 0 {
		t.Errorf("expected no git calls when FETCH_HEAD absent, got %d", len(fr.calls))
	}
}

func TestFetchedFilesReturnsChangedFiles(t *testing.T) {
	staging := t.TempDir()
	os.MkdirAll(filepath.Join(staging, ".git"), 0o755)
	os.WriteFile(filepath.Join(staging, ".git", "FETCH_HEAD"), []byte("abc123"), 0o644)

	fr := &fakeRunner{responses: []fakeResp{okResp("M changed.txt\nA other.txt\n")}}
	g := &Git{stagingDir: staging, runner: fr.run}

	files, err := g.FetchedFiles(context.Background())
	if err != nil {
		t.Fatalf("FetchedFiles: %v", err)
	}

	assertCall(t, fr.calls[0], staging, "diff", "--name-status", "HEAD", "FETCH_HEAD")

	want := []backend.FileChange{
		{Path: "changed.txt", Kind: backend.ChangeModified},
		{Path: "other.txt", Kind: backend.ChangeAdded},
	}
	if !slices.Equal(files, want) {
		t.Errorf("FetchedFiles = %v, want %v", files, want)
	}
}

func TestFetchedFilesEmptyWhenUpToDate(t *testing.T) {
	staging := t.TempDir()
	os.MkdirAll(filepath.Join(staging, ".git"), 0o755)
	os.WriteFile(filepath.Join(staging, ".git", "FETCH_HEAD"), []byte("abc123"), 0o644)

	fr := &fakeRunner{responses: []fakeResp{okResp("")}}
	g := &Git{stagingDir: staging, runner: fr.run}

	files, err := g.FetchedFiles(context.Background())
	if err != nil {
		t.Fatalf("FetchedFiles: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files when up to date, got %d: %v", len(files), files)
	}
}
