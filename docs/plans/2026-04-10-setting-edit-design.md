# Setting Edit 功能设计文档

> **创建日期**：2026-04-10
> **功能**：为 `ai-sync setting` 命令添加编辑修改功能

---

## 需求背景

当前 setting 命令只支持 `create`、`list`、`get`、`delete`、`switch`，用户填错配置只能删除重建，体验不佳。

---

## 功能概述

添加 `ai-sync setting edit <name>` 命令，支持两种交互模式：

1. **命令行参数模式**：通过 flags 指定要修改的字段
2. **编辑器模式**：导出配置文件到编辑器修改

---

## 命令结构

### 命令行参数模式

```bash
# 基本用法
ai-sync setting edit <name> [flags]

# 示例：只修改 Token
ai-sync setting edit my-config -t new-token

# 示例：修改多个字段
ai-sync setting edit my-config -t new-token -a https://api.new.com

# 示例：修改名称
ai-sync setting edit old-name -n new-name
```

**Flags**（全部可选，未指定的字段保持原值）：

| Flag | 简写 | 说明 |
|------|------|------|
| `--name` | `-n` | 新名称 |
| `--token` | `-t` | 新 Token |
| `--api` | `-a` | 新 Base URL |
| `--opus-model` | `-o` | 新 Opus 模型 |
| `--sonnet-model` | `-s` | 新 Sonnet 模型 |

### 编辑器模式

```bash
# 使用默认格式（YAML）
ai-sync setting edit <name> -e

# 指定格式
ai-sync setting edit <name> -e -f json
ai-sync setting edit <name> -e --format yaml
```

**Flags**：

| Flag | 简写 | 说明 |
|------|------|------|
| `--editor` | `-e` | 使用编辑器模式 |
| `--format` | `-f` | 文件格式：`yaml`（默认）或 `json` |

---

## 架构设计

### 分层调用链

```
CLI 层 (setting_edit.go)
  ↓ 解析参数/flags
UseCase 层 (local_workflow.go)
  ↓ 流程编排 + Name 冲突检测
Service 层 (setting/service.go)
  ↓ 业务逻辑 + convert
DAO 层 (ai_setting.go)
  ↓ 数据库更新
Types 层 (types/setting/)
  ↓ 响应结构体
输出
```

### 新增文件

| 文件路径 | 职责 |
|----------|------|
| `internal/cli/cobra/setting_edit.go` | edit 命令定义 |
| `internal/service/setting/service.go` | Setting 业务逻辑层 |
| `internal/types/setting/edit.go` | 编辑相关响应结构体 |

### 修改文件

| 文件路径 | 修改内容 |
|----------|----------|
| `internal/cli/cobra/setting.go` | 添加 edit 子命令 |
| `internal/models/ai_setting.go` | 添加 `UpdateByName` 方法 |
| `internal/app/usecase/local_workflow.go` | 添加 `EditAISetting` 方法 |

---

## 核心逻辑

### 命令行参数模式流程

```
1. 解析参数，获取 <name> 和 flags
2. 读取当前配置（DAO.GetByName）
3. 如果未找到 → 报错
4. 如果指定了 --name：
   - 检查新 name 是否存在
   - 若存在且不是当前配置 → 报错
   - 若是当前配置 → 允许（只是改名）
5. 构建更新数据：
   - 未指定的字段保持原值
   - Token 若未指定则保持原值（敏感字段不回显）
6. 如果是当前生效配置 → 输出提示
7. 执行更新（DAO.UpdateByName）
8. 输出成功信息
```

### Name 冲突检测规则

```go
if newName != oldName {
    existing := dao.GetByName(newName)
    if existing != nil {
        return error("配置名称已存在: %s", newName)
    }
}
```

### Token 处理规则

- **命令行模式**：未指定 `-t`/`--token` 则保持原值
- **编辑器模式**：临时文件中 Token 字段留空，解析时：
  - 若字段为空 → 保持原值
  - 若字段有值 → 使用新值

---

## 编辑器模式

### 流程

```
1. 读取当前配置
2. 生成临时文件路径（/tmp/ai-sync-edit-<name>.<ext>）
3. 根据格式生成配置内容：
   - Token 字段留空 + 注释说明
   - 其他字段预填当前值
4. 调用 $EDITOR 打开临时文件
5. 等待用户保存退出
6. 解析修改后的文件
7. 验证变更（Name 冲突检测等）
8. 应用更新
9. 删除临时文件
```

### YAML 模板

```yaml
# AI 配置编辑
# 留空字段将保持原值

name: my-config
token: ""           # 留空保持原值（敏感信息不显示）
base_url: https://api.anthropic.com
opus_model: claude-opus-4-6
sonnet_model: claude-sonnet-4-6
```

### JSON 模板

```json
{
  "name": "my-config",
  "token": "",
  "base_url": "https://api.anthropic.com",
  "opus_model": "claude-opus-4-6",
  "sonnet_model": "claude-sonnet-4-6"
}
```

### 编辑器检测

```go
editor := os.Getenv("EDITOR")
if editor == "" {
    if runtime.GOOS == "windows" {
        editor = "notepad"
    } else {
        editor = "vim"
    }
}
```

---

## 错误处理

| 场景 | 处理方式 |
|------|----------|
| 配置不存在 | `配置未找到: <name>` |
| Name 已存在 | `配置名称已存在: <newName>` |
| 编辑器启动失败 | `无法启动编辑器: <err>`，回退到命令行模式提示 |
| 临时文件解析失败 | `配置文件格式错误: <err>`，保留临时文件供检查 |
| 用户取消编辑（空文件）| `检测到无变更，已取消编辑` |
| 更新失败 | `更新配置失败: <err>` |

### 输出格式

**成功输出**：
```
配置已更新: my-config

变更内容：
- Token: ****（已修改）
- Base URL: https://api.new.com（已修改）
- Opus 模型: claude-opus-4-6（保持原值）
```

**当前配置提示**：
```
注意：这是当前生效的配置，修改后可能需要重新执行 switch 才能生效。

配置已更新: my-config
```

### 临时文件清理

- 成功应用 → 删除临时文件
- 解析失败 → **保留**临时文件，输出路径供调试
- 用户中断（Ctrl+C）→ 删除临时文件

---

## 可修改字段

全部字段可修改：`Name`、`Token`、`BaseURL`、`OpusModel`、`SonnetModel`

**特殊处理**：
- **Name**：唯一索引，修改需检测冲突
- **Token**：敏感信息，不回显，留空保持原值

---

## 核心方法签名

```go
// DAO 层
func (dao *AISettingDAO) UpdateByName(name string, setting *AISetting) error

// Service 层
func (s *Service) Edit(ctx context.Context, name string, input EditInput) (*EditOutput, error)

// UseCase 层
func (w *LocalWorkflow) EditAISetting(ctx context.Context, input EditAISettingInput) (*EditAISettingResult, error)
```
