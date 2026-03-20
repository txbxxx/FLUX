package snapshot

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"

	"ai-sync-manager/internal/models"
	"ai-sync-manager/internal/service/tool"
	"ai-sync-manager/pkg/logger"

	"go.uber.org/zap"
)

// Collector 文件收集器
type Collector struct {
	detector *tool.ToolDetector
}

// NewCollector 创建文件收集器
func NewCollector(detector *tool.ToolDetector) *Collector {
	return &Collector{
		detector: detector,
	}
}

// Collect 收集配置文件
func (c *Collector) Collect(options CollectOptions) (*CollectResult, error) {
	result := &CollectResult{
		Files: make([]models.SnapshotFile, 0),
		Errors: make([]CollectError, 0),
	}

	// 收集全局配置
	if options.Scope == models.ScopeGlobal || options.Scope == models.ScopeBoth {
		globalFiles, globalErrors := c.collectGlobalFiles(options)
		result.Files = append(result.Files, globalFiles...)
		result.Errors = append(result.Errors, globalErrors...)
	}

	// 收集项目配置
	if options.Scope == models.ScopeProject || options.Scope == models.ScopeBoth {
		if options.ProjectPath == "" {
			logger.Warn("项目路径为空，跳过项目配置收集")
		} else {
			projectFiles, projectErrors := c.collectProjectFiles(options)
			result.Files = append(result.Files, projectFiles...)
			result.Errors = append(result.Errors, projectErrors...)
		}
	}

	// 计算总大小
	for _, file := range result.Files {
		result.TotalSize += file.Size
	}

	logger.Info("文件收集完成",
		zap.Int("total_files", len(result.Files)),
		zap.Int64("total_size", result.TotalSize),
		zap.Int("errors", len(result.Errors)),
	)

	return result, nil
}

// collectGlobalFiles 收集全局配置文件
func (c *Collector) collectGlobalFiles(options CollectOptions) ([]models.SnapshotFile, []CollectError) {
	var files []models.SnapshotFile
	var errors []CollectError

	for _, toolType := range options.Tools {
		basePath := tool.GetDefaultGlobalPath(tool.ToolType(toolType))
		if basePath == "" {
			continue
		}
		collected, errs := c.collectFromPath(basePath, toolType, models.ScopeGlobal, options)
		files = append(files, collected...)
		errors = append(errors, errs...)
	}

	return files, errors
}

// collectProjectFiles 收集项目配置文件
func (c *Collector) collectProjectFiles(options CollectOptions) ([]models.SnapshotFile, []CollectError) {
	var files []models.SnapshotFile
	var errors []CollectError

	for _, toolType := range options.Tools {
		relPath := tool.GetDefaultProjectPath(tool.ToolType(toolType))
		if relPath == "" {
			continue
		}
		basePath := filepath.Join(options.ProjectPath, relPath)
		collected, errs := c.collectFromPath(basePath, toolType, models.ScopeProject, options)
		files = append(files, collected...)
		errors = append(errors, errs...)
	}

	return files, errors
}

// collectFromPath 从指定路径收集文件
func (c *Collector) collectFromPath(
	basePath string,
	toolType string,
	scope models.SnapshotScope,
	options CollectOptions,
) ([]models.SnapshotFile, []CollectError) {
	var files []models.SnapshotFile
	var errors []CollectError

	// 检查路径是否存在
	info, err := os.Stat(basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return files, errors // 路径不存在不是错误
		}
		return files, []CollectError{{Path: basePath, Message: err.Error()}}
	}

	// 如果是文件，直接收集
	if !info.IsDir() {
		file, err := c.collectSingleFile(basePath, toolType, options)
		if err != nil {
			return files, []CollectError{{Path: basePath, Message: err.Error()}}
		}
		if file != nil {
			files = append(files, *file)
		}
		return files, errors
	}

	// 遍历目录
	err = filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			errors = append(errors, CollectError{Path: path, Message: err.Error()})
			return nil
		}

		// 跳过目录
		if info.IsDir() {
			return nil
		}

		// 检查是否应该排除
		if c.shouldExclude(path, options.Excludes) {
			return nil
		}

		// 收集文件
		file, err := c.collectSingleFile(path, toolType, options)
		if err != nil {
			errors = append(errors, CollectError{Path: path, Message: err.Error()})
			return nil
		}
		if file != nil {
			files = append(files, *file)
		}

		return nil
	})

	return files, errors
}

