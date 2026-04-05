package agent

import (
	"os"
	"path/filepath"
)

// KnownAgent describes a built-in agent that negent knows how to sync.
type KnownAgent struct {
	Name      string
	SourceDir string // default source directory (with ~ unexpanded)
}

// KnownAgents returns the list of agents negent has built-in support for.
// Only agents with a working implementation belong here; add new entries
// when their Agent implementation lands.
func KnownAgents() []KnownAgent {
	return []KnownAgent{
		{Name: "claude", SourceDir: "~/.claude"},
		{Name: "copilot", SourceDir: defaultCopilotSourceDir()},
	}
}

// DetectAgents returns the subset of known agents whose source directories
// exist on this machine.
func DetectAgents() []KnownAgent {
	var detected []KnownAgent
	for _, a := range KnownAgents() {
		expanded := ExpandHome(a.SourceDir)
		if info, err := os.Stat(expanded); err == nil && info.IsDir() {
			detected = append(detected, a)
		}
	}
	return detected
}

// ExpandHome replaces a leading ~ with the user's home directory.
func ExpandHome(path string) string {
	if len(path) < 2 || path[:2] != "~/" {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[2:])
}

func defaultCopilotSourceDir() string {
	if envDir := os.Getenv("COPILOT_HOME"); envDir != "" {
		return envDir
	}
	return "~/.copilot"
}
