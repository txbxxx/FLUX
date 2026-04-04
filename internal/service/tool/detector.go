package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ai-sync-manager/pkg/logger"
	"ai-sync-manager/pkg/utils"

	"go.uber.org/zap"
)

// ToolDetector 工具检测器，负责扫描本机 AI 工具（Codex、Claude）的安装状态和配置文件。
// 它通过 RuleResolver 匹配扫描规则，将命中的文件整理为 ToolInstallation 列表返回。
type ToolDetector struct {
	logger   *zap.Logger  // 结构化日志
	resolver *RuleResolver // 规则解析器，用于匹配配置文件
}

// NewToolDetector 创建工具检测器（使用空规则解析器）。
func NewToolDetector() *ToolDetector {
	return NewToolDetectorWithResolver(nil)
}

// NewToolDetectorWithResolver 创建带规则解析器的工具检测器。
// resolver 为 nil 时自动创建空解析器（仅使用硬编码规则）。
func NewToolDetectorWithResolver(resolver *RuleResolver) *ToolDetector {
	if resolver == nil {
		resolver = NewRuleResolver(nil)
	}
	return &ToolDetector{
		logger:   logger.L(),
		resolver: resolver,
	}
}

// DetectAll 检测所有支持的工具，返回全局安装状态。
// 不包含项目级别的扫描，适用于快速检测本机是否安装了 Codex/Claude。
func (d *ToolDetector) DetectAll(ctx context.Context) (*ToolDetectionResult, error) {
	result := &ToolDetectionResult{}

	// 检测 Codex
	codex, err := d.DetectTool(ctx, ToolTypeCodex, nil)
	if err != nil {
		logger.Warn("检测 Codex 失败", zap.Error(err))
	} else {
		result.Codex = codex
	}

	// 检测 Claude
	claude, err := d.DetectTool(ctx, ToolTypeClaude, nil)
	if err != nil {
		logger.Warn("检测 Claude 失败", zap.Error(err))
	} else {
		result.Claude = claude
	}

	return result, nil
}

// DetectWithOptions 使用选项检测工具，支持全局扫描和项目扫描。
// 这是 Scan 用例调用的主入口，返回全局检测结果和所有已注册项目的扫描结果。
func (d *ToolDetector) DetectWithOptions(ctx context.Context, opts *ScanOptions) (*ToolDetectionResult, error) {
	if opts == nil {
		opts = DefaultScanOptions()
	}

	result := &ToolDetectionResult{
		Projects:             []ProjectInfo{},
		ProjectInstallations: []*ToolInstallation{},
	}

	// 第一阶段：检测全局配置目录的安装状态和配置文件。
	// 结果存入 result.Codex 和 result.Claude，供内部逻辑（如路径比较）使用，
	// 但在 Scan 用例层不再直接展示全局条目，只展示项目条目。
	if opts.ScanGlobal {
		codex, _ := d.DetectTool(ctx, ToolTypeCodex, opts)
		if codex != nil {
			result.Codex = codex
		}

		claude, _ := d.DetectTool(ctx, ToolTypeClaude, opts)
		if claude != nil {
			result.Claude = claude
		}
	}

	// 第二阶段：检测所有已注册项目和当前目录的项目配置。
	// 已注册项目来自数据库（通过 RuleManager 注册），
	// 当前目录项目来自扫描（通过 opts.ProjectPaths 或 opts.CurrentDir）。
	if opts.ScanProjects {
		projectInstallations, projects, err := d.detectProjectInstallations(ctx, opts)
		if err != nil {
			return nil, err
		}
		result.ProjectInstallations = append(result.ProjectInstallations, projectInstallations...)
		result.Projects = append(result.Projects, projects...)
	}

	return result, nil
}

