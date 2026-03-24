package tool

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type ConfigTarget struct {
	ToolType     ToolType
	RootPath     string
	RelativePath string
	AbsolutePath string
	IsDir        bool
}

type ConfigEntry struct {
	Name         string
	RelativePath string
	IsDir        bool
}

type ConfigAccessor struct{}

func NewConfigAccessor() *ConfigAccessor {
	return &ConfigAccessor{}
}

func (a *ConfigAccessor) Resolve(toolType ToolType, relativePath string) (*ConfigTarget, error) {
	rootPath := GetDefaultGlobalPath(toolType)
	if strings.TrimSpace(rootPath) == "" {
		return nil, fmt.Errorf("不支持的工具类型 %q", toolType)
	}
	if !dirExists(rootPath) {
		return nil, fmt.Errorf("未找到配置目录 %s", rootPath)
	}

	cleanRelativePath := filepath.Clean(strings.TrimSpace(relativePath))
	if cleanRelativePath == "." || cleanRelativePath == "" {
		return nil, errors.New("请求路径不能为空")
	}
	if filepath.IsAbs(cleanRelativePath) {
		return nil, fmt.Errorf("请求路径超出允许范围：%s", cleanRelativePath)
	}
	if cleanRelativePath == ".." || strings.HasPrefix(cleanRelativePath, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("请求路径超出允许范围：%s", cleanRelativePath)
	}

	if !isAllowedRelativePath(toolType, cleanRelativePath) {
		return nil, fmt.Errorf("请求路径超出允许范围：%s", cleanRelativePath)
	}

	resolvedRootPath, err := filepath.EvalSymlinks(rootPath)
	if err != nil {
		resolvedRootPath, err = filepath.Abs(rootPath)
		if err != nil {
			return nil, fmt.Errorf("解析配置目录失败: %w", err)
		}
	}

	absolutePath := filepath.Join(resolvedRootPath, cleanRelativePath)
	info, err := os.Stat(absolutePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("未找到路径 %s", cleanRelativePath)
		}
		return nil, fmt.Errorf("读取路径失败: %w", err)
	}

	resolvedAbsolutePath, err := filepath.EvalSymlinks(absolutePath)
	if err != nil {
		resolvedAbsolutePath, err = filepath.Abs(absolutePath)
		if err != nil {
			return nil, fmt.Errorf("解析目标路径失败: %w", err)
		}
	}

	resolvedRelativePath, err := filepath.Rel(resolvedRootPath, resolvedAbsolutePath)
	if err != nil {
		return nil, fmt.Errorf("解析相对路径失败: %w", err)
	}
	if resolvedRelativePath == ".." || strings.HasPrefix(resolvedRelativePath, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("请求路径超出允许范围：%s", cleanRelativePath)
	}
	if !isAllowedRelativePath(toolType, resolvedRelativePath) {
		return nil, fmt.Errorf("请求路径超出允许范围：%s", cleanRelativePath)
	}

	return &ConfigTarget{
		ToolType:     toolType,
		RootPath:     resolvedRootPath,
		RelativePath: filepath.Clean(resolvedRelativePath),
		AbsolutePath: resolvedAbsolutePath,
		IsDir:        info.IsDir(),
	}, nil
}

func (a *ConfigAccessor) ListDir(target *ConfigTarget) ([]ConfigEntry, error) {
	if target == nil {
		return nil, errors.New("目标不能为空")
	}
	if !target.IsDir {
		return nil, fmt.Errorf("目标不是目录：%s", target.RelativePath)
	}

	entries, err := os.ReadDir(target.AbsolutePath)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	items := make([]ConfigEntry, 0, len(entries))
	for _, entry := range entries {
		childRelativePath := filepath.Join(target.RelativePath, entry.Name())
		childTarget, err := a.Resolve(target.ToolType, childRelativePath)
		if err != nil {
			continue
		}
		items = append(items, ConfigEntry{
			Name:         entry.Name(),
			RelativePath: childTarget.RelativePath,
			IsDir:        childTarget.IsDir,
		})
	}

	slices.SortStableFunc(items, func(a, b ConfigEntry) int {
		if a.IsDir != b.IsDir {
			if a.IsDir {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	return items, nil
}

func (a *ConfigAccessor) ReadFile(target *ConfigTarget) (string, error) {
	if target == nil {
		return "", errors.New("目标不能为空")
	}
	if target.IsDir {
		return "", fmt.Errorf("目标不是文件：%s", target.RelativePath)
	}

	content, err := os.ReadFile(target.AbsolutePath)
	if err != nil {
		return "", fmt.Errorf("读取文件失败: %w", err)
	}
	if isBinaryContent(content) {
		return "", fmt.Errorf("目标是二进制文件，无法直接读取：%s", target.RelativePath)
	}

	return string(content), nil
}

func (a *ConfigAccessor) WriteFile(target *ConfigTarget, content string) error {
	if target == nil {
		return errors.New("目标不能为空")
	}
	if target.IsDir {
		return fmt.Errorf("目标不是文件：%s", target.RelativePath)
	}

	info, err := os.Stat(target.AbsolutePath)
	if err != nil {
		return fmt.Errorf("读取文件状态失败: %w", err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(target.AbsolutePath), filepath.Base(target.AbsolutePath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if _, err := tempFile.WriteString(content); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	if err := tempFile.Chmod(info.Mode()); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("设置文件权限失败: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("关闭临时文件失败: %w", err)
	}

	if err := os.Rename(tempPath, target.AbsolutePath); err != nil {
		return fmt.Errorf("替换原文件失败: %w", err)
	}

	return nil
}

func isAllowedRelativePath(toolType ToolType, relativePath string) bool {
	cleanRelativePath := filepath.Clean(relativePath)
	if cleanRelativePath == "." || cleanRelativePath == "" {
		return false
	}

	for _, allowedPath := range allowedGlobalPaths(toolType) {
		if cleanRelativePath == allowedPath {
			return true
		}
		if strings.HasPrefix(cleanRelativePath, allowedPath+string(filepath.Separator)) {
			return true
		}
	}

	return false
}

func allowedGlobalPaths(toolType ToolType) []string {
	paths := []string{}

	switch toolType {
	case ToolTypeCodex:
		for _, definition := range GetCodexFileDefinitions() {
			if definition.Scope == ScopeGlobal {
				paths = append(paths, filepath.Clean(definition.Path))
			}
		}
	case ToolTypeClaude:
		for _, definition := range GetClaudeFileDefinitions() {
			if definition.Scope == ScopeGlobal {
				paths = append(paths, filepath.Clean(definition.Path))
			}
		}
	}

	return paths
}

func isBinaryContent(content []byte) bool {
	limit := min(len(content), 512)
	for i := 0; i < limit; i++ {
		if content[i] == 0 {
			return true
		}
	}
	return false
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
