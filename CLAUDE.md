# AI Configuration Sync Manager

> AI 工具配置同步管理器 - 基于 Wails 框架的桌面应用
>
> **开发语言**：请使用中文进行交流和文档编写

## 项目概述

这是一个用于同步和管理多个 AI 编程工具（Codex、Claude、Cursor、Windsurf）配置文件的桌面应用程序。支持快照、版本控制、跨设备同步等功能。

**技术栈：**
- 后端：Go 1.25+ + Wails 框架
- 前端：Vue 3 + TypeScript + Vite
- 数据库：modernc.org/sqlite（纯 Go，无 CGO）
- Git：go-git v5

## 项目结构

```
ai-sync-manager/
├── main.go                 # Wails 应用入口
├── app.go                  # Wails 绑定层主应用
├── wails.json             # Wails 配置
├── go.mod/go.sum          # Go 依赖
│
├── internal/              # 内部包（不对外暴露）
│   ├── models/            # 数据模型 + DAO（Active Record 模式）
│   │   ├── snapshot.go   # Snapshot 模型 + SnapshotDAO
│   │   ├── sync_task.go  # SyncTask 模型 + SyncTaskDAO
│   │   ├── remote_config.go # RemoteConfig 模型 + RemoteConfigDAO
│   │   └── app_config.go # 应用配置模型
│   │
│   ├── service/          # 业务逻辑层
│   │   ├── tool/         # 工具检测模块
│   │   │   ├── detector.go   # 工具检测器
│   │   │   ├── paths.go      # 路径解析
│   │   │   └── types.go      # 类型定义
│   │   │
│   │   ├── git/          # Git 操作模块
│   │   │   ├── client.go     # Git 客户端
│   │   │   ├── auth.go       # 认证处理
│   │   │   └── types.go      # 类型定义
│   │   │
│   │   ├── snapshot/     # 快照管理模块
│   │   │   ├── service.go    # 快照服务
│   │   │   ├── collector.go  # 文件收集器
│   │   │   ├── applier.go    # 快照应用器
│   │   │   ├── comparator.go # 快照比较器
│   │   │   └── types.go      # 类型定义
│   │   │
│   │   └── sync/         # 同步模块
│   │       ├── sync.go       # 同步服务
│   │       ├── packager.go   # 快照打包器
│   │       └── types.go      # 类型定义
│   │
│   └── handler/          # 处理层（如有）
│       └── *.go          # HTTP/gRPC 处理器
│
├── pkg/                  # 公共包（可被外部引用）
│   ├── database/         # 通用数据库工具
│   │   └── db.go         # 连接、迁移、事务管理
│   ├── errors/           # 错误处理
│   ├── logger/           # 日志系统（基于 zap）
│   └── utils/            # 工具函数
│
├── frontend/             # Vue 3 前端
│   ├── src/
│   │   ├── components/   # Vue 组件
│   │   ├── assets/       # 静态资源
│   │   └── main.ts       # 入口文件
│   └── wailsjs/          # Wails 生成的绑定
│
├── cooperation/          # 开发文档
│   ├── api-reference.md
│   └── phase*-*.md       # 各阶段总结
│
└── build/                # 构建输出目录
```

---

## 开发规范

### 1. Models 层规范（数据模型 + DAO）

#### 命名规范
- **结构体命名**：以数据库表名为主，使用驼峰命名
- **DAO 命名**：结构体名 + DAO，例如 `SnapshotDAO`、`SyncTaskDAO`

#### DAO 函数规范
- **返回值**：只能返回 `models` 中定义的结构体，不允许返回 `map`、`interface{}`
- **单一职责**：每个函数只能操作一个数据库表，不允许一个函数操作多个表
- **不做转换**：不对返回的响应结构体进行 convert 操作，直接返回数据库映射的结构体

```go
// ✅ 正确示例
type SnapshotDAO struct {
    db *database.DB
}

// 返回明确的 Snapshot 结构体
func (dao *SnapshotDAO) GetByID(id string) (*models.Snapshot, error) {
    // ...
    return snapshot, nil
}

// ❌ 错误示例
func (dao *SnapshotDAO) GetByID(id string) (map[string]interface{}, error) {
    // 不要返回 map
}

func (dao *SnapshotDAO) GetByID(id string) (*SomeOtherType, error) {
    // 不要转换成其他类型
}

func (dao *SnapshotDAO) GetWithFiles(id string) (*SnapshotWithFiles, error) {
    // 不要一个函数操作多个表（snapshots + snapshot_files）
}
```

#### 文件组织
- 每个 DAO 与其对应的数据模型放在同一个文件中
- 例如：`SnapshotDAO` 放在 `internal/models/snapshot.go`

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
func (dao *SnapshotDAO) Create(snapshot *Snapshot) error { ... }
func (dao *SnapshotDAO) GetByID(id string) (*Snapshot, error) { ... }
```

### 2. Service 层规范（业务逻辑层）

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
        Snapshot: snapshot,
        FileCount: len(snapshot.Files),
        TotalSize: calculateTotalSize(snapshot.Files),
    }

    return stats, nil
}
```

### 3. Handler 层规范（处理层）

#### 职责
- **仅调用 Service**：handler 的作用就是调用 service 层
- **不做数据操作**：不允许直接操作数据库或调用 DAO
- **请求解析**：解析 HTTP/gRPC 请求
- **响应封装**：封装响应格式

```go
// ✅ 正确示例
func (h *Handler) GetSnapshot(c *gin.Context) {
    id := c.Param("id")

    // 仅调用 Service
    snapshot, err := h.service.GetSnapshot(id)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    c.JSON(200, snapshot)
}

// ❌ 错误示例
func (h *Handler) GetSnapshot(c *gin.Context) {
    id := c.Param("id")

    // 不要直接调用 DAO
    snapshotDAO := models.NewSnapshotDAO(h.db)
    snapshot, err := snapshotDAO.GetByID(id)

    // ...
}
```

