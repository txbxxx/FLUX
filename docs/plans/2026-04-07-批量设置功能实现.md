# setting 批量操作实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**目标:** `setting delete` 和 `setting get` 支持批量操作多个配置名称

**架构:** 采用 UseCase 层批量处理方案，CLI 层编排交互和输出，复用现有单条记录的 DAO 操作

**技术栈:** Go, Cobra, GORM, SQLite

---

## Task 1: UseCase 层 - 新增批量数据结构

**文件:** `internal/app/usecase/ai_setting.go`

**Step 1: 在文件末尾（EOF 前）添加批量 Input/Result 结构体定义**

```go
// GetAISettingsBatchInput 批量获取配置的输入。
type GetAISettingsBatchInput struct {
	Names []string // 配置名称列表，至少一个
}

// GetAISettingsBatchResult 批量获取配置的返回值。
type GetAISettingsBatchResult struct {
	Items   []*GetAISettingResult // 成功获取的配置列表
	Failed  []string               // 获取失败的配置名称列表
}

// DeleteAISettingsBatchInput 批量删除配置的输入。
type DeleteAISettingsBatchInput struct {
	Names []string // 配置名称列表，至少一个
}

// DeleteAISettingsBatchResult 批量删除配置的返回值。
type DeleteAISettingsBatchResult struct {
	Deleted []string // 成功删除的配置名称列表
	Failed  []string // 删除失败的配置名称列表（含原因）
}
```

**Step 2: 运行 go build 验证编译**

Run: `go build ./...`
Expected: 无错误输出

**Step 3: 提交**

```bash
git add internal/app/usecase/ai_setting.go
git commit -m "feat(ai-setting): 新增批量操作 Input/Result 结构体定义

- GetAISettingsBatchInput/Result 用于批量获取配置
- DeleteAISettingsBatchInput/Result 用于批量删除配置
- 支持部分成功场景，分别收集成功和失败的列表"
```

---

## Task 2: UseCase 层 - 实现批量获取配置方法

**文件:** `internal/app/usecase/ai_setting.go`

**Step 1: 在文件末尾添加 GetAISettingsBatch 方法实现**

```go
// GetAISettingsBatch 批量获取多个配置的详情。
// 循环调用 GetAISetting，收集成功和失败的结果。
func (w *LocalWorkflow) GetAISettingsBatch(_ context.Context, input GetAISettingsBatchInput) (*GetAISettingsBatchResult, error) {
	// 参数校验：至少一个名称
	if len(input.Names) == 0 {
		return nil, &UserError{
			Message:    "批量获取配置失败：至少需要指定一个配置名称",
			Suggestion: "请提供至少一个配置名称",
			Err:        errors.New("empty names"),
		}
	}

	// 去重
	nameMap := make(map[string]bool)
	for _, name := range input.Names {
		nameMap[name] = true
	}
	uniqueNames := make([]string, 0, len(nameMap))
	for name := range nameMap {
		uniqueNames = append(uniqueNames, name)
	}

	result := &GetAISettingsBatchResult{
		Items:  make([]*GetAISettingResult, 0),
		Failed: make([]string, 0),
	}

	for _, name := range uniqueNames {
		singleResult, err := w.GetAISetting(context.Background(), GetAISettingInput{Name: name})
		if err != nil {
			result.Failed = append(result.Failed, name)
			continue
		}
		result.Items = append(result.Items, singleResult)
	}

	return result, nil
}
```

**Step 2: 运行测试验证编译**

Run: `go build ./...`
Expected: 无错误输出

**Step 3: 提交**

```bash
git add internal/app/usecase/ai_setting.go
git commit -m "feat(ai-setting): 实现 GetAISettingsBatch 批量获取配置方法

- 循环调用 GetAISetting 单条获取逻辑
- 自动去重配置名称
- 收集成功和失败的配置分别返回
- 复用现有错误处理和校验逻辑"
```

---

## Task 3: UseCase 层 - 实现批量删除配置方法

**文件:** `internal/app/usecase/ai_setting.go`

