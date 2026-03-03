package tool_interface

// Tool 所有微工具必须实现的接口
type Tool interface {
	GetName() string                                            // 工具唯一标识
	GetDescription() string                                     // 工具功能描述（供LLM理解）
	GetParameters() map[string]interface{}                      // 参数定义（JSON Schema格式）
	Execute(params map[string]interface{}) (interface{}, error) // 执行逻辑
}

// ToolResult 工具执行结果统一封装
type ToolResult struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}
