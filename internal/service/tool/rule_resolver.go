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

type ResolvedProjectMatch struct {
	ProjectName string
	ProjectPath string
	Matches     []ResolvedRuleMatch
}

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

func NewRuleResolver(store *RuleStore) *RuleResolver {
	return &RuleResolver{store: store}
}

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

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
