package tool

import (
	"os"
	"path/filepath"
	"time"
)

type ResolvedRuleMatch struct {
	ToolType     ToolType
	Scope        ConfigScope
	RelativePath string
	AbsolutePath string
	Category     ConfigCategory
	IsDir        bool
	Size         int64
	ModifiedAt   time.Time
}

// ResolvedProjectMatch 记录某个已注册项目命中的全部规则结果。
type ResolvedProjectMatch struct {
	ProjectName string
	ProjectPath string
	Matches     []ResolvedRuleMatch
}

// ToolRuleReport 是某个工具在当前机器上的完整规则解析结果。
type ToolRuleReport struct {
	ToolType             ToolType
	GlobalPath           string
	Status               InstallationStatus
	MissingRequiredPaths []string
	DefaultMatches       []ResolvedRuleMatch
	CustomMatches        []ResolvedRuleMatch
	MissingCustomRules   []ResolvedRuleMatch
	ProjectMatches       []ResolvedProjectMatch
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
		absolutePath := filepath.Join(basePath, rule.Path)
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
