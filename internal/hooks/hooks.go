package hooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// isNegentCommand reports whether command was installed by negent.
// It matches both bare "negent" and full-path invocations like
// "/usr/local/bin/negent" by comparing the base name of the first word.
func isNegentCommand(command string) bool {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return false
	}
	return filepath.Base(fields[0]) == "negent"
}

// hookEntry is one command entry within a hook matcher.
type hookEntry struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// hookMatcher is one element in a per-event hook array.
type hookMatcher struct {
	Hooks []hookEntry `json:"hooks"`
}

// DefaultClaudeSettingsPath returns the path to Claude Code's settings file.
func DefaultClaudeSettingsPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = os.Getenv("HOME")
	}
	if home == "" {
		home = os.TempDir()
	}
	return filepath.Join(home, ".claude", "settings.json")
}

// InstallClaude installs negent pull/push hooks into the Claude Code settings
// file at settingsPath. negentBin is the absolute path to the negent binary
// (typically obtained via os.Executable()).
//
// The function is idempotent: calling it twice produces the same result.
// Existing settings and hooks from other tools are preserved unchanged.
// If the file does not exist it is created. Invalid JSON causes an error
// without modifying the file.
func InstallClaude(settingsPath, negentBin string) error {
	top, err := readSettings(settingsPath)
	if err != nil {
		return err
	}

	hooksMap, err := extractHooksMap(top)
	if err != nil {
		return err
	}

	pullCmd := negentBin + " pull --quiet"
	pushCmd := negentBin + " push --quiet"

	hooksMap["SessionStart"] = upsertNegentMatcher(hooksMap["SessionStart"], pullCmd)
	hooksMap["Stop"] = upsertNegentMatcher(hooksMap["Stop"], pushCmd)

	return writeSettings(settingsPath, top, hooksMap)
}

// UninstallClaude removes all negent-installed hooks from the Claude Code
// settings file. Hooks from other tools are preserved. Returns nil if the
// file does not exist.
func UninstallClaude(settingsPath string) error {
	if _, err := os.Stat(settingsPath); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	top, err := readSettings(settingsPath)
	if err != nil {
		return err
	}

	hooksMap, err := extractHooksMap(top)
	if err != nil {
		return err
	}

	for event, matchers := range hooksMap {
		filtered := removeNegentMatchers(matchers)
		if len(filtered) == 0 {
			delete(hooksMap, event)
		} else {
			hooksMap[event] = filtered
		}
	}

	if len(hooksMap) == 0 {
		delete(top, "hooks")
		return writeSettingsRaw(settingsPath, top)
	}

	return writeSettings(settingsPath, top, hooksMap)
}

// StatusClaude reports whether negent-installed hooks are present in the
// Claude Code settings file. Returns (false, nil) if the file does not exist
// or no negent hooks are found. Returns an error only if the file exists but
// cannot be parsed.
func StatusClaude(settingsPath string) (bool, error) {
	if _, err := os.Stat(settingsPath); errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	top, err := readSettings(settingsPath)
	if err != nil {
		return false, err
	}

	hooksMap, err := extractHooksMap(top)
	if err != nil {
		return false, err
	}

	for _, matchers := range hooksMap {
		for _, m := range matchers {
			for _, e := range m.Hooks {
				if isNegentCommand(e.Command) {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

// readSettings reads settingsPath and unmarshals it into a raw map.
// Returns an empty map (not an error) if the file does not exist.
func readSettings(settingsPath string) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(settingsPath)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]json.RawMessage{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", settingsPath, err)
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", settingsPath, err)
	}
	return top, nil
}

// extractHooksMap extracts the "hooks" key from top as a map of event →
// []hookMatcher. Returns an empty map if the key is absent.
func extractHooksMap(top map[string]json.RawMessage) (map[string][]hookMatcher, error) {
	raw, ok := top["hooks"]
	if !ok {
		return map[string][]hookMatcher{}, nil
	}

	var hooksMap map[string][]hookMatcher
	if err := json.Unmarshal(raw, &hooksMap); err != nil {
		return nil, fmt.Errorf("parsing hooks section: %w", err)
	}
	return hooksMap, nil
}

// writeSettings serialises hooksMap back into top and writes the result.
func writeSettings(settingsPath string, top map[string]json.RawMessage, hooksMap map[string][]hookMatcher) error {
	raw, err := json.Marshal(hooksMap)
	if err != nil {
		return fmt.Errorf("serialising hooks: %w", err)
	}
	top["hooks"] = raw
	return writeSettingsRaw(settingsPath, top)
}

// writeSettingsRaw marshals top and atomically writes it to settingsPath.
func writeSettingsRaw(settingsPath string, top map[string]json.RawMessage) error {
	data, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return fmt.Errorf("serialising settings: %w", err)
	}
	return writeFileAtomic(settingsPath, data)
}

// upsertNegentMatcher ensures exactly one negent-owned hookMatcher exists in
// matchers for the given command. If one already exists (identified by
// isNegentCommand), its command is updated. Otherwise a new matcher is appended.
func upsertNegentMatcher(matchers []hookMatcher, command string) []hookMatcher {
	for i, m := range matchers {
		for j, e := range m.Hooks {
			if isNegentCommand(e.Command) {
				matchers[i].Hooks[j].Command = command
				return matchers
			}
		}
	}
	return append(matchers, hookMatcher{
		Hooks: []hookEntry{{Type: "command", Command: command}},
	})
}

// removeNegentMatchers returns a copy of matchers with any matcher that
// contains a negent-owned hookEntry removed.
func removeNegentMatchers(matchers []hookMatcher) []hookMatcher {
	var out []hookMatcher
	for _, m := range matchers {
		if !hasNegentEntry(m) {
			out = append(out, m)
		}
	}
	return out
}

func hasNegentEntry(m hookMatcher) bool {
	for _, e := range m.Hooks {
		if isNegentCommand(e.Command) {
			return true
		}
	}
	return false
}

// writeFileAtomic writes data to path via a temporary file + rename so that
// a crash mid-write cannot leave path in a corrupt state.
func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating parent directory: %w", err)
	}
	tmp := path + ".negent.tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		removeErr := os.Remove(tmp)
		if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return fmt.Errorf("renaming temp file: %w (cleanup failed: %v)", err, removeErr)
		}
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}
