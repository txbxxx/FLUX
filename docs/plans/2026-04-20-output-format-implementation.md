# 结构化输出格式支持 - 技术实施计划

> 版本：v1.0 | 日期：2026-04-20 | 状态：待评审

## 📋 技术预研总结

### 已确认的技术事实

| 项目 | 状态 | 说明 |
|------|------|------|
| JSON tags | ✅ 已就绪 | `internal/types/` 所有结构体已有 `json:"..."` tag |
| YAML tags | ❌ 缺失 | 需要补充 `yaml:"..."` tag |
| YAML 依赖 | ✅ 已存在 | `gopkg.in/yaml.v3 v3.0.1` |
| 输出逻辑位置 | 📍 分散 | `root.go`, `setting.go`, 各命令文件中 |
| output 包 | ✅ 已存在 | `internal/cli/output/` 有 `table.go`, `style.go`, `diff.go` |

### 现有输出函数分布

| 函数 | 位置 | 服务命令 |
|------|------|----------|
| `printScanResult` | `root.go:152` | `scan list` |
| `printSnapshotList` | `root.go:280` | `snapshot list` |
| `printSettingList` | `setting.go:225` | `setting list` |
| `printDiffResult` | `snapshot_diff.go` | `snapshot diff` |
| `printBrowseResult` | `get.go` | `get <dir>` |

---

## 🎯 实施方案

### 核心设计原则

1. **轻量抽象**：不引入复杂接口，用简单的分发函数
2. **数据准备与渲染分离**：调用方准备 `tableData` 和 `rawData`，渲染器负责输出
3. **向后兼容**：默认行为（table 格式）保持不变
4. **增量实现**：按命令逐个实现，每个独立测试验证

### 架构设计

```
internal/cli/output/
├── table.go              # 已存在，表格渲染
├── style.go              # 已存在，样式常量
├── diff.go               # 已存在，diff 渲染
├── format.go             # 新增，格式分发与 JSON/YAML 渲染
└── format_test.go        # 新增，格式化测试
```

### 新增 `format.go` 核心代码

```go
package output

import (
    "encoding/json"
    "io"
    
    "gopkg.in/yaml.v3"
)

// Format 输出格式类型
type Format string

const (
    FormatTable Format = "table" // 默认表格输出
    FormatJSON  Format = "json"  // JSON 格式
    FormatYAML  Format = "yaml"  // YAML 格式
)

// Print 根据格式类型分发输出
// tableData: 用于表格渲染的数据（Table 结构体）
// rawData: 用于 JSON/YAML 序列化的原始数据（types 结构体）
func Print(w io.Writer, format Format, tableData *Table, rawData interface{}) error {
    switch format {
    case FormatJSON:
        return printJSON(w, rawData)
    case FormatYAML:
        return printYAML(w, rawData)
    default: // FormatTable
        if tableData != nil {
            _, err := io.WriteString(w, tableData.Render())
            return err
        }
        return nil
    }
}

// printJSON 输出 JSON 格式，带缩进
func printJSON(w io.Writer, data interface{}) error {
    enc := json.NewEncoder(w)
    enc.SetIndent("", "  ")
    return enc.Encode(data)
}

// printYAML 输出 YAML 格式
func printYAML(w io.Writer, data interface{}) error {
    return yaml.NewEncoder(w).Encode(data)
}
```

---

## 📝 实施步骤

### Phase 0: 基础设施（~2h）

#### 任务 0.1: 补充 YAML tags

**文件**: `internal/types/` 下所有文件

**改动**: 为所有有 `json` tag 的字段补充 `yaml` tag

```go
// 改动前
type SnapshotInfo struct {
    ID        uint      `json:"id"`
    Name      string    `json:"name"`
    CreatedAt time.Time `json:"created_at"`
}

// 改动后
type SnapshotInfo struct {
    ID        uint      `json:"id" yaml:"id"`
    Name      string    `json:"name" yaml:"name"`
    CreatedAt time.Time `json:"created_at" yaml:"created_at"`
}
```

