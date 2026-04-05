package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKnownAgents(t *testing.T) {
	agents := KnownAgents()
	if len(agents) == 0 {
		t.Fatal("KnownAgents() returned empty")
	}

	names := make(map[string]bool)
	for _, a := range agents {
		if a.Name == "" {
			t.Error("agent has empty name")
		}
		if a.SourceDir == "" {
			t.Errorf("agent %q has empty source dir", a.Name)
		}
		names[a.Name] = true
	}

	if !names["claude"] {
		t.Error("missing known agent \"claude\"")
	}
	if !names["copilot"] {
		t.Error("missing known agent \"copilot\"")
	}
}

func TestDetectAgents(t *testing.T) {
	// DetectAgents depends on the actual filesystem, so we just verify
	// it returns a subset of known agents without errors
	detected := DetectAgents()
	known := make(map[string]bool)
	for _, ka := range KnownAgents() {
		known[ka.Name] = true
	}
	for _, d := range detected {
		if !known[d.Name] {
			t.Errorf("detected agent %q is not in known agents", d.Name)
		}
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	tests := []struct {
		input string
		want  string
	}{
		{"~/foo", filepath.Join(home, "foo")},
		{"~/.claude", filepath.Join(home, ".claude")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"~", "~"}, // no slash after ~
	}
	for _, tt := range tests {
		got := ExpandHome(tt.input)
		if got != tt.want {
			t.Errorf("ExpandHome(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
