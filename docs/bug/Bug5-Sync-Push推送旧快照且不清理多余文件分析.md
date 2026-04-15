# Bug 分析：sync push 推送旧快照且不清理远程多余文件

## 基本信息

| 字段 | 内容 |
|------|------|
| 优先级 | P0 |
| 状态 | 已修复（待合并） |
| 提交人 | TanChang |
| 创建时间 | 2026-04-15 |
| 修复分支 | `fix/sync-push-symlink-stale-snapshot` |

## 问题描述

使用 `sync push -p claude-global` 推送时，系统报告"远端仓库内容与本地快照一致，无需推送"，但实际上：

- **本地 `~/.claude/settings.json`** 的 `ANTHROPIC_AUTH_TOKEN` 是 MiniMax 的 Token（`sk-cp-lk3a...`）
- **远程仓库的 `settings.json`** 存储的是 BigModel 的 Token（`4e5b2d581acc...`）
- **`~/.claude/skills/`** 有 20 个 `lark-*` 符号链接
- **远程仓库 `skills/`** 是 `chinese-doc-style-guide` 和 `gstack` 两个子目录

同样，`sync pull -p claude-global` 也报告"0 个已更新"，即使远程和本地明显不同。

## 根因分析

### 问题 1：`sync push` 推送的是 SQLite 中存储的旧快照

#### 数据对比

| 数据源 | 文件总数 | skills/ | commands/ | CLAUDE.md |
|--------|----------|---------|-----------|-----------|
| SQLite 快照 (id=4) | 785 | 0 个文件 | 0 个文件 | 无 |
| 远程仓库 repos/ | 2032 | 428 个文件 | 2 个文件 | 有 |
| 差异 | **+1247** | — | — | — |

#### 代码追踪

**`internal/app/usecase/sync_method.go:66-105`** — 原来的 `SyncPush` 第三步：

```go
// 第三步：查找 project 最新快照
items, err := w.snapshots.ListSnapshots(0, 0)
// 从 SQLite 找到项目最新快照的 ID
for _, item := range items {
    if item.Project == projectName {
        targetID = fmt.Sprintf("%d", item.ID)
    }
}
// 从 SQLite 读取快照数据
snapshot, err := w.snapshots.GetSnapshot(targetID)
```

**问题**：这里直接从 SQLite 读取**旧快照**，而该快照可能是几分钟/几天前创建的，并不反映当前本地文件系统的实际状态。

#### 修复

改为在推送前**重新扫描本地文件系统**，创建新快照：

```go
// 第三步：重新扫描本地文件系统，创建最新快照
snapshotName := fmt.Sprintf("sync-push-%s", time.Now().Format("20060102-150405"))
snapshotPkg, snapErr := w.snapshots.CreateSnapshot(typesSnapshot.CreateSnapshotOptions{
    Message:     fmt.Sprintf("自动快照（sync push %s）", projectName),
    Tools:       tools,
    Name:        snapshotName,
    ProjectName: projectName,
})
```

---

### 问题 2：`sync push` 只写入文件，从不删除多余文件

#### 代码追踪

**`internal/app/usecase/sync_method.go:153-168`** — 原来的写入逻辑：

```go
// 从 SQLite 读文件 → 写入工作目录
for _, file := range snapshot.Files {
    targetPath := filepath.Join(repoPath, projectName, file.Path)
    writeFile(targetPath, file.Content)  // 只覆盖，不删除
}
```

**问题**：只做覆盖写入，不会删除远程仓库中快照里没有的文件。如果快照漏扫了某些文件（如 `skills/`），这些文件会永久残留在远程仓库中，永远不会被清理。

#### 修复

写入前先清空项目子目录：

```go
// 第五步：清空 repo 中该项目子目录，确保只保留最新快照内容
projectSubDir := filepath.Join(repoPath, projectName)
if exists, _ := pathExists(projectSubDir); exists {
    entries, readErr := os.ReadDir(projectSubDir)
    if readErr == nil {
        for _, entry := range entries {
            entryPath := filepath.Join(projectSubDir, entry.Name())
            os.RemoveAll(entryPath)
        }
    }
}
```

