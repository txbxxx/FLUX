package utils

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ToString 尽量把常见类型稳定转换成字符串。
// 复杂类型会退回 JSON 序列化或 fmt 输出。
func ToString(v interface{}) string {
	if v == nil {
		return ""
	}

	switch val := v.(type) {
	case string:
		return val
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%f", v)
	case bool:
		return strconv.FormatBool(val)
	case []byte:
		return string(val)
	default:
		// 复杂类型尽量转成 JSON，便于日志和配置展示。
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(jsonBytes)
	}
}

// ToInt 在转换前会先去掉首尾空白。
func ToInt(s string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(s))
}

// ToInt64 在转换前会先去掉首尾空白。
func ToInt64(s string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(s), 10, 64)
}

// ToBool 在转换前会先去掉首尾空白。
func ToBool(s string) (bool, error) {
	return strconv.ParseBool(strings.TrimSpace(s))
}

// ToFloat64 在转换前会先去掉首尾空白。
func ToFloat64(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(s), 64)
}

// MustToString 在结果为空时返回默认值，适合展示层兜底。
func MustToString(v interface{}, defaultVal string) string {
	result := ToString(v)
	if result == "" || result == "<nil>" {
		return defaultVal
	}
	return result
}

// TruncateString 按字符数截断，并给长文本追加省略号。
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return "..."
	}
	return s[:maxLen-3] + "..."
}
