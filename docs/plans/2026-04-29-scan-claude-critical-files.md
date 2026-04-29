# 修复 Scan/Snapshot 遗漏 Claude Code 关键文件

## 背景

当前规则引擎遗漏了两个 Claude Code 关键路径：

1. **`~/.claude.json`**：位于 `~/.claude/` 目录外，包含 MCP Servers 等运行时配置。`resolveRuleDefinitions` 使用 `filepath.Join(basePath, rule.Path)` 拼接路径，无法解析目录外的文件
2. **`~/.claude/rules/`**：用户级规则目录，跨所有项目生效的个人编码规范，未在默认规则中定义

导致 `scan claude` 和 `snapshot create -p claude-global` 无法收集这些配置文件。

## 目标

1. `SyncRuleDefinition` 支持绝对路径规则（`IsAbsolute` 字段），使 `~/.claude.json` 可被 scan/snapshot 收集
2. 补充 `~/.claude/rules/` 到默认规则，使该目录可被 scan/snapshot 收集
3. 不影响现有功能和向后兼容
4. 配置文件 `default.yaml` 和硬编码回退同步更新

## 非目标（明确不做什么）

- 不处理 `~/.claude/projects/*/memory/`（需要动态通配符，复杂度高）
- 不修改 collector 的文件收集逻辑
- 不修改 sync push/pull 的打包逻辑（新文件走现有收集流程即可）
- 不修改自定义规则（`CustomSyncRule`）系统，它已有独立的绝对路径处理

## 改动清单

### 第 1 步：`SyncRuleDefinition` 新增 `IsAbsolute` 字段

**文件**: `internal/service/tool/rule_definitions.go`

```go
type SyncRuleDefinition struct {
    ToolType   ToolType
    Scope      ConfigScope
    Path       string         // 相对路径 或 绝对路径（~ 或 / 开头时 IsAbsolute=true）
    Category   ConfigCategory
    IsDir      bool
    IsAbsolute bool           // 新增：true 时不与 basePath 拼接，直接展开路径
}
```

### 第 2 步：`RuleDef` 配置结构体新增 `IsAbsolute` 字段

**文件**: `pkg/config/config.go`

```go
type RuleDef struct {
    Path       string `yaml:"path"`
    Category   string `yaml:"category"`
    IsDir      bool   `yaml:"is_dir"`
    IsAbsolute bool   `yaml:"is_absolute"`  // 新增
}
```

### 第 3 步：`default.yaml` 新增 2 条规则

**文件**: `pkg/config/default.yaml`

在 `claude.global_rules` 末尾追加：

```yaml
      - path: "rules"
        category: "rules"
        is_dir: true
      - path: "~/.claude.json"
        category: "config"
        is_dir: false
        is_absolute: true
```

### 第 4 步：硬编码回退新增 2 条规则

**文件**: `internal/service/tool/rule_definitions.go`

`DefaultGlobalRules` 的 `ToolTypeClaude` 分支末尾追加：

```go
{ToolType: toolType, Scope: ScopeGlobal, Path: "rules", Category: CategoryRules, IsDir: true},
{ToolType: toolType, Scope: ScopeGlobal, Path: "~/.claude.json", Category: CategoryConfigFile, IsDir: false, IsAbsolute: true},
```

### 第 5 步：`convertRules` 传递 `IsAbsolute`

**文件**: `internal/service/tool/rule_definitions.go`

```go
func convertRules(rules []config.RuleDef, toolType ToolType, scope ConfigScope) []SyncRuleDefinition {
    result := make([]SyncRuleDefinition, len(rules))
    for i, r := range rules {
        result[i] = SyncRuleDefinition{
            ToolType:   toolType,
            Scope:      scope,
            Path:       r.Path,
            Category:   ConfigCategory(r.Category),
            IsDir:      r.IsDir,
            IsAbsolute: r.IsAbsolute,  // 新增
        }
    }
    return result
}
```

### 第 6 步：`resolveRuleDefinitions` 支持绝对路径

**文件**: `internal/service/tool/rule_resolver.go`

修改路径解析逻辑：

