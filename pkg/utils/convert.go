package utils

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ToString 将任意类型转换为字符串
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
		// 尝试 JSON 序列化
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(jsonBytes)
	}
}

// ToInt 将字符串转换为整数
func ToInt(s string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(s))
}

// ToInt64 将字符串转换为 int64
func ToInt64(s string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(s), 10, 64)
}

// ToBool 将字符串转换为布尔值
func ToBool(s string) (bool, error) {
	return strconv.ParseBool(strings.TrimSpace(s))
}

// ToFloat64 将字符串转换为 float64
func ToFloat64(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(s), 64)
}

// MustToString 将任意类型转换为字符串，出错时返回默认值
func MustToString(v interface{}, defaultVal string) string {
	result := ToString(v)
	if result == "" || result == "<nil>" {
		return defaultVal
	}
	return result
}

// TruncateString 截断字符串到指定长度
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return "..."
	}
	return s[:maxLen-3] + "..."
}
