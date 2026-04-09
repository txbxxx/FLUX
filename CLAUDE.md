# AI Sync Manager

> **本地优先的 AI 工具配置管理器** — 纯终端形态的 CLI + TUI 应用
>
> **交流语言**：中文

---

## 项目概述

本地优先的 AI 工具配置管理器，聚焦于本地配置的扫描、快照、浏览和编辑。

**当前能力**：扫描本机 AI 工具配置 · 创建/列出本地快照 · 浏览配置目录与文件 · 终端内直接编辑配置文件 · 自定义扫描规则

**暂未覆盖**：远端仓库同步（push/pull/diff/apply/rollback）· 图形化凭证管理 · 快照应用（回滚）· 快照对比 · 大量文件收集可能较慢

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
ai-sync-manager/
├── cmd/ai-sync/                    # CLI 入口
│   ├── main.go                     # 组装依赖、启动 CLI/TUI
│   └── main_test.go
│
├── internal/
│   ├── app/
│   │   ├── runtime/runtime.go      # 运行时：日志、数据库、服务初始化
│   │   └── usecase/
│   │       ├── local_workflow.go   # 本地工作流：快照、扫描、浏览编排
│   │       └── config_browser.go   # 配置浏览器：目录浏览、文件读写
│   │
│   ├── cli/cobra/                  # Cobra 命令定义
│   │   ├── root.go                 # 根命令 + Workflow 接口
│   │   ├── scan.go                 # scan 命令 + add/remove/list/rules 子命令
│   │   ├── get.go                  # get 命令（浏览/编辑配置）
│   │   ├── snapshot.go             # snapshot 命令组
│   │   ├── snapshot_create.go      # snapshot create
│   │   ├── snapshot_list.go        # snapshot list
│   │   ├── snapshot_delete.go      # snapshot delete
│   │   ├── setting.go              # setting 命令组
│   │   └── tui.go                  # tui 命令
│   │
│   ├── cli/output/                 # 统一 CLI 输出渲染
│   │   ├── table.go                # 通用表格渲染器（Unicode 边框）
│   │   ├── style.go                # 全局样式常量（颜色、间距）
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
│   │   └── editor_model.go         # 编辑器状态模型
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
│   │   ├── git/                    # Git 操作（预留）
│   │   ├── sync/                   # 同步服务（预留）
│   │   └── crypto/                 # AES-256-GCM 加密
│   │
│   └── models/                     # 数据模型 + DAO（同文件，仅表结构+CRUD）
│       ├── snapshot.go             # Snapshot + SnapshotDAO
│       ├── registered_project.go   # RegisteredProject + DAO
│       ├── custom_sync_rule.go     # CustomSyncRule + DAO
│       ├── sync_task.go            # SyncTask（预留）
│       ├── remote_config.go        # RemoteConfig（预留）
│       ├── app_config.go           # 应用配置/统计模型
│       └── utils.go                # 工具函数
│
│   └── types/                      # 对外返回结构体（按功能分子目录）
│       ├── snapshot/               # 快照相关返回结构体
│       ├── scan/                   # 扫描相关返回结构体
│       ├── config/                 # 配置浏览相关返回结构体
│       └── common/                 # 通用/共享返回结构体
│
├── pkg/
│   ├── config/                     # YAML 配置（嵌入默认值 → 用户覆盖）
│   ├── database/db.go              # SQLite 连接、迁移、事务
│   ├── errors/                     # 错误码与包装
│   ├── logger/                     # Zap 日志
│   └── utils/                      # 字符串/文件/转换工具
│
├── configs/default.yaml            # 配置模板（与 pkg/config/default.yaml 同步）
├── docs/                           # 文档（使用指南/架构设计/_old 存档）
├── go.mod · go.sum · Makefile · .golangci.yml · README.md
```

---

## 架构规范

### 数据库设计约束（最大可移植性原则）

所有 DDL 及数据库操作代码必须严格遵守以下禁止项：

1. **禁止使用外键约束（FOREIGN KEY）**：关系完整性在应用层代码逻辑中维护，不得依赖数据库层的 `REFERENCES` 约束
2. **禁止使用触发器（TRIGGER）**：不得创建任何 DML/DDL 触发器，`updated_at` 等字段由应用层（GORM tag）管理
3. **禁止使用存储过程（STORED PROCEDURE）及用户定义函数（UDF）**：所有业务逻辑必须位于应用代码中
4. **禁止使用特定数据库专有扩展**：包括但不限于 PostgreSQL 的 JSONB 运算符、MySQL 的 ENUM 类型、SQL Server 的 IDENTITY_INSERT 语法、Oracle 的 SEQUENCE 伪列（若非 ANSI SQL 标准）
5. **仅允许使用 ANSI SQL 标准数据类型**：VARCHAR、INTEGER、TIMESTAMP、TEXT、BOOLEAN 等
6. **索引（INDEX）允许创建**，但必须使用 `CREATE INDEX` 独立语句，不得依赖隐式索引（PRIMARY KEY 自动创建的索引除外）

> **自查规则**：若检测到 `CREATE TRIGGER`、`ADD FOREIGN KEY`、`CREATE PROCEDURE` 等关键字，必须拒绝并替换为应用层逻辑。

### 分层调用链

```
CLI/TUI 层  →  UseCase 层  →  Service 层  →  DAO（models 内）
   ↓              ↓              ↓              ↓
 仅解析参数     流程编排       单领域逻辑     数据库 CRUD
 仅渲染输出     错误翻译       可调 DAO       只操作一个表
 不含业务逻辑   跨服务协调     不调其他服务    返回明确结构体
