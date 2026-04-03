package snapshot

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"ai-sync-manager/internal/models"
	"ai-sync-manager/internal/service/tool"
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
func (s *Service) CreateSnapshot(options models.CreateSnapshotOptions) (*models.SnapshotPackage, error) {
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
	if name == "" {
		name = fmt.Sprintf("Snapshot-%s", time.Now().Format("20060102-150405"))
	}

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
		Tools:       options.Tools,
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
	pkg := &models.SnapshotPackage{
		Snapshot:    snapshot,
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
func (s *Service) ListSnapshots(limit, offset int) ([]*models.Snapshot, error) {
	snapshotDAO := models.NewSnapshotDAO(s.db)

	snapshots, err := snapshotDAO.List(limit, offset)
	if err != nil {
		return nil, fmt.Errorf("列出快照失败: %w", err)
	}

	return snapshots, nil
}

// DeleteSnapshot 删除快照。
// 当前只删除数据库记录；如果未来增加外部存储，应在这里统一扩展清理逻辑。
func (s *Service) DeleteSnapshot(id string) error {
	logger.Info("删除快照", zap.String("id", id))

	snapshotDAO := models.NewSnapshotDAO(s.db)

	if err := snapshotDAO.Delete(id); err != nil {
		return fmt.Errorf("删除快照失败: %w", err)
	}

	logger.Info("快照删除成功", zap.String("id", id))
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
	for _, file := range snapshot.Files {
		targetPath := filepath.Join(exportPath, file.Path)

		if err := s.collector.CloneFileWithContent(targetPath, file.Content); err != nil {
			logger.Warn("导出文件失败",
				zap.String("path", file.Path),
				zap.Error(err),
			)
			continue
		}
	}

	logger.Info("快照导出完成",
		zap.String("id", id),
		zap.String("export_path", exportPath),
	)

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

	if len(snapshot.Tools) == 0 {
		return fmt.Errorf("快照必须包含至少一个工具")
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
// 当前在内存中过滤，适合当前数据规模。
func (s *Service) GetSnapshotsByTool(toolType string, limit, offset int) ([]*models.Snapshot, error) {
	snapshotDAO := models.NewSnapshotDAO(s.db)

	allSnapshots, err := snapshotDAO.List(0, 0) // 获取所有快照
	if err != nil {
		return nil, err
	}

	// 筛选包含指定工具的快照
	var filtered []*models.Snapshot
	for _, snapshot := range allSnapshots {
		for _, t := range snapshot.Tools {
			if t == toolType {
				filtered = append(filtered, snapshot)
				break
			}
		}
	}

	// 应用分页
	if offset >= len(filtered) {
		return []*models.Snapshot{}, nil
	}

	end := offset + limit
	if end > len(filtered) || limit <= 0 {
		end = len(filtered)
	}

	return filtered[offset:end], nil
}

// GetSnapshotsByTag 按标签筛选快照。
// 当前在内存中过滤，适合当前数据规模。
func (s *Service) GetSnapshotsByTag(tag string, limit, offset int) ([]*models.Snapshot, error) {
	snapshotDAO := models.NewSnapshotDAO(s.db)

	allSnapshots, err := snapshotDAO.List(0, 0) // 获取所有快照
	if err != nil {
		return nil, err
	}

	// 筛选包含指定标签的快照
	var filtered []*models.Snapshot
	for _, snapshot := range allSnapshots {
		for _, t := range snapshot.Tags {
			if t == tag {
				filtered = append(filtered, snapshot)
				break
			}
		}
	}

	// 应用分页
	if offset >= len(filtered) {
		return []*models.Snapshot{}, nil
	}

	end := offset + limit
	if end > len(filtered) || limit <= 0 {
		end = len(filtered)
	}

	return filtered[offset:end], nil
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
