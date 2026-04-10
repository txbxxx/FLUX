package snapshot

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"ai-sync-manager/internal/models"
	"ai-sync-manager/internal/service/tool"
	typesSnapshot "ai-sync-manager/internal/types/snapshot"
	"ai-sync-manager/pkg/database"
	"ai-sync-manager/pkg/logger"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Service 快照管理服务
type Service struct {
	db          *database.DB
	collector   *Collector
	ruleManager *tool.RuleManager
}

// NewService 创建快照服务
func NewService(db *database.DB, resolver *tool.RuleResolver, ruleManager *tool.RuleManager) *Service {
	collector := NewCollector(resolver)

	return &Service{
		db:          db,
		collector:   collector,
		ruleManager: ruleManager,
	}
}

// CreateSnapshot 创建配置快照。
// 它先按规则收集文件，再生成快照记录和可导出的快照包。
func (s *Service) CreateSnapshot(options typesSnapshot.CreateSnapshotOptions) (*typesSnapshot.SnapshotPackage, error) {
	logger.Info("开始创建快照",
		zap.Strings("tools", options.Tools),
		zap.String("project", options.ProjectName),
		zap.String("message", options.Message),
	)

	// 根据项目名获取项目路径
	projectPath, err := s.resolveProjectPath(options.ProjectName, options.Tools)
	if err != nil {
		return nil, err
	}

	// 收集配置文件
	collectOpts := CollectOptions{
		Tools:       options.Tools,
		ProjectPath: projectPath,
	}

	result, err := s.collector.Collect(collectOpts)
	if err != nil {
		return nil, fmt.Errorf("收集配置文件失败: %w", err)
	}

	if len(result.Files) == 0 {
		return nil, fmt.Errorf("未找到任何配置文件")
	}

	// ID 用于数据库和后续同步；名称主要面向用户展示。
	snapshotID := uuid.New().String()
	name := options.Name

	// 创建快照元数据
	metadata := models.SnapshotMetadata{
		OSVersion:   runtime.GOOS + "/" + runtime.GOARCH,
		AppVersion:  "1.0.0", // TODO: 从应用配置获取
		ProjectPath: projectPath,
		Extra:       make(map[string]string),
	}

	// Snapshot 记录完整文件内容，SnapshotPackage 则补充导出/校验所需元数据。
	snapshot := &models.Snapshot{
		ID:          snapshotID,
		Name:        name,
		Description: options.Message,
		Message:     options.Message,
		CreatedAt:   time.Now(),
		Project:     options.ProjectName,
		Metadata:    metadata,
		Files:       result.Files,
		Tags:        options.Tags,
	}

	// 先落库，再返回快照包，保证后续列表和详情查询能立即看到结果。
	snapshotDAO := models.NewSnapshotDAO(s.db)
	if err := snapshotDAO.Create(snapshot); err != nil {
		return nil, fmt.Errorf("保存快照失败: %w", err)
	}

	// 创建快照包
	pkg := &typesSnapshot.SnapshotPackage{
		Snapshot: &typesSnapshot.SnapshotHeader{
			ID:        snapshotID,
			Name:      name,
			Message:   options.Message,
			CreatedAt: time.Now(),
			Project:   options.ProjectName,
		},
		ProjectPath: projectPath,
		CreatedAt:   time.Now(),
		Size:        result.TotalSize,
		FileCount:   len(result.Files),
		Checksum:    models.CalculateChecksum(snapshot.Files),
	}

	logger.Info("快照创建成功",
		zap.String("id", snapshotID),
		zap.String("name", name),
		zap.Int("file_count", len(result.Files)),
		zap.Int64("size", result.TotalSize),
	)

	return pkg, nil
}

// GetSnapshot 获取单个快照详情。
func (s *Service) GetSnapshot(id string) (*models.Snapshot, error) {
	snapshotDAO := models.NewSnapshotDAO(s.db)

	snapshot, err := snapshotDAO.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("获取快照失败: %w", err)
	}

	return snapshot, nil
}

// ListSnapshots 返回分页快照列表。
func (s *Service) ListSnapshots(limit, offset int) ([]*typesSnapshot.SnapshotListItem, error) {
	snapshotDAO := models.NewSnapshotDAO(s.db)

	snapshots, err := snapshotDAO.List(limit, offset)
	if err != nil {
		return nil, fmt.Errorf("列出快照失败: %w", err)
	}

	items := make([]*typesSnapshot.SnapshotListItem, 0, len(snapshots))
	for _, snap := range snapshots {
		items = append(items, &typesSnapshot.SnapshotListItem{
			ID:        snap.ID,
			Name:      snap.Name,
			Message:   snap.Message,
			CreatedAt: snap.CreatedAt,
			Project:   snap.Project,
			FileCount: len(snap.Files),
		})
	}
	return items, nil
}

