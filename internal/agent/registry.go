package agent

import "os"

// KnownAgent describes a built-in agent that negent knows how to sync.
type KnownAgent struct {
	Name      string
	SourceDir string // default source directory (with ~ unexpanded)
}

// KnownAgents returns the list of agents negent has built-in support for.
func KnownAgents() []KnownAgent {
	return []KnownAgent{
		{Name: "claude", SourceDir: "~/.claude"},
		{Name: "codex", SourceDir: "~/.codex"},
		{Name: "copilot", SourceDir: "~/.copilot"},
		{Name: "kiro", SourceDir: "~/.kiro"},
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
	return home + path[1:]
}
