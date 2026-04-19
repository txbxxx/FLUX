# sync pull 快照未更新修复 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 修复 sync pull 在 localHash==remoteHash 时未将远端变更写入 SQLite 快照的 bug，防止后续 push 覆盖远端。

**Architecture:** 在 `localHash == remoteHash` 分支中，无论是否有冲突，都将 autoResolved（remote_added）变更写入快照。有冲突时改用精确的逐文件更新替代 `applyRemoteToSnapshot`（该函数会误删 local_only 文件）。"already up-to-date" 提前返回改为继续执行快照对比逻辑。

**Tech Stack:** Go 1.25+, SQLite (gorm), go-git

---

### Task 1: 修复 "already up-to-date" 提前返回

**Files:**
- Modify: `internal/app/usecase/sync_method.go:429-437`

**Step 1: 修改 fetch 的 "already up-to-date" 处理**

当前代码将 "already up-to-date" 当作特殊情况直接返回 `FilesUpdated: 0`，跳过了后续的快照对比逻辑。应改为不返回，让代码继续走到 `localHash == remoteHash` 分支做快照对比。

```go
// 修改前（第 429-437 行）：
if strings.Contains(err.Error(), "already up-to-date") {
    return &typesSync.SyncPullResult{
        Success:      true,
        Project:      projectName,
        FilesUpdated: 0,
    }, nil
}

// 修改后：不提前返回，继续走后续快照对比逻辑
if strings.Contains(err.Error(), "already up-to-date") {
    // fetch 无新内容，但快照可能与远端不一致，继续走后续对比逻辑
    logger.Info("远端无新提交，继续检查快照一致性",
        zap.String("project", projectName))
}
```

**Step 2: 运行测试确认无回归**

Run: `go build ./...`
Expected: 编译通过

**Step 3: Commit**

```bash
git add internal/app/usecase/sync_method.go
git commit -m "fix: sync pull already-up-to-date 时不再跳过快照对比"
```

---

### Task 2: 提取 applyAutoResolvedToSnapshot 辅助函数

**Files:**
- Modify: `internal/app/usecase/sync_method.go`

**Step 1: 提取共享函数**

将第 548-588 行（无冲突分支中处理 autoResolved 的逻辑）提取为独立函数，供无冲突和有冲突两个分支共用。

```go
// applyAutoResolvedToSnapshot 将 autoResolved 中的 remote_added 文件写入快照。
// 返回实际新增的文件数量。
func (w *LocalWorkflow) applyAutoResolvedToSnapshot(
    autoResolved []typesSync.AutoResolvedInfo,
    repoPath string,
    projectPrefix string,
    localSnapshot *models.Snapshot,
    projectBasePath string,
    projectToolType string,
) int {
    autoAddedCount := 0
    if localSnapshot == nil || len(autoResolved) == 0 {
        return 0
    }
    for _, ar := range autoResolved {
        if ar.Resolution != "remote_added" {
            continue
        }
        if !strings.HasPrefix(ar.Path, projectPrefix) {
            logger.Warn("远端文件路径缺少项目前缀", zap.String("path", ar.Path), zap.String("prefix", projectPrefix))
            continue
        }
        relativePath := strings.TrimPrefix(ar.Path, projectPrefix)
        fullPath := pathpkg.Join(repoPath, projectPrefix, relativePath)
        content, readErr := os.ReadFile(fullPath)
        if readErr != nil {
            logger.Warn("读取远端新增文件失败", zap.String("path", fullPath), zap.Error(readErr))
            continue
        }
        w.updateSnapshotFile(localSnapshot, relativePath, content, projectBasePath, projectToolType)
        // 检测符号链接
        if resolved, linkErr := pathpkg.EvalSymlinks(fullPath); linkErr == nil && resolved != fullPath {
            normalizedRel := pathpkg.ToSlash(relativePath)
            for i := range localSnapshot.Files {
                if pathpkg.ToSlash(localSnapshot.Files[i].Path) == normalizedRel {
                    localSnapshot.Files[i].IsSymlink = true
                    localSnapshot.Files[i].LinkTarget = resolved
                    break
                }
            }
        }
        autoAddedCount++
    }
    return autoAddedCount
}
```

**Step 2: 更新无冲突分支使用新函数**

将第 548-588 行替换为：

```go
// 无冲突时，把远端新增文件写入快照
autoAddedCount := w.applyAutoResolvedToSnapshot(autoResolved, repoPath, projectPrefix, localSnapshot, projectBasePath, projectToolType)
if autoAddedCount > 0 {
    if err := w.snapshots.UpdateSnapshot(localSnapshot); err != nil {
        logger.Error("更新快照失败", zap.Error(err))
    } else {
        logger.Info("快照已更新", zap.Int("新增文件数", autoAddedCount))
    }
}
```

注：`projectBasePath, projectToolType` 已在第 552 行之前定义，需提前获取。检查作用域确认变量已声明。

**Step 3: 运行测试确认编译通过**

Run: `go build ./...`
Expected: 编译通过

**Step 4: Commit**

```bash
git add internal/app/usecase/sync_method.go
git commit -m "refactor: 提取 applyAutoResolvedToSnapshot 辅助函数"
```

---

### Task 3: 修复有冲突分支（sub-problem B）

**Files:**
- Modify: `internal/app/usecase/sync_method.go:492-546`

**Step 1: 重写有冲突分支的快照更新逻辑**

当前代码调用 `applyRemoteToSnapshot`（会遍历整个工作目录并删除快照中不存在的 local_only 文件）。改为精确更新：

