package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	original := &Config{
		Backend: "git",
		Repo:    "git@github.com:user/repo.git",
		Machine: "test-machine",
		Agents: map[string]AgentConfig{
			"claude": {
				Source: "~/.claude",
				Sync:   []string{"config", "custom-code", "memory"},
			},
		},
	}

	if err := Save(original, path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Backend != original.Backend {
		t.Errorf("Backend = %q, want %q", loaded.Backend, original.Backend)
	}
	if loaded.Repo != original.Repo {
		t.Errorf("Repo = %q, want %q", loaded.Repo, original.Repo)
	}
	if loaded.Machine != original.Machine {
		t.Errorf("Machine = %q, want %q", loaded.Machine, original.Machine)
	}

	ac, ok := loaded.Agents["claude"]
	if !ok {
		t.Fatal("missing agent 'claude'")
	}
	if ac.Source != "~/.claude" {
		t.Errorf("Agent source = %q, want %q", ac.Source, "~/.claude")
	}
	if len(ac.Sync) != 3 {
		t.Errorf("Agent sync entries = %d, want 3", len(ac.Sync))
	}
}

func TestLoadMissing(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error loading nonexistent config")
	}
}

func TestExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if Exists(path) {
		t.Fatal("Exists should be false before file is created")
	}

	if err := os.WriteFile(path, []byte("backend: git\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !Exists(path) {
		t.Fatal("Exists should be true after file is created")
	}
}

func TestDefaultPath(t *testing.T) {
	path := DefaultPath()
	if path == "" {
		t.Fatal("DefaultPath returned empty string")
	}
	if filepath.Base(path) != "config.yaml" {
		t.Errorf("DefaultPath base = %q, want config.yaml", filepath.Base(path))
	}
}
