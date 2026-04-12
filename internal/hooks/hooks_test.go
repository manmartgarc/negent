package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const testBin = "/usr/local/bin/negent"

func mustNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// readHooksMap is a test helper that parses settings.json and returns the hooks map.
func readHooksMap(t *testing.T, path string) map[string][]hookMatcher {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading settings: %v", err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		t.Fatalf("parsing settings: %v", err)
	}
	raw, ok := top["hooks"]
	if !ok {
		return nil
	}
	var hm map[string][]hookMatcher
	if err := json.Unmarshal(raw, &hm); err != nil {
		t.Fatalf("parsing hooks: %v", err)
	}
	return hm
}

// negentCommands returns all negent command strings found for the given event.
func negentCommands(hm map[string][]hookMatcher, event string) []string {
	var cmds []string
	for _, m := range hm[event] {
		for _, e := range m.Hooks {
			if isNegentCommand(e.Command) {
				cmds = append(cmds, e.Command)
			}
		}
	}
	return cmds
}

func TestInstallFreshFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	if err := InstallClaude(path, testBin); err != nil {
		t.Fatalf("InstallClaude: %v", err)
	}

	hm := readHooksMap(t, path)

	startCmds := negentCommands(hm, "SessionStart")
	if len(startCmds) != 1 || startCmds[0] != testBin+" pull --quiet" {
		t.Errorf("SessionStart: got %v, want [%q]", startCmds, testBin+" pull --quiet")
	}

	stopCmds := negentCommands(hm, "Stop")
	if len(stopCmds) != 1 || stopCmds[0] != testBin+" push --quiet" {
		t.Errorf("Stop: got %v, want [%q]", stopCmds, testBin+" push --quiet")
	}
}

func TestInstallPreservesOtherKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{"theme":"dark"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := InstallClaude(path, testBin); err != nil {
		t.Fatalf("InstallClaude: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var top map[string]json.RawMessage
	mustNoErr(t, json.Unmarshal(data, &top))

	var theme string
	mustNoErr(t, json.Unmarshal(top["theme"], &theme))
	if theme != "dark" {
		t.Errorf("theme: got %q, want %q", theme, "dark")
	}
}

func TestInstallPreservesOtherHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	initial := `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"other-tool start"}]}]}}`
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := InstallClaude(path, testBin); err != nil {
		t.Fatalf("InstallClaude: %v", err)
	}

	hm := readHooksMap(t, path)
	matchers := hm["SessionStart"]

	// Should have 2 matchers: other-tool's and negent's
	if len(matchers) != 2 {
		t.Errorf("SessionStart matchers: got %d, want 2", len(matchers))
	}

	// other-tool command must still be present
	found := false
	for _, m := range matchers {
		for _, e := range m.Hooks {
			if e.Command == "other-tool start" {
				found = true
			}
		}
	}
	if !found {
		t.Error("other-tool hook was removed")
	}
}

func TestInstallIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	for range 2 {
		if err := InstallClaude(path, testBin); err != nil {
			t.Fatalf("InstallClaude: %v", err)
		}
	}

	hm := readHooksMap(t, path)
	if cmds := negentCommands(hm, "SessionStart"); len(cmds) != 1 {
		t.Errorf("SessionStart negent commands: got %d, want 1", len(cmds))
	}
	if cmds := negentCommands(hm, "Stop"); len(cmds) != 1 {
		t.Errorf("Stop negent commands: got %d, want 1", len(cmds))
	}
}

func TestInstallUpdatesBinaryPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	if err := InstallClaude(path, "/usr/local/bin/negent"); err != nil {
		t.Fatal(err)
	}
	if err := InstallClaude(path, "/home/user/go/bin/negent"); err != nil {
		t.Fatal(err)
	}

	hm := readHooksMap(t, path)
	cmds := negentCommands(hm, "SessionStart")
	if len(cmds) != 1 {
		t.Errorf("SessionStart negent commands: got %d, want 1", len(cmds))
	}
	if len(cmds) > 0 && cmds[0] != "/home/user/go/bin/negent pull --quiet" {
		t.Errorf("command: got %q, want %q", cmds[0], "/home/user/go/bin/negent pull --quiet")
	}
}

func TestInstallInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	original := []byte("{invalid")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := InstallClaude(path, testBin); err == nil {
		t.Fatal("expected error on invalid JSON, got nil")
	}

	// File must be unchanged
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Errorf("file was modified despite parse error")
	}
}

func TestUninstallRemovesOnlyNegent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	initial := `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"other-tool start"}]}]}}`
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := InstallClaude(path, testBin); err != nil {
		t.Fatal(err)
	}
	if err := UninstallClaude(path); err != nil {
		t.Fatalf("UninstallClaude: %v", err)
	}

	hm := readHooksMap(t, path)
	if cmds := negentCommands(hm, "SessionStart"); len(cmds) != 0 {
		t.Errorf("negent commands remain after uninstall: %v", cmds)
	}

	// other-tool hook must still be present
	found := false
	for _, m := range hm["SessionStart"] {
		for _, e := range m.Hooks {
			if e.Command == "other-tool start" {
				found = true
			}
		}
	}
	if !found {
		t.Error("other-tool hook was removed during uninstall")
	}
}

func TestUninstallNoopWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	if err := UninstallClaude(path); err != nil {
		t.Fatalf("UninstallClaude on absent file: %v", err)
	}
}

func TestUninstallRemovesEmptyHooksKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	if err := InstallClaude(path, testBin); err != nil {
		t.Fatal(err)
	}
	if err := UninstallClaude(path); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var top map[string]json.RawMessage
	mustNoErr(t, json.Unmarshal(data, &top))
	if _, ok := top["hooks"]; ok {
		t.Error("\"hooks\" key still present after all hooks removed")
	}
}

func TestStatusEnabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	if err := InstallClaude(path, testBin); err != nil {
		t.Fatal(err)
	}

	enabled, err := StatusClaude(path)
	if err != nil {
		t.Fatalf("StatusClaude: %v", err)
	}
	if !enabled {
		t.Error("expected enabled=true")
	}
}

func TestStatusDisabledNoFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	enabled, err := StatusClaude(path)
	if err != nil {
		t.Fatalf("StatusClaude: %v", err)
	}
	if enabled {
		t.Error("expected enabled=false for absent file")
	}
}

func TestStatusDisabledAfterUninstall(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	if err := InstallClaude(path, testBin); err != nil {
		t.Fatal(err)
	}
	if err := UninstallClaude(path); err != nil {
		t.Fatal(err)
	}

	enabled, err := StatusClaude(path)
	if err != nil {
		t.Fatalf("StatusClaude: %v", err)
	}
	if enabled {
		t.Error("expected enabled=false after uninstall")
	}
}

func TestStatusInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte("{invalid"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := StatusClaude(path)
	if err == nil {
		t.Fatal("expected error on invalid JSON, got nil")
	}
}

func TestWriteFileAtomic_RemovesTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	if err := writeFileAtomic(path, []byte(`{"hooks":{}}`)); err != nil {
		t.Fatal(err)
	}

	matches, err := filepath.Glob(path + ".*.negent.tmp")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temp files should be removed, found %v", matches)
	}
}

func TestWriteFileAtomic_PreservesExistingPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	if err := os.WriteFile(path, []byte(`{"hooks":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := writeFileAtomic(path, []byte(`{"hooks":{"Stop":[]}}`)); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("file mode = %o, want %o", got, 0o600)
	}
}