```

**绝对禁止**：CLI/TUI 直接调用 Service 或 DAO。

### Models 层规则

- **结构体 + DAO 同文件**：`snapshot.go` 包含 `Snapshot` 结构体和 `SnapshotDAO`
- **DAO 返回值**：只返回 models 中定义的数据库结构体，禁止返回 `map`、`interface{}`
- **单一职责**：每个 DAO 函数只操作一个数据库表
- **不做转换**：DAO 不做响应结构体的 convert
- **纯数据定义**：Models 层只负责数据库表结构定义和 CRUD 操作，不包含任何业务逻辑

```go
// internal/models/xxx.go — 标准文件结构
package models

// Example 数据库表结构，无外键定义
type Example struct {
    ID   string `json:"id" db:"id"`
    Name string `json:"name" db:"name"`
}

type ExampleDAO struct { db *database.DB }

func NewExampleDAO(db *database.DB) *ExampleDAO { return &ExampleDAO{db: db} }
func (dao *ExampleDAO) Create(e *Example) error { /* ... */ }
func (dao *ExampleDAO) GetByID(id string) (*Example, error) { /* ... */ }
```

### Types 层规则

所有对调用方（CLI/TUI/UseCase）返回的结构体统一定义在 `internal/types/` 包中，按功能模块分子目录组织。

- **按功能分目录**：`types/snapshot/`、`types/scan/`、`types/config/`、`types/common/`
- **与 Models 区分**：Models 是数据库表映射，Types 是面向调用方的响应结构体
- **Service/UseCase 负责 convert**：从 DAO 拿到 Models 结构体后，转换为 Types 结构体返回
- **命名规范**：目录名即包名，结构体命名语义清晰，如 `types/snapshot.SnapshotDetail`

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

```go
// internal/types/common/pagination.go
package common

