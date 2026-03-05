package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"suka-eva/agent/base/core"
	"suka-eva/agent/base/toolManager"

	pb "github.com/sukasukasuka123/microHub/proto/gen/proto"
	hubbase "github.com/sukasukasuka123/microHub/root_class/hub"
	registry "github.com/sukasukasuka123/microHub/service_registry"
)

func main() {
	configPath := "baseConfig.yaml"
	hubRegistryPath := "config/registry.yaml"
	if len(os.Args) > 1 {
		configPath = filepath.Clean(os.Args[1])
	}

	// ── 1. 初始化 microHub registry ───────────────────────
	if err := registry.Init(hubRegistryPath); err != nil {
		log.Fatalf("❌ registry 初始化失败: %v", err)
	}

	// ── 2. 启动 Hub ────────────────────────────────────────
	hub := hubbase.New(&evaHubHandler{})
	go func() {
		if err := hub.ServeAsync(":50051", 0); err != nil {
			log.Fatalf("❌ Hub 启动失败: %v", err)
		}
	}()
	time.Sleep(100 * time.Millisecond)

	// ── 3. 初始化 SkillManager ─────────────────────────────
	// testFn = nil：开发模式，Promote 直接上线，跳过测试
	sm := toolManager.NewSkillManager(hub, nil)

	// 注册 micro_tool 并直接上线（开发模式）
	// 生产模式下由 test_hub 验证通过后通过 registry.MarkPassed 自动同步
	for _, s := range microTools() {
		if err := sm.Register(s.name, s.desc, s.addr, s.schema); err != nil {
			log.Printf("⚠️  注册 skill %s 失败: %v", s.name, err)
			continue
		}
		if err := sm.Promote(s.name); err != nil {
			log.Printf("⚠️  Promote skill %s 失败: %v", s.name, err)
		}
	}
	sm.PrintStatus()

	// ── 4. 创建 Agent ──────────────────────────────────────
	agent, err := core.NewAgent(configPath, sm)
	if err != nil {
		log.Fatalf("❌ 创建 Agent 失败: %v", err)
	}
	agent.RegisterSystemPrompt(
		"你是 suka-eva，一个通过微服务架构动态扩展 skill 的 AI 助手。\n" +
			"你的每个 skill 都运行在独立的 gRPC 微服务进程里，通过 microHub 调度执行。\n" +
			"请回答用户的问题，需要时主动调用合适的 skill。",
	)

	fmt.Println("🚀 suka-eva 启动")
	fmt.Println("💡 quit=退出 | reset=清空历史 | skills=查看 skill 列表")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("👤 You: ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		switch input {
		case "quit", "exit":
			goto done
		case "reset":
			agent.ResetHistory(true)
			fmt.Println("✅ 历史已清空（system prompt 保留）")
			continue
		case "skills":
			sm.PrintStatus()
			continue
		}

		resp, err := agent.Run(input)
		if err != nil {
			fmt.Printf("❌ Error: %v\n\n", err)
			continue
		}
		fmt.Printf("🤖 eva: %s\n\n", resp)
	}

done:
	fmt.Printf("\n👋 再见 | SessionID: %s | Messages: %d\n",
		agent.GetSessionID(), len(agent.GetHistory()))
}

// ── micro_tool 列表 ───────────────────────────────────────

type skillDef struct {
	name, desc, addr string
	schema           map[string]interface{}
}

func microTools() []skillDef {
	strParam := func(desc string) map[string]interface{} {
		return map[string]interface{}{
			"type":        "string",
			"description": desc,
		}
	}
	return []skillDef{
		{
			name: "echo",
			desc: "原样返回输入内容，用于验证调用链",
			addr: "localhost:50101",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"message": strParam("需要回显的内容"),
				},
				"required": []string{"message"},
			},
		},
		{
			name: "ping",
			desc: "测试目标地址的网络连通性",
			addr: "localhost:50102",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"target": strParam("目标 IP 或域名，默认 127.0.0.1"),
				},
			},
		},
		{
			name: "suka_secret",
			desc: "她不做任何有用的事情。但她在。",
			addr: "localhost:50100",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"knock": strParam("敲门的话（可选）"),
				},
			},
		},
	}
}

// ── Hub Handler ───────────────────────────────────────────

// evaHubHandler suka-eva 侧的 Hub 路由策略
// 按 service_name 精确路由到对应 micro_tool
type evaHubHandler struct{}

func (h *evaHubHandler) ServiceName() string { return "suka-eva-hub" }

func (h *evaHubHandler) Execute(req *pb.ToolRequest) ([]hubbase.DispatchTarget, error) {
	if req == nil {
		return nil, nil
	}
	t, ok := registry.SelectToolByName(req.ServiceName)
	if !ok {
		log.Printf("[Hub] skill=%s 未在 registry 中找到", req.ServiceName)
		return nil, nil
	}
	req.From = h.ServiceName()
	return []hubbase.DispatchTarget{
		{Addr: t.Addr, Request: req, Stream: true},
	}, nil
}

func (h *evaHubHandler) OnResults(results []hubbase.DispatchResult) {
	for _, r := range results {
		if !r.AllOK() {
			log.Printf("[Hub] ✗ addr=%s err=%v", r.Target.Addr, r.Err)
		}
	}
}

func (h *evaHubHandler) Addrs() []string {
	tools := registry.GetAllTools()
	addrs := make([]string, len(tools))
	for i, t := range tools {
		addrs[i] = t.Addr
	}
	return addrs
}