// DetectTool 检测单个工具的全局安装状态。
// 检查全局配置目录是否存在，如果存在则按规则扫描配置文件。
func (d *ToolDetector) DetectTool(ctx context.Context, toolType ToolType, opts *ScanOptions) (*ToolInstallation, error) {
	if opts == nil {
		opts = &ScanOptions{ScanGlobal: true, IncludeFiles: true, MaxDepth: 1}
	}

	result := &ToolInstallation{
		Scope:      ScopeGlobal,
		ToolType:   toolType,
		DetectedAt: time.Now(),
	}

	// 检查全局配置路径是否存在。
	// GetDefaultGlobalPath 会优先读 YAML 配置中的 global_dir，回退到硬编码默认值。
	globalPath := GetDefaultGlobalPath(toolType)
	if !utils.DirExists(globalPath) {
		result.Status = StatusNotInstalled
		return result, nil
	}

	result.GlobalPath = globalPath

	// 如果请求包含文件扫描，使用 RuleResolver 匹配配置文件。
	// ResolveTool 会综合默认规则、自定义全局规则和项目规则来查找文件。
	if opts.IncludeFiles {
		report, err := d.resolver.ResolveTool(toolType)
		if err != nil {
			return nil, err
		}
		result.Status = report.Status

		// 合并默认匹配和自定义匹配的结果
		files := make([]ConfigFile, 0, len(report.DefaultMatches)+len(report.CustomMatches))
		for _, match := range report.DefaultMatches {
			files = append(files, resolvedMatchToConfigFile(match))
		}
		for _, match := range report.CustomMatches {
			files = append(files, resolvedMatchToConfigFile(match))
		}
		result.ConfigFiles = append(result.ConfigFiles, files...)
	} else {
		// 不扫描文件时，只要目录存在就标记为已安装
		result.Status = StatusInstalled
	}

	return result, nil
}

// detectProjectInstallations 检测所有项目级安装，包括已注册项目和当前目录项目。
// 返回结果经过去重处理（按 toolType+projectPath 去重）。
func (d *ToolDetector) detectProjectInstallations(ctx context.Context, opts *ScanOptions) ([]*ToolInstallation, []ProjectInfo, error) {
	var installations []*ToolInstallation
	var projects []ProjectInfo

	// 来源一：数据库中已注册的项目（通过 scan add 命令或自动注册的全局项目）
	if d.resolver != nil && d.resolver.store != nil {
		registered, infos, err := d.detectRegisteredProjectInstallations()
		if err != nil {
			return nil, nil, err
		}
		installations = append(installations, registered...)
		projects = append(projects, infos...)
	}

	// 来源二：当前目录或指定路径的项目（通过 opts.ProjectPaths 传入）
	// 这是为了兼容直接扫描某个目录的场景。
	legacyProjects := d.scanProjects(ctx, opts)
	legacyInstallations := d.buildProjectInstallationsFromProjectInfos(legacyProjects)
	installations = append(installations, legacyInstallations...)
	projects = append(projects, legacyProjects...)

	// 去重：同一 toolType + projectPath 只保留第一条
	return dedupeProjectInstallations(installations), dedupeProjectInfos(projects), nil
}

// detectRegisteredProjectInstallations 扫描数据库中所有已注册项目的配置文件。
//
// 规则选择策略（核心设计决策）：
//
//	全局项目（项目路径 == 全局配置目录）→ 使用全局规则（DefaultGlobalRules）
//	  因为 ~/.claude 目录的布局是全局配置结构（settings.json、commands/ 等），
//	  用项目规则（.claude/、CLAUDE.md）只能扫到极少文件。
//
//	普通项目（项目路径 != 全局配置目录）→ 使用项目规则（ProjectRuleTemplates）
//	  因为用户项目的配置通常在 .claude/ 子目录或 CLAUDE.md 中。
//
// 规则来源均优先读 YAML 配置（用户可自定义），回退到硬编码默认值。
func (d *ToolDetector) detectRegisteredProjectInstallations() ([]*ToolInstallation, []ProjectInfo, error) {
	var installations []*ToolInstallation
	var projects []ProjectInfo

	for _, toolType := range []ToolType{ToolTypeCodex, ToolTypeClaude} {
		registeredProjects, err := d.resolver.store.ListRegisteredProjects(toolType)
		if err != nil {
			return nil, nil, err
		}

		// 获取全局配置路径，用于判断项目是否为全局项目
		globalPath := GetDefaultGlobalPath(toolType)

		for _, project := range registeredProjects {
			// 根据项目路径选择扫描规则集
			rules := ProjectRuleTemplates(toolType)
			if filepath.Clean(project.ProjectPath) == filepath.Clean(globalPath) {
				// 全局项目：目录布局与全局配置目录一致（如 ~/.claude 包含 settings.json），
				// 必须使用全局规则才能完整扫描所有配置文件。
				rules = DefaultGlobalRules(toolType)
			}

			// 用选定的规则集匹配文件
			matches, err := resolveRuleDefinitions(project.ProjectPath, rules)
			if err != nil {
				return nil, nil, err
			}

			// 构建 ToolInstallation：初始状态为 Partial，匹配到文件则升级为 Installed
			installation := &ToolInstallation{
				Scope:       ScopeProject,
				ToolType:    toolType,
				ProjectName: project.ProjectName,
				ProjectPath: project.ProjectPath,
				DetectedAt:  time.Now(),
				Status:      StatusPartial,
			}
			for _, match := range matches {
				installation.ConfigFiles = append(installation.ConfigFiles, resolvedMatchToConfigFile(match))
			}
			if len(installation.ConfigFiles) > 0 {
				installation.Status = StatusInstalled
			}

			installations = append(installations, installation)

			// 同步构建 ProjectInfo（用于兼容旧的扫描结果格式）
			info := ProjectInfo{
				Path: project.ProjectPath,
				Name: project.ProjectName,
			}
			switch toolType {
			case ToolTypeCodex:
				info.HasCodex = true
			case ToolTypeClaude:
				info.HasClaude = true
			}
			projects = append(projects, info)
		}
	}

	return installations, projects, nil
}

