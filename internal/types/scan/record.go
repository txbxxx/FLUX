package scan

// CustomRuleRecord carries custom sync rule data across the Service-UseCase boundary.
type CustomRuleRecord struct {
	ToolType     string `json:"tool_type" yaml:"tool_type"`
	AbsolutePath string `json:"absolute_path" yaml:"absolute_path"`
}

// RegisteredProjectRecord carries registered project data across the Service-UseCase boundary.
type RegisteredProjectRecord struct {
	ToolType    string `json:"tool_type" yaml:"tool_type"`
	ProjectName string `json:"project_name" yaml:"project_name"`
	ProjectPath string `json:"project_path" yaml:"project_path"`
}
