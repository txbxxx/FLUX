package tui

import "strings"

func (m *Model) View() string {
	if m.Quitting {
		return ""
	}

	switch m.Page {
	case PageScan:
		return renderScanPage(m)
	case PageCreate:
		return renderCreatePage(m)
	case PageSnapshots:
		return renderSnapshotsPage(m)
	default:
		return renderHomePage(m)
	}
}

func renderHomePage(m *Model) string {
	var builder strings.Builder
	builder.WriteString("AI Sync Manager\n")
	builder.WriteString("数据目录: " + m.DataDir + "\n\n")

	if m.ErrorMessage != "" {
		builder.WriteString("错误: " + m.ErrorMessage + "\n")
	}
	if m.StatusMessage != "" {
		builder.WriteString("状态: " + m.StatusMessage + "\n")
	}
	if m.ErrorMessage != "" || m.StatusMessage != "" {
		builder.WriteString("\n")
	}

	for index, item := range homeItems {
		cursor := " "
		if index == m.Cursor {
			cursor = ">"
		}
		builder.WriteString(cursor + " " + item + "\n")
	}

	builder.WriteString("\n" + m.Help.View(m.Keys) + "\n")
	return builder.String()
}

func renderCreatePage(m *Model) string {
	var builder strings.Builder
	builder.WriteString("创建快照\n")
	builder.WriteString("按 Tab 切换字段 / Enter 进入下一项 / Esc 返回首页\n\n")

	if m.ErrorMessage != "" {
		builder.WriteString("错误: " + m.ErrorMessage + "\n\n")
	}

	builder.WriteString(renderField("tools", m.Form.Tools, m.FormFocus == formFocusTools, "codex,claude"))
	builder.WriteString(renderField("message", m.Form.Message, m.FormFocus == formFocusMessage, "本次快照说明"))
	builder.WriteString(renderField("name", m.Form.Name, m.FormFocus == formFocusName, "可选快照名称"))
	builder.WriteString(renderField("project-path", m.Form.ProjectPath, m.FormFocus == formFocusProjectPath, "可选项目路径"))

	submitPrefix := " "
	if m.FormFocus == formFocusSubmit {
		submitPrefix = ">"
	}
	builder.WriteString(submitPrefix + " 提交创建\n")
	builder.WriteString("\n" + m.Help.View(m.Keys) + "\n")

	return builder.String()
}

func renderField(label, value string, focused bool, placeholder string) string {
	prefix := " "
	if focused {
		prefix = ">"
	}
	if value == "" {
		value = placeholder
	}
	return prefix + " " + label + ": " + value + "\n"
}