// buildProjectInstallationsFromProjectInfos 将 ProjectInfo 列表转换为 ToolInstallation 列表。
// 用于将 scanProjects 的扫描结果统一为 ToolInstallation 格式。
func (d *ToolDetector) buildProjectInstallationsFromProjectInfos(projects []ProjectInfo) []*ToolInstallation {
	var installations []*ToolInstallation

	for _, project := range projects {
		if project.HasCodex {
			installations = append(installations, newLegacyProjectInstallation(ToolTypeCodex, project))
		}
		if project.HasClaude {
			installations = append(installations, newLegacyProjectInstallation(ToolTypeClaude, project))
		}
	}

	return installations
}

// newLegacyProjectInstallation 为当前目录扫描发现的项目创建 ToolInstallation。
// 使用项目规则模板（ProjectRuleTemplates）匹配配置文件。
func newLegacyProjectInstallation(toolType ToolType, project ProjectInfo) *ToolInstallation {
	installation := &ToolInstallation{
		Scope:       ScopeProject,
		ToolType:    toolType,
		ProjectName: project.Name,
		ProjectPath: project.Path,
		DetectedAt:  time.Now(),
		Status:      StatusPartial,
	}

	// 遍历项目规则模板，检查每个规则对应的文件/目录是否存在
	for _, definition := range ProjectRuleTemplates(toolType) {
		fullPath := filepath.Join(project.Path, definition.Path)
		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}
		// 类型校验：规则期望文件但实际是目录（或反之）时跳过
		if definition.IsDir != info.IsDir() {
			continue
		}

		installation.ConfigFiles = append(installation.ConfigFiles, ConfigFile{
			Name:       filepath.Base(definition.Path),
			Path:       fullPath,
			Category:   definition.Category,
			Scope:      ScopeProject,
			Size:       info.Size(),
			ModifiedAt: info.ModTime(),
			IsDir:      definition.IsDir,
		})
	}

	// 至少有一个配置文件命中才标记为已安装
	if len(installation.ConfigFiles) > 0 {
		installation.Status = StatusInstalled
	}

	return installation
}