**Step 1: 在文件末尾添加 DeleteAISettingsBatch 方法实现**

```go
// DeleteAISettingsBatch 批量删除多个配置。
// 循环调用 DeleteAISetting，收集成功和失败的结果。
func (w *LocalWorkflow) DeleteAISettingsBatch(_ context.Context, input DeleteAISettingsBatchInput) (*DeleteAISettingsBatchResult, error) {
	// 参数校验：至少一个名称
	if len(input.Names) == 0 {
		return nil, &UserError{
			Message:    "批量删除配置失败：至少需要指定一个配置名称",
			Suggestion: "请提供至少一个配置名称",
			Err:        errors.New("empty names"),
		}
	}

	// 去重
	nameMap := make(map[string]bool)
	for _, name := range input.Names {
		nameMap[name] = true
	}
	uniqueNames := make([]string, 0, len(nameMap))
	for name := range nameMap {
		uniqueNames = append(uniqueNames, name)
	}

	result := &DeleteAISettingsBatchResult{
		Deleted: make([]string, 0),
		Failed:  make([]string, 0),
	}

	for _, name := range uniqueNames {
		err := w.DeleteAISetting(context.Background(), DeleteAISettingInput{Name: name})
		if err != nil {
			result.Failed = append(result.Failed, fmt.Sprintf("%s: %s", name, extractErrorMessage(err)))
			continue
		}
		result.Deleted = append(result.Deleted, name)
	}

	return result, nil
}

// extractErrorMessage 从错误中提取用户可读的错误信息
func extractErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	var userErr *UserError
	if errors.As(err, &userErr) {
		return userErr.Message
	}
	return err.Error()
}
```

**Step 2: 运行测试验证编译**

Run: `go build ./...`
Expected: 无错误输出

**Step 3: 提交**

```bash
git add internal/app/usecase/ai_setting.go
git commit -m "feat(ai-setting): 实现 DeleteAISettingsBatch 批量删除配置方法

- 循环调用 DeleteAISetting 单条删除逻辑
- 自动去重配置名称
- 收集成功和失败的配置分别返回
- 新增 extractErrorMessage 辅助函数提取错误信息"
```

---

## Task 4: Workflow 接口 - 声明批量方法

**文件:** `internal/app/usecase/local_workflow.go`

**Step 1: 在 Workflow 接口中添加批量方法声明**

在 `type Workflow interface` 定义中添加（大约第 50-64 行）：

```go
type Workflow interface {
	Scan(ctx context.Context, input ScanInput) (*ScanResult, error)
	AddCustomRule(ctx context.Context, input AddCustomRuleInput) error
	RemoveCustomRule(ctx context.Context, input RemoveCustomRuleInput) error
	AddProject(ctx context.Context, input AddProjectInput) error
	RemoveProject(ctx context.Context, input RemoveProjectInput) error
	ListScanRules(ctx context.Context, input ListScanRulesInput) (*ListScanRulesResult, error)
	CreateSnapshot(ctx context.Context, input CreateSnapshotInput) (*SnapshotSummary, error)
	ListSnapshots(ctx context.Context, input ListSnapshotsInput) (*ListSnapshotsResult, error)
	GetConfig(ctx context.Context, input GetConfigInput) (*GetConfigResult, error)
	SaveConfig(ctx context.Context, input SaveConfigInput) error
	CreateAISetting(ctx context.Context, input CreateAISettingInput) (*CreateAISettingResult, error)
	ListAISettings(ctx context.Context, input ListAISettingsInput) (*ListAISettingsResult, error)
	GetAISetting(ctx context.Context, input GetAISettingInput) (*GetAISettingResult, error)
	DeleteAISetting(ctx context.Context, input DeleteAISettingInput) error
	SwitchAISetting(ctx context.Context, input SwitchAISettingInput) (*SwitchAISettingResult, error)
	// 新增批量方法
	GetAISettingsBatch(ctx context.Context, input GetAISettingsBatchInput) (*GetAISettingsBatchResult, error)
	DeleteAISettingsBatch(ctx context.Context, input DeleteAISettingsBatchInput) (*DeleteAISettingsBatchResult, error)
}
```

