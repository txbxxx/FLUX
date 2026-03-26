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
	resolver *tool.RuleResolver
}

// NewCollector 创建文件收集器。
// collector 只遍历统一规则层已经命中的目录，避免回退成“整目录打包”。
func NewCollector(resolver *tool.RuleResolver) *Collector {
	if resolver == nil {
		resolver = tool.NewRuleResolver(nil)
	}

	return &Collector{
		resolver: resolver,
	}
}

// Collect 收集配置文件
func (c *Collector) Collect(options CollectOptions) (*CollectResult, error) {
	result := &CollectResult{
		Files:  make([]models.SnapshotFile, 0),
		Errors: make([]CollectError, 0),
	}

	if options.Scope == models.ScopeGlobal || options.Scope == models.ScopeBoth {
		globalFiles, globalErrors := c.collectGlobalFiles(options)
		result.Files = append(result.Files, globalFiles...)
		result.Errors = append(result.Errors, globalErrors...)
	}

	if options.Scope == models.ScopeProject || options.Scope == models.ScopeBoth {
		projectFiles, projectErrors := c.collectProjectFiles(options)
		result.Files = append(result.Files, projectFiles...)
		result.Errors = append(result.Errors, projectErrors...)
	}

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

func (c *Collector) collectGlobalFiles(options CollectOptions) ([]models.SnapshotFile, []CollectError) {
	var files []models.SnapshotFile
	var errors []CollectError

	for _, toolName := range options.Tools {
		report, err := c.resolver.ResolveTool(tool.ToolType(toolName))
		if err != nil {
			errors = append(errors, CollectError{Path: toolName, Message: err.Error()})
			continue
		}

		matches := make([]tool.ResolvedRuleMatch, 0, len(report.DefaultMatches)+len(report.CustomMatches))
		matches = append(matches, report.DefaultMatches...)
		matches = append(matches, report.CustomMatches...)

		collected, errs := c.collectResolvedMatches(matches, toolName, options)
		files = append(files, collected...)
		errors = append(errors, errs...)
	}

	return files, errors
}

func (c *Collector) collectProjectFiles(options CollectOptions) ([]models.SnapshotFile, []CollectError) {
	var files []models.SnapshotFile
	var errors []CollectError

	for _, toolName := range options.Tools {
		report, err := c.resolver.ResolveTool(tool.ToolType(toolName))
		if err != nil {
			errors = append(errors, CollectError{Path: toolName, Message: err.Error()})
			continue
		}

		var matches []tool.ResolvedRuleMatch
		for _, project := range report.ProjectMatches {
			matches = append(matches, project.Matches...)
		}

		collected, errs := c.collectResolvedMatches(matches, toolName, options)
		files = append(files, collected...)
		errors = append(errors, errs...)
	}

	return files, errors
}

func (c *Collector) collectResolvedMatches(
	matches []tool.ResolvedRuleMatch,
	toolName string,
	options CollectOptions,
) ([]models.SnapshotFile, []CollectError) {
	var files []models.SnapshotFile
	var errors []CollectError

	seen := map[string]struct{}{}
	for _, match := range matches {
		collected, errs := c.collectMatch(match, toolName, options, seen)
		files = append(files, collected...)
		errors = append(errors, errs...)
	}

	return files, errors
}

func (c *Collector) collectMatch(
	match tool.ResolvedRuleMatch,
	toolName string,
	options CollectOptions,
	seen map[string]struct{},
) ([]models.SnapshotFile, []CollectError) {
	if match.IsDir {
		return c.collectFilesUnderDir(match.AbsolutePath, toolName, options, seen)
	}

	file, err := c.collectSingleFile(match.AbsolutePath, toolName, options, seen)
	if err != nil {
		return nil, []CollectError{{Path: match.AbsolutePath, Message: err.Error()}}
	}
	if file == nil {
		return nil, nil
	}

	return []models.SnapshotFile{*file}, nil
}

func (c *Collector) collectFilesUnderDir(
	basePath string,
	toolName string,
	options CollectOptions,
	seen map[string]struct{},
) ([]models.SnapshotFile, []CollectError) {
	var files []models.SnapshotFile
	var errors []CollectError

	walkErr := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			errors = append(errors, CollectError{Path: path, Message: err.Error()})
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if c.shouldExclude(path, options.Excludes) {
			return nil
		}

		file, err := c.collectSingleFile(path, toolName, options, seen)
		if err != nil {
			errors = append(errors, CollectError{Path: path, Message: err.Error()})
			return nil
		}
		if file != nil {
			files = append(files, *file)
		}

		return nil
	})
	if walkErr != nil {
		errors = append(errors, CollectError{Path: basePath, Message: walkErr.Error()})
	}

	return files, errors
}

// collectSingleFile 收集单个文件。
func (c *Collector) collectSingleFile(
	path string,
	toolName string,
	options CollectOptions,
	seen map[string]struct{},
) (*models.SnapshotFile, error) {
	cleanPath := filepath.Clean(path)
	if _, ok := seen[cleanPath]; ok {
		return nil, nil
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, err
	}

	isBinary := c.isBinaryFile(content)
	category := c.categorizeFile(cleanPath, isBinary)
	if len(options.Categories) > 0 && !c.containsCategory(options.Categories, category) {
		return nil, nil
	}

	hash := c.calculateHash(content)
	relPath, err := filepath.Rel(filepath.VolumeName(cleanPath)+string(filepath.Separator), cleanPath)
	if err != nil {
		relPath = cleanPath
	}

	seen[cleanPath] = struct{}{}

	return &models.SnapshotFile{
		Path:         relPath,
		OriginalPath: cleanPath,
		Size:         info.Size(),
		Hash:         hash,
		ModifiedAt:   info.ModTime(),
		Content:      content,
		ToolType:     toolName,
		Category:     category,
		IsBinary:     isBinary,
	}, nil
}

// isBinaryFile 判断是否为二进制文件
func (c *Collector) isBinaryFile(content []byte) bool {
	if len(content) == 0 {
		return false
	}

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

	switch strings.ToLower(ext) {
	case "md", "markdown":
		return models.CategoryDocs
	case "yml", "yaml", "json", "toml":
		return models.CategoryConfig
	}

	if strings.Contains(strings.ToLower(path), "mcp") {
		return models.CategoryMCP
	}

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
	relPath, err := filepath.Rel(filepath.VolumeName(src)+string(filepath.Separator), src)
	if err != nil {
		relPath = filepath.Base(src)
	}

	backupPath := filepath.Join(backupDir, relPath)

	if err := c.CloneFile(src, backupPath); err != nil {
		return "", err
	}

	return backupPath, nil
}
