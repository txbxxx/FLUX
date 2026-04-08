# Bug 修复记录：性能优化与安全防护（BUG-6/13/15）

> 修复日期：2026-04-08
> 关联计划：bugfix-ai-setting-and-architecture.md

---

## 修复概览

| Bug | 风险 | 问题 | 修复方式 |
|-----|------|------|---------|
| BUG-6 | 中 | 快照按工具/标签查询全量加载内存过滤 | DAO 层新增 SQL LIKE 过滤方法 |
| BUG-13 | 中 | AI Setting 列表在内存中分页 | DAO 层新增 SQL 分页查询方法 |
| BUG-15 | 低 | 文件收集无大小限制可能导致 OOM | 添加 10MB 大小检查与 detector 一致 |

---

## BUG-6: GetSnapshotsByTool/ByTag 全量加载内存过滤

### 问题

`GetSnapshotsByTool` 和 `GetSnapshotsByTag` 通过 `snapshotDAO.List(0, 0)` 获取所有快照到内存后，再用 Go 代码遍历过滤。快照数量增长后会产生不必要的内存占用和性能浪费。

### 修复方式

1. 在 `SnapshotDAO` 新增 `ListByToolType(toolType, limit, offset)` 和 `ListByTag(tag, limit, offset)` 方法
2. 使用 SQL `WHERE tools LIKE '%"toolname"%'` 在数据库层过滤
   - tools 和 tags 字段以 JSON 数组格式存储（如 `["claude","codex"]`），LIKE 匹配带引号的精确值是安全的
3. 提取公共的 `loadSnapshotsWithFiles` 方法，复用文件批量加载逻辑
4. Service 层的 `GetSnapshotsByTool` 和 `GetSnapshotsByTag` 改为直接调用新 DAO 方法

### 涉及文件

- `internal/models/snapshot.go` — 新增 `ListByToolType`、`ListByTag`、`loadSnapshotsWithFiles`
- `internal/service/snapshot/service.go` — 简化为委托调用

### 为什么需要修复

随着快照数量增长，每次查询都全量加载 + 内存过滤会越来越慢。SQL 层过滤只返回符合条件的记录，是更正确的工程实践。

---

## BUG-13: ListAISettings 内存分页

### 问题

`ListAISettings` 从数据库加载全部 AI 配置后，用循环 `if i < offset` / `if len(items) >= limit` 在内存中做分页。数据量增长后性能差。

### 修复方式

1. 在 `AISettingDAO` 新增 `ListPaginated(limit, offset int) ([]*AISetting, int, error)` 方法
   - 先 `COUNT` 查总数，再用 `LIMIT/OFFSET` 分页
2. 在 `AISettingService` 实现 `ListPaginated` 方法，做 models→types 转换
3. 在 `AISettingManager` 接口新增 `ListPaginated` 方法
4. `ListAISettings` UseCase 改用 `ListPaginated` 获取分页数据
5. 当分页结果中未匹配到当前配置时，回退到 `matchCurrentSettingName()` 查找
   - 保证 `Current` 字段始终正确，即使当前生效配置不在当前页

### 涉及文件

- `internal/models/ai_setting.go` — 新增 `ListPaginated`
- `internal/service/setting/service.go` — 新增 `ListPaginated` 实现
- `internal/app/usecase/ai_setting.go` — 更新接口和 `ListAISettings` 方法

### 为什么需要修复

分页的本质是减少数据库返回的数据量。在内存中分页等于先全量加载再丢弃大部分数据，违背了分页的目的。

---

## BUG-15: collectSingleFile 将整个文件读入内存

### 问题

`collectSingleFile` 使用 `os.ReadFile` 读取文件内容，无大小限制。项目中 `detector`（扫描阶段）有 10MB 大小检查，但 `collector`（收集阶段）没有。如果用户配置目录中存在超大文件，collector 会直接将其全部读入内存。

### 修复方式

在 `os.Stat` 之后、`os.ReadFile` 之前添加 10MB 大小检查：
- 超过限制的文件跳过并记录 Debug 日志
- 返回 `nil, nil` 表示跳过（与去重逻辑一致）

### 涉及文件

- `internal/service/snapshot/collector.go` — 添加 `maxFileSize` 常量和大小检查

### 为什么需要修复

防御性编程。虽然 AI 工具配置文件通常很小，但 collector 扫描的是用户目录，无法排除存在大文件的可能。与 detector 保持一致的大小限制，可以避免潜在的内存暴涨问题。
