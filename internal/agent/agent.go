package agent

import (
	"fmt"

	"github.com/manmart/negent/internal/backend"
)

// SyncType identifies an agent-defined kind of data that can be synced.
type SyncType string

// SyncMode controls how staged files of a given type should be merged.
type SyncMode string

const (
	SyncModeReplace    SyncMode = "replace"
	SyncModeAppendOnly SyncMode = "append-only"
)

// SyncTypeSpec describes one syncable type for an agent.
type SyncTypeSpec struct {
	ID          SyncType
	Label       string
	Description string
	Group       string
	Default     bool
	Mode        SyncMode
	Reference   string
}

// SyncFile represents a file to be synced, with its path relative to the
// agent's namespace in the staging directory.
type SyncFile struct {
	// RelPath is relative to the agent's source directory.
	RelPath string

	// StagingPath is relative to the agent's namespace in the staging dir.
	// Usually the same as RelPath, but may differ for agents that need
	// path translation (e.g., Claude's path-encoded project dirs).
	StagingPath string

	// Type is the agent-defined sync type this file belongs to.
	Type SyncType
}

// PlaceResult summarizes the outcome of placing files from staging into
// the agent's local directory.
type PlaceResult struct {
	Placed    int
	Skipped   int
	Unmatched []string // project dirs that couldn't be matched locally
}

// Agent abstracts how a specific AI assistant's data is collected,
// matched, and placed. Each agent knows its own directory layout, file
// sync types, path conventions, and any special handling.
type Agent interface {
	// Name returns the agent identifier (e.g., "claude", "codex").
	Name() string

	// SourceDir returns the agent's config directory (e.g., ~/.claude).
	SourceDir() string

	// SupportedSyncTypes returns the sync types this agent supports.
	SupportedSyncTypes() []SyncTypeSpec

	// DefaultSyncTypes returns which sync types to sync by default.
	DefaultSyncTypes() []SyncType

	// NormalizeSyncTypes validates a config/CLI selection and expands any
	// legacy aliases into canonical sync type IDs.
	NormalizeSyncTypes(selected []string) ([]SyncType, error)

	// Collect gathers files from the agent's source dir for the given
	// sync types, returning paths relative to the source dir.
	Collect(syncTypes []SyncType) ([]SyncFile, error)

	// Place takes files from the staging dir and writes them to the
	// agent's source dir, handling any agent-specific path translation.
	Place(stagingDir string, files []SyncFile) (*PlaceResult, error)

	// Diff compares local state against staged state for the given sync types,
	// returning changes. Only files that would be collected are considered.
	Diff(stagingDir string, syncTypes []SyncType) ([]backend.FileChange, error)

	// SyncTypeForPath returns the sync type a staging-relative path belongs to,
	// or empty string if it cannot be determined.
	SyncTypeForPath(relPath string) SyncType
}

// StagingMapper is an optional interface that agents can implement to
// remap StagingPaths for cross-machine project matching during push and diff.
// When a project exists in staging under a different path encoding (e.g.,
// Linux vs macOS), the mapper rewrites paths to target the existing staging
// directory instead of creating a duplicate.
type StagingMapper interface {
	// MapStagingPaths rewrites StagingPath fields on collected files to target
	// existing staging directories for cross-machine project equivalents.
	MapStagingPaths(stagingDir string, files []SyncFile) ([]SyncFile, error)
}

// SyncTypeMap returns a lookup map for an agent's supported sync types.
func SyncTypeMap(ag Agent) map[SyncType]SyncTypeSpec {
	specs := ag.SupportedSyncTypes()
	out := make(map[SyncType]SyncTypeSpec, len(specs))
	for _, spec := range specs {
		out[spec.ID] = spec
	}
	return out
}

// SyncTypeSet converts a slice of sync types into a set for O(1) lookup.
func SyncTypeSet(syncTypes []SyncType) map[SyncType]bool {
	set := make(map[SyncType]bool, len(syncTypes))
	for _, st := range syncTypes {
		set[st] = true
	}
	return set
}

// LookupMode returns the sync mode for a type using a pre-built spec map,
// defaulting to replace.
func LookupMode(specs map[SyncType]SyncTypeSpec, syncType SyncType) SyncMode {
	spec, ok := specs[syncType]
	if !ok || spec.Mode == "" {
		return SyncModeReplace
	}
	return spec.Mode
}

// ValidateSyncTypes rejects unknown sync type IDs for an agent.
func ValidateSyncTypes(ag Agent, syncTypes []SyncType) error {
	supported := SyncTypeMap(ag)
	for _, syncType := range syncTypes {
		if _, ok := supported[syncType]; !ok {
			return fmt.Errorf("unsupported sync type %q for %s", syncType, ag.Name())
		}
	}
	return nil
}
