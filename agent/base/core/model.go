package core

// BaseConfig 配置文件结构
type BaseConfig struct {
	Database struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
		Name     string `yaml:"name"`
	} `yaml:"database"`
	Agent struct {
		AI_URL     string `yaml:"ai_url"`
		AI_Name    string `yaml:"ai_name"`
		AI_API_Key string `yaml:"ai_api_key"`
		Name       string `yaml:"name"`
	} `yaml:"agent"`
}

// ===========Tool相关==========

// ToolParamDef 工具参数定义（对应 tool_list.yaml）
type ToolParamDef struct {
	Type        string   `yaml:"type"`
	Description string   `yaml:"description"`
	Required    bool     `yaml:"required,omitempty"`
	Enum        []string `yaml:"enum,omitempty"`
}

// ToolDefinition 工具元数据
type ToolDefinition struct {
	Name        string                   `yaml:"name"`
	Endpoint    string                   `yaml:"endpoint,omitempty"`
	Description string                   `yaml:"description"`
	Parameters  map[string]*ToolParamDef `yaml:"parameters"`
}

// ========== OpenAI API 消息 / 工具调用（直接对齐 OpenAI 格式，唯一一套）==========

// ChatMessage 对应 OpenAI messages 数组中的单条消息
type ChatMessage struct {
	Role       string     `json:"role"`                   // system / user / assistant / tool
	Content    string     `json:"content"`                // 消息正文
	ToolCallID string     `json:"tool_call_id,omitempty"` // role=tool 时关联的调用 ID
	Name       string     `json:"name,omitempty"`         // role=tool 时的工具名称
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // role=assistant 发起工具调用时携带
}

// ToolCall 对应 OpenAI tool_calls 数组中的单个调用
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // 固定 "function"
	Function ToolFunction `json:"function"`
}

// ToolFunction 工具调用的函数名与参数
type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON 字符串，需解析后使用
}

// ========== OpenAI Chat Completion ==========

// ChatRequest 对应 POST /v1/chat/completions 请求体
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Tools    []ChatTool    `json:"tools,omitempty"`
	Stream   bool          `json:"stream"`
}

// ChatTool 对应 OpenAI tools 数组中的单个工具定义
type ChatTool struct {
	Type     string       `json:"type"` // 固定 "function"
	Function ChatFunction `json:"function"`
}

// ChatFunction 对应 OpenAI function 的 JSON Schema 定义
type ChatFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"` // JSON Schema object
}

// ChatResponse 对应 POST /v1/chat/completions 响应体
type ChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int         `json:"index"`
		Message      ChatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Error *APIError `json:"error,omitempty"`
}

// ========== OpenAI Embedding ==========

// EmbeddingRequest 对应 POST /v1/embeddings 请求体
type EmbeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

// EmbeddingResponse 对应 POST /v1/embeddings 响应体
type EmbeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Model string    `json:"model"`
	Error *APIError `json:"error,omitempty"`
}

// ========== 通用错误 ==========

// APIError OpenAI 标准错误结构
type APIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// ========== Agent 运行时 ==========

// AgentContext Agent 执行上下文
type AgentContext struct {
	Config    BaseConfig
	ToolList  map[string]*ToolDefinition
	History   []ChatMessage
	SessionID string
}
