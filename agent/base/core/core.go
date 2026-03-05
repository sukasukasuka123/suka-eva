package core

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"suka-eva/agent/base/toolManager"
)

// Agent 封装 Agent 运行时，强制使用 OpenAI 格式
type Agent struct {
	ctx      *AgentContext
	llm      *OpenAIClient
	sm       *toolManager.SkillManager
	maxLoops int
}

// NewAgent 从配置文件创建 Agent
func NewAgent(configPath string, sm *toolManager.SkillManager) (*Agent, error) {
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
		sm:       sm,
		maxLoops: 5,
	}, nil
}

// NewAgentWithClient 使用自定义 OpenAIClient 创建 Agent
func NewAgentWithClient(configPath string, sm *toolManager.SkillManager, llm *OpenAIClient) (*Agent, error) {
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
		sm:       sm,
		maxLoops: 5,
	}, nil
}

// Run 执行单轮用户对话
func (a *Agent) Run(userInput string) (string, error) {
	a.ctx.History = append(a.ctx.History, ChatMessage{
		Role:    "user",
		Content: userInput,
	})

	// 每轮调用前动态拉取最新 skill 列表（热更新自动生效）
	tools := a.buildChatTools()

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

		// 有工具调用：写入 assistant 消息，再逐个执行
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
				toolMsg.Content = result
				log.Printf("[Agent] Tool %s succeeded", toolCall.Function.Name)
			}
			a.ctx.History = append(a.ctx.History, toolMsg)
		}

		// 本轮有工具调用，刷新工具列表再进入下一轮
		// （中途可能有新 skill Passed，保持最新）
		tools = a.buildChatTools()
	}

	log.Printf("[Agent] Max loops (%d) exceeded", a.maxLoops)
	return "抱歉，处理过程过于复杂，我已达到最大尝试次数。请简化您的问题或分步询问。", nil
}

// buildChatTools 从 SkillManager 动态构建 OpenAI 格式工具列表
func (a *Agent) buildChatTools() []ChatTool {
	defs := a.sm.BuildChatTools()
	result := make([]ChatTool, 0, len(defs))
	for _, d := range defs {
		result = append(result, ChatTool{
			Type: "function",
			Function: ChatFunction{
				Name:        d.Name,
				Description: d.Description,
				Parameters:  d.Schema,
			},
		})
	}
	return result
}

// executeToolCall 解析参数并通过 SkillManager 调用对应 micro_tool
func (a *Agent) executeToolCall(toolCall ToolCall) (string, error) {
	name := toolCall.Function.Name

	args, err := ParseJSON(toolCall.Function.Arguments)
	if err != nil {
		return "", fmt.Errorf("parse arguments for %s: %w", name, err)
	}

	start := time.Now()
	result, err := a.sm.Dispatch(context.Background(), name, args)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return "", fmt.Errorf("skill %s [%dms]: %w", name, latency, err)
	}

	log.Printf("[Agent] skill=%s latency=%dms", name, latency)
	return result, nil
}

// ── 会话管理 ──────────────────────────────────────────────

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

func (a *Agent) SetMaxLoops(max int)                        { a.maxLoops = max }
func (a *Agent) GetSessionID() string                       { return a.ctx.SessionID }
func (a *Agent) GetLLMClient() *OpenAIClient                { return a.llm }
func (a *Agent) GetSkillManager() *toolManager.SkillManager { return a.sm }