**影响文件**:
- `internal/types/snapshot/*.go`
- `internal/types/setting/*.go`
- `internal/types/scan/*.go`

**验证方式**: 运行 `go test ./internal/types/...`

---

#### 任务 0.2: 创建 `format.go`

**文件**: `internal/cli/output/format.go`

**内容**: 见上方核心代码设计

**测试文件**: `internal/cli/output/format_test.go`

```go
package output

import (
    "bytes"
    "testing"
    
    "github.com/stretchr/testify/assert"
)

func TestPrintJSON(t *testing.T) {
    data := map[string]interface{}{
        "name": "test",
        "count": 42,
    }
    
    var buf bytes.Buffer
    err := printJSON(&buf, data)
    
    assert.NoError(t, err)
    assert.Contains(t, buf.String(), `"name": "test"`)
    assert.Contains(t, buf.String(), `"count": 42`)
}

func TestPrintYAML(t *testing.T) {
    data := map[string]interface{}{
        "name": "test",
        "count": 42,
    }
    
    var buf bytes.Buffer
    err := printYAML(&buf, data)
    
    assert.NoError(t, err)
    assert.Contains(t, buf.String(), "name: test")
    assert.Contains(t, buf.String(), "count: 42")
}

func TestPrintFormat(t *testing.T) {
    data := map[string]string{"name": "test"}
    table := &Table{
        Columns: []Column{{Title: "Name"}},
        Rows:    []Row{{Cells: []string{"test"}}},
    }
    
    // Test JSON
    var bufJSON bytes.Buffer
    err := Print(&bufJSON, FormatJSON, nil, data)
    assert.NoError(t, err)
    assert.Contains(t, bufJSON.String(), `"name": "test"`)
    
    // Test Table
    var bufTable bytes.Buffer
    err = Print(&bufTable, FormatTable, table, nil)
    assert.NoError(t, err)
    assert.Contains(t, bufTable.String(), "Name")
}
```

**验收**: `go test ./internal/cli/output/...` 通过

---

### Phase 1: snapshot list（~1.5h）

#### 任务 1.1: 修改 `snapshot_list.go`

**文件**: `internal/cli/cobra/snapshot_list.go`

**改动**:

```go
// 在 newSnapshotListCommand 中添加 flag
formatFlag := cmd.Flags().String("format", "table", "输出格式: table, json, yaml")

// 在 RunE 中修改调用逻辑
result, err := w.workflow.ListSnapshots(ctx, limit, offset)
if err != nil {
    return err
}

// 准备表格数据（复用现有逻辑）
tbl := buildSnapshotTable(result)
rawData := toSnapshotListData(result) // 新增转换函数

// 使用新的输出方式
return output.Print(cmd.OutOrStdout(), output.Format(*formatFlag), tbl, rawData)
```

**新增转换函数**:

```go
// toSnapshotListData 将 usecase 返回结果转换为 types 结构体（用于 JSON/YAML）
func toSnapshotListData(result *usecase.ListSnapshotsResult) interface{} {
    items := make([]types.SnapshotInfo, len(result.Items))
    for i, item := range result.Items {
        items[i] = types.SnapshotInfo{
            ID:          item.ID,
            Name:        item.Name,
            Description: item.Description,
            CreatedAt:   item.CreatedAt,
            Project:     item.Project,
            CommitHash:  item.CommitHash,
            IsRemote:    item.IsRemote,
        }
    }
    return map[string]interface{}{
        "total": result.Total,
        "items": items,
    }
}
```

**验收**:

```bash
# 测试 table 格式（默认）
fl snapshot list

# 测试 JSON 格式
fl snapshot list --format=json | jq .

# 测试 YAML 格式
fl snapshot list --format=yaml

# 测试与 --limit 组合
fl snapshot list --format=json --limit=1
```

