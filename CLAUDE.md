# Flux

> **本地优先的 AI 工具配置管理器** — 纯终端形态的 CLI + TUI 应用
>
> **交流语言**：中文

---

## 项目概述

本地优先的 AI 工具配置管理器，聚焦于本地配置的扫描、快照、浏览和编辑。

**当前能力**：扫描本机 AI 工具配置 · 创建/列出/删除/恢复/对比本地快照 · 浏览配置目录与文件 · 终端内直接编辑配置文件 · 自定义扫描规则 · 远端仓库配置（remote add/list/remove）· 同步推送/拉取（sync push/pull）

**暂未覆盖**：图形化凭证管理 · 敏感字段加密存储 · 同步冲突自动合并 · 大量文件收集可能较慢

---

## 快速开始

### 构建

```bash
make build                          # 构建 bin/fl.exe
make build CLI_NAME=my-tool         # 自定义名称
make run                            # 构建并运行
make run ARGS=scan                  # 带参数运行
```

### 测试

```bash
go test ./...                       # 全部测试
go test ./internal/service/snapshot/... -v -cover  # 特定包
go test -coverprofile=coverage.out ./...            # 覆盖率
```

### 直接运行

```bash
go run -buildvcs=false ./cmd/fl <command>
```

### 代码格式化

```bash
make fmt                            # 格式化
make clean                          # 清理
```

---

## 开发工作流

### 分支开发（推荐使用 worktree）

推荐使用 worktree 进行并行开发，避免频繁切换分支：

```bash
# 确保在 master 分支且代码最新
git checkout master && git pull origin master

# 创建 worktree
git worktree add ../AToSync-<prefix>-<desc> <prefix>/<desc>

# 在 worktree 中开发、测试、提交
cd ../AToSync-<prefix>-<desc>
# ... 开发工作 ...
git push -u origin <prefix>/<desc>

# 创建 PR 合并，不直接 merge 到 master
```

**分支前缀**：`feat/` `fix/` `refactor/` `docs/` `test/` `chore/` `perf/` `style/`

### 文档同步检查

每次提交后建议检查以下文档是否需要更新：

| 检查项 | 何时更新 |
|--------|----------|
| CLAUDE.md 项目结构 | 新增/删除文件、架构变更 |
| `docs/使用指南/` | 新命令、参数变化、行为变化 |
| `docs/架构设计/` | 模块新增/重构、数据流变化 |
| `docs/文档索引.md` | 新增或删除任何文档 |

---

## 架构规范

### 分层调用链

```
CLI/TUI 层  →  UseCase 层  →  Service 层  →  DAO（models 内）
   ↓              ↓              ↓              ↓
 仅解析参数     流程编排       单领域逻辑     数据库 CRUD
 仅渲染输出     错误翻译       可调 DAO       只操作一个表
 不含业务逻辑   跨服务协调     不调其他服务    返回明确结构体
```

**推荐做法**：CLI/TUI 层通过 UseCase 层间接调用 Service 和 DAO，以保持清晰的分层结构。

### 数据库设计（最大可移植性原则）

为确保数据库操作具有良好的可移植性，建议遵循以下原则：

1. **关系完整性由应用层维护**：推荐在代码中处理关系逻辑，而非依赖数据库层的 `REFERENCES` 约束
2. **避免使用触发器**：推荐在应用层管理 `updated_at` 等字段，而非依赖 DML/DDL 触发器
3. **业务逻辑位于应用代码**：推荐将所有业务逻辑放在应用层，而非使用存储过程或用户定义函数
4. **使用 ANSI SQL 标准数据类型**：推荐使用 VARCHAR、INTEGER、TIMESTAMP、TEXT、BOOLEAN 等标准类型
5. **索引独立创建**：使用 `CREATE INDEX` 独立语句创建索引，而非依赖隐式索引

### Models 层规范

Models 层负责数据定义和基础 CRUD 操作：