**Step 2: 运行测试验证编译**

Run: `go build ./...`
Expected: 无错误输出

**Step 3: 提交**

```bash
git add internal/app/usecase/local_workflow.go
git commit -m "feat(ai-setting): Workflow 接口新增批量方法声明

- GetAISettingsBatch 批量获取配置方法
- DeleteAISettingsBatch 批量删除配置方法"
```

---

## Task 5: CLI 层 cobra/root.go - 声明批量方法

**文件:** `internal/cli/cobra/root.go`

**Step 1: 在 Workflow 接口中添加批量方法声明**

在 `type Workflow interface` 定义中添加（大约第 20-36 行）：

```go
type Workflow interface {
	Scan(ctx context.Context, input usecase.ScanInput) (*usecase.ScanResult, error)
	AddCustomRule(ctx context.Context, input usecase.AddCustomRuleInput) error
	RemoveCustomRule(ctx context.Context, input usecase.RemoveCustomRuleInput) error
	AddProject(ctx context.Context, input usecase.AddProjectInput) error
	RemoveProject(ctx context.Context, input usecase.RemoveProjectInput) error
	ListScanRules(ctx context.Context, input usecase.ListScanRulesInput) (*usecase.ListScanRulesResult, error)
	CreateSnapshot(ctx context.Context, input usecase.CreateSnapshotInput) (*usecase.SnapshotSummary, error)
	ListSnapshots(ctx context.Context, input usecase.ListSnapshotsInput) (*usecase.ListSnapshotsResult, error)
	GetConfig(ctx context.Context, input usecase.GetConfigInput) (*usecase.GetConfigResult, error)
	SaveConfig(ctx context.Context, input usecase.SaveConfigInput) error
	CreateAISetting(ctx context.Context, input usecase.CreateAISettingInput) (*usecase.CreateAISettingResult, error)
	ListAISettings(ctx context.Context, input usecase.ListAISettingsInput) (*usecase.ListAISettingsResult, error)
	GetAISetting(ctx context.Context, input usecase.GetAISettingInput) (*usecase.GetAISettingResult, error)
	DeleteAISISetting(ctx context.Context, input usecase.DeleteAISettingInput) error
	SwitchAISetting(ctx context.Context, input usecase.SwitchAISettingInput) (*usecase.SwitchAISettingResult, error)
	// 新增批量方法
	GetAISettingsBatch(ctx context.Context, input usecase.GetAISettingsBatchInput) (*usecase.GetAISettingsBatchResult, error)
	DeleteAISettingsBatch(ctx context.Context, input usecase.DeleteAISettingsBatchInput) (*usecase.DeleteAISettingsBatchResult, error)
}
```

**Step 2: 运行测试验证编译**

Run: `go build ./...`
Expected: 无错误输出

**Step 3: 提交**

```bash
git add internal/cli/cobra/root.go
git commit -m "feat(cobra): Workflow 接口新增批量方法声明

- GetAISettingsBatch 批量获取配置方法
- DeleteAISettingsBatch 批量删除配置方法"
```

---

## Task 6: CLI 层 setting.go - 修改 get 命令支持多参数

**文件:** `internal/cli/cobra/setting.go`

**Step 1: 修改 getCommand 参数约束**

将第 84 行：
```go
Args:  spcobra.ExactArgs(1),
```
改为：
```go
Args:  spcobra.MinimumNArgs(1),
```

**Step 2: 修改 getCommand RunE 逻辑实现批量处理**

