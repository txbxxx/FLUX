package setting

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const (
	MaxModels       = 6 // 单个配置最多存储的模型数
	MaxSwitchModels = 3 // switch 时最多激活的模型数
)

// ModelList 存储模型名称列表，支持 JSON 序列化和数据库持久化。
type ModelList []string

// envRoleKeys 定义模型索引到 Claude Code 环境变量的映射。
var envRoleKeys = [MaxSwitchModels]string{
	"ANTHROPIC_DEFAULT_OPUS_MODEL",
	"ANTHROPIC_DEFAULT_SONNET_MODEL",
	"ANTHROPIC_DEFAULT_HAIKU_MODEL",
}

// NewModelListFromInput 解析 CLI 输入为 ModelList。
// inputs 来自 StringSlice flag，每项可能包含逗号或空格分隔的多个模型名。
func NewModelListFromInput(inputs []string) (ModelList, error) {
	var all []string
	for _, input := range inputs {
		// 先按逗号分割
		for _, part := range strings.Split(input, ",") {
			// 再按空格分割
			for _, sub := range strings.Fields(strings.TrimSpace(part)) {
				trimmed := strings.TrimSpace(sub)
				if trimmed != "" {
					all = append(all, trimmed)
				}
			}
		}
	}

	if len(all) == 0 {
		return nil, fmt.Errorf("模型不能为空")
	}
	if len(all) > MaxModels {
		return nil, fmt.Errorf("最多支持 %d 个模型，当前 %d 个", MaxModels, len(all))
	}

	// 去重（保留顺序）
	seen := make(map[string]bool)
	result := make(ModelList, 0, len(all))
	for _, m := range all {
		if !seen[m] {
			seen[m] = true
			result = append(result, m)
		}
	}

	return result, nil
}

// NewSwitchModelListFromInput 解析 switch --model 输入，最多 3 个。
func NewSwitchModelListFromInput(inputs []string) (ModelList, error) {
	list, err := NewModelListFromInput(inputs)
	if err != nil {
		return nil, err
	}
	if len(list) > MaxSwitchModels {
		return nil, fmt.Errorf("最多指定 %d 个模型（opus/sonnet/haiku）", MaxSwitchModels)
	}
	return list, nil
}

var contextSuffixRe = regexp.MustCompile(`^(.+?)\[([^\]]+)\]$`)

// ParseModelContext 从模型名中提取上下文窗口后缀。
// "glm-5.1[1m]" → ("glm-5.1", "1m")
// "glm-5.1" → ("glm-5.1", "")
func ParseModelContext(model string) (name, context string) {
	m := contextSuffixRe.FindStringSubmatch(model)
	if m == nil {
		return model, ""
	}
	return m[1], m[2]
}

// FormatModelContext 将上下文后缀格式化为友好展示。
// "1m" → "1M", "200k" → "200K", "" → ""
func FormatModelContext(ctx string) string {
	if ctx == "" {
		return ""
	}
	return strings.ToUpper(ctx)
}

// ToEnvVars 按索引将模型映射到 Claude Code 环境变量。
// 最多映射前 3 个到 opus/sonnet/haiku。
func (m ModelList) ToEnvVars() map[string]string {
	env := make(map[string]string)
	n := len(m)
	if n > MaxSwitchModels {
		n = MaxSwitchModels
	}
	for i := 0; i < n; i++ {
		if m[i] != "" {
			env[envRoleKeys[i]] = m[i]
		}
	}
	return env
}

// ModelListFromOldFields 从旧的 OpusModel+SonnetModel 字段构建 ModelList（迁移用）。
func ModelListFromOldFields(opus, sonnet string) ModelList {
	var list ModelList
	if opus != "" {
		list = append(list, opus)
	}
	if sonnet != "" {
		list = append(list, sonnet)
	}
	if list == nil {
		list = ModelList{}
	}
	return list
}

// String 返回模型列表的字符串表示。
func (m ModelList) String() string {
	data, _ := json.Marshal([]string(m))
	return string(data)
}

// MarshalJSON 实现 json.Marshaler。
func (m ModelList) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]string(m))
}

// UnmarshalJSON 实现 json.Unmarshaler。
func (m *ModelList) UnmarshalJSON(data []byte) error {
	var list []string
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}
	*m = list
	return nil
}

// Value 实现 driver.Valuer，用于 GORM 写入数据库。
func (m ModelList) Value() (driver.Value, error) {
	if m == nil {
		return "[]", nil
	}
	data, err := json.Marshal([]string(m))
	if err != nil {
		return nil, fmt.Errorf("序列化模型列表失败: %w", err)
	}
	return string(data), nil
}

// Scan 实现 sql.Scanner，用于 GORM 从数据库读取。
func (m *ModelList) Scan(value interface{}) error {
	if value == nil {
		*m = ModelList{}
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		return fmt.Errorf("无法扫描模型列表: %T", value)
	}
	if len(data) == 0 || string(data) == "" {
		*m = ModelList{}
		return nil
	}
	var list []string
	if err := json.Unmarshal(data, &list); err != nil {
		return fmt.Errorf("解析模型列表失败: %w", err)
	}
	*m = list
	return nil
}
