# Bug 分析：sync push 推送旧快照且不清理远程多余文件

## 基本信息

| 字段 | 内容 |
|------|------|
| 优先级 | P0 |
| 状态 | 已合并到 master |
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

改为**从 SQLite 读取现有快照**，并在写入 repo 后用 `git diff` 展示差异预览：

```go
// 第四步：fetch 远端最新代码
fetchResult, fetchErr := gitClient.Fetch(ctx, &git.FetchOptions{...})

// 第五步：从 SQLite 读取该项目的最新快照
items, _ := w.snapshots.ListSnapshots(0, 0)
for _, item := range items {
    if item.Project == projectName {
        targetID = fmt.Sprintf("%d", item.ID)
    }
}
snapshot, _ := w.snapshots.GetSnapshot(targetID)

// 第六步：把快照文件写入 repo 工作目录
// 第七步：git diff 展示快照 vs 远端 HEAD 的差异预览
diffCmd := exec.Command("git", "diff", "--name-status", "origin/"+branch)
diffOutput, _ := diffCmd.Output()
// 解析 A/M/D 分类统计后...

// 第八步：用户确认后才 commit + push
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

### 问题 5：`sync pull` 本地仓库不存在时报错，强制要求先 push

#### 代码追踪

**`internal/app/usecase/sync_method.go:453-458`**：

```go
if !git.IsRepository(repoPath) {
    return nil, &UserError{
        Message:    "拉取失败：本地仓库不存在",
        Suggestion: "请先执行 fl sync push --project " + projectName + " 初始化仓库",
    }
}
```

**问题**：Git 标准流程中，pull 可以在任何时候用，不需要先 push。强制要求先 push 不符合直觉。

#### 修复

改为和 `sync push` 一样的交互：询问用户是否从远程 clone。

---

## 修复文件

| 文件 | 修改内容 |
|------|---------|
| `internal/service/snapshot/collector.go` | 增加符号链接目录的递归遍历；循环引用防护（最大深度 20 + 真实路径去重） |
| `internal/app/usecase/sync_method.go` | 重新设计 SyncPush：fetch 远端→写入 SQLite 快照→git diff 预览→用户确认→commit+push；写入前清空项目子目录；push/pull 仓库不存在时均询问是否 clone；ensureRepoExists 公共方法抽取；fetch 失败时 diff 显示警告 |
| `pkg/git/client.go` | 新增 `Diff` 方法，底层用 go-git `worktree.Status()` 对比工作树与 HEAD；新增 `DiffOptions`、`DiffResult` 类型 |
| `pkg/git/types.go` | 新增 `DiffOptions`、`DiffResult` 类型定义 |

## 额外修复（Code Review 后续）

| 问题 | 修复 |
|------|------|
| P0: `exec.Command("git", ...)` 绕过 pkg/git 抽象 | 已替换为 `gitClient.Diff()`，项目不再依赖系统 git CLI |
| P0: `Diff` 方法 StatusCode 比较错误 | 字符串比较 ("Modified"/"Added"/"Deleted") 改为 `git.Unmodified`/`git.Deleted`/`git.Added` 等常量 |
| P1: mkdirAll/writeFile/RemoveAll 无错误处理 | 增加 `logger.Warn` 跳过失败文件，filesWritten 只在成功时递增 |
| P1: fetch 失败后 diff 无提示 | diff 前显示 `[警告] fetch 失败，差异可能不完整` |
| P1: ensureRepoExists 用 context.Background() | 传入调用方 ctx，clone 可被取消 |
| P1: formatSize 死代码 | 已删除 |
| P2: clone 确认逻辑重复 | 抽取为 `ensureRepoExists` 公共方法，Push/Pull 共用 |
| P2: SyncPull godoc 缺失 | 补回 6 步流程注释 |

## 修复后行为

1. `sync push` 从 SQLite 读取现有快照写入 repo，展示 `git diff` 差异预览，让用户在推送前感知本地与远程的差别
2. 写入 repo 前清空项目子目录，确保远程只有快照中的文件
3. `skills/` 等由符号链接组成的目录能被正确收集
4. 推送后本地和远程 HEAD 一致，pull 不再误报"0 更新"
5. **sync push 交互**：本地仓库不存在时询问是否先拉取远程配置；写入快照后显示差异预览（A/M/D 分类统计）；覆盖远端前需要用户确认
6. **sync pull 交互**：本地仓库不存在时询问是否 clone，用户确认后拉取远端到本地

### 交互流程示例

```
$ fl sync push -p claude-global
本地仓库不存在（C:\Users\xxx\.ai-sync-manager\repos\claude-global）
是否从远程拉取现有配置？[Y/n]: Y
远端配置拉取成功

快照「snapshot-20260415-143022」vs 远端 origin/main 差异预览：
------------------------------------------------------------
  [修改] claude/settings.json
  [新增] claude/skills/lark-approval/SKILL.md
  [新增] claude/skills/lark-base/SKILL.md
  ... 等 18 个新增文件
------------------------------------------------------------

快照「snapshot-20260415-143022」：新增 18，修改 1，删除 0
警告：以快照内容覆盖远端仓库，确认推送？[Y/n]: Y
```

用户输入 `n` 则取消推送，返回"已取消推送"。

### sync pull 交互流程示例

```
$ fl sync pull -p claude-global
本地仓库不存在（C:\Users\xxx\.ai-sync-manager\repos\claude-global）
是否从远程拉取现有配置到本地？[Y/n]: Y
远端配置已拉取到本地（C:\Users\xxx\.ai-sync-manager\repos\claude-global）
...
```

用户输入 `n` 则取消，返回成功。

## 测试场景

| 测试场景 | 输入/操作 | 预期结果 | 实际结果 |
|----------|-----------|----------|----------|
| 推送本地有变化的配置 | 修改 settings.json 后 sync push | 产生新 commit，推送成功 | 通过 |
| 推送本地新增的 skills 文件 | 在 skills/ 下新增文件后 sync push | 新文件出现在远程仓库 | 通过 |
| 推送后远程有本地没有的文件 | 远程有多余文件时 sync push | 多余文件被删除 | 通过 |
| skills/ 全为符号链接 | sync push 包含符号链接的 skills | 符号链接目标内容被收集推送 | 通过 |
| 本地仓库不存在时 push | sync push 新项目 | 询问是否拉取远程，用户可选择 | 通过 |
| 推送时用户取消确认 | sync push 输入 n | 返回"已取消推送"，无任何变更 | 通过 |
| 本地仓库不存在时 pull | sync pull 新项目 | 询问是否从远程 clone，用户可选择 | 通过 |
| pull 时用户取消确认 | sync pull 输入 n | 返回成功，无任何变更 | 通过 |
