# 快照同步功能实施记录

> 创建时间：2026-04-13
> 状态：已完成
> 关联 PR：feat/snapshot-sync (PR #14)、fix/sync-push-empty-commit (PR #16)、fix/git-commit-clean-tree-log (PR #17)
> 关联设计：[同步系统技术架构](../架构设计/同步系统技术架构.md)

---

## 1. 实施概览

基于 [快照同步功能 PRD](./2026-04-11-snapshot-sync-prd.md) 和 [技术设计](./2026-04-12-snapshot-sync-design.md)，分 5 个阶段实施：

| 阶段 | 内容 | 状态 |
|------|------|------|
| 第二阶段 | 快照更新（snapshot update） | ✅ 已完成 |
| 第三阶段 | 远端仓库管理（remote add/list/remove） | ✅ 已完成 |
| 第四阶段 a | 同步推送（sync push） | ✅ 已完成 |
| 第四阶段 b/c | 同步拉取 + 冲突检测（sync pull） | ✅ 已完成 |
| 第五阶段 | 历史版本（history + restore --version） | ✅ 已完成 |

---

## 2. 实施细节

### 2.1 第二阶段：快照更新

**提交**：包含在 feat/snapshot-sync 分支中

**实现方式**：

```
snapshot update <id-or-name> [-m "说明"]
  → UseCase.UpdateSnapshot()
    → 解析快照 ID/名称
    → 推断工具类型和规则
    → 重新扫描文件（Collector）
    → 对比新旧文件哈希
    → 无变化 → 提示"无变更"
    → 有变化 → 更新 SQLite（SnapshotDAO.UpdateWithFiles）
```

**关键文件**：
- `internal/cli/cobra/snapshot_update.go` — CLI 命令
- `internal/app/usecase/update_method.go` — 业务逻辑
- `internal/types/snapshot/update.go` — 类型定义

**输出示例**：
```
快照 "日常配置" 已更新

  新增: 2 个文件
  更新: 3 个文件
  删除: 1 个文件
  未变: 8 个文件
```

### 2.2 第三阶段：远端仓库管理

**提交**：包含在 feat/snapshot-sync 分支中

**实现方式**：

- `remote add <url>` — URL 验证 → 构建配置 → 连通性测试 → 保存数据库
- `remote list` — 列出所有配置，显示状态和同步时间
- `remote remove <name>` — 按名称删除配置

**关键设计决策**：
1. 使用 `RemoteConfigDAOAdapter` 适配器模式桥接 DAO 和接口命名差异
2. 连通性测试使用临时目录浅克隆，不污染工作目录
3. 测试失败仍保存配置（状态标记为 `error`），允许后续重试

**关键文件**：
- `internal/cli/cobra/remote.go` — CLI 命令组
- `internal/app/usecase/remote_method.go` — 业务逻辑
- `internal/types/remote/remote.go` — 类型定义

**输出示例**：
```
远端仓库已添加

  名称:   github-main
  地址:   https://github.com/user/ai-configs.git
  分支:   main
  认证:   Token
```

### 2.3 第四阶段 a：同步推送（sync push）

**提交**：包含在 feat/snapshot-sync 分支中

**实现方式**：

```
sync push --project <name>
  → UseCase.SyncPush()
    → 参数校验 + 获取远端配置
    → 查找 project 最新快照
    → 确保工作目录（clone 或 init）
    → SQLite 读文件 → 写入工作目录
    → Git add + commit + push
```

**关键设计决策**：
1. 工作目录路径：`~/.ai-sync-manager/repos/<project>/`
2. 每个 project 在仓库中按子目录隔离（`<project>/<file-path>`）
3. 空工作树（无变更）commit 失败时，视为成功返回 `FilesPushed: 0`
4. push 失败不影响本地数据

**Bug 修复**：
- PR #16：go-git 返回 `"cannot create empty commit: clean working tree"` 而非 `"nothing to commit"`，导致误报错误
- PR #17：git service 层对空 commit 日志降级为 DEBUG

### 2.4 第四阶段 b/c：同步拉取 + 冲突检测

**提交**：包含在 feat/snapshot-sync 分支中

**实现方式**：

```
sync pull --project <name>
  → UseCase.SyncPull()
    → 获取远端配置 + 确保仓库存在
    → Git fetch（不 merge）
    → 对比 local HEAD 和 remote HEAD
    → 逐文件三方对比（本地快照 vs 本地 Git HEAD vs 远端 HEAD）
    → 有冲突 → 返回冲突列表
    → 无冲突 → Git pull + 更新 SQLite
```

**关键设计决策**：
1. **不做 git merge** — go-git merge 能力有限，使用 fetch + 三方对比更安全
2. 三方对比逻辑：
   - 读取本地快照中的文件内容
   - 读取本地 Git HEAD 中的文件内容
   - 读取远端 commit 中的文件内容
   - 如果本地快照 ≠ 本地 Git HEAD → 本地有修改
   - 如果远端也有修改 → 冲突
3. 冲突不自动合并，返回列表等待用户处理

**冲突信息结构**：
```go
type ConflictInfo struct {
    Path          string  // 文件路径
    ConflictType  string  // 冲突类型（如 "both_modified"）
    LocalSummary  string  // 本地变更摘要
    RemoteSummary string  // 远端变更摘要
}
```

**输出示例（有冲突）**：
```
拉取失败：检测到 2 个文件冲突

  冲突文件：
    settings.json — 本地已修改 (2048 字节), 远端已修改 (3072 字节)
    CLAUDE.md — 本地快照有 512 字节, 远端更新 1024 字节

  请手动解决冲突后重新拉取
```

### 2.5 第五阶段：历史版本

**提交**：包含在 feat/snapshot-sync 分支中

**实现方式**：

#### 查看历史

```
snapshot history <id-or-name>
  → UseCase.SnapshotHistory()
    → 解析快照 → 定位 Git 仓库
    → GitClient.Log() 读取 commit 历史
    → 返回 HistoryResult
```

#### 恢复历史版本

```
snapshot restore <id-or-name> --version <hash>
  → UseCase.RestoreFromHistory()
    → 从指定 commit 读取文件内容
    → 支持 --dry-run 预览
    → 恢复文件到原始路径
```

**关键文件**：
- `internal/cli/cobra/snapshot_history.go` — history 命令
- `internal/cli/cobra/snapshot_restore.go` — restore 新增 --version flag
- `internal/app/usecase/history_method.go` — 业务逻辑
- `internal/types/snapshot/history.go` — 类型定义

**输出示例**：
```
项目: claude-global

共 5 条记录

  [1] a1b2c3d4  2026-04-12 10:30
      Snapshot: 日常备份 v3

  [2] b2c3d4e5  2026-04-11 15:20
      Snapshot: 日常备份 v2

使用 ai-sync snapshot restore <id> --version <hash> 恢复指定版本
```

---

## 3. Git Service 扩展

为支持同步功能，`internal/service/git/client.go` 新增了以下方法：

| 方法 | 用途 | 调用方 |
|------|------|--------|
| `Fetch` | 拉取远端更新但不合并 | SyncPull |
| `Log` | 获取 commit 历史 | SnapshotHistory |
| `GetHeadHash` | 获取当前 HEAD hash | SyncPull |
| `GetRemoteHeadHash` | 获取远端分支 HEAD hash | SyncPull |
| `GetFileContent` | 获取指定 commit 的文件内容 | SyncPull, RestoreFromHistory |
| `GetChangedFiles` | 获取两个 commit 间的差异文件 | SyncPull |

新增类型定义（`internal/service/git/types.go`）：

| 类型 | 用途 |
|------|------|
| `FetchOptions` | fetch 操作参数 |
| `LogOptions` | log 查询参数 |
| `CommitInfo` | commit 信息（hash/message/author/date） |
| `FileDiff` | 文件差异描述（path/status） |

---

## 4. 已知限制与后续优化

| 项目 | 当前状态 | 后续计划 |
|------|---------|---------|
| SyncStatus | 返回"开发中"占位 | 实现状态查询 |
| Pull 冲突自动解决 | 返回冲突列表，不自动合并 | 支持交互式选择（本地/远端/合并） |
| 多远端绑定 | 简化实现，使用第一个活跃配置 | 支持 project 级别绑定远端 |
| Push --all | 仅支持单 project 推送 | 实现批量推送 |
| TUI 同步页面 | 未实现 | 添加同步状态展示页 |

---

## 5. Bug 修复记录

### Bug 1：sync push 空工作树报错（PR #16）

- **现象**：快照内容无变更时，`sync push` 返回 "推送失败：Git commit 失败"
- **根因**：go-git 返回 `"cannot create empty commit: clean working tree"` 而代码只匹配 `"nothing to commit"`
- **修复**：在 `sync_method.go` 增加对 `"empty commit"` 关键字的匹配

### Bug 2：git service 层空 commit 打 ERROR 日志（PR #17）

- **现象**：即使 SyncPush 正确返回成功，日志仍输出 ERROR 级别的 "提交失败"
- **根因**：`client.go` Commit 方法在返回错误前无条件打 `logger.Error`，空工作树错误也被当成真错误
- **修复**：对 `"empty commit"` / `"nothing to commit"` 错误降级为 `logger.Debug`

---

## 6. 关联文档

- [快照同步功能 PRD](./2026-04-11-snapshot-sync-prd.md)
- [快照同步技术设计](./2026-04-12-snapshot-sync-design.md)
- [同步系统技术架构](../架构设计/同步系统技术架构.md)
