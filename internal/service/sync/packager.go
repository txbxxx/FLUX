package sync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ai-sync-manager/internal/models"
	"ai-sync-manager/pkg/logger"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Packager 快照打包器
type Packager struct{}

// NewPackager 创建快照打包器
func NewPackager() *Packager {
	return &Packager{}
}

// PackageSnapshot 将快照打包为 Git 提交
func (p *Packager) PackageSnapshot(
	snapshot *models.Snapshot,
	repoPath string,
) (*PackagedSnapshot, error) {
	logger.Info("打包快照",
		zap.String("snapshot_id", snapshot.ID),
		zap.String("repo_path", repoPath),
	)

	packaged := &PackagedSnapshot{
		Snapshot: snapshot,
		RepoPath: repoPath,
		Metadata: PackageMetadata{},
		Files:    make([]PackagedFile, len(snapshot.Files)),
	}

	// 创建快照元数据文件
	metadataContent, err := p.createMetadataFile(snapshot)
	if err != nil {
		return nil, fmt.Errorf("创建元数据文件失败: %w", err)
	}
	packaged.MetadataFile = metadataContent

	// 打包每个文件
	for i, file := range snapshot.Files {
		packagedFile, err := p.packageFile(file, repoPath)
		if err != nil {
			logger.Warn("打包文件失败",
				zap.String("path", file.Path),
				zap.Error(err),
			)
			continue
		}
		packaged.Files[i] = *packagedFile
	}

	logger.Info("快照打包完成",
		zap.String("snapshot_id", snapshot.ID),
		zap.Int("file_count", len(packaged.Files)),
	)

	return packaged, nil
}

// packageFile 打包单个文件
func (p *Packager) packageFile(
	file models.SnapshotFile,
	repoPath string,
) (*PackagedFile, error) {
	packaged := &PackagedFile{
		File:     file,
		FilePath: p.getRepoFilePath(file, repoPath),
	}

	// 如果文件有内容，确保目标目录存在
	if len(file.Content) > 0 {
		dir := filepath.Dir(packaged.FilePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("创建目录失败: %w", err)
		}
	}

	return packaged, nil
}

// createMetadataFile 创建快照元数据文件
func (p *Packager) createMetadataFile(snapshot *models.Snapshot) ([]byte, error) {
	metadata := struct {
		ID          string                  `json:"id"`
		Name        string                  `json:"name"`
		Description string                  `json:"description"`
		Message     string                  `json:"message"`
		CreatedAt   string                  `json:"created_at"`
		Project     string                  `json:"project"`
		Tags        []string                `json:"tags"`
		Metadata    models.SnapshotMetadata `json:"metadata"`
		FileCount   int                     `json:"file_count"`
	}{
		ID:          snapshot.ID,
		Name:        snapshot.Name,
		Description: snapshot.Description,
		Message:     snapshot.Message,
		CreatedAt:   snapshot.CreatedAt.Format("2006-01-02T15:04:05Z"),
		Project:     snapshot.Project,
		Tags:        snapshot.Tags,
		Metadata:    snapshot.Metadata,
		FileCount:   len(snapshot.Files),
	}

	return json.Marshal(metadata)
}

// getRepoFilePath 获取文件在仓库中的路径
func (p *Packager) getRepoFilePath(file models.SnapshotFile, repoPath string) string {
	// 构造仓库中的文件路径
	// 格式: .ai-sync/snapshots/{snapshot_id}/{file.Path}
	filename := filepath.Base(file.OriginalPath)
	if filename == "" {
		filename = filepath.Base(file.Path)
	}
	snapshotDir := filepath.Join(repoPath, ".ai-sync", "snapshots", file.ToolType, filename)
	return snapshotDir
}

// ParseSnapshotFromCommit 从 Git 提交解析快照
func (p *Packager) ParseSnapshotFromCommit(
	commitHash string,
	metadataContent []byte,
	repoPath string,
) (*models.Snapshot, error) {
	// 解析元数据
	var metadata struct {
		ID          string                  `json:"id"`
		Name        string                  `json:"name"`
		Description string                  `json:"description"`
		Message     string                  `json:"message"`
		CreatedAt   string                  `json:"created_at"`
		Project     string                  `json:"project"`
		Tags        []string                `json:"tags"`
		Metadata    models.SnapshotMetadata `json:"metadata"`
		FileCount   int                     `json:"file_count"`
	}

	if err := json.Unmarshal(metadataContent, &metadata); err != nil {
		return nil, fmt.Errorf("解析元数据失败: %w", err)
	}

	snapshot := &models.Snapshot{
		ID:          metadata.ID,
		Name:        metadata.Name,
		Description: metadata.Description,
		Message:     metadata.Message,
		Project:     metadata.Project,
		Tags:        metadata.Tags,
		Metadata:    metadata.Metadata,
		CommitHash:  commitHash,
	}

	// TODO: 从文件系统读取文件列表

	return snapshot, nil
}

// CreateCommitMessage 创建 Git 提交消息
func (p *Packager) CreateCommitMessage(snapshot *models.Snapshot) string {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("Snapshot: %s\n", snapshot.Name))
	buf.WriteString(fmt.Sprintf("ID: %s\n", snapshot.ID))

	if snapshot.Description != "" {
		buf.WriteString(fmt.Sprintf("\n%s\n", snapshot.Description))
	}

	buf.WriteString("\n")
	buf.WriteString(fmt.Sprintf("Tools: %s\n", snapshot.Project))
	buf.WriteString(fmt.Sprintf("Files: %d\n", len(snapshot.Files)))

	if len(snapshot.Tags) > 0 {
		buf.WriteString(fmt.Sprintf("Tags: %s\n", strings.Join(snapshot.Tags, ", ")))
	}

	return buf.String()
}