// DeleteSnapshot 删除快照。
// 当前只删除数据库记录；如果未来增加外部存储，应在这里统一扩展清理逻辑。
// TODO: 当 sync_tasks 和 sync_history 功能实现后，需在此处添加应用层级联清理：
//
//	删除 sync_history 中 task_id IN (SELECT id FROM sync_tasks WHERE snapshot_id = ?) 的记录，
//	随后删除 sync_tasks 中 snapshot_id = ? 的记录。
//	禁止使用数据库触发器，关系完整性由应用层维护。
func (s *Service) DeleteSnapshot(id string) error {
	logger.Info("删除快照", zap.String("id", id))

	snapshotDAO := models.NewSnapshotDAO(s.db)

	if err := snapshotDAO.Delete(id); err != nil {
		return fmt.Errorf("删除快照失败: %w", err)
	}

	logger.Info("快照删除成功", zap.String("id", id))
	return nil
}

// UpdateSnapshot 更新快照记录（包括文件内容）。
func (s *Service) UpdateSnapshot(snapshot *models.Snapshot) error {
	snapshotDAO := models.NewSnapshotDAO(s.db)

	if err := snapshotDAO.UpdateWithFiles(snapshot); err != nil {
		return fmt.Errorf("更新快照失败: %w", err)
	}

	logger.Info("快照更新成功", zap.String("id", snapshot.ID))
	return nil
}

// GetSnapshotFiles 是 GetSnapshot 的轻量包装，供仅关心文件列表的调用方使用。
func (s *Service) GetSnapshotFiles(id string) ([]models.SnapshotFile, error) {
	snapshot, err := s.GetSnapshot(id)
	if err != nil {
		return nil, err
	}

	return snapshot.Files, nil
}

// ExportSnapshot 把快照文件恢复到指定目录结构下。
func (s *Service) ExportSnapshot(id string, exportPath string) error {
	snapshot, err := s.GetSnapshot(id)
	if err != nil {
		return err
	}

	logger.Info("导出快照",
		zap.String("id", id),
		zap.String("export_path", exportPath),
	)

	// 创建导出目录
	if err := os.MkdirAll(exportPath, 0755); err != nil {
		return fmt.Errorf("创建导出目录失败: %w", err)
	}

	// 这里使用快照内记录的相对路径还原目录结构。
	var failedFiles []string
	for _, file := range snapshot.Files {
		targetPath := filepath.Join(exportPath, file.Path)

		if err := s.collector.CloneFileWithContent(targetPath, file.Content); err != nil {
			logger.Warn("导出文件失败",
				zap.String("path", file.Path),
				zap.Error(err),
			)
			failedFiles = append(failedFiles, file.Path)
		}
	}

	logger.Info("快照导出完成",
		zap.String("id", id),
		zap.String("export_path", exportPath),
		zap.Int("failed_count", len(failedFiles)),
	)

	if len(failedFiles) > 0 {
		return fmt.Errorf("导出完成，但 %d 个文件失败：%s", len(failedFiles), strings.Join(failedFiles, ", "))
	}

	return nil
}

// CreateBackup 把现有文件复制到备份目录，供回滚前保护现场使用。
func (s *Service) CreateBackup(paths []string, backupDir string) (string, error) {
	logger.Info("创建备份",
		zap.Int("file_count", len(paths)),
		zap.String("backup_dir", backupDir),
	)

	// 创建备份目录
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("创建备份目录失败: %w", err)
	}

	// 备份文件
	backedUpCount := 0
	for _, path := range paths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue // 文件不存在，跳过
		}

		backupPath, err := s.collector.BackupFile(path, backupDir)
		if err != nil {
			logger.Warn("备份文件失败",
				zap.String("path", path),
				zap.Error(err),
			)
			continue
		}

		backedUpCount++
		logger.Debug("文件已备份",
			zap.String("src", path),
			zap.String("dst", backupPath),
		)
	}

	logger.Info("备份完成",
		zap.Int("total", len(paths)),
		zap.Int("backed_up", backedUpCount),
	)

	return backupDir, nil
}

// ValidateSnapshot 校验快照最基础的结构完整性，不做深层语义校验。
func (s *Service) ValidateSnapshot(snapshot *models.Snapshot) error {
	if snapshot.ID == "" {
		return fmt.Errorf("快照 ID 不能为空")
	}

	if snapshot.Name == "" {
		return fmt.Errorf("快照名称不能为空")
	}

	if snapshot.Project == "" {
		return fmt.Errorf("快照必须指定项目")
	}

	if len(snapshot.Files) == 0 {
		return fmt.Errorf("快照必须包含至少一个文件")
	}

	// 验证文件数据
	for i, file := range snapshot.Files {
		if file.Path == "" {
			return fmt.Errorf("文件 [%d] 路径不能为空", i)
		}
		if file.OriginalPath == "" {
			return fmt.Errorf("文件 [%d] 原始路径不能为空", i)
		}
		if file.ToolType == "" {
			return fmt.Errorf("文件 [%d] 工具类型不能为空", i)
		}
	}

	return nil
}

// GetSnapshotsByTool 按工具筛选快照。
// 使用 SQL LIKE 在数据库层过滤，避免全量加载到内存。
func (s *Service) GetSnapshotsByTool(toolType string, limit, offset int) ([]*models.Snapshot, error) {
	snapshotDAO := models.NewSnapshotDAO(s.db)
	return snapshotDAO.ListByToolType(toolType, limit, offset)
}