- **结构体 + DAO 同文件**：`snapshot.go` 包含 `Snapshot` 结构体和 `SnapshotDAO`
- **DAO 返回明确结构体**：推荐返回 models 中定义的数据库结构体，而非 `map` 或 `interface{}`
- **单一职责**：每个 DAO 函数建议只操作一个数据库表
- **纯数据定义**：Models 层专注于数据库表结构定义和 CRUD 操作

```go
// internal/models/xxx.go — 标准文件结构
package models

// Example 数据库表结构
type Example struct {
    ID   string `json:"id" db:"id"`
    Name string `json:"name" db:"name"`
}

type ExampleDAO struct { db *database.DB }

func NewExampleDAO(db *database.DB) *ExampleDAO { return &ExampleDAO{db: db} }
func (dao *ExampleDAO) Create(e *Example) error { /* ... */ }
func (dao *ExampleDAO) GetByID(id string) (*Example, error) { /* ... */ }
```

### Types 层规范

对外返回的结构体统一定义在 `internal/types/` 包中，按功能模块组织：

- **按功能分目录**：`types/snapshot/`、`types/scan/`、`types/setting/`、`types/sync/`、`types/remote/`、`types/common/`
- **与 Models 区分**：Models 是数据库表映射，Types 是面向调用方的响应结构体
- **Service/UseCase 负责转换**：从 DAO 获取 Models 结构体后，转换为 Types 结构体返回

```go
// internal/types/snapshot/detail.go
package snapshot

// SnapshotDetail is the response struct for snapshot details returned to callers.
type SnapshotDetail struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    FileCount int       `json:"file_count"`
    CreatedAt time.Time `json:"created_at"`
}
```

### Service 层规范

- 调用 DAO 获取数据，执行业务逻辑，组装结果
- 可组合多个 DAO 的返回值
- **转换职责**：将 Models 结构体转换为 `internal/types/` 中定义的响应结构体返回给调用方

### UseCase 层规范

- 编排多个 Service 完成完整业务流程
- 将底层技术性错误翻译为用户可读错误（`UserError`）
- 决定对外返回的数据结构（定义在 `internal/types/` 中）

---

## 代码规范

### 命名约定

- Go 文件：`snake_case`，测试文件加 `_test.go`
- 包名：小写单词，无下划线
- Receiver：小写缩写（`w *LocalWorkflow`、`dao *SnapshotDAO`），建议避免使用 `this`/`self`

### 注释规范

- **导出类型/函数**：推荐添加 godoc 注释（英文），说明做什么、为什么
- **函数内部**：用 `// 第 X 步：` 标记逻辑块；非显而易见的逻辑用 `// 为什么：` 说明设计决策
- **建议避免**：翻译代码、写创建日期/作者、注释被删除的代码

```go
// Scan scans all registered projects and returns a status summary for each.
//
// 统一项目模型：所有可同步的对象都是"项目"（包括自动注册的全局项目如 claude-global）。
func (w *LocalWorkflow) Scan(ctx context.Context, input ScanInput) (*ScanResult, error) {
    // 第一步：自动推导工具类型
    // 第二步：参数校验
    // 第三步：调用 SnapshotManager

    // 全局项目使用全局规则而不是项目规则。
    // 为什么：~/.claude 的布局是全局配置结构，用项目规则只能扫到极少文件。
    rules = DefaultGlobalRules(toolType)
```

### 错误处理

```go
import "flux/pkg/errors"
return errors.Wrap(err, "创建快照失败")
```

### 日志记录

```go
import "flux/pkg/logger"
logger.Info("操作开始", zap.String("snapshot_id", id), zap.Int("file_count", n))
```

---

## 产品需求流程

### 需求开发流程

```
提出需求 → 写 PRD 文档 → 添加到飞书需求文档（待讨论） → 需求评审和讨论
```

1. **提出需求**：简要描述改进方向，说明问题、目标和用户群体
2. **写 PRD 文档**：需求确认后，在 `docs/plans/` 下编写完整的 PRD 文档
3. **添加到飞书需求文档**：将 PRD 链接添加到飞书多维表格需求文档，状态设为"待讨论"
4. **需求评审和讨论**：与相关方评审 PRD，讨论可行性、优先级和实现方案

