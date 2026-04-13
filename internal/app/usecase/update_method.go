package usecase

import (
	"context"
	"strings"
	"time"

	"flux/internal/models"
	typesSnapshot "flux/internal/types/snapshot"
)

// UpdateSnapshot updates an existing snapshot by re-scanning the associated project's
// configuration files, comparing with the stored data, and persisting changes if any.
//
// 流程：
//  1. 通过 ID 或名称查找快照
//  2. 从快照元数据提取 project 信息，自动推导工具类型
//  3. 重新扫描该 project 的配置文件
//  4. 对比新旧文件哈希集合
//  5. 无变化 → 提示用户；有变化 → 更新 SQLite
func (w *LocalWorkflow) UpdateSnapshot(_ context.Context, input UpdateSnapshotInput) (*typesSnapshot.UpdateSnapshotResult, error) {
	// 第一步：参数校验
	if strings.TrimSpace(input.IDOrName) == "" {
		return nil, &UserError{
			Message:    "更新快照失败：请指定快照 ID 或名称",
			Suggestion: "使用 snapshot list 查看快照列表",
		}
	}

	// 第二步：解析快照 ID（支持名称查找）
	id, err := w.resolveSnapshotID(strings.TrimSpace(input.IDOrName))
	if err != nil {
		return nil, err
	}

	// 第三步：获取快照数据
	snapshot, err := w.snapshots.GetSnapshot(id)
	if err != nil {
		return nil, &UserError{
			Message:    "更新快照失败：快照不存在",
			Suggestion: "请检查快照 ID 或名称是否正确",
			Err:        err,
		}
	}

	// 第四步：从快照中提取 project 信息，推导工具类型
	projectName := strings.TrimSpace(snapshot.Project)
	if projectName == "" {
		return nil, &UserError{
			Message:    "更新快照失败：快照未关联项目",
			Suggestion: "该快照缺少项目信息，无法重新扫描",
		}
	}
	tools := w.inferToolsFromProject(projectName)
	if len(tools) == 0 {
		return nil, &UserError{
			Message:    "更新快照失败：无法推导工具类型",
			Suggestion: "快照关联的项目 \"" + projectName + "\" 无法匹配到已知工具",
		}
	}

	// 第五步：重新扫描文件
	snapshotSvc, ok := w.snapshots.(interface {
		CollectForUpdate(string, []string) ([]models.SnapshotFile, string, error)
	})
	if !ok {
		return nil, &UserError{
			Message:    "更新快照失败：内部服务不可用",
			Suggestion: "请重新启动程序",
		}
	}
	newFiles, _, err := snapshotSvc.CollectForUpdate(projectName, tools)
	if err != nil {
		return nil, &UserError{
			Message:    "更新快照失败：重新扫描配置文件失败",
			Suggestion: "请确认项目 \"" + projectName + "\" 的配置目录存在且有访问权限",
			Err:        err,
		}
	}

	// 第六步：对比新旧文件哈希
	diffSvc, ok := w.snapshots.(interface {
		DiffFileSets([]models.SnapshotFile, []models.SnapshotFile) (int, int, int, int)
	})
	if !ok {
		return nil, &UserError{
			Message:    "更新快照失败：内部服务不可用",
			Suggestion: "请重新启动程序",
		}
	}
	added, updated, removed, unchanged := diffSvc.DiffFileSets(snapshot.Files, newFiles)

	if added == 0 && updated == 0 && removed == 0 {
		return &typesSnapshot.UpdateSnapshotResult{
			SnapshotID:     snapshot.ID,
			SnapshotName:   snapshot.Name,
			FilesUnchanged: unchanged,
			NoChanges:      true,
		}, nil
	}

	// 第七步：更新 SQLite
	snapshot.Files = newFiles
	if strings.TrimSpace(input.Message) != "" {
		snapshot.Message = strings.TrimSpace(input.Message)
	}
	if err := w.snapshots.UpdateSnapshot(snapshot); err != nil {
		return nil, &UserError{
			Message:    "更新快照失败：写入数据库失败",
			Suggestion: "请检查本地数据库是否可访问",
			Err:        err,
		}
	}

	return &typesSnapshot.UpdateSnapshotResult{
		SnapshotID:     snapshot.ID,
		SnapshotName:   snapshot.Name,
		FilesUpdated:   updated,
		FilesAdded:     added,
		FilesRemoved:   removed,
		FilesUnchanged: unchanged,
		NoChanges:      false,
		UpdatedAt:      time.Now(),
	}, nil
}
