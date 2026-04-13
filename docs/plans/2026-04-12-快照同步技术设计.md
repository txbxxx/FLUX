# 快照同步功能技术设计文档

> 创建时间：2026-04-12
> 阶段：第二~五阶段（快照更新 + 远端管理 + 同步操作 + 历史版本）
> 前置依赖：快照恢复功能（第一阶段，已完成）
> 关联 PRD：[快照同步功能 PRD](./2026-04-11-snapshot-sync-prd.md)

---

## 1. 设计背景

快照恢复（第一阶段）已完成，系统具备完整的本地快照能力（创建/列表/删除/恢复/对比）。

本阶段目标：基于 Git 构建透明的版本控制和同步引擎，实现快照更新、远端仓库管理、多设备同步、历史版本管理。

### 当前实现状态

| 模块 | 状态 | 说明 |
|------|------|------|
| `service/git/client.go` | ✅ 已完成 | Clone/Pull/Push/Commit/GetStatus + SSH/Token/Basic Auth |
| `service/sync/sync.go` | ⚠️ 部分实现 | Push 骨架存在，Pull 有 TODO，ValidateRemoteConfig TODO |
| `service/sync/packager.go` | ⚠️ 部分实现 | PackageSnapshot 骨架，manifest 读写 |
| `models/remote_config.go` | ✅ 已完成 | RemoteConfig + AuthConfig + DAO CRUD |
| `models/sync_task.go` | ✅ 已完成 | SyncTask + TaskProgress + DAO |
| `types/sync/sync.go` | ✅ 已完成 | ConflictPolicy/SyncResult/Conflict 等类型 |
| 数据库表 | ✅ 已存在 | remote_configs、sync_tasks、sync_history 表已建 |
| CLI 命令 | ❌ 未实现 | 无 remote / sync / snapshot update / snapshot history |
| 冲突处理 | ❌ 未实现 | 无冲突检测与解决逻辑 |
| GitService 扩展方法 | ❌ 未实现 | 缺 Fetch/Log/Diff/GetChangedFiles/GetFileAtRef |

---

## 2. 设计原则

- **SQLite 是唯一真相源** — 存储最新快照数据；工作目录是 Git 的 workspace，可从 SQLite 重建
- **Git 对用户完全透明** — 用户不直接接触任何 Git 命令
- **本地优先** — 没有 Git 远端仓库也能使用快照更新功能
- **Push 失败安全** — 网络失败不影响本地 SQLite 数据
- **冲突不自动覆盖** — 检测到冲突必须用户确认

---

## 3. 架构总览

### 3.1 双重存储模型

```
┌─────────────────────────────────────────────┐
│              SQLite（真相源）                 │
│  snapshots + snapshot_files                  │
│  存储最新快照的完整文件内容                     │
└───────────────┬─────────────────────────────┘
                ↕ 同步（Push/Pull 时读写）
┌───────────────┴─────────────────────────────┐
│       Git 工作目录（版本引擎）                 │
│  ~/.ai-sync/repos/<project>/                 │
│  文件内容与 SQLite 镜像，Git 负责版本历史       │
└───────────────┬─────────────────────────────┘
                ↕ push/pull
┌───────────────┴─────────────────────────────┐
│       远端仓库（同步通道）                     │
│  GitHub / Gitea / 自建 Git                   │
└─────────────────────────────────────────────┘
```

### 3.2 数据流方向

- **Push**：SQLite → 工作目录（写文件）→ Git commit → Git push → 远端
- **Pull**：远端 → Git fetch → 工作目录（读文件）→ SQLite
- **Update**：磁盘文件 → Collector 收集 → SQLite（UpdateWithFiles）→ Git commit（可选）

### 3.3 多 Project 共享仓库

```
repos/
└── shared-repo/              # 本地 Git 仓库（可对应多个 project）
    ├── claude-global/        # project-a 的文件
    │   ├── settings.json
    │   └── .claude/CLAUDE.md
    ├── codex-global/         # project-b 的文件
    │   └── config.yaml
    └── .ai-sync/
        └── manifest.json     # 快照元数据索引
```

一个本地 Git 仓库可被多个 project 共用，每个 project 只操作自己的子目录。

---

## 4. 分层调用链

### 4.1 新增的调用路径

