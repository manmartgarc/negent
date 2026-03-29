package cmd

import (
	"testing"

	"github.com/manmart/negent/internal/agent/claude"
	"github.com/manmart/negent/internal/config"
)

func TestBuildAgentsNormalizesLegacyClaudeSyncKeys(t *testing.T) {
	cfg := &config.Config{
		Agents: map[string]config.AgentConfig{
			"claude": {
				Source: "~/.claude",
				Sync:   []string{"config", "custom-code", "memory", "sessions", "history"},
			},
		},
	}

	_, syncTypes, err := buildAgents(cfg)
	if err != nil {
		t.Fatalf("buildAgents() returned error: %v", err)
	}

	want := []string{
		string(claude.SyncTypeClaudeMD),
		string(claude.SyncTypeCommands),
		string(claude.SyncTypeSkills),
		string(claude.SyncTypeAgents),
		string(claude.SyncTypeRules),
		string(claude.SyncTypeOutputStyle),
		string(claude.SyncTypeAgentMemory),
		string(claude.SyncTypeAutoMemory),
		string(claude.SyncTypeSessions),
		string(claude.SyncTypeHistory),
	}

	got := syncTypes["claude"]
	if len(got) != len(want) {
		t.Fatalf("buildAgents() returned %d sync types, want %d", len(got), len(want))
	}
	for i, syncType := range got {
		if string(syncType) != want[i] {
			t.Fatalf("syncTypes[%d] = %q, want %q", i, syncType, want[i])
		}
	}
}

func TestDefaultSyncTypeStrings(t *testing.T) {
	ag := claude.New("")
	got := defaultSyncTypeStrings(ag)
	if len(got) == 0 {
		t.Fatal("defaultSyncTypeStrings() returned no defaults")
	}
	if got[0] != string(claude.SyncTypeClaudeMD) {
		t.Fatalf("defaultSyncTypeStrings()[0] = %q, want %q", got[0], claude.SyncTypeClaudeMD)
	}
}