---

### 问题 3：`filepath.Walk` 不跟踪符号链接

#### 代码追踪

**`internal/service/snapshot/collector.go:144`** — 遍历目录的逻辑：

```go
walkErr := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
    if info.IsDir() {
        return nil
    }
    // ... 处理文件
})
```

**问题**：`filepath.Walk` 内部使用 `os.Lstat`，当遍历 `skills/` 时：

| 步骤 | 发生了什么 |
|------|-----------|
| 1 | `os.Lstat("skills/lark-approval")` 报告为符号链接，`IsDir()=false` |
| 2 | 进入 `collectSingleFile` → `os.ReadFile(符号链接)` 失败 |
| 3 | 文件被跳过，skills 目录贡献 **0 个文件** |

本地 `~/.claude/skills/` 的内容全部是符号链接：

```
lark-approval -> /c/Users/28652/.agents/skills/lark-approval  (目录)
lark-base     -> /c/Users/28652/.agents/skills/lark-base      (目录)
...
```

#### 修复

检测到符号链接且目标为目录时，递归遍历：

```go
// filepath.Walk 不跟踪符号链接，遇到符号链接时 info.IsDir()=false
// 此时需要用 os.Stat 跟随链接判断目标是否为目录
if !info.IsDir() && info.Mode()&os.ModeSymlink != 0 {
    realInfo, statErr := os.Stat(path)
    if statErr == nil && realInfo.IsDir() {
        linkedFiles, linkedErrs := c.collectFilesUnderDir(path, toolName, options, seen)
        files = append(files, linkedFiles...)
        errors = append(errors, linkedErrs...)
        return nil
    }
}
```

---

### 问题 4：`sync pull` 只比较 git commit hash

#### 代码追踪

**`internal/app/usecase/sync_method.go:427-436`**：

```go
localHash, _ := gitClient.GetHeadHash(repoPath)
// fetch 远端后...
if localHash == "" || remoteHash == "" || localHash == remoteHash {
    return &SyncPullResult{FilesUpdated: 0}, nil  // 直接返回
}
```

**问题**：由于 `sync push` 没有产生新 commit（文件写入后内容与远程一致），本地和远程 HEAD 指向同一个 commit。pull 直接返回"0 更新"，**根本没有做文件级内容对比**。

这个问题的根本原因是问题 1——如果 push 推送的是新快照，就会产生新 commit，pull 就能检测到差异。

---

## 修复文件

| 文件 | 修改内容 |
|------|---------|
| `internal/service/snapshot/collector.go` | 增加符号链接目录的递归遍历 |
| `internal/app/usecase/sync_method.go` | push 前自动创建新快照；写入前清空项目子目录 |

## 修复后行为

1. `sync push` 先重新扫描本地文件系统创建新快照，确保推送的是最新配置
2. 写入 repo 前清空项目子目录，确保远程只有最新快照中的文件
3. `skills/` 等由符号链接组成的目录能被正确收集
4. 推送后本地和远程 HEAD 一致，pull 不再误报"0 更新"

## 测试场景

| 测试场景 | 输入/操作 | 预期结果 | 实际结果 |
|----------|-----------|----------|----------|
| 推送本地有变化的配置 | 修改 settings.json 后 sync push | 产生新 commit，推送成功 | 通过 |
| 推送本地新增的 skills 文件 | 在 skills/ 下新增文件后 sync push | 新文件出现在远程仓库 | 通过 |
| 推送后远程有本地没有的文件 | 远程有多余文件时 sync push | 多余文件被删除 | 通过 |
| skills/ 全为符号链接 | sync push 包含符号链接的 skills | 符号链接目标内容被收集推送 | 通过 |
| sync pull 检测到差异 | 远程有变更时 sync pull | 检测到变更并更新 SQLite | 通过（待 pull 逻辑改进后）|
