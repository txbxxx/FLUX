package tool

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ConfigTarget 表示一个已通过规则校验的配置文件或目录目标。
// 由 ConfigAccessor.Resolve 返回，所有路径均经过符号链接解析和越界检查。
type ConfigTarget struct {
	ToolType     ToolType // 所属工具类型（codex / claude）
	RootPath     string   // 配置根目录的绝对路径（全局目录或项目根目录）
	RelativePath string   // 相对于 RootPath 的路径，根目录时为 "."
	AbsolutePath string   // 文件/目录在磁盘上的绝对路径（已解析符号链接）
	IsDir        bool     // 是否为目录
}

// ConfigEntry 表示目录列表中的单个条目，用于 ListDir 的返回结果。
type ConfigEntry struct {
	Name         string // 文件或目录名（不含路径前缀）
	RelativePath string // 相对于配置根目录的路径
	IsDir        bool   // 是否为目录
}

// ConfigAccessor 只允许访问统一规则源已经放行的目标。
// 相对路径继续表示默认全局规则；绝对路径则必须是已注册规则命中的真实路径。
type ConfigAccessor struct {
	resolver *RuleResolver
}

// NewConfigAccessor 使用给定的规则解析器创建新的配置访问器。
//
// 如果未提供解析器，则使用默认的空解析器。
// 访问器强制要求所有路径必须通过配置规则的允许校验。
func NewConfigAccessor(resolvers ...*RuleResolver) *ConfigAccessor {
	var resolver *RuleResolver
	if len(resolvers) > 0 {
		resolver = resolvers[0]
	}
	if resolver == nil {
		resolver = NewRuleResolver(nil)
	}

	return &ConfigAccessor{resolver: resolver}
}

// Resolve 通过安全校验将请求路径解析为配置目标。
//
// 相对路径相对于工具的全局配置目录解析。
// 绝对路径必须匹配指定工具类型允许的规则。
// 所有路径都会检查符号链接攻击和目录遍历尝试。
func (a *ConfigAccessor) Resolve(toolType ToolType, requestPath string) (*ConfigTarget, error) {
	report, err := a.resolver.ResolveTool(toolType)
	if err != nil {
		return nil, fmt.Errorf("解析规则失败: %w", err)
	}
	if strings.TrimSpace(report.GlobalPath) == "" {
		return nil, fmt.Errorf("不支持的工具类型 %q", toolType)
	}

	requestPath = strings.TrimSpace(requestPath)
	if filepath.IsAbs(requestPath) {
		return a.resolveAbsoluteTarget(toolType, report, requestPath)
	}

	return a.resolveRelativeTarget(toolType, report, requestPath)
}

func (a *ConfigAccessor) resolveRelativeTarget(toolType ToolType, report *ToolRuleReport, requestPath string) (*ConfigTarget, error) {
	if !dirExists(report.GlobalPath) {
		return nil, fmt.Errorf("未找到配置目录 %s", report.GlobalPath)
	}

	cleanRelativePath := filepath.Clean(requestPath)
	if cleanRelativePath == "." || cleanRelativePath == "" {
		resolvedRootPath, err := filepath.EvalSymlinks(report.GlobalPath)
		if err != nil {
			resolvedRootPath, err = filepath.Abs(report.GlobalPath)
			if err != nil {
				return nil, fmt.Errorf("解析配置目录失败: %w", err)
			}
		}
		return &ConfigTarget{
			ToolType:     toolType,
			RootPath:     resolvedRootPath,
			RelativePath: ".",
			AbsolutePath: resolvedRootPath,
			IsDir:        true,
		}, nil
	}
	if filepath.IsAbs(cleanRelativePath) {
		return nil, fmt.Errorf("请求路径超出允许范围：%s", cleanRelativePath)
	}
	if cleanRelativePath == ".." || strings.HasPrefix(cleanRelativePath, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("请求路径超出允许范围：%s", cleanRelativePath)
	}

	resolvedRootPath, err := filepath.EvalSymlinks(report.GlobalPath)
	if err != nil {
		resolvedRootPath, err = filepath.Abs(report.GlobalPath)
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
	if !isAllowedRelativePath(report.DefaultMatches, resolvedRelativePath) {
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

func (a *ConfigAccessor) resolveAbsoluteTarget(toolType ToolType, report *ToolRuleReport, requestPath string) (*ConfigTarget, error) {
	resolvedAbsolutePath, info, err := normalizeExistingPath(requestPath)
	if err != nil {
		return nil, err
	}

	match, ok := findAllowedAbsoluteMatch(report, resolvedAbsolutePath)
	if !ok {
		return nil, fmt.Errorf("请求路径超出允许范围：%s", requestPath)
	}

	rootPath := filepath.Dir(match.AbsolutePath)
	if match.IsDir {
		rootPath = match.AbsolutePath
	}

	return &ConfigTarget{
		ToolType:     toolType,
		RootPath:     rootPath,
		RelativePath: resolvedAbsolutePath,
		AbsolutePath: resolvedAbsolutePath,
		IsDir:        info.IsDir(),
	}, nil
}

// ListDir 返回配置目录中条目的有序列表。
//
// 目录排在文件前面，每组内的条目按名称不区分大小写排序。
// 仅包含可通过解析规则访问的条目。
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
		childRequestPath := filepath.Join(target.RelativePath, entry.Name())
		if filepath.IsAbs(target.RelativePath) {
			childRequestPath = filepath.Join(target.AbsolutePath, entry.Name())
		}

		childTarget, err := a.Resolve(target.ToolType, childRequestPath)
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

// ReadFile 读取配置文件的内容。
//
// 通过检查前 512 字节中是否存在空字节来检测二进制文件，检测到时返回错误。
// 内容以 UTF-8 字符串形式返回。
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

// WriteFile 将内容原子写入配置文件。
//
// 内容先写入临时文件，然后重命名到目标路径以保证原子性。
// 保留原始文件权限。
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

func isAllowedRelativePath(matches []ResolvedRuleMatch, relativePath string) bool {
	cleanRelativePath := filepath.Clean(relativePath)
	if cleanRelativePath == "." || cleanRelativePath == "" {
		return false
	}

	for _, match := range matches {
		allowedPath := filepath.Clean(match.RelativePath)
		if cleanRelativePath == allowedPath {
			return true
		}
		if match.IsDir && strings.HasPrefix(cleanRelativePath, allowedPath+string(filepath.Separator)) {
			return true
		}
	}

	return false
}

func findAllowedAbsoluteMatch(report *ToolRuleReport, targetPath string) (ResolvedRuleMatch, bool) {
	for _, match := range allowedMatches(report) {
		allowedPath := filepath.Clean(match.AbsolutePath)
		if targetPath == allowedPath {
			return match, true
		}
		if match.IsDir && strings.HasPrefix(targetPath, allowedPath+string(filepath.Separator)) {
			return match, true
		}
	}

	return ResolvedRuleMatch{}, false
}

func allowedMatches(report *ToolRuleReport) []ResolvedRuleMatch {
	matches := make([]ResolvedRuleMatch, 0, len(report.DefaultMatches)+len(report.CustomMatches))
	matches = append(matches, report.DefaultMatches...)
	matches = append(matches, report.CustomMatches...)
	for _, project := range report.ProjectMatches {
		matches = append(matches, project.Matches...)
	}
	return matches
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