```
CLI (remote.go / sync.go / snapshot_update.go)
  → UseCase (LocalWorkflow / SyncWorkflow)
    → Service 层
      ├→ SnapshotService   快照 CRUD（已有）
      ├→ GitService        Git 操作（已有 + 扩展）
      ├→ SyncService       同步编排（部分已有 + 完善）
      └→ Packager          文件打包/解析（已有 + 完善）
        → DAO (SnapshotDAO / RemoteConfigDAO / SyncTaskDAO)
```

### 4.2 新增 Workflow 方法

```go
// 快照更新
UpdateSnapshot(ctx context.Context, input UpdateSnapshotInput) (*UpdateSnapshotResult, error)

// 远端管理
AddRemote(ctx context.Context, input AddRemoteInput) (*AddRemoteResult, error)
ListRemotes(ctx context.Context) (*ListRemotesResult, error)
RemoveRemote(ctx context.Context, input RemoveRemoteInput) error

// 同步操作
SyncPush(ctx context.Context, input SyncPushInput) (*SyncPushResult, error)
SyncPull(ctx context.Context, input SyncPullInput) (*SyncPullResult, error)
SyncStatus(ctx context.Context, input SyncStatusInput) (*SyncStatusResult, error)

// 历史版本
SnapshotHistory(ctx context.Context, input SnapshotHistoryInput) (*SnapshotHistoryResult, error)
RestoreVersion(ctx context.Context, input RestoreVersionInput) (*RestoreResult, error)
```

---

## 5. 第二阶段：快照更新（snapshot update）

### 5.1 数据流

```
snapshot update <id-or-name> [-m "说明"]
  │
  ├─ 1. 查找快照 → SnapshotDAO.GetByID() / List()
  ├─ 2. 提取 project + toolType
  ├─ 3. 重新扫描 → Collector.Collect(rules, project)
  ├─ 4. 对比新旧文件哈希
  │     无变化 → 提示"无变更"并退出
  ├─ 5. 更新 SQLite → SnapshotDAO.UpdateWithFiles(snapshot)
  ├─ 6. Git commit（如果 project 有关联的 Git 仓库）
  └─ 7. 返回结果
```

### 5.2 代码变更清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/cli/cobra/snapshot_update.go` | 新增 | CLI 命令：参数解析、输出渲染 |
| `internal/cli/cobra/snapshot.go` | 修改 | 注册 `update` 子命令 |
| `internal/app/usecase/local_workflow.go` | 修改 | 新增 `UpdateSnapshot` 方法和 Input/Result 类型 |
| `internal/types/snapshot/update.go` | 新增 | `UpdateSnapshotInput` + `UpdateSnapshotResult` |
| `internal/service/snapshot/service.go` | 修改 | 新增 `UpdateSnapshot` 方法（重扫 + 对比逻辑） |

**不需要变更：**
- DAO 层 — `UpdateWithFiles` 已有
- Collector — 已有完整的文件收集能力
- GitService — `Commit` 方法已有（可选调用）
- 数据库表结构 — 无需变更

### 5.3 接口定义

```go
// UpdateSnapshotInput 更新快照的输入参数
type UpdateSnapshotInput struct {
    IDOrName string  // 快照 ID 或名称
    Message  string  // 更新说明（可选，默认 "快照更新"）
}

// UpdateSnapshotResult 更新快照的返回结果
type UpdateSnapshotResult struct {
    SnapshotID    string `json:"snapshot_id"`
    SnapshotName  string `json:"snapshot_name"`
    FilesUpdated  int    `json:"files_updated"`
    FilesAdded    int    `json:"files_added"`
    FilesRemoved  int    `json:"files_removed"`
    FilesUnchanged int   `json:"files_unchanged"`
    CommitHash    string `json:"commit_hash,omitempty"`
    NoChanges     bool   `json:"no_changes"`
}
```

### 5.4 核心逻辑

```go
func (w *LocalWorkflow) UpdateSnapshot(ctx context.Context, input UpdateSnapshotInput) (*UpdateSnapshotResult, error) {
    // 第一步：查找快照
    snapshot, err := w.resolveSnapshot(input.IDOrName)

    // 第二步：推断工具类型和规则
    toolType := inferToolType(snapshot.Project)
    rules := w.ruleResolver.Resolve(toolType, snapshot.Project)

    // 第三步：重新扫描文件
    newFiles, err := w.collector.Collect(ctx, rules, snapshot.Project)

    // 第四步：对比哈希
    changes := diffFileSets(snapshot.Files, newFiles)
    if changes.IsEmpty() {
        return &UpdateSnapshotResult{NoChanges: true}, nil
    }

    // 第五步：更新 SQLite
    snapshot.Files = newFiles
    if input.Message != "" {
        snapshot.Message = input.Message
    }
    w.snapshotService.Update(ctx, snapshot)

    // 第六步：Git commit（可选）
    commitHash := ""
    if w.gitService != nil && hasRemoteConfig(snapshot.Project) {
        commitHash, _ = w.gitService.Commit(...)
    }

    return buildUpdateResult(snapshot, changes, commitHash), nil
}
```

