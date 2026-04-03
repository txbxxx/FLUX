# AI Sync Manager

> **本地优先的 AI 工具配置管理器** - 纯终端形态的 CLI + TUI 应用
>
> **开发语言**：请使用中文进行交流和文档编写

---

## 项目概述

AI Sync Manager 是一个本地优先的 AI 工具配置管理器，聚焦于本地配置的扫描、快照、浏览和编辑功能。

### 核心定位

- **本地优先**：不依赖远程服务，所有配置存储在本地
- **终端形态**：提供 CLI 命令行和轻量 TUI 终端界面两种交互方式
- **配置管理**：支持 Codex、Claude、Cursor、Windsurf 等主流 AI 工具的配置文件管理

### 当前能力

- ✅ 扫描本机 AI 工具配置
- ✅ 创建本地快照
- ✅ 列出和管理本地快照
- ✅ 浏览配置目录与文件
- ✅ 在终端内直接编辑配置文件
- ✅ 自定义扫描规则

### 暂未覆盖

- ⏸️ 远端仓库同步（push/pull/diff/apply/rollback）
- ⏸️ 图形化凭证管理界面

---

## 技术栈

| 类别 | 技术 | 版本 |
|------|------|------|
| 语言 | Go | 1.25+ |
| CLI 框架 | Cobra | v1.9.1 |
| TUI 框架 | Bubbletea + Lipgloss | v1.3.10 / v1.1.0 |
| 数据库 | SQLite (modernc.org/sqlite) | v1.47.0 |
| Git 操作 | go-git | v5.17.0 |
| 日志 | Zap | v1.27.1 |
| 测试 | Testify | v1.10.0 |

---

## 项目结构

```
ai-sync-manager/
├── cmd/
│   └── ai-sync/                    # CLI 入口
│       ├── main.go                 # 主函数：组装依赖、启动 CLI/TUI
│       └── main_test.go
│
├── internal/
│   ├── app/                        # 应用层
│   │   ├── runtime/                # 运行时管理
│   │   │   └── runtime.go          # 依赖组装、数据库初始化
│   │   └── usecase/                # 用例层（业务流程编排）
│   │       ├── local_workflow.go   # 本地工作流：快照、扫描、浏览
│   │       └── config_browser.go   # 配置浏览器：目录浏览、文件读取
│   │
│   ├── cli/                        # CLI 层
│   │   └── cobra/                  # Cobra 命令定义
│   │       ├── root.go             # 根命令
│   │       ├── scan.go             # scan 命令
│   │       ├── snapshot.go         # snapshot 子命令组
│   │       ├── snapshot_create.go  # snapshot create 命令
│   │       ├── snapshot_list.go    # snapshot list 命令
│   │       ├── get.go              # get 命令（浏览配置）
│   │       └── tui.go              # tui 命令
│   │
│   ├── tui/                        # TUI 层
│   │   ├── app.go                  # Bubbletea 应用
│   │   ├── model.go                # TUI 状态模型
│   │   ├── view.go                 # 视图渲染
│   │   ├── update.go               # 消息更新处理
│   │   ├── page_scan.go            # 扫描页面
│   │   ├── page_snapshots.go       # 快照列表页面
│   │   ├── form_create.go          # 创建快照表单
│   │   ├── editor.go               # 配置编辑器
│   │   └── editor_model.go         # 编辑器状态模型
│   │
│   ├── service/                    # 服务层（业务逻辑）
│   │   ├── tool/                   # 工具检测服务
│   │   │   ├── detector.go         # 工具检测器
│   │   │   ├── accessor.go         # 配置访问器
│   │   │   └── paths.go            # 路径解析
│   │   ├── snapshot/               # 快照管理服务
│   │   │   ├── service.go          # 快照 CRUD
│   │   │   ├── collector.go        # 文件收集器
│   │   │   ├── applier.go          # 快照应用器
│   │   │   └── comparator.go       # 快照比较器
│   │   ├── git/                    # Git 操作服务
│   │   │   ├── client.go           # Git 客户端
│   │   │   ├── auth.go             # 认证处理
│   │   │   └── types.go            # 类型定义
│   │   ├── sync/                   # 同步服务
│   │   │   ├── sync.go             # 同步逻辑
│   │   │   └── packager.go         # 快照打包器
│   │   └── crypto/                 # 加密服务
│   │       ├── service.go          # AES-256-GCM 加密
│   │       └── types.go            # 加密相关类型
│   │
│   └── models/                     # 数据模型层（含 DAO）
│       ├── snapshot.go             # Snapshot 模型 + SnapshotDAO
│       ├── sync_task.go            # SyncTask 模型 + SyncTaskDAO
│       ├── remote_config.go        # RemoteConfig 模型 + RemoteConfigDAO
│       ├── registered_project.go   # RegisteredProject 模型 + DAO
│       ├── custom_sync_rule.go     # CustomSyncRule 模型 + DAO
│       ├── app_config.go           # 应用配置模型
│       └── utils.go                # 模型工具函数
│
├── pkg/                  # 公共包（可被外部引用）
│   ├── config/           # YAML 配置加载
│   │   ├── config.go     # 配置类型、加载、合并逻辑
│   │   ├── embed.go      # go:embed 嵌入默认配置
│   │   └── default.yaml  # 默认配置（嵌入二进制）
│   ├── database/         # 通用数据库工具
│   │   └── db.go         # 连接、迁移、事务管理
│   ├── errors/           # 错误处理
│   ├── logger/           # 日志系统（基于 zap）
│   └── utils/            # 工具函数
│
├── configs/              # 配置文件模板（供参考）
│   └── default.yaml      # 默认配置模板（与 pkg/config/default.yaml 同步）
│
├── docs/                           # 文档
│   ├── 使用指南/                   # 使用指南
│   ├── 架构设计/                   # 架构文档
│   ├── 协作文档/                   # 协作文档
│   └── plans/                      # 计划文档
│
├── go.mod / go.sum                 # Go 依赖管理
├── Makefile                        # 构建脚本
├── .golangci.yml                   # Golangci-lint 配置
└── README.md                       # 项目说明
```