---

## 代码规范

### 文件命名
- Go 文件：使用 `snake_case` 命名（如 `snapshot_service.go`）
- 测试文件：添加 `_test.go` 后缀

### 包命名
- 包名使用小写单词，避免下划线
- 包名应简短且有意义

### 结构体方法
```go
// 好的示例：receiver 使用小写缩写
func (s *Service) CreateSnapshot() error { }
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

### 添加新功能
1. 在 `internal/models/` 中定义数据模型和 DAO
2. 在 `internal/service/` 中实现业务逻辑
3. 在 `internal/handler/` 中添加处理器（如需要）
4. 编写单元测试（目标覆盖率 > 30%）
5. 在 `cooperation/` 中更新文档

### 测试
```bash
# 运行所有测试
go test ./... -v

# 运行特定包的测试
go test ./internal/service/snapshot/... -v -cover

# 查看覆盖率
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 构建
```bash
# 开发模式（热重载）
wails dev

# 生产构建
wails build

# 清理
wails build -clean
```

---

## 关键模块说明

### 1. 通用数据库工具 (`pkg/database`)
- **DB**: 数据库连接管理
- **迁移**: 自动执行表结构迁移
- **事务**: 提供事务处理接口
- 无业务逻辑，可被其他项目复用

### 2. 数据模型层 (`internal/models`)
- **Snapshot**: 快照模型 + SnapshotDAO
- **SyncTask**: 同步任务模型 + SyncTaskDAO
- **RemoteConfig**: 远端配置模型 + RemoteConfigDAO
- Active Record 模式：模型与 DAO 在同一文件

### 3. 工具检测模块 (`internal/service/tool`)
- **Detector**: 检测已安装的 AI 工具
- **Paths**: 获取工具配置文件路径
- 支持工具：Codex、Claude、Cursor、Windsurf

### 4. Git 操作模块 (`internal/service/git`)
- **GitClient**: 封装 go-git 操作
- 支持：clone、pull、push、commit、status
- 认证：SSH、Token、Basic Auth

### 5. 快照管理模块 (`internal/service/snapshot`)
- **Service**: 快照 CRUD 操作
- **Collector**: 收集配置文件
- **Applier**: 应用快照到文件系统
- **Comparator**: 比较快照差异

### 6. 同步模块 (`internal/service/sync`)
- **Service**: 推送/拉取快照到 Git
- **Packager**: 打包快照为 Git 提交
- 支持远程仓库同步

### 7. 加密模块 (`internal/service/crypto`)
- **Service**: AES-256-GCM 加密服务
- 支持：密码、Token、SSH 密钥加密
- 密钥来源：环境变量、密钥文件、自动生成
- **便捷方法**: EncryptPassword, EncryptToken, EncryptSSHKey

---

## 数据库

使用 SQLite，通过 `modernc.org/sqlite` 驱动（无 CGO 依赖）。

**DAO 使用模式：**
```go
// Service 层使用
import (
    "ai-sync-manager/internal/models"
    "ai-sync-manager/pkg/database"
)

func (s *Service) GetSnapshot(id string) (*models.Snapshot, error) {
    // 创建 DAO
    snapshotDAO := models.NewSnapshotDAO(s.db)

    // 调用 DAO 方法
    return snapshotDAO.GetByID(id)
}
```

---

## 常见任务

### 添加新的数据模型和 DAO
```go
// 1. 在 internal/models/ 中创建文件
// internal/models/example.go

package models

// 数据模型（对应数据库表）
type Example struct {
    ID   string `json:"id" db:"id"`
    Name string `json:"name" db:"name"`
}

// DAO
type ExampleDAO struct {
    db *database.DB
}

func NewExampleDAO(db *database.DB) *ExampleDAO {
    return &ExampleDAO{db: db}
}

// DAO 方法（返回明确的 Example 结构体）
func (dao *ExampleDAO) Create(example *Example) error { ... }
func (dao *ExampleDAO) GetByID(id string) (*Example, error) { ... }
```

### 添加新的数据库表
1. 在 `pkg/database/db.go` 的 `createTables()` 中添加表定义
2. 在 `internal/models/` 中定义 Go 结构体和 DAO
3. 编写测试验证

### 扩展快照文件类别
```go
// 在 internal/models/snapshot.go 中添加
const (
    CategoryConfig FileCategory = "config"
    // ...
    CategoryNewType FileCategory = "newtype"
)
```

---

## 依赖项

主要 Go 依赖：
- `github.com/wailsapp/wails/v2` - Wails 框架
- `github.com/go-git/go-git/v5` - Git 操作
- `modernc.org/sqlite` - SQLite 驱动
- `go.uber.org/zap` - 结构化日志
- `github.com/stretchr/testify` - 测试断言

---

## 已知限制

1. **Windows Git 路径**: 某些 Git 操作在 Windows 上可能需要特殊处理
2. **SSH 密钥**: 需要用户配置 SSH 密钥路径
3. **大文件**: 大量文件收集可能较慢

---

## 开发进度

- ✅ Phase 0: 项目基础设施
- ✅ Phase 1: 工具检测模块
- ✅ Phase 2: Git 操作模块
- ✅ Phase 3: 数据模型层
- ✅ Phase 4: 数据库持久化（已重构：DAO 与 Model 合并）
- ✅ Phase 5: 快照管理模块
- ✅ Phase 6: Git 集成模块
- ✅ Phase 7: 加密模块（已完成）
- ⏸️ Phase 8: 前端界面（暂缓）
- ✅ 前端 API 文档（见 `cooperation/frontend-api-reference.md`）
