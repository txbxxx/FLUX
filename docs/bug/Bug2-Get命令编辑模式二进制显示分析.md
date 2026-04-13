# Bug #2 分析报告: get -e 后编辑文件显示为二进制

## Bug 信息
- **状态**: 🔴 未解决
- **优先级**: P0
- **终端**: Windows
- **修复版本**: V1.1

## 问题描述
执行 `ai-sync get claude-global settings.json -e` 后，编辑的文件内容会以异常方式显示，用户描述为"二进制"显示。

## 根本原因分析

### 涉及代码
1. `internal/cli/cobra/get.go` - 命令入口
2. `internal/tui/editor.go` - 编辑器启动
3. `internal/tui/editor_model.go` - 编辑器模型

### 代码流程

**命令入口（get.go 第 36-47 行）**：
```go
if edit {
    if deps.Editor == nil {
        return errors.New("编辑器未就绪，无法进入编辑模式")
    }
    return deps.Editor.Run(cmd.Context(), result, func(content string) error {
        return deps.Workflow.SaveConfig(cmd.Context(), usecase.SaveConfigInput{
            Tool:     args[0],
            Path:     path,
            Content:  content,
            Snapshot: snapshot,
        })
    })
}
```

**文件读取（config_browser.go 第 136-143 行）**：
```go
content, err := w.accessor.ReadFile(target)
if err != nil {
    return nil, &UserError{
        Message:    "无法读取文件",
        Suggestion: "请检查文件内容和访问权限后重试",
        Err:        err,
    }
}
```

**编辑器初始化（editor_model.go 第 101-103 行）**：
```go
func newEditorModel(result *usecase.GetConfigResult, save func(string) error) *EditorModel {
    buffer := newEditorBuffer(result.Content)  // 使用文件内容初始化缓冲区
    // ...
}
```

**缓冲区初始化（editor_model.go 第 48-59 行）**：
```go
func (b *editorBuffer) SetValue(content string) {
    b.lineEnding = "\n"
    if strings.Contains(content, "\r\n") {
        b.lineEnding = "\r\n"
    }
    normalized := strings.ReplaceAll(content, "\r\n", "\n")
    b.lines = strings.Split(normalized, "\n")  // 按行分割
    if len(b.lines) == 0 {
        b.lines = []string{""}
    }
}
```

## 可能原因

### 原因 1: 特殊字符渲染问题

JSON 文件可能包含：
- Unicode 控制字符
- 零宽字符
- 非 UTF-8 字节序列
- 终端控制序列

这些字符在 Bubbletea 的终端渲染中可能导致显示异常。

### 原因 2: 行尾处理问题

Windows 系统默认使用 `\r\n` 作为行尾符，而代码中的处理可能导致：
- 行尾符显示为 `^M` 或其他控制字符
- 换行异常导致行重叠或错位

### 原因 3: 终端编码问题

Windows 终端的编码设置可能不是 UTF-8，导致：
- 中文字符显示为乱码
- 特殊符号显示异常

## 修复方案

### 方案 1: 添加内容清理（推荐）

在读取文件后、传递给编辑器前，清理特殊字符：

```go
// 在 config_browser.go 的 GetConfig 中
content, err := w.accessor.ReadFile(target)
if err != nil {
    return nil, &UserError{...}
}

// 清理内容：移除控制字符、规范化换行符
content = sanitizeContent(content)

result.Kind = ConfigTargetFile
result.Content = content
// ...
```

```go
// 添加辅助函数
func sanitizeContent(content string) string {
    // 移除 ASCII 控制字符（保留 \t, \n, \r）
    var sb strings.Builder
    for _, r := range content {
        if r == '\t' || r == '\n' || r == '\r' || r >= 32 {
            sb.WriteRune(r)
        }
    }
    return sb.String()
}
```

### 方案 2: 增强编辑器渲染

在 `editor_model.go` 的 `renderBuffer` 中：
- 检测并跳过不可打印字符
- 为控制字符提供占位显示
- 确保行尾正确处理

### 方案 3: 添加文件编码检测

在 `accessor.ReadFile` 中：
- 检测文件编码（UTF-8, UTF-16, GBK 等）
- 统一转换为 UTF-8
- 处理 BOM（Byte Order Mark）

## 需要进一步调查

1. **获取实际文件内容**：需要查看用户编辑的 `settings.json` 文件内容
2. **终端环境信息**：确认用户的终端类型（PowerShell, CMD, Windows Terminal）
3. **具体显示效果**：需要截图或详细描述"二进制"显示的具体表现

## 临时解决方案

如果用户遇到此问题，可以使用以下方式绕过：
1. 不使用 `-e` 参数，直接查看文件内容
2. 使用外部编辑器直接编辑配置文件
3. 使用 `setting edit` 命令编辑配置（而非直接编辑 JSON）