// ValidateSnapshotForPush 验证快照是否可以推送
func (p *Packager) ValidateSnapshotForPush(snapshot *models.Snapshot) error {
	if snapshot.ID == "" {
		return fmt.Errorf("快照 ID 不能为空")
	}

	if len(snapshot.Files) == 0 {
		return fmt.Errorf("快照必须包含至少一个文件")
	}

	for i, file := range snapshot.Files {
		if file.Path == "" {
			return fmt.Errorf("文件 [%d] 路径不能为空", i)
		}
		if file.OriginalPath == "" {
			return fmt.Errorf("文件 [%d] 原始路径不能为空", i)
		}
	}

	return nil
}

// PackagedSnapshot 打包的快照
type PackagedSnapshot struct {
	Snapshot     *models.Snapshot // 快照对象
	RepoPath     string           // 仓库路径
	Metadata     PackageMetadata  // 打包元数据
	MetadataFile []byte           // 元数据文件内容
	Files        []PackagedFile   // 打包的文件列表
}

// PackageMetadata 打包元数据
type PackageMetadata struct {
	PackagedAt string `json:"packaged_at"` // 打包时间
	TotalSize  int64  `json:"total_size"`  // 总大小
	FileCount  int    `json:"file_count"`  // 文件数量
	Compressed bool   `json:"compressed"`  // 是否压缩
}

// PackagedFile 打包的文件
type PackagedFile struct {
	File     models.SnapshotFile // 文件对象
	FilePath string              // 仓库中的路径
}

// SnapshotFileIndex 快照文件索引
type SnapshotFileIndex struct {
	SnapshotID string            `json:"snapshot_id"` // 快照 ID
	FilePaths  []string          `json:"file_paths"`  // 文件路径列表
	Metadata   map[string]string `json:"metadata"`    // 元数据
}

// CreateIndex 创建快照文件索引
func (p *Packager) CreateIndex(snapshot *models.Snapshot) *SnapshotFileIndex {
	paths := make([]string, len(snapshot.Files))
	for i, file := range snapshot.Files {
		paths[i] = file.Path
	}

	return &SnapshotFileIndex{
		SnapshotID: snapshot.ID,
		FilePaths:  paths,
		Metadata: map[string]string{
			"name":        snapshot.Name,
			"description": snapshot.Description,
			"created_at":  snapshot.CreatedAt.Format("2006-01-02T15:04:05Z"),
			"tools":       snapshot.Project,
		},
	}
}

// ParseSnapshotIDFromCommit 从提交消息中解析快照 ID
func (p *Packager) ParseSnapshotIDFromCommit(message string) string {
	// 提交消息格式: "Snapshot: {name}\nID: {snapshot_id}"
	lines := strings.Split(message, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "ID: ") {
			return strings.TrimPrefix(line, "ID: ")
		}
	}
	return ""
}

// IsSnapshotCommit 检查是否为快照提交
func (p *Packager) IsSnapshotCommit(message string) bool {
	return strings.HasPrefix(message, "Snapshot:") ||
		strings.Contains(message, "snapshot_id:")
}

// ExtractSnapshotID 从目录路径提取快照 ID
func (p *Packager) ExtractSnapshotID(path string) string {
	// 路径格式: .ai-sync/snapshots/{tool_type}/{file_name}
	// 或从索引文件中解析
	// 支持正斜杠和反斜杠作为分隔符
	if strings.Contains(path, ".ai-sync/snapshots/") || strings.Contains(path, ".ai-sync\\snapshots\\") {
		// 先尝试正斜杠分割
		parts := strings.Split(path, "/")
		for _, part := range parts {
			if looksLikeUUID(part) {
				return part
			}
		}
		// 再尝试反斜杠分割
		parts = strings.Split(path, "\\")
		for _, part := range parts {
			if looksLikeUUID(part) {
				return part
			}
		}
	}
	return ""
}

// looksLikeUUID 检查字符串是否像 UUID
func looksLikeUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	parts := strings.Split(s, "-")
	if len(parts) != 5 {
		return false
	}
	expectedLengths := []int{8, 4, 4, 4, 12}
	for i, part := range parts {
		if len(part) != expectedLengths[i] {
			return false
		}
	}
	return true
}

// GenerateSnapshotID 生成快照 ID
func GenerateSnapshotID() string {
	return uuid.New().String()
}

// CreateSnapshotManifest 创建快照清单文件
func (p *Packager) CreateSnapshotManifest(
	snapshots []*models.Snapshot,
	repoPath string,
) error {
	manifest := make(map[string]interface{})

	for _, snapshot := range snapshots {
		manifest[snapshot.ID] = map[string]interface{}{
			"name":        snapshot.Name,
			"description": snapshot.Description,
			"message":     snapshot.Message,
			"created_at":  snapshot.CreatedAt.Format("2006-01-02T15:04:05Z"),
			"commit_hash": snapshot.CommitHash,
			"project":     snapshot.Project,
			"tags":        snapshot.Tags,
			"file_count":  len(snapshot.Files),
		}
	}

	content, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化清单失败: %w", err)
	}

	manifestPath := filepath.Join(repoPath, ".ai-sync", "manifest.json")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(manifestPath, content, 0644)
}

// ReadSnapshotManifest 读取快照清单文件
func (p *Packager) ReadSnapshotManifest(repoPath string) (map[string]interface{}, error) {
	manifestPath := filepath.Join(repoPath, ".ai-sync", "manifest.json")
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	var manifest map[string]interface{}
	if err := json.Unmarshal(content, &manifest); err != nil {
		return nil, err
	}

	return manifest, nil
}
