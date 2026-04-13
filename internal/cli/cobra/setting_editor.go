package cobra

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	spcobra "github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"flux/internal/app/usecase"
	"flux/internal/tui"
	typesSetting "flux/internal/types/setting"
)

// EditorConfig 编辑器配置。
type EditorConfig struct {
	Name    string // 配置名称
	Content string // 模板内容
	Format  string // 文件格式：yaml 或 json
}

// runSettingEditorMode 运行 TUI 内置编辑器模式。
func runSettingEditorMode(cmd *spcobra.Command, deps Dependencies, name string) error {
	// 读取现有配置
	getResult, err := deps.Workflow.GetAISetting(cmd.Context(), usecase.GetAISettingInput{Name: name})
	if err != nil {
		return err
	}

	// 使用 TUI 编辑器
	editor := tui.NewSettingEditor(os.Stdin, os.Stdout)
	return editor.Run(context.Background(), getResult, func(input *usecase.EditAISettingInput) error {
		result, err := deps.Workflow.EditAISetting(cmd.Context(), *input)
		if err != nil {
			return err
		}
		printEditResult(cmd.OutOrStdout(), result)
		return nil
	})
}

// runEditorMode 运行外部编辑器模式（已弃用，保留向后兼容）。
func runEditorMode(cmd *spcobra.Command, deps Dependencies, name, format string) error {
	// 读取现有配置
	getResult, err := deps.Workflow.GetAISetting(cmd.Context(), usecase.GetAISettingInput{Name: name})
	if err != nil {
		return err
	}

	// 确定格式
	if format == "" {
		format = "yaml"
	}
	if format != "yaml" && format != "json" {
		return fmt.Errorf("不支持的文件格式: %s（仅支持 yaml 或 json）", format)
	}

	// 生成临时文件
	// 使用用户主目录而非系统临时目录，避免 notepad 另存为问题
	homeDir, _ := os.UserHomeDir()
	tempDir := filepath.Join(homeDir, ".ai-sync-temp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		// 降级到系统临时目录
		tempDir = os.TempDir()
	}

	// 使用固定文件名而非随机名称，让 notepad 认为这是正常文件
	ext := "." + format
	tempFileName := "ai-sync-edit-" + name + ext
	tempPath := filepath.Join(tempDir, tempFileName)

	// 删除旧文件（如果存在）
	os.Remove(tempPath)
	defer os.Remove(tempPath)

	// 写入模板内容
	template := generateEditorTemplate(getResult, format)
	if err := os.WriteFile(tempPath, []byte(template), 0644); err != nil {
		return fmt.Errorf("写入临时文件失败: %w", err)
	}

	// 启动编辑器
	editorName, editorArgs := getEditorCommand()
	cmdArgs := append(editorArgs, tempPath)
	execCmd := exec.Command(editorName, cmdArgs...)

	if err := execCmd.Run(); err != nil {
		return fmt.Errorf("启动编辑器失败: %w", err)
	}

	// 短暂等待确保编辑器完全释放文件句柄
	// 为什么：Windows GUI 编辑器可能需要时间刷新文件缓冲区
	time.Sleep(100 * time.Millisecond)

	// 读取编辑后的内容
	editedContent, err := os.ReadFile(tempPath)
	if err != nil {
		return fmt.Errorf("读取编辑后文件失败: %w", err)
	}

	// 检查文件是否被修改（通过文件大小或内容）
	if len(editedContent) == 0 {
		return fmt.Errorf("编辑后文件为空，请确保保存了修改")
	}

	// 解析编辑后的内容
	editInput, err := parseEditedContent(name, string(editedContent), format)
	if err != nil {
		return fmt.Errorf("解析配置文件失败: %w\n临时文件已保留在: %s", err, tempPath)
	}

	// 执行更新
	result, err := deps.Workflow.EditAISetting(cmd.Context(), *editInput)
	if err != nil {
		return err
	}

	printEditResult(cmd.OutOrStdout(), result)
	return nil
}

// generateEditorTemplate 生成编辑器模板。
// 使用 <unchanged> 占位符表示保持原值，允许用户清空字段。
func generateEditorTemplate(setting *usecase.GetAISettingResult, format string) string {
	template := typesSetting.EditorTemplate{
		Name:        setting.Name,
		Token:       "<unchanged>", // 占位符表示保持原值
		BaseURL:     setting.BaseURL,
		OpusModel:   setting.OpusModel,
		SonnetModel: setting.SonnetModel,
	}

	var content string

	if format == "yaml" {
		// YAML 格式带注释
		lines := []string{
			"# AI 配置编辑",
			"#",
			"# 操作说明：",
			"# 1. 按 Ctrl+S 直接保存（不要\"另存为\"）",
			"# 2. 关闭窗口后自动应用变更",
			"#",
			"# 字段说明：",
			"# - <unchanged> = 保持原值",
			"# - 留空（空字符串）= 清空该字段",
			"# - 其他值 = 更新为新值",
			"#",
			"",
		}
		data, _ := yaml.Marshal(template)
		lines = append(lines, string(data))
		content = strings.Join(lines, "\n")
	} else {
		// JSON 格式
		data, _ := json.MarshalIndent(template, "", "  ")
		content = string(data)
	}

	return content
}

// parseEditedContent 解析编辑后的内容。
// <unchanged> 占位符会被转换为空字符串，由 EditAISetting 识别为保持原值。
func parseEditedContent(originalName, content, format string) (*usecase.EditAISettingInput, error) {
	var template typesSetting.EditorTemplate
	var err error

	if format == "yaml" {
		err = yaml.Unmarshal([]byte(content), &template)
	} else {
		err = json.Unmarshal([]byte(content), &template)
	}

	if err != nil {
		return nil, fmt.Errorf("解析文件失败: %w", err)
	}

	// <unchanged> 占位符转换为空字符串，表示保持原值
	if template.Token == "<unchanged>" {
		template.Token = ""
	}

	input := &usecase.EditAISettingInput{
		Name:        originalName,
		NewName:     template.Name,
		Token:       template.Token,
		BaseURL:     template.BaseURL,
		OpusModel:   template.OpusModel,
		SonnetModel: template.SonnetModel,
	}

	return input, nil
}

// getEditorCommand 获取编辑器命令及参数。
// 返回命令名和参数列表，Windows notepad 使用 start /wait 等待关闭。
func getEditorCommand() (string, []string) {
	editor := os.Getenv("EDITOR")
	if editor != "" {
		return editor, nil
	}

	if runtime.GOOS == "windows" {
		// 使用 cmd /c start /wait 等待 notepad 关闭
		return "cmd", []string{"/c", "start", "/wait", "notepad"}
	}
	return "vim", nil
}
