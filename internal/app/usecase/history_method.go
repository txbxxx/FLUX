package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"ai-sync-manager/internal/service/git"
	typesSnapshot "ai-sync-manager/internal/types/snapshot"
)

// SnapshotHistory shows the version history of a snapshot from the git repository.
//
// 流程：
//  1. 解析快照 ID/名称 → 获取项目名
//  2. 定位项目的 git 仓库
//  3. 读取 git log → 返回历史记录
func (w *LocalWorkflow) SnapshotHistory(ctx context.Context, input SnapshotHistoryInput) (*typesSnapshot.HistoryResult, error) {
	// 第一步：参数校验
	if strings.TrimSpace(input.IDOrName) == "" {
		return nil, &UserError{
			Message:    "查看历史失败：请指定快照 ID 或名称",
			Suggestion: "使用 snapshot list 查看快照列表",
		}
	}

	// 第二步：解析快照 → 获取项目名
	id, err := w.resolveSnapshotID(input.IDOrName)
	if err != nil {
		return nil, err
	}

	snapshot, err := w.snapshots.GetSnapshot(id)
	if err != nil {
		return nil, &UserError{
			Message:    "查看历史失败：快照不存在",
			Suggestion: "请检查快照 ID 或名称",
			Err:        err,
		}
	}

	projectName := snapshot.Project
	if projectName == "" {
		return nil, &UserError{
			Message:    "查看历史失败：快照没有关联的项目",
			Suggestion: "请检查快照数据是否完整",
		}
	}

	// 第三步：定位 git 仓库
	repoPath := filepath.Join(w.dataDir, "repos", projectName)
	if !git.IsRepository(repoPath) {
		return nil, &UserError{
			Message:    "查看历史失败：项目仓库不存在",
			Suggestion: "请先执行 ai-sync sync push --project " + projectName + " 创建仓库",
		}
	}

	// 第四步：读取 git log
	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}

	gitClient := git.NewGitClient()
	commits, err := gitClient.Log(ctx, &git.LogOptions{
		Path:  repoPath,
		Limit: limit,
	})
	if err != nil {
		return nil, &UserError{
			Message:    "查看历史失败：无法读取仓库日志",
			Suggestion: "请检查仓库状态",
			Err:        err,
		}
	}

	// 第五步：转换为历史记录
	entries := make([]typesSnapshot.HistoryEntry, 0, len(commits))
	for _, commit := range commits {
		// 过滤只保留包含项目信息的 commit
		if !strings.Contains(commit.Message, "Project: "+projectName) && !strings.Contains(commit.Message, "Snapshot:") {
			// 也包含非 snapshot commit（如手动 commit）
		}
		entries = append(entries, typesSnapshot.HistoryEntry{
			CommitHash: commit.Hash,
			Message:    strings.TrimSpace(commit.Message),
			Author:     commit.Author,
			Date:       commit.Date,
		})
	}

	return &typesSnapshot.HistoryResult{
		Project: projectName,
		Entries: entries,
		Total:   len(entries),
	}, nil
}

// RestoreFromHistory restores a snapshot from a specific git commit version.
//
// 流程：
//  1. 解析快照 → 获取项目名
//  2. 从 git commit 读取文件内容
//  3. 构造临时快照 → 调用恢复流程
func (w *LocalWorkflow) RestoreFromHistory(ctx context.Context, input RestoreFromHistoryInput) (*typesSnapshot.RestoreResult, error) {
	// 第一步：参数校验
	if strings.TrimSpace(input.IDOrName) == "" {
		return nil, &UserError{
			Message:    "历史恢复失败：请指定快照 ID 或名称",
			Suggestion: "使用 snapshot list 查看快照列表",
		}
	}
	if strings.TrimSpace(input.CommitHash) == "" {
		return nil, &UserError{
			Message:    "历史恢复失败：请指定版本哈希",
			Suggestion: "使用 snapshot history 查看版本列表，选择一个版本哈希",
		}
	}

	// 第二步：解析快照
	id, err := w.resolveSnapshotID(input.IDOrName)
	if err != nil {
		return nil, err
	}

	snapshot, err := w.snapshots.GetSnapshot(id)
	if err != nil {
		return nil, &UserError{
			Message:    "历史恢复失败：快照不存在",
			Suggestion: "请检查快照 ID 或名称",
			Err:        err,
		}
	}

	projectName := snapshot.Project
	repoPath := filepath.Join(w.dataDir, "repos", projectName)
	if !git.IsRepository(repoPath) {
		return nil, &UserError{
			Message:    "历史恢复失败：项目仓库不存在",
			Suggestion: "请先执行 ai-sync sync push --project " + projectName,
		}
	}

	// 第三步：从 git commit 读取历史版本的文件
	gitClient := git.NewGitClient()
	projectPrefix := projectName + "/"

	// 确定要恢复的文件列表
	targetFiles := input.Files
	if len(targetFiles) == 0 {
		targetFiles = make([]string, 0, len(snapshot.Files))
		for _, f := range snapshot.Files {
			targetFiles = append(targetFiles, f.Path)
		}
	}

	// 从历史 commit 读取每个文件的内容
	type fileRestore struct {
		Path     string
		Content  []byte
		Original string // 原始磁盘路径
	}
	var restoreFiles []fileRestore

	for _, relPath := range targetFiles {
		gitPath := projectPrefix + relPath
		content, readErr := gitClient.GetFileContent(ctx, repoPath, gitPath, input.CommitHash)
		if readErr != nil {
			continue // 文件可能在该版本中不存在
		}

		// 查找原始磁盘路径
		originalPath := ""
		for _, f := range snapshot.Files {
			if f.Path == relPath {
				originalPath = f.OriginalPath
				break
			}
		}
		if originalPath == "" {
			continue
		}

		restoreFiles = append(restoreFiles, fileRestore{
			Path:     relPath,
			Content:  content,
			Original: originalPath,
		})
	}

	if len(restoreFiles) == 0 {
		return nil, &UserError{
			Message:    "历史恢复失败：指定版本中没有找到可恢复的文件",
			Suggestion: "请检查版本哈希和文件路径",
		}
	}

	// 第四步：dry-run 模式只返回预览
	if input.DryRun {
		result := &typesSnapshot.RestoreResult{
			SnapshotID:   id,
			SnapshotName: snapshot.Name,
			AppliedCount: len(restoreFiles),
		}
		for _, rf := range restoreFiles {
			result.AppliedFiles = append(result.AppliedFiles, typesSnapshot.AppliedFile{
				Path:   rf.Path,
				Action: "restored",
			})
		}
		return result, nil
	}

	// 第五步：写入文件到磁盘
	result := &typesSnapshot.RestoreResult{
		SnapshotID:   id,
		SnapshotName: snapshot.Name,
	}

	for _, rf := range restoreFiles {
		if writeErr := writeFile(rf.Original, rf.Content); writeErr != nil {
			result.ErrorCount++
			result.Errors = append(result.Errors, typesSnapshot.ApplyError{
				Path:    rf.Path,
				Message: fmt.Sprintf("写入失败: %v", writeErr),
			})
			continue
		}
		result.AppliedCount++
		result.AppliedFiles = append(result.AppliedFiles, typesSnapshot.AppliedFile{
			Path:   rf.Path,
			Action: "restored",
		})
	}

	return result, nil
}
