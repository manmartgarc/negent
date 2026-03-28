package agent

import "github.com/manmart/negent/internal/backend"

// Category represents a type of data that can be synced.
type Category string

const (
	CategoryConfig     Category = "config"
	CategoryCustomCode Category = "custom-code"
	CategoryMemory     Category = "memory"
	CategorySessions   Category = "sessions"
	CategoryHistory    Category = "history"
	CategoryPlugins    Category = "plugins"
)

// AllCategories returns all defined sync categories.
func AllCategories() []Category {
	return []Category{
		CategoryConfig,
		CategoryCustomCode,
		CategoryMemory,
		CategorySessions,
		CategoryHistory,
		CategoryPlugins,
	}
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

	// Category this file belongs to.
	Category Category
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
// categories, path conventions, and any special handling.
type Agent interface {
	// Name returns the agent identifier (e.g., "claude", "codex").
	Name() string

	// SourceDir returns the agent's config directory (e.g., ~/.claude).
	SourceDir() string

	// Collect gathers files from the agent's source dir for the given
	// categories, returning paths relative to the source dir.
	Collect(categories []Category) ([]SyncFile, error)

	// Place takes files from the staging dir and writes them to the
	// agent's source dir, handling any agent-specific path translation.
	Place(stagingDir string, files []SyncFile) (*PlaceResult, error)

	// Diff compares local state against staged state, returning changes.
	Diff(stagingDir string) ([]backend.FileChange, error)

	// DefaultCategories returns which categories to sync by default.
	DefaultCategories() []Category
}
