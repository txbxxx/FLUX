package cobra

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	spcobra "github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"ai-sync-manager/internal/app/usecase"
	typesSetting "ai-sync-manager/internal/types/setting"
)

// EditorConfig 编辑器配置。
type EditorConfig struct {
	Name    string // 配置名称
	Content string // 模板内容
	Format  string // 文件格式：yaml 或 json
}

// runEditorMode 运行编辑器模式。
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
	tempDir := os.TempDir()
	ext := "." + format
	tempFile, err := os.CreateTemp(tempDir, "ai-sync-edit-*"+ext)
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	// 写入模板内容
	template := generateEditorTemplate(getResult, format)
	if err := os.WriteFile(tempPath, []byte(template), 0644); err != nil {
		return fmt.Errorf("写入临时文件失败: %w", err)
	}

	// 启动编辑器
	editor := getEditor()
	editorCmd := exec.Command(editor, tempPath)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr

	if err := editorCmd.Run(); err != nil {
		return fmt.Errorf("启动编辑器失败: %w", err)
	}

	// 读取编辑后的内容
	editedContent, err := os.ReadFile(tempPath)
	if err != nil {
		return fmt.Errorf("读取编辑后文件失败: %w", err)
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
func generateEditorTemplate(setting *usecase.GetAISettingResult, format string) string {
	template := typesSetting.EditorTemplate{
		Name:        setting.Name,
		Token:       "", // 留空表示保持原值
		BaseURL:     setting.BaseURL,
		OpusModel:   setting.OpusModel,
		SonnetModel: setting.SonnetModel,
	}

	var content string

	if format == "yaml" {
		// YAML 格式带注释
		lines := []string{
			"# AI 配置编辑",
			"# 留空字段将保持原值（敏感信息不显示）",
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

// getEditor 获取编辑器命令。
func getEditor() string {
	editor := os.Getenv("EDITOR")
	if editor != "" {
		return editor
	}

	if runtime.GOOS == "windows" {
		return "notepad"
	}
	return "vim"
}
