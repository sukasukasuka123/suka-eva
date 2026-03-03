package tool

import (
	"context"
)

// Tool 泛型定义：I=入参结构, O=出参结构
type Tool[I any, O any] struct {
	Name        string
	Description string
	Endpoint    string                              // 空字符串 = 本地执行，非空 = 远程 HTTP 端点
	Parameters  map[string]interface{}              // JSON Schema，供 LLM 理解
	Execute     func(context.Context, I) (O, error) // ✅ ctx 放前面，Go 惯例
}

// ==================== Option 模式（替代默认参数） ====================

type ToolOption[I, O any] func(*Tool[I, O])

// WithEndpoint 设置远程端点（可选）
func WithEndpoint[I, O any](url string) ToolOption[I, O] {
	return func(t *Tool[I, O]) { t.Endpoint = url }
}

// WithSchema 设置参数 JSON Schema（可选，不传则用空 object）
func WithSchema[I, O any](schema map[string]interface{}) ToolOption[I, O] {
	return func(t *Tool[I, O]) { t.Parameters = schema }
}

// ==================== 构造函数 ====================

// NewTool 创建泛型 Tool（类型自动推断，写法清爽）
// 示例: NewTool("echo", "回显工具", echoExec, WithSchema(schema), WithEndpoint(""))
func NewTool[I any, O any](
	name, description string,
	execute func(context.Context, I) (O, error),
	opts ...ToolOption[I, O],
) *Tool[I, O] {
	t := &Tool[I, O]{
		Name:        name,
		Description: description,
		Parameters:  map[string]interface{}{"type": "object"}, // 默认空 schema
		Execute:     execute,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// GetName/GetDescription 辅助方法（满足某些接口需求）
func (t *Tool[I, O]) GetName() string                       { return t.Name }
func (t *Tool[I, O]) GetDescription() string                { return t.Description }
func (t *Tool[I, O]) GetParameters() map[string]interface{} { return t.Parameters }
