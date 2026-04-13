# setting 批量操作设计文档

**日期**: 2026-04-07
**功能**: `setting delete` 和 `setting get` 支持多个配置名称

---

## 需求背景

当前 `setting delete` 和 `setting get` 命令只支持单个配置名称：
- `setting delete <name>` - 删除单个配置
- `setting get <name>` - 获取单个配置详情

用户需要批量操作能力：
- `setting delete glm3 glm5` - 一次删除多个配置
- `setting get test_ok glm5` - 一次查看多个配置详情

---

## 功能设计

### 1. setting get 批量获取

**输入**：`setting get <name1> <name2> ...`

**输出格式**：连续展示，每个配置完整信息用 `---` 分隔

```
配置: glm3
配置 ID: xxx
Token: ****
Base URL: test.com
Opus 模型: model1
当前生效: 否
---
配置: glm5
配置 ID: yyy
Token: ****
Base URL: test.com
Sonnet 模型: model2
当前生效: 是
---

汇总：成功 2 个，失败 0 个
```

### 2. setting delete 批量删除

**输入**：`setting delete <name1> <name2> ...`

**交互流程**：
1. 列出将要删除的配置
2. 要求用户确认（输入 `y`/`yes`）
3. 执行删除
4. 输出汇总结果

```
将删除以下配置：
- glm3
- glm5
- glm2

确认删除？(y/yes): y

已删除: glm3, glm5
删除失败: glm2 (配置不存在)

汇总：成功 2 个，失败 1 个
```

---

## 技术设计

### 架构分层

采用 **方案 B：UseCase 层批量处理**

```
CLI 层 → UseCase 层 → Service/DAO 层
  收集    批量处理     单条CRUD
```

### 接口定义

**新增 UseCase 层结构体**（`internal/app/usecase/ai_setting.go`）：

```go
// GetAISettingsBatchInput 批量获取配置的输入
type GetAISettingsBatchInput struct {
    Names []string // 配置名称列表，至少一个
}

// GetAISettingsBatchResult 批量获取配置的返回值
type GetAISettingsBatchResult struct {
    Items   []*GetAISettingResult // 成功获取的配置列表
    Failed  []string               // 获取失败的配置名称列表
}

// DeleteAISettingsBatchInput 批量删除配置的输入
type DeleteAISettingsBatchInput struct {
    Names []string // 配置名称列表，至少一个
}

// DeleteAISettingsBatchResult 批量删除配置的返回值
type DeleteAISettingsBatchResult struct {
    Deleted []string // 成功删除的配置名称列表
    Failed  []string // 删除失败的配置名称列表（含原因）
}
```

**新增 Workflow 接口方法**（`internal/app/usecase/local_workflow.go`）：

```go
type Workflow interface {
    // ... 现有方法
    GetAISettingsBatch(ctx context.Context, input GetAISettingsBatchInput) (*GetAISettingsBatchResult, error)
    DeleteAISettingsBatch(ctx context.Context, input DeleteAISettingsBatchInput) (*DeleteAISettingsBatchResult, error)
}
```

### CLI 层修改

**`setting.go` 修改点**：

1. **参数约束**：
   - `get`: `ExactArgs(1)` → `MinimumNArgs(1)`
   - `delete`: `ExactArgs(1)` → `MinimumNArgs(1)`

2. **确认逻辑**（delete 专用）：
   - 显示待删除列表
   - 调用 `confirm` 函数等待用户输入

3. **输出格式化**：
   - get: 循环调用 `printSettingDetail`，中间插入分隔线
   - delete: 输出成功/失败汇总

### UseCase 层实现逻辑

**`GetAISettingsBatch` 实现要点**：

```go
func (w *LocalWorkflow) GetAISettingsBatch(ctx context.Context, input GetAISettingsBatchInput) (*GetAISettingsBatchResult, error) {
    // 参数校验：至少一个名称
    if len(input.Names) == 0 {
        return nil, &UserError{Message: "至少需要指定一个配置名称"}
    }

    result := &GetAISettingsBatchResult{
        Items:  make([]*GetAISettingResult, 0),
        Failed: make([]string, 0),
    }

    for _, name := range input.Names {
        singleResult, err := w.GetAISetting(ctx, GetAISettingInput{Name: name})
        if err != nil {
            result.Failed = append(result.Failed, name)
            continue
        }
        result.Items = append(result.Items, singleResult)
    }

    return result, nil
}
```

**`DeleteAISettingsBatch` 实现要点**：

类似逻辑，循环调用 `DeleteAISetting`，收集成功/失败列表。

---

## 修改文件清单

| 文件 | 修改内容 |
|------|----------|
| `internal/cli/cobra/setting.go` | Args 约束、确认逻辑、输出格式 |
| `internal/app/usecase/ai_setting.go` | 新增批量 Input/Result 结构体、新增批量方法 |
| `internal/app/usecase/local_workflow.go` | Workflow 接口新增批量方法声明 |
| `internal/cli/cobra/root.go` | Workflow 接口新增批量方法声明 |

---

## 测试用例

| 场景 | 输入 | 预期结果 |
|------|------|----------|
| 批量获取全部成功 | `setting get glm3 glm5` | 连续展示2个配置，汇总成功2个 |
| 批量获取部分失败 | `setting get glm3 notexist` | 展示glm3，提示notexist不存在，汇总成功1个失败1个 |
| 批量删除确认 | `setting delete glm3 glm5` + 输入 `y` | 先显示确认列表，确认后删除 |
| 批量删除取消 | `setting delete glm3 glm5` + 输入 `n` | 取消删除，不执行任何操作 |
| 批量删除部分失败 | `setting delete glm3 notexist` | 删除glm3，提示notexist不存在，汇总成功1个失败1个 |