```go
if len(snapshotConflicts) > 0 {
    // 逐文件交互解决
    var keepLocalFiles, useRemoteFiles []string
    cancelled, resolvedKeepLocal, resolvedUseRemote, err := w.resolveConflicts(ctx, snapshotConflicts, repoPath, remoteHash, localHash, localSnapshot, projectPrefix, autoResolved)
    if err != nil {
        return nil, err
    }
    if cancelled {
        // 用户取消：仍应用 auto-resolved 的 remote_added，避免快照严重脱节
        projectBasePath, projectToolType := w.resolveProjectInfo(projectName)
        autoAdded := w.applyAutoResolvedToSnapshot(autoResolved, repoPath, projectPrefix, localSnapshot, projectBasePath, projectToolType)
        if autoAdded > 0 {
            if saveErr := w.snapshots.UpdateSnapshot(localSnapshot); saveErr != nil {
                logger.Error("更新快照失败（取消后自动同步）", zap.Error(saveErr))
            }
        }
        return &typesSync.SyncPullResult{
            Success:      true,
            Cancelled:    true,
            Project:      projectName,
            FilesUpdated: autoAdded,
            AutoResolved: autoResolved,
        }, nil
    }
    keepLocalFiles = resolvedKeepLocal
    useRemoteFiles = resolvedUseRemote

    // 第一步：应用 auto-resolved（remote_added 文件写入快照）
    projectBasePath, projectToolType := w.resolveProjectInfo(projectName)
    autoAddedCount := w.applyAutoResolvedToSnapshot(autoResolved, repoPath, projectPrefix, localSnapshot, projectBasePath, projectToolType)

    // 第二步：应用用户选择"使用远端"的冲突文件
    for _, relativePath := range useRemoteFiles {
        fullPath := pathpkg.Join(repoPath, projectPrefix, relativePath)
        content, readErr := os.ReadFile(fullPath)
        if readErr != nil {
            logger.Warn("读取远端文件失败", zap.String("file", relativePath), zap.Error(readErr))
            continue
        }
        w.updateSnapshotFile(localSnapshot, relativePath, content, projectBasePath, projectToolType)
    }

    // 第三步：恢复用户选择保留本地的文件到工作目录
    for _, relativePath := range keepLocalFiles {
        for _, f := range localSnapshot.Files {
            if pathpkg.ToSlash(f.Path) == pathpkg.ToSlash(relativePath) {
                targetPath := pathpkg.Join(repoPath, projectPrefix, relativePath)
                dir := pathpkg.Dir(targetPath)
                if err := os.MkdirAll(dir, 0755); err != nil {
                    logger.Warn("创建目录失败", zap.String("dir", dir), zap.Error(err))
                    continue
                }
                if err := os.WriteFile(targetPath, f.Content, 0644); err != nil {
                    logger.Warn("恢复本地文件失败", zap.String("file", relativePath), zap.Error(err))
                }
                break
            }
        }
    }

    // 第四步：持久化快照
    totalUpdated := autoAddedCount + len(useRemoteFiles)
    if totalUpdated > 0 {
        if err := w.snapshots.UpdateSnapshot(localSnapshot); err != nil {
            logger.Error("更新快照失败", zap.Error(err))
        } else {
            logger.Info("快照已更新",
                zap.Int("auto_added", autoAddedCount),
                zap.Int("conflict_use_remote", len(useRemoteFiles)))
        }
    }

    fmt.Println("快照已更新。")
    return &typesSync.SyncPullResult{
        Success:      true,
        Project:      projectName,
        FilesUpdated: totalUpdated,
        AutoResolved: autoResolved,
    }, nil
}
```

关键变更：
- **不再调用 `applyRemoteToSnapshot`**（它会删除 local_only 文件）
- 改为只更新用户选择的远端文件 + auto-resolved 的 remote_added 文件
- local_only 文件保持不动（它们在快照中保留，不出现在远端）
- 用户取消时也应用 auto-resolved

**Step 2: 运行编译确认**

Run: `go build ./...`
Expected: 编译通过

**Step 3: Commit**

```bash
git add internal/app/usecase/sync_method.go
git commit -m "fix: sync pull 冲突分支改用精确更新，保留 local_only 文件"
```

---

### Task 4: 确保无冲突分支变量声明正确

**Files:**
- Modify: `internal/app/usecase/sync_method.go`

**Step 1: 检查并调整变量作用域**

确认 `projectBasePath` 和 `projectToolType` 在无冲突分支使用 `applyAutoResolvedToSnapshot` 时已正确声明。当前代码在第 552 行 `projectBasePath, projectToolType := w.resolveProjectInfo(projectName)` 声明，提取函数后需保留此声明。

**Step 2: 运行全部测试**

Run: `go test ./internal/... -v -count=1 2>&1 | tail -30`
Expected: 所有测试通过（pkg/config 的预存失败不涉及）

**Step 3: Commit**

```bash
git add internal/app/usecase/sync_method.go
git commit -m "fix: 确认无冲突分支变量作用域正确"
```

---

### Task 5: 手动集成验证

**Step 1: 构建**

Run: `make build`
Expected: 生成 `bin/fl.exe`

**Step 2: 按复现场景手动测试**

场景 1（本机 push 后再 pull）：
```bash
./bin/fl.exe sync push -p claude-global
# 在 AI 工具中安装新插件
./bin/fl.exe snapshot create -p claude-global
./bin/fl.exe sync pull -p claude-global
# 预期：FilesUpdated > 0，auto-resolved 的 remote_added 文件已写入快照
./bin/fl.exe sync push -p claude-global --dry-run
# 预期：差异预览为空或仅包含真实差异
```

**Step 3: 最终 Commit**

```bash
git add -A
git commit -m "test: 手动集成验证通过"
```
