package tool

import "flux/pkg/config"

// SyncRuleDefinition 表示统一规则层的单条规则定义。
// 无论是内置默认规则还是从 YAML 配置加载的自定义规则，最终都统一为此结构体。
type SyncRuleDefinition struct {
	ToolType ToolType        // 规则所属的工具类型（codex / claude）
	Scope    ConfigScope     // 规则作用域：global（全局配置目录）或 project（项目目录）
	Path     string          // 相对于配置根目录的路径，如 "config.toml"、"commands"
	Category ConfigCategory  // 配置类别，用于分组展示和路由（如 skills、commands、plugins）
	IsDir    bool            // 规则目标是否为目录；目录类型会递归扫描子文件
}

// DefaultGlobalRules 返回工具的默认全局规则。
// 优先从 YAML 配置读取，回退到硬编码默认值。
func DefaultGlobalRules(toolType ToolType) []SyncRuleDefinition {
	if tc, ok := toolsCfg[string(toolType)]; ok && len(tc.GlobalRules) > 0 {
		return convertRules(tc.GlobalRules, toolType, ScopeGlobal)
	}

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
// 优先从 YAML 配置读取，回退到硬编码默认值。
func ProjectRuleTemplates(toolType ToolType) []SyncRuleDefinition {
	if tc, ok := toolsCfg[string(toolType)]; ok && len(tc.ProjectRules) > 0 {
		return convertRules(tc.ProjectRules, toolType, ScopeProject)
	}

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
// 优先从 YAML 配置读取，回退到硬编码默认值。
func RequiredGlobalRulePaths(toolType ToolType) []string {
	if tc, ok := toolsCfg[string(toolType)]; ok && len(tc.RequiredGlobalPaths) > 0 {
		return tc.RequiredGlobalPaths
	}

	switch toolType {
	case ToolTypeCodex:
		return []string{"config.toml"}
	case ToolTypeClaude:
		return []string{"settings.json"}
	default:
		return nil
	}
}

// convertRules 将配置规则定义转换为内部 SyncRuleDefinition。
func convertRules(rules []config.RuleDef, toolType ToolType, scope ConfigScope) []SyncRuleDefinition {
	result := make([]SyncRuleDefinition, len(rules))
	for i, r := range rules {
		result[i] = SyncRuleDefinition{
			ToolType: toolType,
			Scope:    scope,
			Path:     r.Path,
			Category: ConfigCategory(r.Category),
			IsDir:    r.IsDir,
		}
	}
	return result
}
