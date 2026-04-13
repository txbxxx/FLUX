# 结构化输出格式支持 PRD

> 版本：v1.0 | 日期：2026-04-12

## 1. 背景与动机

当前所有 CLI 命令的输出面向人类阅读，设计为终端文本展示。但高级用户和自动化场景（脚本、CI 流水线）需要机器可读的输出格式。

现有命令如 `snapshot list`、`scan list`、`setting list`、`snapshot diff` 均无机器可读格式，用户无法将 AToSync 集成到自己的工具链中。

## 2. 目标用户

- 需要将 AToSync 集成到脚本或 CI 流水线的开发者用户
- 希望用 jq/yq 等工具处理 AToSync 输出的数据

## 3. 核心需求

### FR-01 统一 --format 参数

所有有结构化输出的命令新增 `--format` 参数，支持三种格式：

| 值 | 说明 | 默认命令 |
|----|------|---------|
| `table` | 表格/文本输出（默认，保持现有行为） | 所有命令 |
| `json` | JSON 格式输出 | list 类命令 |
| `yaml` | YAML 格式输出 | list 类命令 |

**实现方式**：在命令入口层（CLI）增加 `--format` flag，根据值选择渲染器。Service/UseCase 层保持不变。

### FR-02 覆盖的命令

| 命令 | 场景 | table 输出 | json 输出 | yaml 输出 |
|------|------|-----------|-----------|-----------|
| `snapshot list` | 快照列表 | 表格 | `{"total": N, "items": [...]}` | 嵌套 YAML |
| `scan list` | 扫描结果 | 表格 | `{"items": [...]}` | 嵌套 YAML |
| `setting list` | 配置列表 | 表格 | `{"items": [...]}` | 嵌套 YAML |
| `get <dir>` | 目录浏览 | 文本目录树 | `{"path": "...", "entries": [...]}` | 嵌套 YAML |
| `snapshot diff` | 快照对比 | unified diff 文本 | 完整 DiffResult 结构 | YAML |

**不涉及**：`snapshot diff` 的文本 diff 视图本身不是结构化数据，`--format=json/yaml` 时输出结构化 diff 结果；`get <file>` 直接输出文件内容，不加 `--format`。

### FR-03 JSON 输出示例

#### snapshot list --format=json

```json
{
  "total": 2,
  "items": [
    {
      "id": "a1b2c3d4-xxxx-xxxx",
      "name": "2026-04-01 备份",
      "message": "升级 Claude Code 前备份",
      "created_at": "2026-04-01T10:30:00+08:00",
      "project": "claude-global",
      "file_count": 15,
      "size": 524288
    }
  ]
}
```

#### scan list --format=json

```json
{
  "items": [
    {
      "tool": "claude",
      "scope": "global",
      "path": "/home/user/.claude",
      "files": [
        {"path": "settings.json", "size": 2048, "category": "config"},
        {"path": "commands/", "category": "directory"}
      ]
    }
  ]
}
```

#### setting list --format=json

```json
{
  "items": [
    {
      "name": "claude-api",
      "api_url": "https://api.anthropic.com",
      "opus_model": "claude-opus-4-5",
      "sonnet_model": "claude-sonnet-4-5",
      "is_active": true,
      "created_at": "2026-04-01T08:00:00+08:00"
    }
  ]
}
```

#### get <dir> --format=json

```json
{
  "path": "/home/user/.claude",
  "entries": [
    {"name": "settings.json", "path": "settings.json", "is_dir": false, "size": 2048},
    {"name": "commands", "path": "commands/", "is_dir": true},
    {"name": "skills", "path": "skills/", "is_dir": true}
  ]
}
```

#### snapshot diff --format=json

```json
{
  "source_name": "2026-04-01 备份",
  "target_name": "2026-04-05 备份",
  "has_diff": true,
  "stats": {
    "total_files": 5,
    "added_files": 1,
    "modified_files": 3,
    "deleted_files": 1,
    "add_lines": 15,
    "del_lines": 50
  },
  "files": [
    {
      "path": "claude/settings.json",
      "status": "modified",
      "is_binary": false,
      "old_size": 2048,
      "new_size": 2100,
      "add_lines": 5,
      "del_lines": 2,
      "tool_type": "claude",
      "hunks": [
        {
          "old_start": 3,
          "old_count": 10,
          "new_start": 3,
          "new_count": 13,
          "lines": [
            {"type": "context", "content": "  \"version\": 2,", "old_line_no": 3, "new_line_no": 3},
            {"type": "deleted", "content": "-  \"model\": \"sonnet\",", "old_line_no": 4, "new_line_no": 0},
            {"type": "added", "content": "+  \"model\": \"opus\",", "old_line_no": 0, "new_line_no": 4}
          ]
        }
      ]
    }
  ]
}
```