// GetSnapshotsByTag 按标签筛选快照。
// 使用 SQL LIKE 在数据库层过滤，避免全量加载到内存。
func (s *Service) GetSnapshotsByTag(tag string, limit, offset int) ([]*models.Snapshot, error) {
	snapshotDAO := models.NewSnapshotDAO(s.db)
	return snapshotDAO.ListByTag(tag, limit, offset)
}

// CountSnapshots 返回当前本地快照总数。
func (s *Service) CountSnapshots() (int, error) {
	snapshotDAO := models.NewSnapshotDAO(s.db)
	return snapshotDAO.Count()
}

// resolveProjectPath 根据项目名解析项目路径。
// 从 RuleManager 获取已注册的项目信息。
func (s *Service) resolveProjectPath(projectName string, tools []string) (string, error) {
	if s.ruleManager == nil {
		return "", fmt.Errorf("规则管理器未初始化")
	}

	// 列出所有已注册项目，查找匹配的项目名
	projects, err := s.ruleManager.ListRegisteredProjects(nil)
	if err != nil {
		return "", fmt.Errorf("查询注册项目失败: %w", err)
	}

	for _, p := range projects {
		if p.ProjectName == projectName {
			// 验证工具类型是否匹配
			if len(tools) == 1 {
				requestedTool := tool.ToolType(tools[0])
				if tool.ToolType(p.ToolType) != requestedTool {
					return "", fmt.Errorf("项目 %s 的工具类型是 %s，但请求的是 %s",
						projectName, p.ToolType, tools[0])
				}
			}
			return p.ProjectPath, nil
		}
	}

	return "", fmt.Errorf("未找到项目: %s（请先使用 scan add 注册，或确保已自动注册全局项目）", projectName)
}

// RestoreSnapshot restores a snapshot's files to their original paths on disk.
// It supports selective file restoration via the files parameter, dry-run preview,
// and automatic backup before writing.
func (s *Service) RestoreSnapshot(id string, files []string, options typesSnapshot.ApplyOptions) (*typesSnapshot.RestoreResult, error) {
	logger.Info("开始恢复快照",
		zap.String("id", id),
		zap.Bool("dry_run", options.DryRun),
		zap.Int("specified_files", len(files)),
	)

	// 第一步：从数据库读取快照数据（含文件内容）
	snapshot, err := s.GetSnapshot(id)
	if err != nil {
		return nil, fmt.Errorf("获取快照失败: %w", err)
	}

	// 第二步：如果指定了文件列表，进行过滤
	if len(files) > 0 {
		filtered, filterErr := filterSnapshotFiles(snapshot.Files, files)
		if filterErr != nil {
			return nil, filterErr
		}
		snapshot.Files = filtered
	}

	if len(snapshot.Files) == 0 {
		return nil, fmt.Errorf("快照中没有可恢复的文件")
	}

	// 第三步：使用 Applier 执行恢复（Applier 内部处理备份 + 写入）
	applier := NewApplier(s.collector)
	applyResult, err := applier.ApplySnapshot(snapshot, options)
	if err != nil {
		return nil, fmt.Errorf("恢复快照失败: %w", err)
	}

	// 第四步：转换为 RestoreResult
	result := &typesSnapshot.RestoreResult{
		SnapshotID:    snapshot.ID,
		SnapshotName:  snapshot.Name,
		AppliedFiles:  applyResult.AppliedFiles,
		SkippedFiles:  applyResult.SkippedFiles,
		Errors:        applyResult.Errors,
		BackupPath:    applyResult.BackupPath,
		TotalFiles:    applyResult.Summary.TotalFiles,
		AppliedCount:  len(applyResult.AppliedFiles),
		SkippedCount:  len(applyResult.SkippedFiles),
		ErrorCount:    len(applyResult.Errors),
		DryRun:        options.DryRun,
	}

	logger.Info("快照恢复完成",
		zap.String("id", id),
		zap.Int("applied", result.AppliedCount),
		zap.Int("skipped", result.SkippedCount),
		zap.Int("errors", result.ErrorCount),
		zap.Bool("dry_run", options.DryRun),
	)

	return result, nil
}

// filterSnapshotFiles filters snapshot files to only include those matching the specified paths.
// Returns an error if any specified path is not found in the snapshot.
func filterSnapshotFiles(allFiles []models.SnapshotFile, specifiedPaths []string) ([]models.SnapshotFile, error) {
	pathSet := make(map[string]bool, len(specifiedPaths))
	for _, p := range specifiedPaths {
		pathSet[p] = true
	}

	var result []models.SnapshotFile
	var notFound []string

	for _, file := range allFiles {
		if pathSet[file.Path] || pathSet[file.OriginalPath] {
			result = append(result, file)
			delete(pathSet, file.Path)
			delete(pathSet, file.OriginalPath)
		}
	}

	// 检查是否有未找到的路径
	for p := range pathSet {
		notFound = append(notFound, p)
	}

	if len(notFound) > 0 {
		return nil, fmt.Errorf("以下文件不在快照中: %s", strings.Join(notFound, ", "))
	}

	return result, nil
}