// collectSingleFile 收集单个文件
func (c *Collector) collectSingleFile(
	path string,
	toolType string,
	options CollectOptions,
) (*models.SnapshotFile, error) {
	// 读取文件信息
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// 读取文件内容
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// 判断是否为二进制文件
	isBinary := c.isBinaryFile(content)

	// 获取文件类别
	category := c.categorizeFile(path, isBinary)

	// 检查类别过滤
	if len(options.Categories) > 0 && !c.containsCategory(options.Categories, category) {
		return nil, nil
	}

	// 计算哈希
	hash := c.calculateHash(content)

	// 获取相对路径
	relPath, err := filepath.Rel(filepath.VolumeName(path)+string(filepath.Separator), path)
	if err != nil {
		relPath = path
	}

	file := &models.SnapshotFile{
		Path:         relPath,
		OriginalPath: path,
		Size:         info.Size(),
		Hash:         hash,
		ModifiedAt:   info.ModTime(),
		Content:      content,
		ToolType:     toolType,
		Category:     category,
		IsBinary:     isBinary,
	}

	return file, nil
}

// isBinaryFile 判断是否为二进制文件
func (c *Collector) isBinaryFile(content []byte) bool {
	if len(content) == 0 {
		return false
	}

	// 检查前 512 字节
	limit := 512
	if len(content) < limit {
		limit = len(content)
	}

	for i := 0; i < limit; i++ {
		if content[i] == 0 {
			return true
		}
	}

	return false
}

// categorizeFile 根据路径和内容判断文件类别
func (c *Collector) categorizeFile(path string, isBinary bool) models.FileCategory {
	filename := filepath.Base(path)
	ext := strings.TrimPrefix(filepath.Ext(path), ".")

	// 根据文件名判断
	switch strings.ToLower(filename) {
	case "skills.yml", "skills.yaml":
		return models.CategorySkills
	case "commands.yml", "commands.yaml":
		return models.CategoryCommands
	case "plugins.yml", "plugins.yaml":
		return models.CategoryPlugins
	case "agents.md", "agents.yml", "agents.yaml":
		return models.CategoryAgents
	case "rules.md", "rules.yml", "rules.yaml":
		return models.CategoryRules
	}

	// 根据扩展名判断
	switch strings.ToLower(ext) {
	case "md", "markdown":
		return models.CategoryDocs
	case "yml", "yaml", "json", "toml":
		return models.CategoryConfig
	}

	// MCP 配置
	if strings.Contains(strings.ToLower(path), "mcp") {
		return models.CategoryMCP
	}

	// 二进制文件
	if isBinary {
		return models.CategoryOther
	}

	return models.CategoryConfig
}

// calculateHash 计算文件哈希
func (c *Collector) calculateHash(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

// shouldExclude 检查是否应该排除
func (c *Collector) shouldExclude(path string, excludes []string) bool {
	for _, pattern := range excludes {
		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err == nil && matched {
			return true
		}
		// 检查路径包含
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

// containsCategory 检查类别是否在列表中
func (c *Collector) containsCategory(categories []models.FileCategory, category models.FileCategory) bool {
	for _, cat := range categories {
		if cat == category {
			return true
		}
	}
	return false
}

// ReadFileContent 读取文件内容（按行）
func (c *Collector) ReadFileContent(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines, scanner.Err()
}

// ReadFileContentAsString 读取文件内容为字符串
func (c *Collector) ReadFileContentAsString(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// CloneFile 克隆文件到目标路径
func (c *Collector) CloneFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// CloneFileWithContent 使用指定内容创建文件
func (c *Collector) CloneFileWithContent(dst string, content []byte) error {
	// 确保目标目录存在
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	return os.WriteFile(dst, content, 0644)
}

// CompareFileContent 比较两个文件的内容
func (c *Collector) CompareFileContent(path1, path2 string) (bool, error) {
	content1, err := os.ReadFile(path1)
	if err != nil {
		return false, err
	}

	content2, err := os.ReadFile(path2)
	if err != nil {
		return false, err
	}

	return bytes.Equal(content1, content2), nil
}

// BackupFile 备份文件
func (c *Collector) BackupFile(src, backupDir string) (string, error) {
	// 创建备份路径
	relPath, err := filepath.Rel(filepath.VolumeName(src)+string(filepath.Separator), src)
	if err != nil {
		relPath = filepath.Base(src)
	}

	backupPath := filepath.Join(backupDir, relPath)

	// 复制文件
	if err := c.CloneFile(src, backupPath); err != nil {
		return "", err
	}

	return backupPath, nil
}
