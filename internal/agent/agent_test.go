package agent

import (
	"testing"

	"github.com/manmart/negent/internal/backend"
)

type stubAgent struct {
	specs []SyncTypeSpec
}

func (s stubAgent) Name() string                                              { return "stub" }
func (s stubAgent) SourceDir() string                                         { return "/tmp/stub" }
func (s stubAgent) SupportedSyncTypes() []SyncTypeSpec                        { return s.specs }
func (s stubAgent) DefaultSyncTypes() []SyncType                              { return nil }
func (s stubAgent) NormalizeSyncTypes(_ []string) ([]SyncType, error)         { return nil, nil }
func (s stubAgent) Collect(_ []SyncType) ([]SyncFile, error)                  { return nil, nil }
func (s stubAgent) Place(_ string, _ []SyncFile) (*PlaceResult, error)        { return nil, nil }
func (s stubAgent) Diff(_ string, _ []SyncType) ([]backend.FileChange, error) { return nil, nil }
func (s stubAgent) SyncTypeForPath(_ string) SyncType                         { return "" }

func TestSyncTypeMap(t *testing.T) {
	ag := stubAgent{
		specs: []SyncTypeSpec{
			{ID: "claude-md", Mode: SyncModeReplace},
			{ID: "sessions", Mode: SyncModeAppendOnly},
		},
	}

	got := SyncTypeMap(ag)
	if len(got) != 2 {
		t.Fatalf("SyncTypeMap() returned %d specs, want 2", len(got))
	}
	if got["sessions"].Mode != SyncModeAppendOnly {
		t.Fatalf("sessions mode = %q, want %q", got["sessions"].Mode, SyncModeAppendOnly)
	}
}

func TestLookupModeDefaultsToReplace(t *testing.T) {
	specs := map[SyncType]SyncTypeSpec{
		"claude-md": {ID: "claude-md"},
	}

	if mode := LookupMode(specs, "claude-md"); mode != SyncModeReplace {
		t.Fatalf("LookupMode() = %q, want %q", mode, SyncModeReplace)
	}
	if mode := LookupMode(specs, "missing"); mode != SyncModeReplace {
		t.Fatalf("LookupMode() for missing type = %q, want %q", mode, SyncModeReplace)
	}
}

func TestSyncTypeSet(t *testing.T) {
	set := SyncTypeSet([]SyncType{"claude-md", "sessions"})
	if len(set) != 2 {
		t.Fatalf("SyncTypeSet() returned %d entries, want 2", len(set))
	}
	if !set["claude-md"] || !set["sessions"] {
		t.Fatal("SyncTypeSet() missing expected entries")
	}
}

func TestValidateSyncTypes(t *testing.T) {
	ag := stubAgent{specs: []SyncTypeSpec{{ID: "claude-md"}, {ID: "sessions"}}}

	if err := ValidateSyncTypes(ag, []SyncType{"claude-md", "sessions"}); err != nil {
		t.Fatalf("ValidateSyncTypes() returned unexpected error: %v", err)
	}
	if err := ValidateSyncTypes(ag, []SyncType{"missing"}); err == nil {
		t.Fatal("ValidateSyncTypes() should reject unsupported sync types")
	}
}