### 5.5 "无变更"判断逻辑

```go
// diffFileSets 对比新旧文件集合，返回变更摘要
func diffFileSets(oldFiles, newFiles []SnapshotFile) *FileChanges {
    oldMap := buildPathHashMap(oldFiles)  // map[path]hash
    newMap := buildPathHashMap(newFiles)

    changes := &FileChanges{}
    for path, hash := range newMap {
        if oldHash, exists := oldMap[path]; !exists {
            changes.Added = append(changes.Added, path)
        } else if oldHash != hash {
            changes.Updated = append(changes.Updated, path)
        } else {
            changes.Unchanged++
        }
    }
    for path := range oldMap {
        if _, exists := newMap[path]; !exists {
            changes.Removed = append(changes.Removed, path)
        }
    }
    return changes
}
```

### 5.6 CLI 输出示例

有变更时：

```
快照 "日常配置" 已更新

  新增: 2 个文件
  更新: 3 个文件
  删除: 1 个文件
  未变: 8 个文件

  (Git commit: a1b2c3d)
```

无变更时：

```
快照 "日常配置" 无变更，所有文件内容与当前配置一致。
```

---

## 6. 第三阶段：远端仓库管理（remote）

### 6.1 数据流

```
remote add <url> [--token | --ssh] [--branch main] [--project <name>]
  │
  ├─ 1. 验证 URL 格式
  ├─ 2. 构建 RemoteConfig + AuthConfig
  ├─ 3. 连通性测试 → GitService.Clone(浅克隆)
  │     失败 → 保存为 StatusError，报错退出
  ├─ 4. 保存到数据库 → RemoteConfigDAO.Create()
  ├─ 5. 初始化工作目录 repos/<project>/（如果指定了 project）
  └─ 6. 返回结果

remote list
  │
  └─ RemoteConfigDAO.List() → 表格渲染

remote remove <name>
  │
  ├─ RemoteConfigDAO.Delete(name)
  └─ 提示用户是否清理本地工作目录
```

### 6.2 代码变更清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/cli/cobra/remote.go` | 新增 | `remote` 命令组 + add/list/remove 子命令 |
| `internal/cli/cobra/root.go` | 修改 | 注册 `remote` 命令 |
| `internal/app/usecase/local_workflow.go` | 修改 | 新增 AddRemote/ListRemotes/RemoveRemote 方法 |
| `internal/types/remote/remote.go` | 新增 | AddRemoteInput/AddRemoteResult/ListRemotesResult 等 |
| `internal/service/sync/sync.go` | 修改 | 实现 ValidateRemoteConfig（当前 TODO） |

**不需要变更：**
- RemoteConfigDAO — CRUD 已完整
- GitService — Clone 已有
- 数据库表结构 — 无需变更

### 6.3 接口定义

```go
// AddRemoteInput 添加远端仓库
type AddRemoteInput struct {
    URL      string  // 仓库地址
    Name     string  // 配置名称（可选，默认从 URL 推导）
    Branch   string  // 分支名（默认 main）
    AuthType string  // 认证类型: "" / "ssh" / "token" / "basic"
    Token    string  // Token（AuthType=token 时）
    Username string  // 用户名（AuthType=basic 时）
    Password string  // 密码（AuthType=basic 时）
    SSHKey   string  // SSH 密钥路径（AuthType=ssh 时）
    Project  string  // 绑定项目名（可选）
}

// AddRemoteResult 添加远端仓库结果
type AddRemoteResult struct {
    ConfigID  string `json:"config_id"`
    Name      string `json:"name"`
    URL       string `json:"url"`
    Branch    string `json:"branch"`
    Connected bool   `json:"connected"`
    Project   string `json:"project,omitempty"`
}

// ListRemotesResult 远端列表
type ListRemotesResult struct {
    Remotes []RemoteItem `json:"remotes"`
}

// RemoteItem 远端配置摘要
type RemoteItem struct {
    Name       string     `json:"name"`
    URL        string     `json:"url"`
    Branch     string     `json:"branch"`
    IsDefault  bool       `json:"is_default"`
    Status     string     `json:"status"`
    LastSynced *time.Time `json:"last_synced,omitempty"`
    Projects   []string   `json:"projects"`  // 绑定的 project 列表
}

// RemoveRemoteInput 删除远端配置
type RemoveRemoteInput struct {
    Name string  // 配置名称
    Force bool   // 是否强制删除（即使有 project 绑定）
}
```

