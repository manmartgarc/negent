package e2e_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var testHarness harness

type harness struct {
	repoRoot   string
	workDir    string
	binaryPath string
}

type Machine struct {
	Name        string
	RootDir     string
	HomeDir     string
	ConfigHome  string
	DataHome    string
	CopilotHome string
}

type RunOptions struct {
	Dir string
	Env map[string]string
}

type CommandResult struct {
	Stdout string
	Stderr string
	Err    error
}

func TestMain(m *testing.M) {
	if err := setUpHarness(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	code := m.Run()
	if err := os.RemoveAll(testHarness.workDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if code == 0 {
			code = 1
		}
	}

	os.Exit(code)
}

func setUpHarness() error {
	repoRoot, err := repoRoot()
	if err != nil {
		return err
	}

	workDir, err := os.MkdirTemp(repoRoot, ".negent-e2e-")
	if err != nil {
		return fmt.Errorf("creating e2e workspace: %w", err)
	}

	testHarness = harness{
		repoRoot:   repoRoot,
		workDir:    workDir,
		binaryPath: filepath.Join(workDir, "negent"),
	}

	if err := os.MkdirAll(filepath.Join(workDir, "xdg-config"), 0o755); err != nil {
		return fmt.Errorf("creating harness config dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(workDir, "xdg-data"), 0o755); err != nil {
		return fmt.Errorf("creating harness data dir: %w", err)
	}

	cmd := exec.Command("go", "build", "-o", testHarness.binaryPath, ".")
	cmd.Dir = testHarness.repoRoot
	cmd.Env = os.Environ()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("building e2e binary: %w\n%s", err, out)
	}

	return nil
}

func repoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("locating e2e package")
	}
	return filepath.Dir(filepath.Dir(file)), nil
}

func newMachine(t *testing.T, name string) *Machine {
	t.Helper()

	rootDir := newTestDir(t, name)
	machine := &Machine{
		Name:       name,
		RootDir:    rootDir,
		HomeDir:    filepath.Join(rootDir, "home"),
		ConfigHome: filepath.Join(rootDir, "xdg", "config"),
		DataHome:   filepath.Join(rootDir, "xdg", "data"),
	}

	mustMkdirAll(t, machine.HomeDir)
	mustMkdirAll(t, machine.ConfigHome)
	mustMkdirAll(t, machine.DataHome)

	return machine
}

func (m *Machine) ConfigPath() string {
	return filepath.Join(m.ConfigHome, "negent", "config.yaml")
}

func (m *Machine) StagingDir() string {
	return filepath.Join(m.DataHome, "negent", "repo")
}

func (m *Machine) EnableCopilotHome(t *testing.T) string {
	t.Helper()

	if m.CopilotHome == "" {
		m.CopilotHome = filepath.Join(m.RootDir, "copilot-home")
	}
	mustMkdirAll(t, m.CopilotHome)
	return m.CopilotHome
}

func (m *Machine) AgentHome(agentName string) string {
	switch agentName {
	case "claude":
		return filepath.Join(m.HomeDir, ".claude")
	case "copilot":
		if m.CopilotHome != "" {
			return m.CopilotHome
		}
		return filepath.Join(m.HomeDir, ".copilot")
	default:
		return filepath.Join(m.HomeDir, "."+agentName)
	}
}

func (m *Machine) SeedAgentFile(t *testing.T, agentName, relativePath, contents string) string {
	t.Helper()

	return writeFile(t, filepath.Join(m.AgentHome(agentName), relativePath), contents)
}

func (m *Machine) envMap() map[string]string {
	env := map[string]string{
		"HOME":            m.HomeDir,
		"XDG_CONFIG_HOME": m.ConfigHome,
		"XDG_DATA_HOME":   m.DataHome,
	}
	if m.CopilotHome != "" {
		env["COPILOT_HOME"] = m.CopilotHome
	}
	return env
}

func newBareRemote(t *testing.T) string {
	t.Helper()

	remoteRoot := newTestDir(t, "remote")
	remotePath := filepath.Join(remoteRoot, "remote.git")
	mustMkdirAll(t, filepath.Dir(remotePath))

	result := runCommand(filepath.Dir(remotePath), nil, "git", "init", "--bare", "--initial-branch=main", remotePath)
	if result.Err != nil {
		t.Fatalf("init bare remote: %v\nstdout:\n%s\nstderr:\n%s", result.Err, result.Stdout, result.Stderr)
	}

	return remotePath
}

func runNegent(t *testing.T, machine *Machine, opts RunOptions, args ...string) CommandResult {
	t.Helper()

	env := machine.envMap()
	for key, value := range opts.Env {
		env[key] = value
	}

	dir := opts.Dir
	if dir == "" {
		dir = testHarness.repoRoot
	}

	return runCommand(dir, env, testHarness.binaryPath, args...)
}

func runCommand(dir string, env map[string]string, name string, args ...string) CommandResult {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = mergedEnv(env)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return CommandResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
		Err:    err,
	}
}

func mergedEnv(overrides map[string]string) []string {
	env := map[string]string{
		"GIT_AUTHOR_EMAIL":    "negent-e2e@example.com",
		"GIT_AUTHOR_NAME":     "Negent E2E",
		"GIT_COMMITTER_EMAIL": "negent-e2e@example.com",
		"GIT_COMMITTER_NAME":  "Negent E2E",
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_TERMINAL_PROMPT": "0",
		"HOME":                testHarness.workDir,
		"LANG":                "C",
		"PATH":                os.Getenv("PATH"),
		"TERM":                "dumb",
		"XDG_CONFIG_HOME":     filepath.Join(testHarness.workDir, "xdg-config"),
		"XDG_DATA_HOME":       filepath.Join(testHarness.workDir, "xdg-data"),
	}

	for key, value := range overrides {
		env[key] = value
	}

	out := make([]string, 0, len(env))
	for key, value := range env {
		out = append(out, key+"="+value)
	}

	return out
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()

	result := runCommand(dir, nil, "git", args...)
	if result.Err != nil {
		t.Fatalf("git %s: %v\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), result.Err, result.Stdout, result.Stderr)
	}
	return strings.TrimSpace(result.Stdout)
}

func newTestDir(t *testing.T, prefix string) string {
	t.Helper()

	dir, err := os.MkdirTemp(testHarness.workDir, sanitizeName(prefix)+"-")
	if err != nil {
		t.Fatalf("creating test dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})
	return dir
}

func sanitizeName(name string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", " ", "-")
	return replacer.Replace(name)
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func writeFile(t *testing.T, path, contents string) string {
	t.Helper()

	mustMkdirAll(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}
