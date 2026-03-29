package backend

import "context"

// ChangeKind describes how a file changed.
type ChangeKind string

const (
	ChangeAdded    ChangeKind = "added"
	ChangeModified ChangeKind = "modified"
	ChangeDeleted  ChangeKind = "deleted"
)

// FileChange represents a single file difference between local staging and remote.
type FileChange struct {
	Path string
	Kind ChangeKind
}

// BackendConfig holds backend-specific configuration.
// Each backend implementation extracts what it needs from this map.
type BackendConfig map[string]string

// Backend abstracts the remote storage layer.
// Git is the first implementation; future: S3, SSH/rsync.
type Backend interface {
	// Init sets up the backend (e.g., clone repo, create bucket, verify SSH access).
	Init(ctx context.Context, cfg BackendConfig) error

	// Fetch downloads the latest remote state without updating the working tree.
	// This allows conflict detection before merging remote changes locally.
	Fetch(ctx context.Context) error

	// FetchedFiles returns staging-relative paths of files that differ between
	// the current HEAD and the most recent fetch. Returns nil if nothing was fetched
	// or the remote has no new changes.
	FetchedFiles(ctx context.Context) ([]string, error)

	// Push writes the local staging directory to the remote.
	Push(ctx context.Context, msg string) error

	// Pull integrates previously fetched remote changes into the working tree.
	Pull(ctx context.Context) error

	// Status returns the diff between local staging and remote.
	Status(ctx context.Context) ([]FileChange, error)

	// StagingDir returns the path to the local working copy / staging area.
	StagingDir() string
}