### 6.4 ValidateRemoteConfig 实现策略

```go
func (s *Service) ValidateRemoteConfig(ctx context.Context, config *models.RemoteConfig) (*RepositoryTestResult, error) {
    // 使用临时目录做浅克隆测试
    tmpDir, _ := os.MkdirTemp("", "ai-sync-remote-test-")
    defer os.RemoveAll(tmpDir)

    start := time.Now()
    _, err := s.gitClient.Clone(ctx, &CloneOptions{
        URL:   config.URL,
        Path:  tmpDir,
        Auth:  toGitAuthConfig(&config.Auth),
        Branch: config.Branch,
        Depth:  1,  // 浅克隆，快速测试
    })
    latency := time.Since(start)

    if err != nil {
        return &RepositoryTestResult{
            Success: false,
            Error:   err.Error(),
        }, err
    }

    return &RepositoryTestResult{
        Success:  true,
        URL:      config.URL,
        Branch:   config.Branch,
        Latency:  latency,
    }, nil
}
```

### 6.5 CLI 输出示例

添加成功：

```
远端仓库已添加

  名称:   github-main
  地址:   https://github.com/user/ai-configs.git
  分支:   main
  认证:   Token
  绑定:   claude-global

  连通测试: 成功 (延迟 1.2s)
```

列表：

```
已配置的远端仓库

  名称           地址                                       分支    状态      最后同步
  ───────────── ───────────────────────────────────────── ────── ──────── ──────────
  github-main    https://github.com/user/ai-configs.git    main    活跃      2小时前
  gitea-home     https://gitea.local/user/configs.git      main    未验证    -
```

---

## 7. 第四阶段：同步操作（sync push/pull）

### 7.1 总体架构

同步操作是整个 PRD 中最复杂的部分，分三步交付：

- **4a**：Push 实现（SQLite → 工作目录 → Git push）
- **4b**：Pull 无冲突（Git fetch → 解析变更 → 更新 SQLite）
- **4c**：Pull 有冲突（冲突检测 → diff 展示 → 用户决策）

### 7.2 工作目录管理

#### 目录结构

```
~/.ai-sync/
├── data.db
├── backup/
└── repos/                            # Git 工作目录根
    ├── claude-global/                # 独占仓库（单 project 专用）
    │   ├── .git/
    │   ├── settings.json
    │   └── .claude/CLAUDE.md
    └── shared-configs/               # 共享仓库（多 project 共用）
        ├── .git/
        ├── claude-global/
        │   └── settings.json
        └── codex-global/
            └── config.yaml
```

#### 仓库映射关系

project 和 repo 的映射关系存储在 `RemoteConfig` 中。新增字段设计：

- 独占仓库：project 直接映射到 `repos/<project-name>/`
- 共享仓库：多个 project 映射到同一个 `repos/<repo-name>/`，各自用子目录隔离

### 7.3 4a: Push 流程

#### 数据流

```
sync push --project claude-global
  │
  ├─ 1. 查找 project 最新快照 → SnapshotDAO
  ├─ 2. 查找 project 的远端配置 → RemoteConfigDAO
  │     未配置 → 报错："请先执行 ai-sync remote add"
  ├─ 3. 确保工作目录存在
  │     不存在 → GitService.Clone()
  │     已存在 → 跳过
  ├─ 4. 从 SQLite 读文件 → 写入工作目录
  │     Packager.PackageSnapshot(snapshot, repoPath)
  ├─ 5. Git add + commit → GitService.Commit()
  │     commit message 包含 snapshot ID 和元数据
  ├─ 6. Git push → GitService.Push()
  │     失败 → 报错，本地数据不受影响
  ├─ 7. 记录同步历史 → SyncTaskDAO
  └─ 8. 返回 PushResult
```

