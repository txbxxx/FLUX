package snapshot

import "time"

// SnapshotHeader holds the essential snapshot identity fields
// returned as part of a SnapshotPackage.
type SnapshotHeader struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
	Project   string    `json:"project"`
}

// SnapshotListItem holds snapshot summary data for list display.
type SnapshotListItem struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
	Project   string    `json:"project"`
	FileCount int       `json:"file_count"`
}
