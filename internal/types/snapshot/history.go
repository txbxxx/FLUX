package snapshot

import "time"

// HistoryEntry represents a single version in the snapshot history.
type HistoryEntry struct {
	CommitHash string    `json:"commit_hash"`
	Message    string    `json:"message"`
	Author     string    `json:"author"`
	Date       time.Time `json:"date"`
}

// HistoryResult is the result of viewing snapshot history.
type HistoryResult struct {
	Project string         `json:"project"`
	Entries []HistoryEntry `json:"entries"`
	Total   int            `json:"total"`
}