#### 代码变更清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/cli/cobra/sync.go` | 新增 | `sync` 命令组 + push/pull/status 子命令 |
| `internal/cli/cobra/root.go` | 修改 | 注册 `sync` 命令 |
| `internal/app/usecase/sync_workflow.go` | 新增 | 同步相关 UseCase（避免 local_workflow 过大） |
| `internal/types/sync/operation.go` | 新增 | SyncPushInput/SyncPushResult/SyncPullInput 等 |
| `internal/service/sync/sync.go` | 修改 | 完善 PushSnapshot 实现 |
| `internal/service/sync/packager.go` | 修改 | 完善 PackageSnapshot |
| `internal/models/sync_history.go` | 新增 | SyncHistory DAO（表已存在但无 DAO） |

#### 接口定义

```go
// SyncPushInput 推送输入
type SyncPushInput struct {
    Project string  // 项目名称（必填）
    All     bool    // 推送所有 project
}

// SyncPushResult 推送结果
type SyncPushResult struct {
    Success     bool     `json:"success"`
    Project     string   `json:"project"`
    FilesPushed int      `json:"files_pushed"`
    CommitHash  string   `json:"commit_hash"`
    RemoteURL   string   `json:"remote_url"`
    Error       string   `json:"error,omitempty"`
}
```

### 7.4 4b: Pull 无冲突流程

#### 数据流

```
sync pull --project claude-global
  │
  ├─ 1. 查找远端配置 → RemoteConfigDAO
  ├─ 2. 记录本地 HEAD → beforeHash
  ├─ 3. Git pull → GitService.Pull()
  ├─ 4. 获取新 HEAD → afterHash
  ├─ 5. 获取变更文件列表 → GitService.GetChangedFiles(before, after)
  │     无变更 → 提示"已是最新"
  ├─ 6. 过滤本 project 子目录下的文件
  ├─ 7. 读取变更文件内容 → 写入 SQLite
  │     for each changedFile:
  │       content := os.ReadFile(repoPath + file)
  │       更新对应快照的文件数据
  ├─ 8. 记录同步历史
  └─ 9. 返回 PullResult
```

#### GitService 需要新增的方法

```go
// GetChangedFiles 获取两个 commit 之间的差异文件列表
func (c *GitClient) GetChangedFiles(path, beforeHash, afterHash string) ([]FileDiff, error)

// FileDiff 文件差异描述
type FileDiff struct {
    Path    string     `json:"path"`     // 文件路径
    Status  string     `json:"status"`   // added / modified / deleted
    OldHash string     `json:"old_hash"` // 变更前哈希
    NewHash string     `json:"new_hash"` // 变更后哈希
}

// GetFileContent 获取指定路径的文件当前内容
func (c *GitClient) GetFileContent(path, filePath string) ([]byte, error)

// GetHeadHash 获取当前 HEAD 的 commit hash
func (c *GitClient) GetHeadHash(path string) (string, error)
```

### 7.5 4c: Pull 有冲突流程

#### 冲突检测策略

**核心设计决策：不做 git merge，改用 fetch + 对比**

原因：
1. go-git 的 merge 能力有限，复杂的三方合并不可靠
2. 配置文件冲突通常需要用户明确选择，自动合并风险高
3. fetch + compare 更安全，且符合 "冲突不自动覆盖" 原则

#### 数据流

```
sync pull --project claude-global
  │
  ├─ 1. 查找远端配置 → RemoteConfigDAO
  ├─ 2. Git fetch（不 merge）→ GitService.Fetch()
  ├─ 3. 对比 local HEAD 和 remote HEAD
  │     GitService.GetChangedFiles(local, remote, projectSubDir)
  ├─ 4. 逐文件判断冲突
  │     for each changedFile:
  │       localHash := getLocalFileHash(file)
  │       remoteHash := getRemoteFileHash(file)
  │       lastSyncHash := getLastSyncHash(file)  // 上次同步时的哈希
  │
  │       if localHash == lastSyncHash:
  │         // 本地未修改 → 直接用远端版本（快进）
  │         autoMerge(file)
  │       else if localHash != lastSyncHash && remoteHash != lastSyncHash:
  │         // 双方都修改了 → 冲突
  │         conflicts.add(file)
  │       else:
  │         // 只有一方修改 → 自动合并
  │         autoMerge(file)
  │
  ├─ 5a. 无冲突 → 应用所有变更 → 更新 SQLite
  ├─ 5b. 有冲突 → 展示 diff → 用户决策
  └─ 6. 记录同步历史
```