---

### Phase 2: setting list（~1h）

#### 任务 2.1: 修改 `setting.go`

**文件**: `internal/cli/cobra/setting.go`

**改动**: 类似 snapshot list

```go
// 在 newSettingListCommand 中添加 flag
formatFlag := cmd.Flags().String("format", "table", "输出格式: table, json, yaml")

// 修改 printSettingList 调用
rawData := toSettingListData(result)
tbl := buildSettingTable(result)
return output.Print(cmd.OutOrStdout(), output.Format(*formatFlag), tbl, rawData)
```

**新增转换函数**:

```go
func toSettingListData(result *usecase.ListAISettingsResult) interface{} {
    return map[string]interface{}{
        "total":   result.Total,
        "current": result.Current,
        "items":   result.Items, // 直接使用，已有 json tag
    }
}
```

**注意**: token 字段在 types 层已脱敏，无需额外处理

**验收**:

```bash
fl setting list --format=json
fl setting list --format=yaml
```

---

### Phase 3: scan list（~1.5h）

#### 任务 3.1: 修改 `scan.go` 和 `root.go`

**文件**: `internal/cli/cobra/scan.go`, `internal/cli/cobra/root.go`

**改动**: 
1. 在 `newScanListCommand` 添加 `--format` flag
2. 修改 `printScanResult` 支持格式参数

**新增转换函数**:

```go
func toScanListData(result *usecase.ScanResult) interface{} {
    items := make([]map[string]interface{}, len(result.Tools))
    for i, tool := range result.Tools {
        items[i] = map[string]interface{}{
            "project":     tool.ProjectName,
            "tool_type":   tool.Tool,
            "path":        tool.Path,
            "status":      tool.ResultText,
            "config_count": tool.ConfigCount,
            "items":       tool.Items, // 详细配置项
        }
    }
    return map[string]interface{}{
        "items": items,
    }
}
```

**注意**: `--verbose` 在 JSON/YAML 模式下始终包含完整数据

**验收**:

```bash
fl scan list --format=json
fl scan list --format=json --verbose
```

---

### Phase 4: get <dir>（~1.5h）

#### 任务 4.1: 修改 `get.go`

**文件**: `internal/cli/cobra/get.go`

**改动**: 
1. 添加 `--format` flag
2. 只对目录浏览生效，文件内容直接输出

```go
// 判断是目录还是文件
if fileInfo.IsDir() {
    // 目录：支持 --format
    tbl := buildBrowseTable(result)
    rawData := toBrowseData(result)
    return output.Print(cmd.OutOrStdout(), output.Format(*formatFlag), tbl, rawData)
} else {
    // 文件：直接输出内容，不支持 --format
    if *formatFlag != "table" {
        return fmt.Errorf("--format 只支持目录浏览，文件内容直接输出")
    }
    // 现有文件输出逻辑
}
```

**新增转换函数**:

```go
func toBrowseData(result *usecase.BrowseResult) interface{} {
    entries := make([]map[string]interface{}, len(result.Entries))
    for i, entry := range result.Entries {
        entries[i] = map[string]interface{}{
            "name":   entry.Name,
            "path":   entry.Path,
            "is_dir": entry.IsDir,
            "size":   entry.Size,
        }
    }
    return map[string]interface{}{
        "path":    result.Path,
        "entries": entries,
    }
}
```

**验收**:

```bash
fl get claude skills/ --format=json
fl get claude skills/ --format=yaml
fl get claude settings.json # 文件，--format 不生效
```

---

### Phase 5: snapshot diff（~2h）

#### 任务 5.1: 修改 `snapshot_diff.go`

**文件**: `internal/cli/cobra/snapshot_diff.go`

**改动**: 最复杂，需要处理 DiffResult 结构

**新增转换函数**:

```go
func toDiffData(result *typesSnapshot.DiffResult) interface{} {
    return result // 直接使用，已有完整的 json tag
}
```