### 需求格式（提出时）

```
**改进名称**：
**问题描述**：
**当前状态**：
**改进方案**：
**目标用户**：
**优先级建议**：
```

### PRD 文档格式（docs/plans/）

```markdown
# [功能名称] PRD

## 背景
## 目标
## 非目标（明确不做什么）
## 用户故事
## 功能详情
## 验收标准
## 技术方案（可选）
## 排期估算（可选）
```

---

## 提交规范

### 基本格式

**语言**：Commit Message **使用中文**（仅 type 前缀如 `feat:` `fix:` 保留英文），描述、说明、测试表格等内容建议使用中文。

```
<type>: <中文简要描述>

完成了什么：
有什么作用：
可能的影响：（无则省略）

| 测试场景 | 输入/操作 | 预期结果 | 实际结果 |
|----------|-----------|----------|----------|
| xxx      | xxx       | xxx      | 通过     |
```

### Type 列表

`feat` `fix` `refactor` `docs` `test` `chore` `perf` `style`

### PR 标题规范

创建 PR 时，标题必须包含对应的需求名称，格式：

```
<type>: <中文简要描述> - <需求名称>
```

**示例**：
- `feat: AI 配置支持多模型列表 - AI配置多模型支持与上下文设置`
- `fix: snapshot restore dry-run 输出优化 - 快照恢复dry-run模式输出优化`

### 示例

```
feat: get 命令支持省略 path 参数

完成了什么：
- get 命令参数从 ExactArgs(2) 改为 MinimumNArgs(1)
- accessor 支持空路径时返回根目录

有什么作用：
- 用户执行 "ai-sync get claude" 即可查看根目录

| 测试场景 | 输入/操作 | 预期结果 | 实际结果 |
|----------|-----------|----------|----------|
| 省略 path | ai-sync get claude | 列出根目录 | 通过 |
| 指定 path | ai-sync get claude settings.json | 显示文件内容 | 通过 |
| 编辑模式 | ai-sync get claude settings.json -e | 进入编辑器 | 通过 |
| go test | go test ./... | 全部通过 | 通过 |
```

---

## 技术栈与依赖

| 类别 | 技术 | 版本 | 用途 |
|------|------|------|------|
| 语言 | Go | 1.25+ | — |
| CLI | Cobra | v1.9.1 | 命令行框架 |
| TUI | Bubbletea + Lipgloss | v1.3.10 / v1.1.0 | 终端交互界面 |
| 数据库 | SQLite (modernc.org/sqlite) | v1.47.0 | 本地持久化 |
| Git | go-git | v5.17.0 | 仓库操作 |
| 日志 | Zap | v1.27.1 | 结构化日志 |
| 测试 | Testify | v1.10.0 | 断言 |
| UUID | google/uuid | v1.6.0 | ID 生成 |

---

## 项目结构