```go
func resolveRuleDefinitions(basePath string, rules []SyncRuleDefinition) ([]ResolvedRuleMatch, error) {
    matches := make([]ResolvedRuleMatch, 0, len(rules))
    for _, rule := range rules {
        var absolutePath string
        if rule.IsAbsolute {
            absolutePath = expandPath(rule.Path)
        } else {
            absolutePath = filepath.Join(basePath, rule.Path)
        }
        info, err := os.Stat(absolutePath)
        if err != nil {
            if os.IsNotExist(err) {
                continue
            }
            return nil, err
        }
        if rule.IsDir != info.IsDir() {
            continue
        }
        matches = append(matches, ResolvedRuleMatch{
            ToolType:     rule.ToolType,
            Scope:        rule.Scope,
            RelativePath: rule.Path,
            AbsolutePath: absolutePath,
            Category:     rule.Category,
            IsDir:        rule.IsDir,
            Size:         info.Size(),
            ModifiedAt:   info.ModTime(),
        })
    }
    return matches, nil
}
```

新增 `expandPath` 工具函数：

```go
// expandPath 将 ~ 开头的路径展开为真实用户目录路径。
func expandPath(path string) string {
    if strings.HasPrefix(path, "~/") {
        home, err := os.UserHomeDir()
        if err != nil {
            return path
        }
        return filepath.Join(home, path[2:])
    }
    return path
}
```

### 第 7 步：单元测试

**文件**: `internal/service/tool/rule_resolver_test.go`（新增或追加）

- `TestResolveRuleDefinitions_AbsolutePath`：验证 `~/.claude.json` 绝对路径规则正确解析
- `TestResolveRuleDefinitions_RulesDirectory`：验证 `rules` 目录规则正确解析
- `TestExpandPath`：验证 `~` 展开、非 `~` 路径不变
- `TestResolveRuleDefinitions_MixedRules`：验证混合相对/绝对路径规则

## 涉及文件汇总

| 文件 | 改动 |
|------|------|
| `pkg/config/config.go` | `RuleDef` 新增 `IsAbsolute` 字段 |
| `pkg/config/default.yaml` | claude.global_rules 新增 2 条规则 |
| `internal/service/tool/rule_definitions.go` | `SyncRuleDefinition` 新增 `IsAbsolute`；`DefaultGlobalRules` 新增 2 条规则；`convertRules` 传递新字段 |
| `internal/service/tool/rule_resolver.go` | `resolveRuleDefinitions` 绝对路径分支 + 新增 `expandPath` |
| `internal/service/tool/rule_resolver_test.go` | 新增单元测试 |

## 验收标准

| # | 验收条件 | 验证方式 |
|---|----------|----------|
| 1 | `fl scan claude --verbose` 输出包含 `~/.claude.json` | 手动验证 |
| 2 | `fl scan claude --verbose` 输出包含 `~/.claude/rules/` 目录 | 手动验证 |
| 3 | `~/.claude.json` 不存在时不报错，正常跳过 | 删除文件后执行 scan |
| 4 | `~/.claude/rules/` 不存在时不报错，正常跳过 | 删除目录后执行 scan |
| 5 | `fl snapshot create -p claude-global` 快照包含上述文件 | 创建快照后检查文件列表 |
| 6 | 现有规则不受影响（`IsAbsolute` 默认 false） | `go test ./...` |
| 7 | 所有单元测试通过 | `go test ./internal/service/tool/... -v` |

## 向后兼容

- `IsAbsolute` 默认值 `false`，现有规则零影响
- 文件/目录不存在时走 `os.IsNotExist` 跳过，行为不变
- 已有快照不会自动包含新增文件，需新建快照

## 实施状态

**状态：已完成** | 提交 `1d94a70` | 分支 `fix/scan-missing-claude-files`

所有 7 步改动已按计划完成，无偏差：
1. `SyncRuleDefinition.IsAbsolute` — 已添加
2. `RuleDef.IsAbsolute` — 已添加
3. `default.yaml` 新增 2 条规则 — 已添加
4. 硬编码回退新增 2 条规则 — 已添加
5. `convertRules` 传递 `IsAbsolute` — 已完成
6. `resolveRuleDefinitions` 绝对路径分支 + `expandPath` — 已完成
7. 单元测试 5 个新用例 — 全部通过

集成验证：`snapshot create -p claude-global` 收集 978 文件，数据库确认 `~/.claude.json` 已包含在内。
