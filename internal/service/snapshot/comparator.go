package snapshot

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"flux/internal/models"
	"flux/pkg/logger"
	"flux/pkg/utils"

	typesSnapshot "flux/internal/types/snapshot"

	"go.uber.org/zap"
)

// Comparator 快照比较器
type Comparator struct {
	collector *Collector
}

// NewComparator 创建快照比较器
func NewComparator(collector *Collector) *Comparator {
	return &Comparator{
		collector: collector,
	}
}

// CompareSnapshots 比较两个快照的差异
func (c *Comparator) CompareSnapshots(source, target *models.Snapshot) (*typesSnapshot.ChangeSummary, error) {
	logger.Info("比较快照",
		zap.Uint64("source_id", uint64(source.ID)),
		zap.Uint64("target_id", uint64(target.ID)),
	)

	summary := &typesSnapshot.ChangeSummary{
		FilesByTool:     make(map[string]int),
		FilesByCategory: make(map[string]int),
	}

	// 构建目标快照文件映射
	targetFiles := make(map[string]models.SnapshotFile)
	for _, file := range target.Files {
		targetFiles[file.Path] = file
	}

	// 比较源快照文件
	sourceSeen := make(map[string]bool)
	for _, sourceFile := range source.Files {
		sourceSeen[sourceFile.Path] = true

		targetFile, exists := targetFiles[sourceFile.Path]

		if !exists {
			// 新建文件
			summary.Created++
			c.logFileChange(sourceFile.Path, "created", "")
		} else if sourceFile.Hash != targetFile.Hash {
			// 更新文件
			summary.Updated++
			c.logFileChange(sourceFile.Path, "updated", "")
		} else {
			// 无变化
			c.logFileChange(sourceFile.Path, "same", "")
		}

		summary.FilesByTool[sourceFile.ToolType]++
		summary.FilesByCategory[string(sourceFile.Category)]++
	}

	// 检查删除的文件
	for _, targetFile := range target.Files {
		if !sourceSeen[targetFile.Path] {
			summary.Deleted++
			c.logFileChange(targetFile.Path, "deleted", "")
		}
	}

	summary.TotalFiles = len(source.Files)

	logger.Info("快照比较完成",
		zap.Int("created", summary.Created),
		zap.Int("updated", summary.Updated),
		zap.Int("deleted", summary.Deleted),
	)

	return summary, nil
}

// CompareWithFileSystem 将快照与文件系统比较
func (c *Comparator) CompareWithFileSystem(
	snapshot *models.Snapshot,
	projectPath string,
) (*typesSnapshot.ChangeSummary, error) {
	logger.Info("比较快照与文件系统",
		zap.Uint64("snapshot_id", uint64(snapshot.ID)),
		zap.String("project_path", projectPath),
	)

	summary := &typesSnapshot.ChangeSummary{
		FilesByTool:     make(map[string]int),
		FilesByCategory: make(map[string]int),
	}

	for _, snapshotFile := range snapshot.Files {
		filePath := snapshotFile.OriginalPath

		// 检查文件是否存在
		info, err := os.Stat(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				summary.Created++
				c.logFileChange(filePath, "missing", "文件不存在")
			} else {
				logger.Warn("无法访问文件",
					zap.String("path", filePath),
					zap.Error(err),
				)
				summary.Skipped++
			}
			continue
		}

		// 跳过目录
		if info.IsDir() {
			summary.Skipped++
			continue
		}

		// 读取当前文件内容
		currentContent, err := os.ReadFile(filePath)
		if err != nil {
			logger.Warn("无法读取文件",
				zap.String("path", filePath),
				zap.Error(err),
			)
			summary.Skipped++
			continue
		}

		// 计算当前文件哈希
		currentHash := utils.SHA256Hash(currentContent)

		// 比较哈希
		if currentHash != snapshotFile.Hash {
			summary.Updated++
			c.logFileChange(filePath, "different", "")
		} else {
			c.logFileChange(filePath, "same", "")
		}

		summary.FilesByTool[snapshotFile.ToolType]++
		summary.FilesByCategory[string(snapshotFile.Category)]++
		summary.TotalFiles++
	}

	logger.Info("文件系统比较完成",
		zap.Int("total", summary.TotalFiles),
		zap.Int("created", summary.Created),
		zap.Int("updated", summary.Updated),
		zap.Int("skipped", summary.Skipped),
	)

	return summary, nil
}

