# Bug 修复计划：AI Setting 模块缺陷 & 架构违规

> 审查日期：2026-04-07
> 状态：待修复

---

## 一、高风险问题

### BUG-1: generateUUID 使用纳秒时间戳，存在 ID 碰撞风险

- **文件**: `internal/app/usecase/ai_setting.go:528-530`
- **现状**: `generateUUID()` 使用 `time.Now().UnixNano()` 生成 ID
- **问题**: 并发批量操作时可能产生相同 ID，且与项目其他模块不一致（`snapshot/service.go` 已正确使用 `uuid.New().String()`）
- **修复**: 替换为 `uuid.New().String()`，项目已引入 `github.com/google/uuid v1.6.0`

### BUG-2: SwitchAISetting 非原子写入 settings.json

- **文件**: `internal/app/usecase/ai_setting.go:408`
- **现状**: 直接使用 `os.WriteFile` 覆盖配置文件
- **问题**: 写入过程中崩溃会导致文件被截断为空或部分写入，用户配置丢失
- **修复**: 采用"先写临时文件再 rename"模式，与 `accessor.go:239-261` 的 `WriteFile` 方法一致

### BUG-3: 类型断言无保护，可能 panic

- **文件**: `internal/app/usecase/ai_setting.go:384-385`
- **现状**: 对 `newSettings["env"]` 多次执行 `.(map[string]string)` 原始断言
- **问题**: 若 JSON 中 env 的实际类型不是 `map[string]string`（如为 nil 或嵌套结构），程序直接 panic
- **修复**: 使用 comma-ok 模式做安全断言，类型不匹配时返回明确错误

### BUG-4: UseCase 层直接 import models 包

- **文件**: `internal/app/usecase/local_workflow.go:10`、`internal/app/usecase/ai_setting.go:13`
- **现状**: UseCase 接口签名直接暴露 `models.Snapshot`、`models.CustomSyncRule`、`models.AISetting` 等 models 类型
- **问题**: 违反分层架构（CLI/TUI → UseCase → Service → DAO），Service 层也未做 Models→Types 转换，形成连锁违规
- **修复**:
  1. 在 `internal/types/` 中定义 UseCase 层需要的响应结构体
  2. Service 层方法返回 Types 而非 Models
  3. UseCase 接口签名改为使用 Types 类型
  4. 移除 UseCase 层对 models 包的 import

---

## 二、中风险问题

### BUG-5: 批量操作丢弃 context

- **文件**: `internal/app/usecase/ai_setting.go:582`、`ai_setting.go:621`
- **现状**: `GetAISettingsBatch` 和 `DeleteAISettingsBatch` 接收 `context.Context` 参数但未使用，内部调用 `context.Background()`
- **问题**: 调用方的 context 取消信号无法传递，超时/取消机制失效
- **修复**: 将方法参数中的 ctx 传递给内部调用

### BUG-6: GetSnapshotsByTool/ByTag 全量加载内存过滤

- **文件**: `internal/service/snapshot/service.go:288-318`、`service.go:322-352`
- **现状**: 通过 `List(0, 0)` 获取所有快照到内存后再按 ToolType/Tag 过滤
- **问题**: 快照数量增长后产生严重性能问题和内存浪费
- **修复**: 在 DAO 层新增带 WHERE 条件的查询方法，在 SQL 层过滤

### BUG-7: ExportSnapshot 导出失败静默跳过

- **文件**: `internal/service/snapshot/service.go:192-198`
- **现状**: 单个文件导出失败时仅打 Warn 日志，函数最终返回 nil
- **问题**: 调用方无法感知哪些文件导出失败
- **修复**: 收集失败文件列表，通过返回值或 error 告知调用方

### BUG-8: Database 单例首次失败后不可重试

- **文件**: `pkg/database/db.go:33-72`
- **现状**: `sync.Once` 保证只初始化一次，首次失败后 `once.Do` 不再执行
- **问题**: 后续调用 `InitDB` 返回 `nil, nil`（instance 为空但无错误），调用方误以为初始化成功
- **修复**: 初始化失败时重置 once 或在返回时检查 instance 是否为 nil

### BUG-9: snapshotRow 使用 GORM foreignKey 标签