#### 冲突解决交互

```
$ ai-sync sync pull --project claude-global

检测到 2 个文件有冲突：

  ❌ settings.json — 本地和远端都做了修改
  ❌ .claude/CLAUDE.md — 本地和远端都做了修改

查看详细 diff？[Y/n] y

═══ settings.json ═══
  本地修改 (L15-20):
    "model": "opus-4",
    "temperature": 0.8,

  远端修改 (R15-20):
    "model": "sonnet-4",
    "temperature": 0.5,

═══ .claude/CLAUDE.md ═══
  本地新增 (末尾):
    ## 新增规则
    - 使用中文回复

  远端新增 (末尾):
    ## 测试规则
    - 运行测试后提交

请为每个文件选择处理方式：
  [1] 使用本地版本（保留本地，跳过远端变更）
  [2] 使用远端版本（用远端覆盖本地）
  [3] 查看/编辑合并结果
  [4] 跳过此文件
  [5] 取消本次拉取

settings.json [1-5]: 2
.claude/CLAUDE.md [1-5]: 1

处理结果：
  settings.json    → 使用远端版本 ✓
  .claude/CLAUDE.md → 保留本地版本 ✓

已更新 1 个文件到 SQLite。
```

#### 冲突检测实现

```go
// internal/service/sync/conflict.go

// ConflictDetector 冲突检测器
type ConflictDetector struct {
    syncHistoryDAO *models.SyncHistoryDAO
}

// DetectConflicts 检测冲突文件
func (d *ConflictDetector) DetectConflicts(
    localFiles map[string]string,    // path -> hash
    remoteFiles map[string]string,   // path -> hash
    lastSyncState map[string]string, // path -> hash (上次同步时的状态)
) ([]Conflict, []string) {
    var conflicts []Conflict
    var autoMerged []string

    for path, remoteHash := range remoteFiles {
        localHash, localExists := localFiles[path]
        lastHash, hadLastSync := lastSyncState[path]

        if !localExists {
            // 本地不存在 → 新增文件，无冲突
            autoMerged = append(autoMerged, path)
            continue
        }

        if !hadLastSync {
            // 无上次同步记录 → 首次同步，以远端为准
            autoMerged = append(autoMerged, path)
            continue
        }

        if localHash == lastHash {
            // 本地未修改 → 快进，用远端
            autoMerged = append(autoMerged, path)
            continue
        }

        if localHash == remoteHash {
            // 哈希相同 → 无实际冲突
            autoMerged = append(autoMerged, path)
            continue
        }

        // 双方都修改了 → 冲突
        conflicts = append(conflicts, Conflict{
            Path:        path,
            LocalHash:   localHash,
            RemoteHash:  remoteHash,
            BaseHash:    lastHash,
        })
    }
    return conflicts, autoMerged
}
```

#### 需要新增的文件

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/service/sync/conflict.go` | 新增 | 冲突检测逻辑 |
| `internal/service/git/client.go` | 修改 | 新增 Fetch/GetChangedFiles/GetFileContent/GetHeadHash |
| `internal/service/sync/sync.go` | 修改 | 完善 Pull 逻辑，集成冲突检测 |
| `internal/app/usecase/sync_workflow.go` | 修改 | 完善 SyncPull 方法 |

#### GitService 新增方法签名

```go
// Fetch 拉取远端变更但不合并
func (c *GitClient) Fetch(ctx context.Context, opts *FetchOptions) (*OperationResult, error)

type FetchOptions struct {
    Path   string        // 仓库路径
    Auth   *GitAuthConfig
    Remote string        // 远端名称（默认 origin）
}

// GetChangedFiles 获取两个 commit 之间的差异文件
func (c *GitClient) GetChangedFiles(path, beforeHash, afterHash string) ([]FileDiff, error)

// GetFileContent 获取指定路径文件内容
func (c *GitClient) GetFileContent(path, filePath string) ([]byte, error)

