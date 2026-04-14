package sync

// SyncPushInput is the input for pushing snapshots to remote.
type SyncPushInput struct {
	Project string // Project name (required)
	All     bool   // Push all projects
}

// SyncPushResult is the result of a push operation.
type SyncPushResult struct {
	Success     bool   `json:"success"`
	Project     string `json:"project"`
	FilesPushed int    `json:"files_pushed"`
	CommitHash  string `json:"commit_hash"`
	RemoteURL   string `json:"remote_url"`
	Verified    bool   `json:"verified"`
	Error       string `json:"error,omitempty"`
}

// SyncPullInput is the input for pulling snapshots from remote.
type SyncPullInput struct {
	Project string // Project name (required)
	All     bool   // Pull all projects
}

// SyncPullResult is the result of a pull operation.
type SyncPullResult struct {
	Success       bool           `json:"success"`
	Project       string         `json:"project"`
	FilesUpdated  int            `json:"files_updated"`
	HasConflicts  bool           `json:"has_conflicts"`
	ConflictCount int            `json:"conflict_count"`
	Conflicts     []ConflictInfo `json:"conflicts,omitempty"`
	Error         string         `json:"error,omitempty"`
}

// ConflictInfo describes a file conflict detected during sync pull.
type ConflictInfo struct {
	Path          string `json:"path"`
	ConflictType  string `json:"conflict_type"`  // "both_modified", "local_modified_remote_added", etc.
	LocalSummary  string `json:"local_summary"`  // Brief summary of local version
	RemoteSummary string `json:"remote_summary"` // Brief summary of remote version
}

// SyncStatusInput is the input for checking sync status.
type SyncStatusInput struct {
	Project string // Project name (optional)
}

// SyncStatusResult is the result of a sync status check.
type SyncStatusResult struct {
	Project     string `json:"project"`
	RemoteURL   string `json:"remote_url"`
	Branch      string `json:"branch"`
	AheadCount  int    `json:"ahead_count"`
	BehindCount int    `json:"behind_count"`
	LastSynced  string `json:"last_synced,omitempty"`
	IsSynced    bool   `json:"is_synced"`
}