---

## 开发规范

### 1. 分层架构规范

```
┌─────────────────────────────────────────────────────────┐
│  CLI / TUI 层 (internal/cli, internal/tui)              │
│  职责：命令解析、用户交互、视图渲染                       │
│  原则：不包含业务逻辑，仅调用 usecase 层                  │
└─────────────────────────────────────────────────────────┘
                          ▼
┌─────────────────────────────────────────────────────────┐
│  用例层 (internal/app/usecase)                          │
│  职责：业务流程编排、协调多个服务                         │
│  原则：不直接访问数据库，通过 service 层操作              │
└─────────────────────────────────────────────────────────┘
                          ▼
┌─────────────────────────────────────────────────────────┐
│  服务层 (internal/service)                              │
│  职责：单一业务领域逻辑（工具检测、快照、Git、加密）       │
│  原则：可调用 DAO，不调用其他服务                        │
└─────────────────────────────────────────────────────────┘
                          ▼
┌─────────────────────────────────────────────────────────┐
│  数据层 (internal/models + pkg/database)                │
│  职责：数据库操作、数据持久化                            │
│  原则：只包含数据模型和 DAO，无业务逻辑                   │
└─────────────────────────────────────────────────────────┘
```

### 2. Models 层规范（数据模型 + DAO）

#### 命名规范
- **结构体命名**：以数据库表名为主，使用驼峰命名
- **DAO 命名**：结构体名 + DAO，例如 `SnapshotDAO`、`SyncTaskDAO`

#### DAO 函数规范
- **返回值**：只能返回 `models` 中定义的结构体，不允许返回 `map`、`interface{}`
- **单一职责**：每个函数只能操作一个数据库表
- **不做转换**：不对返回的响应结构体进行 convert 操作

```go
// ✅ 正确示例
type SnapshotDAO struct {
    db *database.DB
}

// 返回明确的 Snapshot 结构体
func (dao *SnapshotDAO) GetByID(id string) (*Snapshot, error) {
    // ...
    return snapshot, nil
}

// ❌ 错误示例
func (dao *SnapshotDAO) GetByID(id string) (map[string]interface{}, error) {
    // 不要返回 map
}

func (dao *SnapshotDAO) GetWithFiles(id string) (*SnapshotWithFiles, error) {
    // 不要一个函数操作多个表
}
```

#### 文件组织
- 每个 DAO 与其对应的数据模型放在同一个文件中

```go
// internal/models/snapshot.go
package models

// 数据模型
type Snapshot struct {
    ID   string `json:"id" db:"id"`
    Name string `json:"name" db:"name"`
    // ...
}

// DAO
type SnapshotDAO struct {
    db *database.DB
}

// DAO 方法
func NewSnapshotDAO(db *database.DB) *SnapshotDAO
func (dao *SnapshotDAO) Create(snapshot *Snapshot) error
func (dao *SnapshotDAO) GetByID(id string) (*Snapshot, error)
```