// dedupeProjectInstallations 按 toolType+projectPath 去重。
// 因为已注册项目和当前目录扫描可能产生重复条目（如项目同时被注册和被扫描到），
// 只保留第一次出现的条目。
func dedupeProjectInstallations(items []*ToolInstallation) []*ToolInstallation {
	seen := map[string]struct{}{}
	result := make([]*ToolInstallation, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		key := item.ToolType.String() + "|" + item.ProjectPath
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}

// dedupeProjectInfos 按路径去重，合并同路径下多个工具的标记。
// 例如同一目录同时有 .codex 和 .claude 时，合并为一条记录并设置 HasCodex=true, HasClaude=true。
func dedupeProjectInfos(items []ProjectInfo) []ProjectInfo {
	seen := map[string]int{}
	result := make([]ProjectInfo, 0, len(items))
	for _, item := range items {
		if idx, ok := seen[item.Path]; ok {
			// 合并：取任一为 true 即为 true
			result[idx].HasCodex = result[idx].HasCodex || item.HasCodex
			result[idx].HasClaude = result[idx].HasClaude || item.HasClaude
			if strings.TrimSpace(result[idx].Name) == "" {
				result[idx].Name = item.Name
			}
			continue
		}
		seen[item.Path] = len(result)
		result = append(result, item)
	}
	return result
}

// resolvedMatchToConfigFile 将规则解析器的匹配结果转换为 ConfigFile。
// Name 优先使用 RelativePath（保留路径层次），无相对路径时回退到 AbsolutePath 的 basename。
func resolvedMatchToConfigFile(match ResolvedRuleMatch) ConfigFile {
	name := match.RelativePath
	if match.AbsolutePath != "" {
		name = filepath.Base(match.AbsolutePath)
	}

	return ConfigFile{
		Name:       name,
		Path:       match.AbsolutePath,
		Category:   match.Category,
		Scope:      match.Scope,
		Size:       match.Size,
		ModifiedAt: match.ModifiedAt,
		IsDir:      match.IsDir,
	}
}

// scanConfigDir 按作用域扫描配置目录，分发到具体的工具扫描方法。
// 这是一个遗留方法，新代码应优先使用 RuleResolver 统一处理。
func (d *ToolDetector) scanConfigDir(ctx context.Context, configPath string, scope ConfigScope, toolType ToolType, opts *ScanOptions) []ConfigFile {
	var files []ConfigFile

	switch toolType {
	case ToolTypeCodex:
		files = d.scanCodexConfig(ctx, configPath, scope, opts)
	case ToolTypeClaude:
		files = d.scanClaudeConfig(ctx, configPath, scope, opts)
	}

	return files
}

// scanCodexConfig 扫描 Codex 配置目录，按文件定义列表匹配。
// 这是一个遗留方法，保留用于兼容旧的扫描逻辑。
func (d *ToolDetector) scanCodexConfig(ctx context.Context, configPath string, scope ConfigScope, opts *ScanOptions) []ConfigFile {
	var files []ConfigFile

	definitions := GetCodexFileDefinitions()

	for _, def := range definitions {
		// 过滤作用域：只处理与当前扫描作用域匹配的规则
		if def.Scope != scope {
			continue
		}

		fullPath := filepath.Join(configPath, def.Path)

		// 检查文件/目录是否存在
		if def.IsDir {
			if !utils.DirExists(fullPath) {
				continue
			}
		} else {
			if !utils.FileExists(fullPath) {
				continue
			}
		}

		// 获取文件信息
		info, err := os.Stat(fullPath)
		if err != nil {
			logger.Warn("获取文件信息失败",
				zap.String("path", fullPath),
				zap.Error(err))
			continue
		}

		file := ConfigFile{
			Name:       def.Name,
			Path:       fullPath,
			Category:   def.Category,
			Scope:      scope,
			Size:       info.Size(),
			ModifiedAt: info.ModTime(),
			IsDir:      def.IsDir,
		}

		files = append(files, file)

		// 目录类型且深度允许时，递归扫描子目录内容
		if def.IsDir && opts.MaxDepth > 1 {
			subFiles := d.scanDirectoryContents(ctx, fullPath, def.Category, scope, opts.MaxDepth-1)
			files = append(files, subFiles...)
		}
	}

	return files
}

// scanClaudeConfig 扫描 Claude 配置目录，逻辑与 scanCodexConfig 对称。
func (d *ToolDetector) scanClaudeConfig(ctx context.Context, configPath string, scope ConfigScope, opts *ScanOptions) []ConfigFile {
	var files []ConfigFile

	definitions := GetClaudeFileDefinitions()

	for _, def := range definitions {
		if def.Scope != scope {
			continue
		}

		fullPath := filepath.Join(configPath, def.Path)

		if def.IsDir {
			if !utils.DirExists(fullPath) {
				continue
			}
		} else {
			if !utils.FileExists(fullPath) {
				continue
			}
		}

		info, err := os.Stat(fullPath)
		if err != nil {
			logger.Warn("获取文件信息失败",
				zap.String("path", fullPath),
				zap.Error(err))
			continue
		}

		file := ConfigFile{
			Name:       def.Name,
			Path:       fullPath,
			Category:   def.Category,
			Scope:      scope,
			Size:       info.Size(),
			ModifiedAt: info.ModTime(),
			IsDir:      def.IsDir,
		}

		files = append(files, file)

		if def.IsDir && opts.MaxDepth > 1 {
			subFiles := d.scanDirectoryContents(ctx, fullPath, def.Category, scope, opts.MaxDepth-1)
			files = append(files, subFiles...)
		}
	}

	return files
}

// scanDirectoryContents 递归扫描目录内容，收集非隐藏文件。
// maxDepth 控制递归深度，防止扫描过深影响性能。
func (d *ToolDetector) scanDirectoryContents(ctx context.Context, dirPath string, category ConfigCategory, scope ConfigScope, maxDepth int) []ConfigFile {
	var files []ConfigFile

	if maxDepth <= 0 {
		return files
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		logger.Warn("读取目录失败",
			zap.String("path", dirPath),
			zap.Error(err))
		return files
	}

	for _, entry := range entries {
		fullPath := filepath.Join(dirPath, entry.Name())

		// 跳过隐藏文件和目录（以 . 开头）
		if len(entry.Name()) > 0 && entry.Name()[0] == '.' {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// 跳过过大的文件（>10MB），避免内存问题
		if !info.IsDir() && info.Size() > 10*1024*1024 {
			logger.Debug("跳过过大文件",
				zap.String("path", fullPath),
				zap.Int64("size", info.Size()))
			continue
		}

		file := ConfigFile{
			Name:       entry.Name(),
			Path:       fullPath,
			Category:   category,
			Scope:      scope,
			Size:       info.Size(),
			ModifiedAt: info.ModTime(),
			IsDir:      info.IsDir(),
		}

		files = append(files, file)

		// 目录类型继续递归，深度减一
		if info.IsDir() {
			subFiles := d.scanDirectoryContents(ctx, fullPath, category, scope, maxDepth-1)
			files = append(files, subFiles...)
		}
	}

	return files
}

// scanProjects 扫描指定路径下的项目配置。
// 检查每个路径下是否存在 Codex 或 Claude 的项目标识（如 .codex/、.claude/、AGENTS.md）。
// 未指定路径时使用 CurrentDir 作为扫描目标。
func (d *ToolDetector) scanProjects(ctx context.Context, opts *ScanOptions) []ProjectInfo {
	var projects []ProjectInfo

	projectPaths := opts.ProjectPaths
	if len(projectPaths) == 0 && opts.CurrentDir != "" {
		projectPaths = []string{opts.CurrentDir}
	}

	for _, path := range projectPaths {
		projectPath := utils.ExpandUserHome(path)

		if !utils.DirExists(projectPath) {
			continue
		}

		project := ProjectInfo{
			Path: projectPath,
			Name: filepath.Base(projectPath),
		}

		// 检查 Codex 项目标识：.codex/ 目录或 AGENTS.md 文件
		codexPath := filepath.Join(projectPath, GetDefaultProjectPath(ToolTypeCodex))
		if utils.DirExists(codexPath) || utils.FileExists(filepath.Join(projectPath, "AGENTS.md")) {
			project.HasCodex = true
		}

		// 检查 Claude 项目标识：.claude/ 目录
		claudePath := filepath.Join(projectPath, GetDefaultProjectPath(ToolTypeClaude))
		if utils.DirExists(claudePath) {
			project.HasClaude = true
		}

		// 只保留有任一工具配置的项目
		if project.HasCodex || project.HasClaude {
			projects = append(projects, project)
		}
	}

	return projects
}

// WalkConfigDir 遍历配置目录（用于深度扫描）。
// 跳过隐藏目录和权限不足的路径。
func (d *ToolDetector) WalkConfigDir(configPath string) error {
	return filepath.Walk(configPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsPermission(err) {
				logger.Warn("权限不足，跳过", zap.String("path", path))
				return nil
			}
			return err
		}

		// 跳过隐藏目录（以 . 开头），避免扫描 .git 等无关目录
		if info.IsDir() && len(info.Name()) > 0 && info.Name()[0] == '.' {
			return filepath.SkipDir
		}

		return nil
	})
}