将第 85-95 行：
```go
RunE: func(cmd *spcobra.Command, args []string) error {
    result, err := deps.Workflow.GetAISetting(cmd.Context(), usecase.GetAISettingInput{
        Name: args[0],
    })
    if err != nil {
        return err
    }

    printSettingDetail(cmd.OutOrStdout(), result)
    return nil
},
```
改为：
```go
RunE: func(cmd *spcobra.Command, args []string) error {
    // 判断是否为批量操作
    if len(args) > 1 {
        // 批量获取
        batchResult, err := deps.Workflow.GetAISettingsBatch(cmd.Context(), usecase.GetAISettingsBatchInput{
            Names: args,
        })
        if err != nil {
            return err
        }

        printBatchSettingDetails(cmd.OutOrStdout(), batchResult)
        return nil
    }

    // 单个获取（保持原有逻辑）
    result, err := deps.Workflow.GetAISetting(cmd.Context(), usecase.GetAISettingInput{
        Name: args[0],
    })
    if err != nil {
        return err
    }

    printSettingDetail(cmd.OutOrStdout(), result)
    return nil
},
```

**Step 3: 在文件末尾添加 printBatchSettingDetails 输出函数**

在文件末尾（`printSettingUsage` 函数之后）添加：

```go
// printBatchSettingDetails 输出批量获取配置的结果。
func printBatchSettingDetails(w io.Writer, result *usecase.GetAISettingsBatchResult) {
	for i, item := range result.Items {
		if i > 0 {
			fmt.Fprintln(w, "---")
		}
		fmt.Fprintf(w, "配置: %s\n", item.Name)
		printSettingDetail(w, item)
	}

	// 输出失败汇总
	if len(result.Failed) > 0 {
		fmt.Fprintf(w, "\n获取失败的配置: %s\n", strings.Join(result.Failed, ", "))
	}

	// 输出总汇总
	total := len(result.Items) + len(result.Failed)
	fmt.Fprintf(w, "\n汇总：成功 %d 个，失败 %d 个（共 %d 个）\n", 
		len(result.Items), len(result.Failed), total)
}
```

**Step 4: 运行测试验证编译**

Run: `go build ./...`
Expected: 无错误输出

**Step 5: 提交**

```bash
git add internal/cli/cobra/setting.go
git commit -m "feat(cobra): setting get 命令支持批量获取配置

- Args 约束从 ExactArgs(1) 改为 MinimumNArgs(1)
- 多参数时调用 GetAISettingsBatch 批量方法
- 单参数时保持原有逻辑不变
- 新增 printBatchSettingDetails 函数输出批量结果
- 用 --- 分隔多个配置，最后输出成功/失败汇总"
```

---

## Task 7: CLI 层 setting.go - 修改 delete 命令支持多参数

**文件:** `internal/cli/cobra/setting.go`

**Step 1: 修改 deleteCommand 参数约束**

将第 101 行：
```go
Args:  spcobra.ExactArgs(1),
```
改为：
```go
Args:  spcobra.MinimumNArgs(1),
```

**Step 2: 修改 deleteCommand RunE 逻辑实现批量处理和确认**

将第 102-112 行：
```go
RunE: func(cmd *spcobra.Command, args []string) error {
    if err := deps.Workflow.DeleteAISetting(cmd.Context(), usecase.DeleteAISettingInput{
        Name: args[0],
    }); err != nil {
        return err
    }

    fmt.Fprintf(cmd.OutOrStdout(), "配置已删除: %s\n", args[0])
    return nil
},
```
改为：
```go
RunE: func(cmd *spcobra.Command, args []string) error {
    // 判断是否为批量操作
    if len(args) > 1 {
        // 批量删除：先确认，再执行
        fmt.Fprintln(cmd.OutOrStdout(), "将删除以下配置：")
        for _, name := range args {
            fmt.Fprintf(cmd.OutOrStdout(), "- %s\n", name)
        }

        // 确认删除
        fmt.Fprint(cmd.OutOrStderr(), "确认删除？(y/yes): ")
        var confirm string
        fmt.Scanln(&confirm)
        if confirm != "y" && confirm != "yes" {
            fmt.Fprintln(cmd.OutOrStdout(), "已取消删除")
            return nil
        }

        batchResult, err := deps.Workflow.DeleteAISettingsBatch(cmd.Context(), usecase.DeleteAISettingsBatchInput{
            Names: args,
        })
        if err != nil {
            return err
        }

        printBatchDeleteResult(cmd.OutOrStdout(), batchResult)
        return nil
    }

    // 单个删除（保持原有逻辑）
    if err := deps.Workflow.DeleteAISetting(cmd.Context(), usecase.DeleteAISettingInput{
        Name: args[0],
    }); err != nil {
        return err
    }

    fmt.Fprintf(cmd.OutOrStdout(), "配置已删除: %s\n", args[0])
    return nil
},
```