### FR-04 YAML 输出示例

YAML 输出与 JSON 结构一一对应，嵌套层级保持一致。YAML 更适合人类阅读的结构化输出场景。

### FR-05 format 与其他参数的关系

| 命令 | `--format` 与其他参数的关系 |
|------|--------------------------|
| `snapshot list --format=json` | 与 `--limit`/`--offset` 正交，互不影响 |
| `scan list --format=json` | 与 `--verbose` 正交 |
| `setting list --format=json` | 与 `--limit`/`--offset` 正交 |
| `get <dir> --format=json` | 与 `--snapshot` 正交 |
| `snapshot diff --format=json` | 与 `--verbose`/`--side-by-side`/`--tool`/`--path`/`--color` 正交；`--format` 时忽略颜色相关参数 |

**行为**：加 `--format=json` 或 `--format=yaml` 时，忽略所有纯展示类参数（如 `--color`、`--side-by-side`），直接输出结构化数据。

## 4. 非目标

- 不改造 Service/UseCase 层的数据结构（只改渲染层）
- 不支持 `--format=json` 下的颜色控制
- `snapshot diff --format=json` 退出码不反映差异存在与否（与其他 json 输出命令保持一致）
- 不在 TUI 中实现 `--format`（TUI 天然是可视化界面）

## 5. 技术方案

### 实现位置

在 CLI 层（`internal/cli/cobra/`）的渲染函数中增加 format 判断：

```
Command RunE
  ↓
Workflow 调用（不变）
  ↓
渲染函数（新增 format 判断）
  ├── printSnapshotListJSON()  ← 已存在，移到通用工具
  ├── printSnapshotListYAML()  ← 新增
  ├── printSnapshotList()      ← 现有表格
  └── （其他命令同理）
```

### 依赖

- JSON：`encoding/json`（Go 标准库）
- YAML：需新增依赖，`gopkg.in/yaml.v3`（与 go.mod 中现有依赖一致）

### 数据结构 JSON tag

确保所有对外返回的结构体（`SnapshotSummary`、`ScanResult`、`AISettingSummary` 等）已有 `json:"..."` tag。无 tag 的字段需补加。

## 6. 命令速查

```bash
# 快照列表
ai-sync snapshot list --format=json
ai-sync snapshot list --format=yaml
ai-sync snapshot list --format=table    # 默认，可省略

# 扫描结果
ai-sync scan list --format=json
ai-sync scan list --format=yaml

# 配置列表
ai-sync setting list --format=json
ai-sync setting list --format=yaml

# 目录浏览
ai-sync get claude skills/ --format=json
ai-sync get claude skills/ --format=yaml

# 快照对比
ai-sync snapshot diff abc123 def456 --format=json
ai-sync snapshot diff abc123 def456 --format=yaml

# 组合使用
ai-sync snapshot list --format=json --limit=10 --offset=0 | jq '.items[] | select(.file_count > 10)'
```

## 7. 验收标准

| 场景 | 预期 |
|------|------|
| `snapshot list --format=json` | 输出有效 JSON，可被 jq 解析 |
| `snapshot list --format=yaml` | 输出有效 YAML，可被 yq 解析 |
| `snapshot list`（不加 format） | 保持现有表格输出不变 |
| `scan list --format=json` | 输出有效 JSON |
| `setting list --format=json` | 输出有效 JSON，token 被 mask |
| `get <dir> --format=json` | 输出目录结构化数据 |
| `snapshot diff --format=json` | 输出完整 DiffResult，包含 hunks |
| `snapshot diff --format=json` 退出码 | 始终 0（即使有差异），不反映 diff 状态 |
| 加 `--format=json` 同时加 `--color=always` | 忽略 `--color`，正常输出 JSON |
| `go test ./...` | 全部通过 |
