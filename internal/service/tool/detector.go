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

// ToolDetector 工具检测器
type ToolDetector struct {
	logger   *zap.Logger
	resolver *RuleResolver
}

// NewToolDetector 创建工具检测器
func NewToolDetector() *ToolDetector {
	return NewToolDetectorWithResolver(nil)
}

// NewToolDetectorWithResolver 创建带规则解析器的工具检测器。
func NewToolDetectorWithResolver(resolver *RuleResolver) *ToolDetector {
	if resolver == nil {
		resolver = NewRuleResolver(nil)
	}
	return &ToolDetector{
		logger:   logger.L(),
		resolver: resolver,
	}
}

// DetectAll 检测所有支持的工具
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

// DetectWithOptions 使用选项检测工具
func (d *ToolDetector) DetectWithOptions(ctx context.Context, opts *ScanOptions) (*ToolDetectionResult, error) {
	if opts == nil {
		opts = DefaultScanOptions()
	}

	result := &ToolDetectionResult{
		Projects:             []ProjectInfo{},
		ProjectInstallations: []*ToolInstallation{},
	}

	// 检测全局配置
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

	// 检测项目配置
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

// DetectTool 检测单个工具
func (d *ToolDetector) DetectTool(ctx context.Context, toolType ToolType, opts *ScanOptions) (*ToolInstallation, error) {
	if opts == nil {
		opts = &ScanOptions{ScanGlobal: true, IncludeFiles: true, MaxDepth: 1}
	}

	result := &ToolInstallation{
		Scope:      ScopeGlobal,
		ToolType:   toolType,
		DetectedAt: time.Now(),
	}

	// 检查全局配置路径
	globalPath := GetDefaultGlobalPath(toolType)
	if !utils.DirExists(globalPath) {
		result.Status = StatusNotInstalled
		return result, nil
	}

	result.GlobalPath = globalPath

	// 扫描配置文件
	if opts.IncludeFiles {
		report, err := d.resolver.ResolveTool(toolType)
		if err != nil {
			return nil, err
		}
		result.Status = report.Status

		files := make([]ConfigFile, 0, len(report.DefaultMatches)+len(report.CustomMatches))
		for _, match := range report.DefaultMatches {
			files = append(files, resolvedMatchToConfigFile(match))
		}
		for _, match := range report.CustomMatches {
			files = append(files, resolvedMatchToConfigFile(match))
		}
		result.ConfigFiles = append(result.ConfigFiles, files...)
	} else {
		result.Status = StatusInstalled
	}

	return result, nil
}

func (d *ToolDetector) detectProjectInstallations(ctx context.Context, opts *ScanOptions) ([]*ToolInstallation, []ProjectInfo, error) {
	var installations []*ToolInstallation
	var projects []ProjectInfo

	if d.resolver != nil && d.resolver.store != nil {
		registered, infos, err := d.detectRegisteredProjectInstallations()
		if err != nil {
			return nil, nil, err
		}
		installations = append(installations, registered...)
		projects = append(projects, infos...)
	}

	legacyProjects := d.scanProjects(ctx, opts)
	legacyInstallations := d.buildProjectInstallationsFromProjectInfos(legacyProjects)
	installations = append(installations, legacyInstallations...)
	projects = append(projects, legacyProjects...)

	return dedupeProjectInstallations(installations), dedupeProjectInfos(projects), nil
}

func (d *ToolDetector) detectRegisteredProjectInstallations() ([]*ToolInstallation, []ProjectInfo, error) {
	var installations []*ToolInstallation
	var projects []ProjectInfo

	for _, toolType := range []ToolType{ToolTypeCodex, ToolTypeClaude} {
		registeredProjects, err := d.resolver.store.ListRegisteredProjects(toolType)
		if err != nil {
			return nil, nil, err
		}

		for _, project := range registeredProjects {
			matches, err := resolveRuleDefinitions(project.ProjectPath, ProjectRuleTemplates(toolType))
			if err != nil {
				return nil, nil, err
			}

			// 已注册项目在 scan 结果中是独立扫描对象，
			// 不能再合并进全局工具摘要。
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

func newLegacyProjectInstallation(toolType ToolType, project ProjectInfo) *ToolInstallation {
	installation := &ToolInstallation{
		Scope:       ScopeProject,
		ToolType:    toolType,
		ProjectName: project.Name,
		ProjectPath: project.Path,
		DetectedAt:  time.Now(),
		Status:      StatusPartial,
	}

	for _, definition := range ProjectRuleTemplates(toolType) {
		fullPath := filepath.Join(project.Path, definition.Path)
		info, err := os.Stat(fullPath)
		if err != nil {
			continue
		}
		if definition.IsDir != info.IsDir() {
			continue
		}

		installation.ConfigFiles = append(installation.ConfigFiles, ConfigFile{
			// 兼容旧 project scan 逻辑时，Name 仅保留基础文件名，
			// 展示层会结合 ProjectPath 重新计算相对路径。
			Name:       filepath.Base(definition.Path),
			Path:       fullPath,
			Category:   definition.Category,
			Scope:      ScopeProject,
			Size:       info.Size(),
			ModifiedAt: info.ModTime(),
			IsDir:      definition.IsDir,
		})
	}

	if len(installation.ConfigFiles) > 0 {
		installation.Status = StatusInstalled
	}

	return installation
}

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

func dedupeProjectInfos(items []ProjectInfo) []ProjectInfo {
	seen := map[string]int{}
	result := make([]ProjectInfo, 0, len(items))
	for _, item := range items {
		if idx, ok := seen[item.Path]; ok {
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

// scanConfigDir 扫描配置目录
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

// scanCodexConfig 扫描 Codex 配置
func (d *ToolDetector) scanCodexConfig(ctx context.Context, configPath string, scope ConfigScope, opts *ScanOptions) []ConfigFile {
	var files []ConfigFile

	definitions := GetCodexFileDefinitions()

	for _, def := range definitions {
		// 过滤作用域
		if def.Scope != scope {
			continue
		}

		fullPath := filepath.Join(configPath, def.Path)

		// 检查是否存在
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

		// 如果是目录且需要列出文件，递归扫描（根据 MaxDepth）
		if def.IsDir && opts.MaxDepth > 1 {
			subFiles := d.scanDirectoryContents(ctx, fullPath, def.Category, scope, opts.MaxDepth-1)
			files = append(files, subFiles...)
		}
	}

	return files
}

// scanClaudeConfig 扫描 Claude 配置
func (d *ToolDetector) scanClaudeConfig(ctx context.Context, configPath string, scope ConfigScope, opts *ScanOptions) []ConfigFile {
	var files []ConfigFile

	definitions := GetClaudeFileDefinitions()

	for _, def := range definitions {
		// 过滤作用域
		if def.Scope != scope {
			continue
		}

		fullPath := filepath.Join(configPath, def.Path)

		// 检查是否存在
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

		// 如果是目录且需要列出文件，递归扫描
		if def.IsDir && opts.MaxDepth > 1 {
			subFiles := d.scanDirectoryContents(ctx, fullPath, def.Category, scope, opts.MaxDepth-1)
			files = append(files, subFiles...)
		}
	}

	return files
}

// scanDirectoryContents 扫描目录内容
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

		// 跳过隐藏文件和目录
		if len(entry.Name()) > 0 && entry.Name()[0] == '.' {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// 跳过过大的文件
		if !info.IsDir() && info.Size() > 10*1024*1024 { // 10MB
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

		// 递归扫描子目录
		if info.IsDir() {
			subFiles := d.scanDirectoryContents(ctx, fullPath, category, scope, maxDepth-1)
			files = append(files, subFiles...)
		}
	}

	return files
}

// scanProjects 扫描项目配置
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

		// 检查 Codex 项目配置
		codexPath := filepath.Join(projectPath, GetDefaultProjectPath(ToolTypeCodex))
		if utils.DirExists(codexPath) || utils.FileExists(filepath.Join(projectPath, "AGENTS.md")) {
			project.HasCodex = true
		}

		// 检查 Claude 项目配置
		claudePath := filepath.Join(projectPath, GetDefaultProjectPath(ToolTypeClaude))
		if utils.DirExists(claudePath) {
			project.HasClaude = true
		}

		// 只保留有配置的项目
		if project.HasCodex || project.HasClaude {
			projects = append(projects, project)
		}
	}

	return projects
}

// WalkConfigDir 遍历配置目录（用于深度扫描）
func (d *ToolDetector) WalkConfigDir(configPath string) error {
	return filepath.Walk(configPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsPermission(err) {
				logger.Warn("权限不足，跳过", zap.String("path", path))
				return nil
			}
			return err
		}

		// 跳过隐藏目录
		if info.IsDir() && len(info.Name()) > 0 && info.Name()[0] == '.' {
			return filepath.SkipDir
		}

		return nil
	})
}
