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
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2) // 与 JSON 保持一致的缩进
	return encoder.Encode(data)
}
