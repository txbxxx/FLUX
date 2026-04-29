package tool

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ResolvedRuleMatch 表示一条规则在文件系统上的实际命中结果。
// 将规则定义与磁盘真实文件/目录关联起来，包含文件大小、修改时间等运行时信息。
type ResolvedRuleMatch struct {
	ToolType     ToolType       // 命中文件所属的工具类型（codex / claude）
	Scope        ConfigScope    // 规则作用域：global 或 project
	RelativePath string         // 相对于配置根目录的路径（如 "skills/xxx.md"）
	AbsolutePath string         // 文件/目录在磁盘上的绝对路径
	Category     ConfigCategory // 配置类别（skills / commands / plugins 等）
	IsDir        bool           // 是否为目录
	Size         int64          // 文件大小（字节），目录时为 0
	ModifiedAt   time.Time      // 最后修改时间
}

// ResolvedProjectMatch 记录某个已注册项目命中的全部规则结果。
type ResolvedProjectMatch struct {
	ProjectName string              // 项目名称（如 "demo"、"flux"）
	ProjectPath string              // 项目根目录的绝对路径
	Matches     []ResolvedRuleMatch // 该项目下所有命中的配置文件/目录
}

// ToolRuleReport 是某个工具在当前机器上的完整规则解析报告。
// 包含全局扫描结果、自定义规则命中、已注册项目扫描结果，以及缺失的关键路径。
type ToolRuleReport struct {
	ToolType             ToolType               // 工具类型（codex / claude）
	GlobalPath           string                 // 全局配置根目录的绝对路径（如 ~/.codex）
	Status               InstallationStatus     // 综合安装状态（installed / partial / not_installed）
	MissingRequiredPaths []string               // 缺失的关键文件列表（如 config.toml 不存在时记录于此）
	DefaultMatches       []ResolvedRuleMatch    // 内置默认规则命中的文件列表
	CustomMatches        []ResolvedRuleMatch    // 用户自定义规则命中的文件列表
	MissingCustomRules   []ResolvedRuleMatch    // 用户自定义规则中路径在磁盘上不存在的条目
	ProjectMatches       []ResolvedProjectMatch // 所有已注册项目的扫描结果
}

// RuleResolver 负责把默认规则、项目模板和自定义规则解析成当前命中结果。
type RuleResolver struct {
	store *RuleStore
}

// NewRuleResolver 创建规则解析器；store 为空时只解析内置默认规则。
func NewRuleResolver(store *RuleStore) *RuleResolver {
	return &RuleResolver{store: store}
}

// ResolveTool 把默认规则、自定义规则和项目规则统一解析成一个报告。
func (r *RuleResolver) ResolveTool(toolType ToolType) (*ToolRuleReport, error) {
	report := &ToolRuleReport{
		ToolType:   toolType,
		GlobalPath: GetDefaultGlobalPath(toolType),
	}

	// 默认规则只依赖全局根目录，但 resolver 不能在这里提前返回，
	// 否则自定义绝对文件和已注册项目会被误判为“不存在”。
	globalInstalled := dirExists(report.GlobalPath)
	if globalInstalled {
		for _, requiredPath := range RequiredGlobalRulePaths(toolType) {
			if !fileExists(filepath.Join(report.GlobalPath, requiredPath)) {
				report.MissingRequiredPaths = append(report.MissingRequiredPaths, requiredPath)
			}
		}

		defaultMatches, err := resolveRuleDefinitions(report.GlobalPath, DefaultGlobalRules(toolType))
		if err != nil {
			return nil, err
		}
		report.DefaultMatches = defaultMatches
	}

	if r != nil && r.store != nil {
		customRules, err := r.store.ListCustomRules(toolType)
		if err != nil {
			return nil, err
		}
		for _, rule := range customRules {
			match, ok, err := resolveAbsoluteFileRule(toolType, rule.AbsolutePath)
			if err != nil {
				return nil, err
			}
			if ok {
				report.CustomMatches = append(report.CustomMatches, match)
			} else {
				report.MissingCustomRules = append(report.MissingCustomRules, ResolvedRuleMatch{
					ToolType:     toolType,
					Scope:        ScopeGlobal,
					AbsolutePath: rule.AbsolutePath,
				})
			}
		}

		projects, err := r.store.ListRegisteredProjects(toolType)
		if err != nil {
			return nil, err
		}
		for _, project := range projects {
			matches, err := resolveRuleDefinitions(project.ProjectPath, ProjectRuleTemplates(toolType))
			if err != nil {
				return nil, err
			}
			if len(matches) == 0 {
				continue
			}
			report.ProjectMatches = append(report.ProjectMatches, ResolvedProjectMatch{
				ProjectName: project.ProjectName,
				ProjectPath: project.ProjectPath,
				Matches:     matches,
			})
		}
	}

	matchCount := len(report.DefaultMatches) + len(report.CustomMatches)
	for _, project := range report.ProjectMatches {
		matchCount += len(project.Matches)
	}

	switch {
	case !globalInstalled && matchCount == 0:
		report.Status = StatusNotInstalled
	case !globalInstalled:
		report.Status = StatusPartial
	case len(report.MissingRequiredPaths) > 0:
		report.Status = StatusPartial
	case matchCount == 0:
		report.Status = StatusPartial
	default:
		report.Status = StatusInstalled
	}

	return report, nil
}

// resolveRuleDefinitions 把规则定义映射成当前文件系统上真实存在的命中项。
func resolveRuleDefinitions(basePath string, rules []SyncRuleDefinition) ([]ResolvedRuleMatch, error) {
	matches := make([]ResolvedRuleMatch, 0, len(rules))
	for _, rule := range rules {
		var absolutePath string
		if rule.IsAbsolute {
			absolutePath = expandPath(rule.Path)
		} else {
			absolutePath = filepath.Join(basePath, rule.Path)
		}

		info, err := os.Stat(absolutePath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		if rule.IsDir != info.IsDir() {
			continue
		}

		matches = append(matches, ResolvedRuleMatch{
			ToolType:     rule.ToolType,
			Scope:        rule.Scope,
			RelativePath: rule.Path,
			AbsolutePath: absolutePath,
			Category:     rule.Category,
			IsDir:        rule.IsDir,
			Size:         info.Size(),
			ModifiedAt:   info.ModTime(),
		})
	}
	return matches, nil
}

// expandPath 将 ~ 开头的路径展开为真实用户目录路径。
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// resolveAbsoluteFileRule 用于解析用户登记的绝对路径文件规则。
func resolveAbsoluteFileRule(toolType ToolType, absolutePath string) (ResolvedRuleMatch, bool, error) {
	info, err := os.Stat(absolutePath)
	if err != nil {
		if os.IsNotExist(err) {
			return ResolvedRuleMatch{}, false, nil
		}
		return ResolvedRuleMatch{}, false, err
	}
	if info.IsDir() {
		return ResolvedRuleMatch{}, false, nil
	}

	return ResolvedRuleMatch{
		ToolType:     toolType,
		Scope:        ScopeGlobal,
		AbsolutePath: absolutePath,
		RelativePath: filepath.Base(absolutePath),
		Category:     CategoryConfigFile,
		IsDir:        false,
		Size:         info.Size(),
		ModifiedAt:   info.ModTime(),
	}, true, nil
}

// fileExists 用于 required-path 检查，只认普通文件不认目录。
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