// Pagination holds pagination metadata for list responses.
type Pagination struct {
    Total  int `json:"total"`
    Offset int `json:"offset"`
    Limit  int `json:"limit"`
}
```

### Service 层规则

- 调用 DAO 获取数据，执行业务逻辑，组装结果
- 可组合多个 DAO 的返回值
- **转换职责**：将 Models 结构体转换为 `internal/types/` 中定义的响应结构体返回给调用方

### UseCase 层规则

- 编排多个 Service 完成完整业务流程
- 将底层技术性错误翻译为用户可读错误（`UserError`）
- 决定对外返回的数据结构（定义在 `internal/types/` 中）

---

## 代码规范

### 命名

- Go 文件：`snake_case`，测试文件加 `_test.go`
- 包名：小写单词，无下划线
- Receiver：小写缩写（`w *LocalWorkflow`、`dao *SnapshotDAO`），禁止 `this`/`self`

### 注释

- **导出类型/函数**：必须有 godoc 注释（英文），说明做什么、为什么
- **函数内部**：用 `// 第 X 步：` 标记逻辑块；非显而易见的逻辑用 `// 为什么：` 说明设计决策
- **不要**：翻译代码、写创建日期/作者、注释被删除的代码

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
import "ai-sync-manager/pkg/errors"
return errors.Wrap(err, "创建快照失败")
```

### 日志

```go
import "ai-sync-manager/pkg/logger"
logger.Info("操作开始", zap.String("snapshot_id", id), zap.Int("file_count", n))
```

---

## 开发工作流

### 分支开发（必须使用 worktree）

禁止直接在 `master` 编码。每次从 `master` 切出 worktree：

```bash
git checkout master && git pull origin master
git worktree add ../AToSync-feat-xxx feat/xxx
# ... 在 worktree 中开发 ...
git push -u origin feat/xxx
git checkout master && git merge feat/xxx && git push origin master
git worktree remove ../AToSync-feat-xxx && git branch -d feat/xxx
```

**分支前缀**：`feat/` `fix/` `refactor/` `docs/` `test/` `chore/` `perf/` `style/`

### 文档同步检查

每次提交后检查：

| 检查项 | 何时更新 |
|--------|----------|
| CLAUDE.md 项目结构 | 新增/删除文件、架构变更 |
| `docs/使用指南/` | 新命令、参数变化、行为变化 |
| `docs/架构设计/` | 模块新增/重构、数据流变化 |
| `docs/文档索引.md` | 新增或删除任何文档 |

---

## 提交规范

**语言**：Commit Message **全部使用中文**（仅 type 前缀如 `feat:` `fix:` 保留英文），描述、说明、测试表格等内容必须为中文。禁止使用英文描述。

**格式**：

```
<type>: <中文简要描述>

完成了什么：
有什么作用：
可能的影响：（无则省略）

| 测试场景 | 输入/操作 | 预期结果 | 实际结果 |
|----------|-----------|----------|----------|
| xxx      | xxx       | xxx      | 通过     |
```

**Type 列表**：`feat` `fix` `refactor` `docs` `test` `chore` `perf` `style`

**示例**：

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

## CLI 命令速查

| 命令 | 用法 | 关键 flag |
|------|------|-----------|
| scan | `ai-sync scan [app-or-project...]` | — |
| scan add | `ai-sync scan add <app> <path>` | `--project/-p` 注册项目 |
| scan remove | `ai-sync scan remove <app> <path>` | `--project/-p` 删除项目 |
| scan list | `ai-sync scan list [app-or-project]` | `--verbose/-v` 显示详细配置项 |
| scan rules | `ai-sync scan rules [app-or-project]` | — |
| get | `ai-sync get <project> [path]` | `--edit/-e` 编辑模式 `--snapshot/-s` 浏览快照 |
| setting create | `ai-sync setting create` | `--name` 名称 `--token` 令牌 `--api` API地址 `--opus-model` Opus模型 `--sonnet-model` Sonnet模型 |
| setting list | `ai-sync setting list` | `-l` 条数 `-o` 偏移 |
| setting get | `ai-sync setting get <name>` | — |
| setting delete | `ai-sync setting delete <name>` | — |
| setting switch | `ai-sync setting switch <name>` | — |
| snapshot create | `ai-sync snapshot create` | `-t` 工具 `-m` 说明 `-n` 名称 `-p` 项目 |
| snapshot list | `ai-sync snapshot list` | `-l` 条数 `-o` 偏移 |
| snapshot delete | `ai-sync snapshot delete <id-or-name>` | — |
| tui | `ai-sync tui` | — |

---

## 常用命令

```bash
make build                          # 构建 bin/ai-sync.exe
make build CLI_NAME=my-tool         # 自定义名称
make run                            # 构建并运行
make run ARGS=scan                  # 带参数运行
make fmt                            # 格式化
make clean                          # 清理

go test ./...                       # 全部测试
go test ./internal/service/snapshot/... -v -cover  # 特定包
go test -coverprofile=coverage.out ./...            # 覆盖率
go run -buildvcs=false ./cmd/ai-sync <command>      # 直接运行
```

---

## 文档索引

- [文档索引](docs/文档索引.md)
- [命令行与终端界面 MVP 使用指南](docs/使用指南/命令行与终端界面MVP使用指南.md)
- [快照功能使用指南](docs/使用指南/快照功能使用指南.md)
- [终端架构详解](docs/架构设计/终端架构详解.md)
- [快照系统技术架构](docs/架构设计/快照系统技术架构.md)
- [历史存档](docs/_old/README.md) — 已废弃的 Wails GUI 时代文档