```
flux/
├── cmd/fl/                         # CLI 入口
│   ├── main.go                     # 组装依赖、启动 CLI/TUI
│   └── main_test.go
│
├── internal/
│   ├── app/
│   │   ├── runtime/runtime.go      # 运行时：日志、数据库、服务初始化
│   │   └── usecase/
│   │       ├── local_workflow.go   # 本地工作流：快照、扫描、浏览编排
│   │       ├── config_browser.go   # 配置浏览器：目录浏览、文件读写
│   │       ├── ai_setting.go       # AI 配置管理编排
│   │       ├── remote_method.go    # 远端仓库操作编排
│   │       ├── sync_method.go      # 同步操作编排
│   │       └── update_method.go    # 更新操作编排
│   │
│   ├── cli/cobra/                  # Cobra 命令定义
│   │   ├── root.go                 # 根命令 + Workflow 接口
│   │   ├── scan.go                 # scan 命令 + add/remove/list/rules 子命令
│   │   ├── get.go                  # get 命令（浏览/编辑配置）
│   │   ├── snapshot.go             # snapshot 命令组
│   │   ├── snapshot_create.go      # snapshot create
│   │   ├── snapshot_list.go        # snapshot list
│   │   ├── snapshot_delete.go      # snapshot delete
│   │   ├── snapshot_update.go      # snapshot update
│   │   ├── snapshot_restore.go     # snapshot restore
│   │   ├── snapshot_diff.go       # snapshot diff
│   │   ├── setting.go              # setting 命令组
│   │   ├── setting_edit.go         # setting edit
│   │   ├── setting_editor.go       # setting editor (TUI)
│   │   ├── remote.go               # remote 命令组
│   │   ├── sync.go                 # sync 命令组（push/pull/status）
│   │   └── tui.go                  # tui 命令
│   │
│   ├── cli/output/                 # 统一 CLI 输出渲染
│   │   ├── table.go                # 通用表格渲染器（Unicode 边框）
│   │   ├── style.go                # 全局样式常量（颜色、间距）
│   │   ├── diff.go                 # Diff 结果渲染（摘要/Unified/并排）
│   │   ├── format.go               # 格式化输出（--format 支持）
│   │   └── table_test.go           # 表格渲染测试
│   │
│   ├── tui/                        # Bubbletea TUI
│   │   ├── app.go                  # Program 启动
│   │   ├── model.go                # 页面枚举、状态字段
│   │   ├── update.go               # 事件调度中心
│   │   ├── view.go                 # 首页 + 创建页渲染
│   │   ├── page_scan.go            # 扫描结果页
│   │   ├── page_snapshots.go       # 快照列表页
│   │   ├── form_create.go          # 创建快照表单
│   │   ├── editor.go               # 配置编辑器启动器
│   │   ├── editor_model.go         # 编辑器状态模型
│   │   ├── setting_editor.go       # Setting 编辑器 TUI
│   │   └── setting_editor_model.go # Setting 编辑器模型
│   │
│   ├── service/
│   │   ├── tool/                   # 工具检测
│   │   │   ├── detector.go         # 检测器：扫描配置目录
│   │   │   ├── accessor.go         # 访问器：读取文件内容
│   │   │   ├── paths.go            # 默认路径定义
│   │   │   ├── rule_definitions.go # 内置规则定义
│   │   │   ├── rule_manager.go     # 自定义规则 CRUD
│   │   │   ├── rule_resolver.go    # 规则解析（合并默认+自定义+项目）
│   │   │   ├── rule_store.go       # 规则持久化
│   │   │   └── types.go            # 类型定义
│   │   ├── snapshot/               # 快照管理
│   │   │   ├── service.go          # 快照 CRUD、导出、备份、校验
│   │   │   ├── collector.go        # 文件收集（规则匹配→遍历→哈希→分类→去重）
│   │   │   ├── applier.go          # 快照应用（预留）
│   │   │   ├── comparator.go       # 快照对比（预留）
│   │   │   └── types.go            # 类型定义
│   │   ├── sync/                   # 同步服务
│   │   └── setting/                # Setting 管理
│   │
│   └── models/                     # 数据模型 + DAO（同文件，仅表结构+CRUD）
│       ├── snapshot.go             # Snapshot + SnapshotDAO
│       ├── registered_project.go   # RegisteredProject + DAO
│       ├── custom_sync_rule.go     # CustomSyncRule + DAO
│       ├── ai_setting.go           # AISetting + DAO
│       ├── sync_task.go            # SyncTask（预留）
│       ├── remote_config.go        # RemoteConfig（预留）
│       ├── app_config.go           # 应用配置/统计模型
│       └── utils.go                # 工具函数
│
│   ├── util/                       # 内部工具
│   │   └── diffutil/               # Diff 工具
│   │
│   └── types/                      # 对外返回结构体（按功能分子目录）
│       ├── snapshot/               # 快照相关返回结构体
│       ├── scan/                   # 扫描相关返回结构体
│       ├── setting/                # Setting 相关返回结构体
│       ├── sync/                   # 同步相关返回结构体
│       ├── remote/                 # 远端相关返回结构体
│       └── common/                 # 通用/共享返回结构体
│
├── pkg/
│   ├── config/                     # YAML 配置（嵌入默认值 → 用户覆盖）
│   ├── crypto/                     # 加密工具
│   ├── database/db.go              # SQLite 连接、迁移、事务
│   ├── errors/                     # 错误码与包装
│   ├── git/                        # Git 操作（clone/pull/push/fetch/commit）
│   ├── logger/                     # Zap 日志
│   └── utils/                      # 字符串/文件/转换工具
│
├── docs/                           # 文档（使用指南/架构设计/_old 存档）
├── go.mod · go.sum · Makefile · .golangci.yml · README.md
```