// CompareFiles 比较两个文件
func (c *Comparator) CompareFiles(path1, path2 string) (*FileComparison, error) {
	logger.Debug("比较文件",
		zap.String("path1", path1),
		zap.String("path2", path2),
	)

	comparison := &FileComparison{
		Path:   path1,
		Status: FileStatusSame,
	}

	// 读取源文件
	sourceContent, err := os.ReadFile(path1)
	if err != nil {
		return nil, err
	}
	comparison.SourceHash = utils.SHA256Hash(sourceContent)

	// 读取目标文件
	targetContent, err := os.ReadFile(path2)
	if err != nil {
		if os.IsNotExist(err) {
			comparison.Status = FileStatusCreated
			return comparison, nil
		}
		return nil, err
	}
	comparison.TargetHash = utils.SHA256Hash(targetContent)

	// 比较哈希
	if comparison.SourceHash != comparison.TargetHash {
		comparison.Status = FileStatusUpdated
	}

	return comparison, nil
}

// DetectConflicts 检测应用快照时的潜在冲突
func (c *Comparator) DetectConflicts(
	snapshot *models.Snapshot,
	force bool,
) ([]typesSnapshot.ApplyError, error) {
	logger.Info("检测冲突",
		zap.Uint64("snapshot_id", uint64(snapshot.ID)),
		zap.Bool("force", force),
	)

	conflicts := make([]typesSnapshot.ApplyError, 0)

	for _, file := range snapshot.Files {
		// 检查文件是否存在
		info, err := os.Stat(file.OriginalPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue // 文件不存在，无冲突
			}
			conflicts = append(conflicts, typesSnapshot.ApplyError{
				Path:    file.OriginalPath,
				Message: err.Error(),
			})
			continue
		}

		// 跳过目录
		if info.IsDir() {
			conflicts = append(conflicts, typesSnapshot.ApplyError{
				Path:    file.OriginalPath,
				Message: "路径是目录，无法应用文件",
			})
			continue
		}

		// 读取现有文件
		existingContent, err := os.ReadFile(file.OriginalPath)
		if err != nil {
			conflicts = append(conflicts, typesSnapshot.ApplyError{
				Path:    file.OriginalPath,
				Message: err.Error(),
			})
			continue
		}

		// 计算现有文件哈希
		existingHash := utils.SHA256Hash(existingContent)

		// 检查是否冲突
		if existingHash != file.Hash && !force {
			conflicts = append(conflicts, typesSnapshot.ApplyError{
				Path:    file.OriginalPath,
				Message: "文件内容不同且未启用强制覆盖",
			})
		}
	}

	if len(conflicts) > 0 {
		logger.Warn("检测到冲突",
			zap.Uint64("snapshot_id", uint64(snapshot.ID)),
			zap.Int("conflicts", len(conflicts)),
		)
	}

	return conflicts, nil
}

// GetFileChanges 获取文件变更详情
func (c *Comparator) GetFileChanges(
	source, target *models.Snapshot,
) ([]FileComparison, error) {
	logger.Info("获取文件变更详情",
		zap.Uint64("source_id", uint64(source.ID)),
		zap.Uint64("target_id", uint64(target.ID)),
	)

	// 构建文件映射
	sourceFiles := make(map[string]models.SnapshotFile)
	for _, file := range source.Files {
		sourceFiles[file.Path] = file
	}

	targetFiles := make(map[string]models.SnapshotFile)
	for _, file := range target.Files {
		targetFiles[file.Path] = file
	}

	// 收集所有唯一路径
	allPaths := make(map[string]bool)
	for path := range sourceFiles {
		allPaths[path] = true
	}
	for path := range targetFiles {
		allPaths[path] = true
	}

	// 比较每个文件
	changes := make([]FileComparison, 0, len(allPaths))
	for path := range allPaths {
		sourceFile, sourceExists := sourceFiles[path]
		targetFile, targetExists := targetFiles[path]

		comparison := FileComparison{Path: path}

		if !sourceExists {
			comparison.Status = FileStatusDeleted
			comparison.TargetHash = targetFile.Hash
		} else if !targetExists {
			comparison.Status = FileStatusCreated
			comparison.SourceHash = sourceFile.Hash
		} else if sourceFile.Hash != targetFile.Hash {
			comparison.Status = FileStatusUpdated
			comparison.SourceHash = sourceFile.Hash
			comparison.TargetHash = targetFile.Hash
		} else {
			comparison.Status = FileStatusSame
			comparison.SourceHash = sourceFile.Hash
			comparison.TargetHash = targetFile.Hash
		}

		changes = append(changes, comparison)
	}

	return changes, nil
}