**行为说明**:
- `--format=json/yaml`: 输出完整 DiffResult，包含 hunks
- `--color`, `--side-by-side` 在结构化模式下静默忽略
- `--verbose` 在结构化模式下始终输出完整数据

**验收**:

```bash
# 创建两个快照
fl snapshot create -m "test1"
fl snapshot create -m "test2"

# 对比
fl snapshot diff <id1> <id2> --format=json
fl snapshot diff <id1> <id2> --format=yaml
```

---

## 🧪 测试策略

### 单元测试

每个 phase 都需要对应的单元测试：

```go
// output/format_test.go
func TestPrintSnapshotListJSON(t *testing.T)
func TestPrintSettingListYAML(t *testing.T)
func TestPrintScanListJSON(t *testing.T)
// ...
```

### 集成测试

```bash
# 测试所有命令的三种格式
for cmd in "snapshot list" "scan list" "setting list"; do
    for fmt in table json yaml; do
        echo "Testing: fl $cmd --format=$fmt"
        fl $cmd --format=$fmt
    done
done
```

### 验收测试清单

- [ ] 所有命令的 `--format=table` 与现有输出一致
- [ ] 所有命令的 `--format=json` 输出有效 JSON
- [ ] 所有命令的 `--format=yaml` 输出有效 YAML
- [ ] JSON/YAML 输出可被 jq/yq 解析
- [ ] `--format` 与其他参数（`--limit`, `--verbose` 等）正交
- [ ] `setting list` 的 token 已脱敏
- [ ] `get <file>` 时 `--format` 不生效
- [ ] `snapshot diff --format=json` 包含完整 hunks
- [ ] 所有现有测试通过

---

## 📊 工作量估算

| Phase | 任务 | 预估时间 | 优先级 |
|-------|------|----------|--------|
| Phase 0 | 基础设施 | 2h | P0 |
| Phase 1 | snapshot list | 1.5h | P0 |
| Phase 2 | setting list | 1h | P0 |
| Phase 3 | scan list | 1.5h | P1 |
| Phase 4 | get <dir> | 1.5h | P1 |
| Phase 5 | snapshot diff | 2h | P2 |
| 测试与修复 | - | 2h | P0 |
| **总计** | - | **11.5h** | - |

---

## 🚀 实施顺序建议

1. **第一批（P0）**: Phase 0 + Phase 1 + Phase 2 + 测试
   - 目标：打通基础流程，验证方案可行性
   - 预计：4.5h

2. **第二批（P1）**: Phase 3 + Phase 4 + 测试
   - 目标：覆盖主要 list 命令
   - 预计：3.5h

3. **第三批（P2）**: Phase 5 + 全面测试
   - 目标：完成所有命令
   - 预计：4h

---

## ⚠️ 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| types 结构体字段变更 | JSON/YAML 输出不稳定 | 保持 types 层稳定，所有变更需 review |
| YAML tags 遗漏 | 输出字段名不一致 | 编写脚本自动检查 yaml tag 覆盖率 |
| 大数据量性能 | JSON 序列化慢 | 设置合理的 `--limit` 默认值 |
| 用户文档滞后 | 用户不知道新功能 | 同步更新使用指南 |

---

## 📚 参考资料

- PRD: `docs/plans/2026-04-12-输出格式需求文档.md`
- 相关设计: `docs/plans/2026-04-09-统一输出格式技术设计.md`
- YAML 库文档: https://pkg.go.dev/gopkg.in/yaml.v3

---

## ✅ 下一步行动

1. **评审本计划**：确认技术方案和实施顺序
2. **创建开发分支**：`feat/output-format`
3. **开始 Phase 0**：补充 YAML tags，创建 format.go
4. **持续验证**：每个 phase 完成后运行测试套件

---

**文档状态**: 🟡 待评审  
**预计完成**: 1.5 个工作日  
**负责人**: 待分配