---

## CLI 命令速查

| 命令 | 用法 | 关键 flag |
|------|------|-----------|
| scan | `fl scan [app-or-project...]` | — |
| scan add | `fl scan add <app> <path>` | `--project/-p` 注册项目 |
| scan remove | `fl scan remove <app> <path>` | `--project/-p` 删除项目 |
| scan list | `fl scan list [app-or-project]` | `--verbose/-v` 显示详细配置项 |
| scan rules | `fl scan rules [app-or-project]` | — |
| get | `fl get <project> [path]` | `--edit/-e` 编辑模式 `--snapshot/-s` 浏览快照 |
| setting create | `fl setting create` | `--name` 名称 `--token` 令牌 `--api` API地址 `--opus-model` Opus模型 `--sonnet-model` Sonnet模型 |
| setting list | `fl setting list` | `-l` 条数 `-o` 偏移 |
| setting get | `fl setting get <name>` | — |
| setting delete | `fl setting delete <name>` | — |
| setting switch | `fl setting switch <name>` | — |
| setting edit | `fl setting edit <name>` | `-e` 编辑器模式 `-n` 名称 `-t` Token `-a` API地址 `-o` Opus模型 `-s` Sonnet模型 |
| snapshot create | `fl snapshot create` | `-t` 工具 `-m` 说明 `-n` 名称 `-p` 项目 |
| snapshot list | `fl snapshot list` | `-l` 条数 `-o` 偏移 `--format` 格式 |
| snapshot delete | `fl snapshot delete <id-or-name>` | — |
| snapshot update | `fl snapshot update <id-or-name>` | `-m` 说明 |
| snapshot restore | `fl snapshot restore <id-or-name>` | `--files` 选择性恢复 `--dry-run` 预览 `--force` 跳过确认 |
| snapshot diff | `fl snapshot diff <source-id> [<target-id>]` | `-v` 内容差异 `--side-by-side` 并排 `--tool` 工具过滤 `--path` 路径过滤 `--color` 颜色控制 `--format` 格式 |
| remote add | `fl remote add <url>` | `--name` 名称 `--branch` 分支 `--auth` 认证类型 `--token` 令牌 `--ssh-key` SSH密钥 |
| remote list | `fl remote list` | — |
| remote remove | `fl remote remove <name>` | — |
| sync push | `fl sync push` | `-p/--project` 项目 `--all` 全部 |
| sync pull | `fl sync pull` | `-p/--project` 项目 `--all` 全部 |
| sync status | `fl sync status` | `-p/--project` 项目 |
| tui | `fl tui` | — |

---

## 文档索引

- [文档索引](docs/文档索引.md)
- [命令行与终端界面 MVP 使用指南](docs/使用指南/命令行与终端界面MVP使用指南.md)
- [快照功能使用指南](docs/使用指南/快照功能使用指南.md)
- [终端架构详解](docs/架构设计/终端架构详解.md)
- [快照系统技术架构](docs/架构设计/快照系统技术架构.md)
- [历史存档](docs/_old/README.md) — 已废弃的 Wails GUI 时代文档
