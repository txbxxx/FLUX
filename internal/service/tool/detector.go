package tool

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"ai-sync-manager/pkg/logger"
	"ai-sync-manager/pkg/utils"

	"go.uber.org/zap"
)

// ToolDetector 工具检测器
type ToolDetector struct {
	logger *zap.Logger
}

// NewToolDetector 创建工具检测器
func NewToolDetector() *ToolDetector {
	return &ToolDetector{
		logger: logger.L(),
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
		Projects: []ProjectInfo{},
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
		projects := d.scanProjects(ctx, opts)
		result.Projects = projects

		// 将项目配置信息添加到工具安装信息中
		d.mergeProjectConfigs(result)
	}

	return result, nil
}

// DetectTool 检测单个工具
func (d *ToolDetector) DetectTool(ctx context.Context, toolType ToolType, opts *ScanOptions) (*ToolInstallation, error) {
	if opts == nil {
		opts = &ScanOptions{ScanGlobal: true, IncludeFiles: true, MaxDepth: 1}
	}

	result := &ToolInstallation{
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
	result.Status = StatusInstalled

	// 扫描配置文件
	if opts.IncludeFiles {
		files := d.scanConfigDir(ctx, globalPath, ScopeGlobal, toolType, opts)
		result.ConfigFiles = append(result.ConfigFiles, files...)

		// 如果没有找到任何配置文件，标记为部分安装
		if len(files) == 0 {
			result.Status = StatusPartial
		}
	}

	return result, nil
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

// mergeProjectConfigs 将项目配置合并到工具安装信息中
func (d *ToolDetector) mergeProjectConfigs(result *ToolDetectionResult) {
	for _, project := range result.Projects {
		projectPath := project.Path

		// 处理 Codex 项目配置
		if project.HasCodex && result.Codex != nil {
			projectFiles := d.scanCodexConfig(
				context.Background(),
				projectPath,
				ScopeProject,
				&ScanOptions{IncludeFiles: true, MaxDepth: 1},
			)

			result.Codex.ConfigFiles = append(result.Codex.ConfigFiles, projectFiles...)
			if len(result.Codex.ProjectPaths) == 0 {
				result.Codex.ProjectPaths = []string{}
			}
			result.Codex.ProjectPaths = append(result.Codex.ProjectPaths, projectPath)
		}

		// 处理 Claude 项目配置
		if project.HasClaude && result.Claude != nil {
			projectFiles := d.scanClaudeConfig(
				context.Background(),
				projectPath,
				ScopeProject,
				&ScanOptions{IncludeFiles: true, MaxDepth: 1},
			)

			result.Claude.ConfigFiles = append(result.Claude.ConfigFiles, projectFiles...)
			if len(result.Claude.ProjectPaths) == 0 {
				result.Claude.ProjectPaths = []string{}
			}
			result.Claude.ProjectPaths = append(result.Claude.ProjectPaths, projectPath)
		}
	}
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
