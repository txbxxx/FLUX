package snapshot

import "time"

// UpdateSnapshotResult holds the result of a snapshot update operation.
type UpdateSnapshotResult struct {
	SnapshotID     uint      `json:"snapshot_id"`
	SnapshotName   string    `json:"snapshot_name"`
	FilesUpdated   int       `json:"files_updated"`
	FilesAdded     int       `json:"files_added"`
	FilesRemoved   int       `json:"files_removed"`
	FilesUnchanged int       `json:"files_unchanged"`
	CommitHash     string    `json:"commit_hash,omitempty"`
	NoChanges      bool      `json:"no_changes"`
	UpdatedAt      time.Time `json:"updated_at"`
}
