package core

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/spf13/viper"
)

// LoadConfig 用 viper 加载 yaml 配置文件
func LoadConfig(path string) (BaseConfig, error) {
	var config BaseConfig
	viper.SetConfigFile(path)
	if err := viper.ReadInConfig(); err != nil {
		return BaseConfig{}, err
	}
	if err := viper.Unmarshal(&config); err != nil {
		return BaseConfig{}, err
	}
	return config, nil
}

// ParseJSON 安全解析 JSON 字符串为 map
func ParseJSON(s string) (map[string]interface{}, error) {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}
	return m, nil
}

// FormatToolResult 格式化工具返回结果为字符串
func FormatToolResult(result interface{}) string {
	if result == nil {
		return "null"
	}
	if str, ok := result.(string); ok {
		return str
	}
	if b, err := json.MarshalIndent(result, "", "  "); err == nil {
		return string(b)
	}
	return fmt.Sprintf("%v", result)
}

// TruncateHistory 截断历史，保留 system + 最近 N 条（防止 token 溢出）
// TODO: 集成 tiktoken 计算真实 token 数
func TruncateHistory(messages []ChatMessage, maxTokens int) []ChatMessage {
	if len(messages) <= 10 {
		return messages
	}
	sys := []ChatMessage{}
	if messages[0].Role == "system" {
		sys = append(sys, messages[0])
		messages = messages[1:]
	}
	start := len(messages) - 9
	if start < 0 {
		start = 0
	}
	return append(sys, messages[start:]...)
}

func getToolDescription(raw any) string {
	type describer interface{ GetDescription() string }
	if d, ok := raw.(describer); ok {
		return d.GetDescription()
	}
	// fallback: 反射
	v := reflect.ValueOf(raw)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if f := v.FieldByName("Description"); f.IsValid() {
		return f.String()
	}
	return ""
}