- **文件**: `internal/models/snapshot.go:199`
- **现状**: `Files []snapshotFileRow \`gorm:"foreignKey:SnapshotID;references:ID"\``
- **问题**: CLAUDE.md 明确"禁止外键"，虽然是应用层标签而非数据库约束，但与规范意图冲突
- **修复**: 评估是否可以用显式查询替代 Preload，或在 CLAUDE.md 中补充说明应用层关联标签的例外

### BUG-10: GORM record 结构体在 database/db.go 和 models/snapshot.go 中重复定义

- **文件**: `pkg/database/db.go`、`internal/models/snapshot.go:187-222`
- **现状**: `database/db.go` 中有 `snapshotRecord` 用于建表迁移，`models/snapshot.go` 中有 `snapshotRow` 用于 CRUD
- **问题**: 两套结构体字段必须手动保持一致，无编译期保证
- **修复**: 统一到一处定义，另一处引用；或消除其中一套

### BUG-11: 多个导出函数缺少 godoc 注释

- **文件**: `internal/app/usecase/config_browser.go`（`GetConfig`、`SaveConfig`）、`internal/service/tool/rule_manager.go`（`AddCustomRule` 等）、`internal/service/tool/accessor.go`（`Resolve`、`ListDir` 等）
- **现状**: CLAUDE.md 要求"导出类型/函数必须有 godoc 注释（英文）"，但上述函数缺少
- **修复**: 补充英文 godoc 注释

### BUG-12: CLAUDE.md 项目结构与实际代码不同步

- **文件**: `CLAUDE.md`
- **现状**: 项目结构未列出 `internal/service/setting/`、`internal/types/setting/`、`internal/types/sync/`、`internal/models/ai_setting.go`、`internal/cli/cobra/setting.go`
- **问题**: 违反"CLAUDE.md 项目结构在新增/删除文件时必须更新"的规范
- **修复**: 更新 CLAUDE.md 项目结构部分

### BUG-13: ListAISettings 分页在内存中实现

- **文件**: `internal/app/usecase/ai_setting.go:197-205`
- **现状**: 从数据库加载全部配置后用循环做 offset/limit 分页
- **问题**: 数据量增长后性能差
- **修复**: 将分页逻辑下推到 DAO 层 SQL 查询

---

## 三、低风险问题

### BUG-14: generateUUID 注释误导

- **文件**: `internal/app/usecase/ai_setting.go:527`
- **现状**: 注释"实际应使用 google/uuid"，但项目已引入该依赖
- **修复**: BUG-1 修复后此注释一并删除

### BUG-15: collectSingleFile 将整个文件读入内存

- **文件**: `internal/service/snapshot/collector.go:192`
- **现状**: `os.ReadFile` 无大小限制
- **问题**: detector 有 10MB 检查，但 collector 路径没有
- **修复**: 在 collector 中也加入文件大小检查

### BUG-16: 通用错误类型定义位置不当

- **文件**: `internal/models/ai_setting.go:74-82`
- **现状**: `ErrRecordNotFound` 定义在业务文件中，其他 DAO 各自定义错误
- **修复**: 统一到 `models/utils.go` 或 `pkg/errors/`

---

## 修复优先级与任务划分

| 批次 | 任务编号 | 说明 | 依赖 |
|------|---------|------|------|
| P0 | BUG-1 | UUID 碰撞 | 无 |
| P0 | BUG-2 | 非原子写入 | 无 |
| P0 | BUG-3 | 类型断言 panic | 无 |
| P1 | BUG-4 | UseCase import models | 无 |
| P1 | BUG-5 | context 丢弃 | BUG-4 |
| P1 | BUG-13 | 内存分页 | BUG-4 |
| P2 | BUG-6 | 全量加载过滤 | 无 |
| P2 | BUG-7 | 导出静默跳过 | 无 |
| P2 | BUG-8 | DB 单例不可重试 | 无 |
| P3 | BUG-9 | GORM foreignKey | 无 |
| P3 | BUG-10 | record 重复定义 | BUG-9 |
| P3 | BUG-11 | godoc 缺失 | BUG-4 |
| P3 | BUG-12 | CLAUDE.md 同步 | BUG-4, BUG-11 |
| P4 | BUG-14 | 注释误导 | BUG-1 |
| P4 | BUG-15 | 文件大小检查 | 无 |
| P4 | BUG-16 | 错误类型位置 | 无 |