### 3. Service 层规范（业务逻辑层）

#### 职责
- **调用 DAO**：从数据库获取数据或操作数据库
- **业务处理**：对数据库返回的数据进行业务逻辑处理
- **数据组装**：组合多个数据源的结果

```go
// ✅ 正确示例
func (s *Service) GetSnapshotWithStats(id string) (*SnapshotStats, error) {
    // 1. 调用 DAO 获取数据
    snapshotDAO := models.NewSnapshotDAO(s.db)
    snapshot, err := snapshotDAO.GetByID(id)
    if err != nil {
        return nil, err
    }

    // 2. 业务逻辑处理
    stats := &SnapshotStats{
        Snapshot:  snapshot,
        FileCount: len(snapshot.Files),
        TotalSize: calculateTotalSize(snapshot.Files),
    }

    return stats, nil
}
```

### 4. UseCase 层规范（用例层）

#### 职责
- **流程编排**：协调多个 Service 完成复杂业务流程
- **跨服务协调**：调用多个服务组装完整业务结果

```go
// ✅ 正确示例：local_workflow.go
func (w *LocalWorkflow) CreateSnapshot(name, message string) (*Snapshot, error) {
    // 1. 调用 detector 扫描工具
    tools, err := w.detector.DetectAll()
    if err != nil {
        return nil, err
    }

    // 2. 调用 collector 收集文件
    files, err := w.collector.Collect(tools)
    if err != nil {
        return nil, err
    }

    // 3. 调用 snapshotService 创建快照
    snapshot := &Snapshot{
        Name:    name,
        Message: message,
        Files:   files,
    }
    return w.snapshotService.Create(snapshot)
}
```

### 5. CLI/TUI 层规范

#### 职责
- **仅调用 UseCase**：不直接调用 Service 或 DAO
- **用户交互**：处理命令行输入、终端界面渲染
- **响应封装**：封装响应格式

```go
// ✅ 正确示例：CLI 命令
var createCmd = &cobra.Command{
    RunE: func(cmd *cobra.Command, args []string) error {
        // 仅调用 UseCase
        snapshot, err := workflow.CreateSnapshot(name, message)
        if err != nil {
            return err
        }

        fmt.Printf("快照已创建: %s\n", snapshot.ID)
        return nil
    },
}

// ❌ 错误示例
func (cmd *createCmd) Run() {
    // 不要直接调用 Service
    snapshotService := service.NewSnapshotService()
    snapshot, _ := snapshotService.Create(...)
}
```

---

## 代码规范

### 文件命名
- Go 文件：使用 `snake_case` 命名（如 `local_workflow.go`）
- 测试文件：添加 `_test.go` 后缀

### 包命名
- 包名使用小写单词，避免下划线
- 包名应简短且有意义

### 结构体方法
```go
// 好的示例：receiver 使用小写缩写
func (w *LocalWorkflow) CreateSnapshot() error { }
func (c *Collector) Collect() error { }
func (dao *SnapshotDAO) GetByID(id string) (*Snapshot, error) { }

// 避免使用 this、self 等命名
```

### 错误处理
```go
// 使用 pkg/errors 包装错误
import "ai-sync-manager/pkg/errors"

return errors.Wrap(err, "创建快照失败")
```

### 日志记录
```go
import "ai-sync-manager/pkg/logger"

logger.Info("操作开始",
    zap.String("snapshot_id", snapshot.ID),
    zap.Int("file_count", len(snapshot.Files)),
)
```

---

## 开发指南

### 添加新 CLI 命令

### 8. YAML 配置系统 (`pkg/config`)
- **Config**: 应用全局配置结构体
- **加载链**: 嵌入默认值 (`default.yaml`) → 用户覆盖 (`~/.ai-sync-manager/config.yaml`)
- **覆盖范围**: 应用配置、日志、数据库、同步默认值、工具规则定义
- **向后兼容**: 无用户配置文件时行为与改造前完全一致
- **接口约定**: `DatabaseConfig` 实现 `database.DatabaseConfig` 接口（`GetFilename`, `GetMaxOpenConns` 等）

---

## 开发指南

### 添加新 CLI 命令

1. 在 `internal/cli/cobra/` 中创建命令文件
2. 在 `root.go` 中注册子命令
3. 在 `usecase` 层添加业务流程（如需要）
4. 编写测试

```go
// internal/cli/cobra/hello.go
package cobra

var helloCmd = &cobra.Command{
    Use:   "hello",
    Short: "打招呼",
    RunE: func(cmd *cobra.Command, args []string) error {
        fmt.Println("Hello, AI Sync Manager!")
        return nil
    },
}

func init() {
    rootCmd.AddCommand(helloCmd)
}
```

