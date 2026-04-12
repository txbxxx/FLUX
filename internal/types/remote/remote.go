package remote

import "time"

// AddRemoteInput is the input for adding a remote repository.
type AddRemoteInput struct {
	URL      string // Git repository URL
	Name     string // Configuration name (optional, derived from URL if empty)
	Branch   string // Branch name (default: main)
	AuthType string // Auth type: "" / "ssh" / "token" / "basic"
	Token    string // Personal access token (when AuthType=token)
	Username string // Username (when AuthType=basic)
	Password string // Password (when AuthType=basic)
	SSHKey   string // SSH key path (when AuthType=ssh)
	Project  string // Bind to project name (optional)
}

// AddRemoteResult is the result of adding a remote repository.
type AddRemoteResult struct {
	ConfigID  string `json:"config_id"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	Branch    string `json:"branch"`
	Connected bool   `json:"connected"`
	Project   string `json:"project,omitempty"`
}

// ListRemotesResult is the result of listing remote configurations.
type ListRemotesResult struct {
	Remotes []RemoteItem `json:"remotes"`
}

// RemoteItem is a summary of a single remote configuration.
type RemoteItem struct {
	Name       string     `json:"name"`
	URL        string     `json:"url"`
	Branch     string     `json:"branch"`
	IsDefault  bool       `json:"is_default"`
	Status     string     `json:"status"`
	LastSynced *time.Time `json:"last_synced,omitempty"`
	Projects   []string   `json:"projects"`
}

// RemoveRemoteInput is the input for removing a remote configuration.
type RemoveRemoteInput struct {
	Name  string // Configuration name
	Force bool   // Force removal even if bound to projects
}
