package cmd

import (
	"strings"
	"testing"

	"github.com/manmart/negent/internal/config"
)

func TestApplyNonInteractiveConfigEditsRejectsUnknownAgent(t *testing.T) {
	configEnableAgentsFlag = []string{"does-not-exist"}
	configDisableAgentsFlag = nil
	configBackendFlag = ""
	configRepoFlag = ""
	configMachineFlag = ""
	defer func() {
		configEnableAgentsFlag = nil
		configDisableAgentsFlag = nil
	}()

	cfg := &config.Config{Agents: map[string]config.AgentConfig{}}
	err := applyNonInteractiveConfigEdits(cfg)
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
	if !strings.Contains(err.Error(), `unknown agent "does-not-exist"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
