# Bug #1 分析报告: setting edit -e 后 Token 变成加密显示格式

## Bug 信息
- **状态**: 🔴 未解决
- **优先级**: P0
- **终端**: Windows
- **修复版本**: V1.1

## 问题描述
使用 `fl setting edit GLM_lite -e` 编辑配置后，数据库中保存的 token 值变成了加密显示格式（如 `4e5b****QEXh`），导致实际使用时 token 无效。

## 根本原因分析

### 问题代码位置
`internal/tui/setting_editor_model.go`

### 关键代码段

**初始化时（第 48-59 行）**：
```go
values := []string{
    original.Name,
    maskTokenForEditor(original.Token),  // ❌ 问题：Token被脱敏后作为初始值
    original.BaseURL,
    original.OpusModel,
    original.SonnetModel,
}
```

**保存时（第 217-227 行）**：
```go
func (m *SettingEditorModel) saveChanges() (tea.Model, tea.Cmd) {
    input := &usecase.EditAISettingInput{
        Name:        m.original.Name,
        NewName:     (*m.inputs[0]).Value(),
        Token:       (*m.inputs[1]).Value(),  // ❌ 问题：直接使用输入框的值（可能是脱敏值）
        BaseURL:     (*m.inputs[2]).Value(),
        OpusModel:   (*m.inputs[3]).Value(),
        SonnetModel: (*m.inputs[4]).Value(),
    }
    // ...
}
```

**脱敏函数（第 240-249 行）**：
```go
func maskTokenForEditor(token string) string {
    if token == "" {
        return "<unchanged>"
    }
    if len(token) > 8 {
        return token[:4] + "****" + token[len(token)-4:]  // 输出格式：前4位 + **** + 后4位
    }
    return "****"
}
```

### 问题流程

1. **读取阶段**: `GetAISetting` 返回完整的 token（如 `4e5b...valid...QEXh`）
2. **显示阶段**: `maskTokenForEditor` 将 token 转换为脱敏格式（`4e5b****QEXh`）并显示
3. **编辑阶段**: 用户如果没有修改 Token 字段，输入框中保持脱敏值
4. **保存阶段**: `saveChanges` 直接读取输入框值，将脱敏值保存到数据库

## 修复方案

### 方案 1: 保存时检测并还原（推荐）

在 `saveChanges` 函数中添加逻辑：当 Token 值与脱敏格式匹配时，使用原始完整 token。

```go
func (m *SettingEditorModel) saveChanges() (tea.Model, tea.Cmd) {
    tokenValue := (*m.inputs[1]).Value()

    // 如果 token 值与脱敏显示值匹配，说明用户没有修改，使用原始值
    if tokenValue == maskTokenForEditor(m.original.Token) || tokenValue == "<unchanged>" {
        tokenValue = m.original.Token
    }

    input := &usecase.EditAISettingInput{
        Name:        m.original.Name,
        NewName:     (*m.inputs[0]).Value(),
        Token:       tokenValue,  // ✅ 使用完整 token
        BaseURL:     (*m.inputs[2]).Value(),
        OpusModel:   (*m.inputs[3]).Value(),
        SonnetModel: (*m.inputs[4]).Value(),
    }
    // ...
}
```

## 影响范围
- 所有使用 `setting edit -e` 编辑配置的场景
- 可能导致已保存的配置 token 失效
- 影响用户对 Claude API 的正常访问

## 验证方法
1. 创建测试配置
2. 使用 `setting edit -e` 编辑（不修改 token）
3. 保存后查询数据库验证 token 完整性
4. 使用 `setting switch` 切换配置验证功能正常