// GetHeadHash 获取 HEAD hash
func (c *GitClient) GetHeadHash(path string) (string, error)
```

### 7.6 同步状态记录

每次 push/pull 完成后，需要记录同步状态用于下次冲突检测：

- 更新 `RemoteConfig.LastSynced` 时间
- 在 `sync_history` 表记录操作详情
- 保存每个文件的同步时哈希（用于三方对比）

```go
// SyncState 同步状态（用于冲突检测的基准）
type SyncState struct {
    Project    string            `json:"project"`
    RemoteName string            `json:"remote_name"`
    CommitHash string            `json:"commit_hash"`
    FileHashes map[string]string `json:"file_hashes"` // path -> hash
    SyncedAt   time.Time         `json:"synced_at"`
}
```

同步状态可以存储在工作目录的 `.ai-sync/` 目录中，作为 JSON 文件：

```
repos/<project>/.ai-sync/
├── manifest.json          # 快照元数据索引
└── sync-state.json        # 上次同步状态（冲突检测基准）
```

---

## 8. 第五阶段：历史版本（snapshot history + restore --history）

### 8.1 数据流

#### 查看历史

```
snapshot history <id-or-name>
  │
  ├─ 1. 查找快照 → SnapshotDAO
  ├─ 2. 获取 Git 仓库路径
  ├─ 3. 读取 Git log → GitService.Log()
  │     过滤与该快照相关的 commit（通过 commit message 中的 snapshot ID）
  ├─ 4. 展示历史列表
  └─ 5. 返回 SnapshotHistoryResult
```

#### 恢复历史版本

```
snapshot restore <id-or-name> --version <hash>
  │
  ├─ 1. 从 Git 获取指定 commit 的文件内容
  │     GitService.GetFileAtRef(repoPath, file, hash)
  ├─ 2. 展示 diff（当前版本 vs 历史版本）
  ├─ 3. 用户确认 → 走标准 restore 流程
  └─ 4. 更新 SQLite
```

### 8.2 代码变更清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/cli/cobra/snapshot_history.go` | 新增 | `snapshot history` 命令 |
| `internal/cli/cobra/snapshot.go` | 修改 | 注册 history 子命令 |
| `internal/cli/cobra/snapshot_restore.go` | 修改 | 新增 `--history` 和 `--version` flag |
| `internal/app/usecase/local_workflow.go` | 修改 | 新增 SnapshotHistory/RestoreVersion 方法 |
| `internal/types/snapshot/history.go` | 新增 | HistoryItem/HistoryResult 类型 |
| `internal/service/git/client.go` | 修改 | 新增 Log/GetFileAtRef 方法 |

### 8.3 接口定义

```go
// SnapshotHistoryInput 查看快照历史输入
type SnapshotHistoryInput struct {
    IDOrName string  // 快照 ID 或名称
    Limit    int     // 最大返回条数（默认 20）
}

// SnapshotHistoryResult 快照历史结果
type SnapshotHistoryResult struct {
    SnapshotID   string         `json:"snapshot_id"`
    SnapshotName string         `json:"snapshot_name"`
    History      []HistoryItem  `json:"history"`
}

// HistoryItem 历史版本条目
type HistoryItem struct {
    CommitHash string    `json:"commit_hash"`
    Message    string    `json:"message"`
    Date       time.Time `json:"date"`
    Author     string    `json:"author"`
    FileCount  int       `json:"file_count"`
}

// RestoreVersionInput 恢复历史版本输入
type RestoreVersionInput struct {
    IDOrName    string  // 快照 ID 或名称
    CommitHash  string  // 要恢复到的 commit hash
    Force       bool    // 跳过确认
    DryRun      bool    // 预览模式
}
```

### 8.4 GitService 新增方法

```go
// Log 获取 commit 历史
func (c *GitClient) Log(ctx context.Context, opts *LogOptions) ([]CommitInfo, error)

type LogOptions struct {
    Path     string        // 仓库路径
    FilePath string        // 文件路径过滤（可选）
    Since    time.Time     // 起始时间（可选）
    Until    time.Time     // 截止时间（可选）
    Limit    int           // 最大返回数量（默认 50）
}

type CommitInfo struct {
    Hash      string    `json:"hash"`
    Message   string    `json:"message"`
    Author    string    `json:"author"`
    Date      time.Time `json:"date"`
}

// GetFileAtRef 获取指定 commit 中的文件内容
func (c *GitClient) GetFileAtRef(ctx context.Context, path, filePath, ref string) ([]byte, error)
```

### 8.5 CLI 输出示例

历史列表：

```
快照 "日常配置" 的版本历史

  #  提交哈希     时间                 说明                     文件数
  ─  ────────── ─────────────────── ──────────────────────── ──────
  1  a1b2c3d     2026-04-12 14:30    更新了 settings.json      5
  2  e4f5g6h     2026-04-10 09:15    初始快照                   12
```