**Step 3: 在文件末尾添加 printBatchDeleteResult 输出函数**

在 `printBatchSettingDetails` 函数之后添加：

```go
// printBatchDeleteResult 输出批量删除配置的结果。
func printBatchDeleteResult(w io.Writer, result *usecase.DeleteAISettingsBatchResult) {
	if len(result.Deleted) > 0 {
		fmt.Fprintf(w, "已删除: %s\n", strings.Join(result.Deleted, ", "))
	}

	if len(result.Failed) > 0 {
		fmt.Fprintf(w, "删除失败: %s\n", strings.Join(result.Failed, ", "))
	}

	// 输出总汇总
	total := len(result.Deleted) + len(result.Failed)
	fmt.Fprintf(w, "汇总：成功 %d 个，失败 %d 个（共 %d 个）\n", 
		len(result.Deleted), len(result.Failed), total)
}
```

**Step 4: 运行测试验证编译**

Run: `go build ./...`
Expected: 无错误输出

**Step 5: 提交**

```bash
git add internal/cli/cobra/setting.go
git commit -m "feat(cobra): setting delete 命令支持批量删除配置

- Args 约束从 ExactArgs(1) 改为 MinimumNArgs(1)
- 多参数时先显示确认列表，等待用户输入 y/yes 确认
- 确认后调用 DeleteAISettingsBatch 批量方法
- 单参数时保持原有逻辑不变
- 新增 printBatchDeleteResult 函数输出批量结果
- 显示成功/失败汇总"
```

---

## Task 8: 测试验证

**Step 1: 构建二进制文件**

Run: `go build -o bin/ai-sync.exe ./cmd/ai-sync/`
Expected: 编译成功，生成 `bin/ai-sync.exe`

**Step 2: 测试 setting get 单个配置（保持兼容）**

Run: `./bin/ai-sync.exe setting get glm3`
Expected: 显示单个配置详情（原有行为不变）

**Step 3: 测试 setting get 批量配置（全部成功）**

Run: `./bin/ai-sync.exe setting get glm3 glm5`
Expected: 连续显示两个配置，用 `---` 分隔，最后显示汇总

**Step 4: 测试 setting get 批量配置（部分失败）**

Run: `./bin/ai-sync.exe setting get glm3 notexist`
Expected: 显示 glm3 详情，提示 notexist 不存在，显示汇总

**Step 5: 测试 setting delete 单个配置（保持兼容）**

Run: `./bin/ai-sync.exe setting delete glm_test` (输入 `y`)
Expected: 显示确认，删除成功

**Step 6: 测试 setting delete 批量配置（取消确认）**

Run: `./bin/ai-sync.exe setting delete glm3 glm5` (输入 `n`)
Expected: 显示确认列表，输入 `n` 后显示"已取消删除"

**Step 7: 测试 setting delete 批量配置（确认并执行）**

Run: `./bin/ai-sync.exe setting delete glm3 glm5` (输入 `y`)
Expected: 显示确认列表，确认后删除，显示汇总结果

**Step 8: 提交测试更新**

如需新增测试用例，按规范提交。

---

## 实施顺序

按 Task 1 → Task 8 顺序执行，每步完成后提交。

## 相关参考

- 设计文档: `docs/plans/2026-04-07-setting-batch-design.md`
- 现有单条操作实现: `internal/app/usecase/ai_setting.go`
- CLI 层现有实现: `internal/cli/cobra/setting.go`
- Workflow 接口定义: `internal/app/usecase/local_workflow.go`
