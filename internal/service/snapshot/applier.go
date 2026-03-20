package snapshot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ai-sync-manager/internal/models"
	"ai-sync-manager/pkg/logger"

	"go.uber.org/zap"
)

// Applier 快照应用器
type Applier struct {
	collector *Collector
}

// NewApplier 创建快照应用器
func NewApplier(collector *Collector) *Applier {
	return &Applier{
		collector: collector,
	}
}

// ApplySnapshot 应用快照到本地
func (a *Applier) ApplySnapshot(
	snapshot *models.Snapshot,
	options models.ApplyOptions,
) (*models.ApplyResult, error) {
	logger.Info("开始应用快照",
		zap.String("id", snapshot.ID),
		zap.String("name", snapshot.Name),
		zap.Bool("create_backup", options.CreateBackup),
		zap.Bool("force", options.Force),
		zap.Bool("dry_run", options.DryRun),
	)

	result := &models.ApplyResult{
		Success:      true,
		AppliedFiles: make([]models.AppliedFile, 0),
		SkippedFiles: make([]models.SkippedFile, 0),
		Errors:       make([]models.ApplyError, 0),
		Summary:      models.ChangeSummary{},
	}

	// 创建备份
	if options.CreateBackup && !options.DryRun {
		backupPaths := a.getFilesToBackup(snapshot)
		if len(backupPaths) > 0 {
			backupPath, err := a.createBackup(backupPaths, options.BackupPath)
			if err != nil {
				return nil, fmt.Errorf("创建备份失败: %w", err)
			}
			result.BackupPath = backupPath
			logger.Info("备份创建完成", zap.String("path", backupPath))
		}
	}

	// 应用每个文件
	for _, file := range snapshot.Files {
		applied, skipped, err := a.applyFile(file, options)
		if err != nil {
			result.Success = false
			result.Errors = append(result.Errors, models.ApplyError{
				Path:    file.OriginalPath,
				Message: err.Error(),
			})
		} else if applied != nil {
			result.AppliedFiles = append(result.AppliedFiles, *applied)
			result.Summary.Updated++
		} else if skipped != nil {
			result.SkippedFiles = append(result.SkippedFiles, *skipped)
			result.Summary.Skipped++
		}
		result.Summary.TotalFiles++
	}

	// 统计按工具和类别分组的文件数
	result.Summary.FilesByTool = make(map[string]int)
	result.Summary.FilesByCategory = make(map[string]int)
	for _, file := range snapshot.Files {
		result.Summary.FilesByTool[file.ToolType]++
		result.Summary.FilesByCategory[string(file.Category)]++
	}

	if result.Success {
		logger.Info("快照应用完成",
			zap.String("id", snapshot.ID),
			zap.Int("applied", len(result.AppliedFiles)),
			zap.Int("skipped", len(result.SkippedFiles)),
		)
	} else {
		logger.Warn("快照应用部分失败",
			zap.String("id", snapshot.ID),
			zap.Int("errors", len(result.Errors)),
		)
	}

	return result, nil
}

