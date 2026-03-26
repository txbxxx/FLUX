package tool

// SyncRuleDefinition 表示统一规则层的单条规则定义。
type SyncRuleDefinition struct {
	ToolType ToolType
	Scope    ConfigScope
	Path     string
	Category ConfigCategory
	IsDir    bool
}

// DefaultGlobalRules 返回工具的默认全局规则。
func DefaultGlobalRules(toolType ToolType) []SyncRuleDefinition {
	switch toolType {
	case ToolTypeCodex:
		return []SyncRuleDefinition{
			{ToolType: toolType, Scope: ScopeGlobal, Path: "config.toml", Category: CategoryConfigFile, IsDir: false},
			{ToolType: toolType, Scope: ScopeGlobal, Path: "skills", Category: CategorySkills, IsDir: true},
			{ToolType: toolType, Scope: ScopeGlobal, Path: "rules", Category: CategoryRules, IsDir: true},
			{ToolType: toolType, Scope: ScopeGlobal, Path: "superpowers", Category: CategoryConfigFile, IsDir: true},
		}
	case ToolTypeClaude:
		return []SyncRuleDefinition{
			{ToolType: toolType, Scope: ScopeGlobal, Path: "settings.json", Category: CategoryConfigFile, IsDir: false},
			{ToolType: toolType, Scope: ScopeGlobal, Path: "CLAUDE.md", Category: CategoryConfigFile, IsDir: false},
			{ToolType: toolType, Scope: ScopeGlobal, Path: "commands", Category: CategoryCommands, IsDir: true},
			{ToolType: toolType, Scope: ScopeGlobal, Path: "skills", Category: CategorySkills, IsDir: true},
			{ToolType: toolType, Scope: ScopeGlobal, Path: "output-styles", Category: CategoryConfigFile, IsDir: true},
			{ToolType: toolType, Scope: ScopeGlobal, Path: "plugins/blocklist.json", Category: CategoryPlugins, IsDir: false},
			{ToolType: toolType, Scope: ScopeGlobal, Path: "plugins/installed_plugins.json", Category: CategoryPlugins, IsDir: false},
			{ToolType: toolType, Scope: ScopeGlobal, Path: "plugins/known_marketplaces.json", Category: CategoryPlugins, IsDir: false},
		}
	default:
		return nil
	}
}

// ProjectRuleTemplates 返回工具的项目规则模板。
func ProjectRuleTemplates(toolType ToolType) []SyncRuleDefinition {
	switch toolType {
	case ToolTypeCodex:
		return []SyncRuleDefinition{
			{ToolType: toolType, Scope: ScopeProject, Path: ".codex", Category: CategoryConfigFile, IsDir: true},
			{ToolType: toolType, Scope: ScopeProject, Path: "AGENTS.md", Category: CategoryAgents, IsDir: false},
		}
	case ToolTypeClaude:
		return []SyncRuleDefinition{
			{ToolType: toolType, Scope: ScopeProject, Path: ".claude", Category: CategoryConfigFile, IsDir: true},
			{ToolType: toolType, Scope: ScopeProject, Path: "CLAUDE.md", Category: CategoryConfigFile, IsDir: false},
		}
	default:
		return nil
	}
}

// RequiredGlobalRulePaths 返回全局关键文件路径。
func RequiredGlobalRulePaths(toolType ToolType) []string {
	switch toolType {
	case ToolTypeCodex:
		return []string{"config.toml"}
	case ToolTypeClaude:
		return []string{"settings.json"}
	default:
		return nil
	}
}
