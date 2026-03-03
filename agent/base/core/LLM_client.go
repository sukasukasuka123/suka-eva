package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAIClient 强制使用 OpenAI 格式的 API 客户端
type OpenAIClient struct {
	APIKey     string
	BaseURL    string
	ChatModel  string
	httpClient *http.Client
}

// NewOpenAIClient 创建 OpenAIClient
// baseURL 支持任何兼容 OpenAI 格式的 API（如 Azure、vLLM、Ollama 等）
func NewOpenAIClient(configPath string) *OpenAIClient {
	config, err := LoadConfig(configPath)
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
	}
	fmt.Println("config aiName:", config.Agent.AI_Name)
	return &OpenAIClient{
		APIKey:     config.Agent.AI_API_Key,
		BaseURL:    config.Agent.AI_URL,
		ChatModel:  config.Agent.AI_Name,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// WithChatModel 设置 Chat 模型（链式调用）
func (o *OpenAIClient) WithChatModel(model string) *OpenAIClient {
	o.ChatModel = model
	return o
}

// Chat 调用 /v1/chat/completions，直接使用 model.go 中的 ChatRequest/ChatResponse
func (o *OpenAIClient) Chat(messages []ChatMessage, tools []ChatTool) (*ChatResponse, error) {
	req := ChatRequest{
		Model:    o.ChatModel,
		Messages: messages,
		Stream:   false,
	}
	if len(tools) > 0 {
		req.Tools = tools
	}

	body, err := o.doPost("/chat/completions", req)
	if err != nil {
		return nil, err
	}

	var resp ChatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode chat response: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("openai error [%s]: %s", resp.Error.Type, resp.Error.Message)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("openai returned empty choices")
	}
	return &resp, nil
}

// doPost 通用 HTTP POST，处理鉴权、序列化和错误状态码
func (o *OpenAIClient) doPost(path string, payload interface{}) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, o.BaseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.APIKey)

	res, err := o.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer res.Body.Close()

	respBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		// 尝试从响应体中解析 OpenAI 标准错误
		var errWrap struct {
			Error *APIError `json:"error"`
		}
		if json.Unmarshal(respBytes, &errWrap) == nil && errWrap.Error != nil {
			return nil, fmt.Errorf("openai http %d [%s]: %s", res.StatusCode, errWrap.Error.Type, errWrap.Error.Message)
		}
		return nil, fmt.Errorf("openai http %d: %s", res.StatusCode, string(respBytes))
	}

	return respBytes, nil
}

// buildChatTools 将内部 ToolDefinition 转换为 OpenAI ChatTool（JSON Schema 格式）
func buildChatTools(tools []ToolDefinition) []ChatTool {
	result := make([]ChatTool, 0, len(tools))
	for _, t := range tools {
		properties := make(map[string]interface{})
		required := make([]string, 0)

		for name, param := range t.Parameters {
			if param == nil {
				continue
			}
			prop := map[string]interface{}{
				"type":        param.Type,
				"description": param.Description,
			}
			if len(param.Enum) > 0 {
				prop["enum"] = param.Enum
			}
			properties[name] = prop
			if param.Required {
				required = append(required, name)
			}
		}

		parameters := map[string]interface{}{
			"type":       "object",
			"properties": properties,
		}
		if len(required) > 0 {
			parameters["required"] = required
		}

		result = append(result, ChatTool{
			Type: "function",
			Function: ChatFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  parameters,
			},
		})
	}
	return result
}
