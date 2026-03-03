package core

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"suka-eva/agent/base/tool" // 替换为你的实际模块路径
)

// Agent 封装 Agent 运行时，强制使用 OpenAI 格式
type Agent struct {
	ctx      *AgentContext
	llm      *OpenAIClient
	registry *tool.ToolRegistry // ← 替代原来的 ToolList + toolCaller
	maxLoops int
}

// NewAgent 从配置文件创建 Agent
func NewAgent(
	configPath string,
	registry *tool.ToolRegistry,
) (*Agent, error) {
	config, err := LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	fmt.Printf("Loaded config for AI: %s\n", config.Agent.AI_Name)
	return &Agent{
		ctx: &AgentContext{
			Config:    config,
			History:   make([]ChatMessage, 0, 20),
			SessionID: fmt.Sprintf("sess_%d_%s", time.Now().UnixNano(), config.Agent.AI_Name),
		},
		llm:      NewOpenAIClient(configPath),
		registry: registry,
		maxLoops: 5,
	}, nil
}

// NewAgentWithClient 使用自定义 OpenAIClient 创建 Agent
func NewAgentWithClient(
	configPath string,
	registry *tool.ToolRegistry,
	llm *OpenAIClient,
) (*Agent, error) {
	config, err := LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return &Agent{
		ctx: &AgentContext{
			Config:    config,
			History:   make([]ChatMessage, 0, 20),
			SessionID: fmt.Sprintf("sess_%d_%s", time.Now().UnixNano(), config.Agent.AI_Name),
		},
		llm:      llm,
		registry: registry,
		maxLoops: 5,
	}, nil
}

// Run 执行单轮用户对话
func (a *Agent) Run(userInput string) (string, error) {
	a.ctx.History = append(a.ctx.History, ChatMessage{
		Role:    "user",
		Content: userInput,
	})

	tools, err := a.buildChatToolsFromRegistry()
	if err != nil {
		return "", fmt.Errorf("build tools: %w", err)
	}

	for loop := 0; loop < a.maxLoops; loop++ {
		resp, err := a.llm.Chat(a.ctx.History, tools)
		if err != nil {
			return "", fmt.Errorf("llm chat failed: %w", err)
		}

		choice := resp.Choices[0]

		// 无工具调用：直接返回
		if len(choice.Message.ToolCalls) == 0 {
			a.ctx.History = append(a.ctx.History, choice.Message)
			a.ctx.History = TruncateHistory(a.ctx.History, 4000)
			return choice.Message.Content, nil
		}

		// 有工具调用：写入 assistant 消息（含 tool_calls），再逐个执行
		log.Printf("[Agent] Loop %d: processing %d tool calls", loop+1, len(choice.Message.ToolCalls))
		a.ctx.History = append(a.ctx.History, choice.Message)

		for _, toolCall := range choice.Message.ToolCalls {
			result, err := a.executeToolCall(toolCall)

			toolMsg := ChatMessage{
				Role:       "tool",
				ToolCallID: toolCall.ID,
				Name:       toolCall.Function.Name,
			}
			if err != nil {
				toolMsg.Content = fmt.Sprintf(`{"error": "%s"}`, strings.ReplaceAll(err.Error(), `"`, `\"`))
				log.Printf("[Agent] Tool %s failed: %v", toolCall.Function.Name, err)
			} else {
				toolMsg.Content = FormatToolResult(result)
				log.Printf("[Agent] Tool %s succeeded", toolCall.Function.Name)
			}
			a.ctx.History = append(a.ctx.History, toolMsg)
		}
	}

	log.Printf("[Agent] Max loops (%d) exceeded", a.maxLoops)
	return "抱歉，处理过程过于复杂，我已达到最大尝试次数。请简化您的问题或分步询问。", nil
}

// buildChatToolsFromRegistry 从 ToolRegistry 构建 OpenAI 格式工具列表
func (a *Agent) buildChatToolsFromRegistry() ([]ChatTool, error) {
	result := make([]ChatTool, 0, len(a.registry.Tools))

	for name := range a.registry.Tools {
		schema, err := a.registry.GetSchema(name)
		if err != nil {
			return nil, fmt.Errorf("get schema for tool %q: %w", name, err)
		}

		// 从 schema 中提取 description（如果工具存了的话）
		// 也可以通过反射拿 Name/Description，这里用统一接口
		raw := a.registry.Tools[name]
		desc := getToolDescription(raw)

		result = append(result, ChatTool{
			Type: "function",
			Function: ChatFunction{
				Name:        name,
				Description: desc,
				Parameters:  schema,
			},
		})
	}
	return result, nil
}

// executeToolCall 解析参数并调用 ToolRegistry
func (a *Agent) executeToolCall(toolCall ToolCall) (interface{}, error) {
	name := toolCall.Function.Name

	args, err := ParseJSON(toolCall.Function.Arguments)
	if err != nil {
		return nil, fmt.Errorf("parse arguments: %w", err)
	}

	start := time.Now()
	result, err := a.registry.Call(context.Background(), name, args)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return nil, fmt.Errorf("tool %s [%dms]: %w", name, latency, err)
	}

	log.Printf("[Tool] %s | latency: %dms", name, latency)
	return result, nil
}

// =========== 会话管理（不变） ===========

func (a *Agent) RegisterSystemPrompt(prompt string) {
	sysMsg := ChatMessage{Role: "system", Content: prompt}
	if len(a.ctx.History) == 0 {
		a.ctx.History = append(a.ctx.History, sysMsg)
		return
	}
	if a.ctx.History[0].Role == "system" {
		a.ctx.History[0] = sysMsg
	} else {
		a.ctx.History = append([]ChatMessage{sysMsg}, a.ctx.History...)
	}
}

func (a *Agent) GetHistory() []ChatMessage {
	hist := make([]ChatMessage, len(a.ctx.History))
	copy(hist, a.ctx.History)
	return hist
}

func (a *Agent) ResetHistory(keepSystem bool) {
	if keepSystem && len(a.ctx.History) > 0 && a.ctx.History[0].Role == "system" {
		a.ctx.History = []ChatMessage{a.ctx.History[0]}
	} else {
		a.ctx.History = make([]ChatMessage, 0, 20)
	}
}

func (a *Agent) SetMaxLoops(max int)         { a.maxLoops = max }
func (a *Agent) GetSessionID() string        { return a.ctx.SessionID }
func (a *Agent) GetLLMClient() *OpenAIClient { return a.llm }