// applyFile 应用单个文件
func (a *Applier) applyFile(
	file models.SnapshotFile,
	options models.ApplyOptions,
) (*models.AppliedFile, *models.SkippedFile, error) {
	targetPath := file.OriginalPath

	// 检查文件是否已存在
	exists := a.fileExists(targetPath)

	if exists && !options.Force {
		// 检查内容是否相同
		if a.contentMatches(targetPath, file.Content) {
			return nil, &models.SkippedFile{
				Path:   targetPath,
				Reason: "内容相同",
			}, nil
		}
	}

	// 干运行模式
	if options.DryRun {
		action := "created"
		if exists {
			action = "updated"
		}
		return &models.AppliedFile{
			Path:         targetPath,
			OriginalPath: file.OriginalPath,
			Action:       action,
		}, nil, nil
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return nil, nil, fmt.Errorf("创建目录失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(targetPath, file.Content, 0644); err != nil {
		return nil, nil, fmt.Errorf("写入文件失败: %w", err)
	}

	// 确定操作类型
	action := "created"
	if exists {
		action = "updated"
	}

	return &models.AppliedFile{
		Path:         targetPath,
		OriginalPath: file.OriginalPath,
		Action:       action,
	}, nil, nil
}

// getFilesToBackup 获取需要备份的文件列表
func (a *Applier) getFilesToBackup(snapshot *models.Snapshot) []string {
	var paths []string
	for _, file := range snapshot.Files {
		if a.fileExists(file.OriginalPath) {
			paths = append(paths, file.OriginalPath)
		}
	}
	return paths
}

// createBackup 创建备份
func (a *Applier) createBackup(paths []string, backupDir string) (string, error) {
	if backupDir == "" {
		// 使用临时目录
		tmpDir := os.TempDir()
		backupDir = filepath.Join(tmpDir, fmt.Sprintf("ai-sync-manager-backup-%d", getCurrentTimestamp()))
	}

	for _, path := range paths {
		if _, err := a.collector.BackupFile(path, backupDir); err != nil {
			logger.Warn("备份文件失败",
				zap.String("path", path),
				zap.Error(err),
			)
		}
	}

	return backupDir, nil
}

// fileExists 检查文件是否存在
func (a *Applier) fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// contentMatches 检查文件内容是否匹配
func (a *Applier) contentMatches(path string, content []byte) bool {
	existing, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	// 简单比较
	if len(existing) != len(content) {
		return false
	}

	for i := range existing {
		if existing[i] != content[i] {
			return false
		}
	}

	return true
}

// getCurrentTimestamp 获取当前时间戳
func getCurrentTimestamp() int64 {
	return time.Now().Unix()
}

// RestoreFile 恢复文件
func (a *Applier) RestoreFile(backupPath, targetPath string) error {
	logger.Info("恢复文件",
		zap.String("backup", backupPath),
		zap.String("target", targetPath),
	)

	if err := a.collector.CloneFile(backupPath, targetPath); err != nil {
		return fmt.Errorf("恢复文件失败: %w", err)
	}

	return nil
}

// RestoreFromBackup 从备份目录恢复所有文件
func (a *Applier) RestoreFromBackup(backupDir string, snapshot *models.Snapshot) error {
	logger.Info("从备份恢复",
		zap.String("backup_dir", backupDir),
		zap.String("snapshot_id", snapshot.ID),
	)

	errors := make([]string, 0)

	for _, file := range snapshot.Files {
		backupPath := filepath.Join(backupDir, file.Path)
		targetPath := file.OriginalPath

		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			logger.Debug("备份文件不存在，跳过",
				zap.String("backup", backupPath),
			)
			continue
		}

		if err := a.RestoreFile(backupPath, targetPath); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", file.Path, err.Error()))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("部分文件恢复失败: %s", strings.Join(errors, "; "))
	}

	logger.Info("恢复完成",
		zap.String("backup_dir", backupDir),
		zap.Int("file_count", len(snapshot.Files)),
	)

	return nil
}

// DeleteSnapshotFiles 删除快照中的所有文件
func (a *Applier) DeleteSnapshotFiles(snapshot *models.Snapshot) ([]string, error) {
	logger.Info("删除快照文件",
		zap.String("id", snapshot.ID),
	)

	deleted := make([]string, 0)
	errors := make([]string, 0)

	for _, file := range snapshot.Files {
		if err := os.Remove(file.OriginalPath); err != nil {
			if !os.IsNotExist(err) {
				errors = append(errors, fmt.Sprintf("%s: %s", file.Path, err.Error()))
			}
		} else {
			deleted = append(deleted, file.OriginalPath)
		}
	}

	if len(errors) > 0 {
		return deleted, fmt.Errorf("部分文件删除失败: %s", strings.Join(errors, "; "))
	}

	logger.Info("文件删除完成",
		zap.String("id", snapshot.ID),
		zap.Int("deleted", len(deleted)),
	)

	return deleted, nil
}

// CleanEmptyDirectories 清理空目录
func (a *Applier) CleanEmptyDirectories(basePath string) ([]string, error) {
	logger.Info("清理空目录", zap.String("base_path", basePath))

	cleaned := make([]string, 0)

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			return nil
		}

		// 跳过根目录
		if path == basePath {
			return nil
		}

		// 检查目录是否为空
		isEmpty, err := isDirEmpty(path)
		if err != nil {
			return err
		}

		if isEmpty {
			if err := os.Remove(path); err != nil {
				return err
			}
			cleaned = append(cleaned, path)
		}

		return nil
	})

	if err != nil {
		return cleaned, err
	}

	logger.Info("空目录清理完成",
		zap.String("base_path", basePath),
		zap.Int("cleaned", len(cleaned)),
	)

	return cleaned, nil
}

// isDirEmpty 检查目录是否为空
func isDirEmpty(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == nil {
		return false, nil
	}
	if err.Error() == "EOF" {
		return true, nil
	}
	return false, err
}