恢复历史版本：

```
$ ai-sync snapshot restore "日常配置" --version e4f5g6h

即将恢复到版本 e4f5g6h (2026-04-10 09:15 "初始快照")

变更预览：
  settings.json        — 12 行差异
  .claude/CLAUDE.md    — 5 行差异
  plugins/config.json  — 新增

备份已创建: ~/.ai-sync/backup/20260412-150000/

确认恢复？[y/N]
```

---

## 9. 错误处理

| 场景 | 处理方式 | UserError Message |
|------|---------|-------------------|
| 快照不存在 | 报错退出 | "快照 'xxx' 不存在。使用 ai-sync snapshot list 查看所有快照" |
| 未配置远端仓库 | 提示引导 | "请先执行 ai-sync remote add <url> 添加远端仓库" |
| 远端不可达 | 报错展示原因 | "无法连接远端仓库：{具体原因}。请检查网络和仓库地址" |
| Push 失败 | 报错，本地不变 | "推送失败：{原因}。本地数据未受影响" |
| Pull 有冲突 | 展示 diff 等待决策 | "检测到 N 个文件冲突，请选择处理方式" |
| 工作目录损坏 | 提示重建 | "工作目录异常，执行 ai-sync sync push 重建" |
| 快照更新无变更 | 友好提示 | "快照 'xxx' 无变更，所有文件内容一致" |
| Git 未安装 | 首次使用时检测 | "同步功能需要 Git。请安装 Git 后重试" |
| 认证失败 | 提示检查凭证 | "远端认证失败，请检查 Token/SSH 密钥配置" |

---

## 10. 测试策略

### 10.1 单元测试

| 模块 | 测试重点 |
|------|---------|
| `diffFileSets` | 新增/更新/删除/无变更场景 |
| `ConflictDetector` | 双方修改/单方修改/新增文件/首次同步 |
| `Packager` | 文件写入/manifest 生成/路径安全检查 |
| `GitService` 新方法 | Fetch/Diff/Log 的 mock 测试 |

### 10.2 集成测试

| 场景 | 测试方法 |
|------|---------|
| 完整 Push 流程 | 用本地 Git 仓库模拟远端 |
| 完整 Pull 无冲突 | 两步 push+pull |
| 冲突检测 | 两端修改后 pull |
| 快照更新 + Git commit | update 后检查 git log |

### 10.3 手动验证

使用两个本地目录模拟多设备：
1. 目录 A 做 push
2. 目录 B 做 pull
3. 两端都修改后 pull 验证冲突检测

---

## 11. 文件变更总览

### 新增文件（9 个）

| 文件路径 | 说明 |
|---------|------|
| `internal/cli/cobra/snapshot_update.go` | snapshot update 命令 |
| `internal/cli/cobra/snapshot_history.go` | snapshot history 命令 |
| `internal/cli/cobra/remote.go` | remote 命令组 |
| `internal/cli/cobra/sync.go` | sync 命令组 |
| `internal/types/snapshot/update.go` | 更新快照类型 |
| `internal/types/snapshot/history.go` | 历史版本类型 |
| `internal/types/remote/remote.go` | 远端管理类型 |
| `internal/app/usecase/sync_workflow.go` | 同步 UseCase |
| `internal/service/sync/conflict.go` | 冲突检测器 |

### 修改文件

| 文件路径 | 修改内容 |
|---------|---------|
| `internal/cli/cobra/root.go` | 注册 remote、sync 命令 |
| `internal/cli/cobra/snapshot.go` | 注册 update、history 子命令 |
| `internal/cli/cobra/snapshot_restore.go` | 新增 --history、--version flag |
| `internal/app/usecase/local_workflow.go` | 新增 Workflow 接口方法 |
| `internal/service/git/client.go` | 新增 Fetch/Log/Diff/GetFileAtRef 等方法 |
| `internal/service/sync/sync.go` | 完善 Pull、ValidateRemoteConfig |
| `internal/service/sync/packager.go` | 完善 PackageSnapshot |
| `internal/service/snapshot/service.go` | 新增 UpdateSnapshot 方法 |
| `internal/app/runtime/runtime.go` | 初始化 GitService/SyncService 并注入 |

### 不需要变更

- 数据库表结构 — 所有必需表已存在
- DAO 层 — 所有 CRUD 方法已实现
- Collector — 文件收集能力完整
- TUI — 本阶段不做 TUI 页面