### 添加新 TUI 页面

1. 在 `internal/tui/model.go` 中添加页面状态
2. 在 `internal/tui/view.go` 中添加视图渲染
3. 在 `internal/tui/update.go` 中添加消息处理
4. 在 `internal/tui/page_xxx.go` 中实现页面逻辑

### 添加新数据模型和 DAO

```go
// internal/models/example.go
package models

type Example struct {
    ID   string `json:"id" db:"id"`
    Name string `json:"name" db:"name"`
}

type ExampleDAO struct {
    db *database.DB
}

func NewExampleDAO(db *database.DB) *ExampleDAO {
    return &ExampleDAO{db: db}
}

func (dao *ExampleDAO) Create(example *Example) error { ... }
func (dao *ExampleDAO) GetByID(id string) (*Example, error) { ... }
```

---

## 常用命令

### 构建

```bash
# 构建默认二进制 (bin/ai-sync.exe)
make build

# 自定义 CLI 名称
make build CLI_NAME=my-tool

# 直接运行
make run

# 运行带参数
make run ARGS=scan
```

### 测试

```bash
# 运行所有测试
go test ./...

# 运行特定包测试
go test ./internal/service/snapshot/... -v -cover

# 查看覆盖率
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 开发

```bash
# 代码格式化
make fmt

# 清理构建产物
make clean

# 直接运行 CLI
go run -buildvcs=false ./cmd/ai-sync <command>
```

---

## 关键模块说明

### 1. 运行时管理 (`internal/app/runtime`)
负责组装所有依赖项，提供统一的应用运行时。

### 2. 用例层 (`internal/app/usecase`)
- **LocalWorkflow**: 本地工作流，编排快照创建、扫描、浏览等流程
- **ConfigBrowser**: 配置浏览器，处理目录浏览和文件读取

### 3. CLI 层 (`internal/cli/cobra`)
基于 Cobra 框架的命令行界面，提供 `scan`、`snapshot`、`get`、`tui` 等命令。

### 4. TUI 层 (`internal/tui`)
基于 Bubbletea 的终端用户界面，提供交互式快照管理体验。

### 5. 服务层 (`internal/service`)
- **tool**: 工具检测和配置访问
- **snapshot**: 快照管理（创建、读取、比较、应用）
- **git**: Git 操作（为未来同步功能预留）
- **sync**: 同步服务（为未来同步功能预留）
- **crypto**: 加密服务

### 6. 数据层 (`internal/models` + `pkg/database`)
- 数据模型定义
- DAO 实现
- 数据库连接和迁移管理

---

## 依赖项

主要 Go 依赖：

| 依赖 | 版本 | 用途 |
|------|------|------|
| github.com/spf13/cobra | v1.9.1 | CLI 框架 |
| github.com/charmbracelet/bubbletea | v1.3.10 | TUI 框架 |
| github.com/charmbracelet/lipgloss | v1.1.0 | TUI 样式 |
| github.com/go-git/go-git/v5 | v5.17.0 | Git 操作 |
| modernc.org/sqlite | v1.47.0 | SQLite 驱动 |
| go.uber.org/zap | v1.27.1 | 结构化日志 |
| github.com/stretchr/testify | v1.10.0 | 测试断言 |
| github.com/google/uuid | v1.6.0 | UUID 生成 |

---

## 开发进度

- ✅ Phase 0: 项目基础设施
- ✅ Phase 1: 工具检测模块
- ✅ Phase 2: Git 操作模块
- ✅ Phase 3: 数据模型层
- ✅ Phase 4: 数据库持久化（DAO 与 Model 合并）
- ✅ Phase 5: 快照管理模块
- ✅ Phase 6: Git 集成模块
- ✅ Phase 7: 加密模块
- ✅ Phase 8: CLI 命令行界面
- ✅ Phase 9: TUI 终端界面
- ⏸️ Phase 10: 远程同步功能（待开发）

---

## 已知限制

1. **Windows 路径**: 某些 Git 操作在 Windows 上可能需要特殊处理
2. **大文件**: 大量文件收集可能较慢
3. **远程同步**: 当前仅支持本地快照，远程同步功能待实现

---

## 文档索引

- [命令行与终端界面 MVP 使用指南](docs/使用指南/命令行与终端界面MVP使用指南.md)
- [终端架构详解](docs/架构设计/终端架构详解.md)
- [文档索引](docs/文档索引.md)