// FindChangedFiles 查找变更的文件
func (c *Comparator) FindChangedFiles(
	snapshot *models.Snapshot,
	since time.Time, // 这里需要导入 time 包
) ([]models.SnapshotFile, error) {
	changed := make([]models.SnapshotFile, 0)

	for _, file := range snapshot.Files {
		if file.ModifiedAt.After(since) {
			changed = append(changed, file)
		}
	}

	return changed, nil
}


// logFileChange 记录文件变更日志
func (c *Comparator) logFileChange(path, status, reason string) {
	logger.Debug("文件变更",
		zap.String("path", path),
		zap.String("status", status),
		zap.String("reason", reason),
	)
}

// GetDiff 获取两个文件的差异（简单行比较）
func (c *Comparator) GetDiff(path1, path2 string) ([]string, error) {
	content1, err := c.collector.ReadFileContent(path1)
	if err != nil {
		return nil, err
	}

	content2, err := c.collector.ReadFileContent(path2)
	if err != nil {
		return nil, err
	}

	// 简单比较：返回不同的行
	diff := make([]string, 0)
	maxLen := len(content1)
	if len(content2) > maxLen {
		maxLen = len(content2)
	}

	for i := 0; i < maxLen; i++ {
		line1 := ""
		line2 := ""

		if i < len(content1) {
			line1 = content1[i]
		}
		if i < len(content2) {
			line2 = content2[i]
		}

		if line1 != line2 {
			if line1 != "" {
				diff = append(diff, "- "+line1)
			}
			if line2 != "" {
				diff = append(diff, "+ "+line2)
			}
		}
	}

	return diff, nil
}

// ValidateFileIntegrity 验证文件完整性
func (c *Comparator) ValidateFileIntegrity(snapshot *models.Snapshot) ([]string, error) {
	logger.Info("验证文件完整性",
		zap.Uint64("snapshot_id", uint64(snapshot.ID)),
	)

	invalid := make([]string, 0)

	for _, file := range snapshot.Files {
		// 验证文件路径
		if file.Path == "" {
			invalid = append(invalid, "文件路径为空")
			continue
		}

		// 验证哈希
		if file.Hash == "" {
			invalid = append(invalid, file.Path+": 哈希为空")
			continue
		}

		// 重新计算哈希并验证
		calculatedHash := utils.SHA256Hash(file.Content)
		if calculatedHash != file.Hash {
			invalid = append(invalid, file.Path+": 哈希不匹配")
		}
	}

	if len(invalid) > 0 {
		logger.Warn("文件完整性验证失败",
			zap.Uint64("snapshot_id", uint64(snapshot.ID)),
			zap.Int("invalid", len(invalid)),
		)
	}

	return invalid, nil
}

// GetSnapshotSize 获取快照大小（字节数）
func (c *Comparator) GetSnapshotSize(snapshot *models.Snapshot) int64 {
	var total int64
	for _, file := range snapshot.Files {
		total += file.Size
	}
	return total
}

// GetFilesByCategory 按类别获取文件
func (c *Comparator) GetFilesByCategory(
	snapshot *models.Snapshot,
	category models.FileCategory,
) []models.SnapshotFile {
	files := make([]models.SnapshotFile, 0)
	for _, file := range snapshot.Files {
		if file.Category == category {
			files = append(files, file)
		}
	}
	return files
}

// GetFilesByTool 按工具获取文件
func (c *Comparator) GetFilesByTool(
	snapshot *models.Snapshot,
	toolType string,
) []models.SnapshotFile {
	files := make([]models.SnapshotFile, 0)
	for _, file := range snapshot.Files {
		if file.ToolType == toolType {
			files = append(files, file)
		}
	}
	return files
}

// SearchFiles 搜索文件
func (c *Comparator) SearchFiles(
	snapshot *models.Snapshot,
	pattern string,
) []models.SnapshotFile {
	files := make([]models.SnapshotFile, 0)
	for _, file := range snapshot.Files {
		if strings.Contains(strings.ToLower(file.Path), strings.ToLower(pattern)) ||
			strings.Contains(strings.ToLower(filepath.Base(file.Path)), strings.ToLower(pattern)) {
			files = append(files, file)
		}
	}
	return files
}
