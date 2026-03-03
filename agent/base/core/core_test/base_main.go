// cmd/zero-tool-demo/main.go
package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"suka-eva/agent/base/core"
	"suka-eva/agent/base/tool"
)

func main() {
	configPath := "baseConfig.yaml"
	if len(os.Args) > 1 {
		configPath = filepath.Clean(os.Args[1])
	}

	// 空 registry：不注册任何工具，纯对话模式
	registry := tool.NewRegistry()

	agent, err := core.NewAgent(configPath, registry)
	if err != nil {
		log.Fatalf("❌ 创建 Agent 失败: %v", err)
	}
	agent.RegisterSystemPrompt("你是一个携带各种skill甚至可以自己创造skill的 AI 助手，请回答用户的问题。")

	fmt.Println("🚀 Zero-Tool Agent Demo 启动")
	fmt.Println("💡 输入 'quit' / 'exit' 退出 | 输入 'reset' 清空历史")

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
		if input == "quit" || input == "exit" {
			break
		}
		if input == "reset" {
			agent.ResetHistory(true)
			fmt.Println("✅ 历史已清空（system prompt 保留）")
			continue
		}

		resp, err := agent.Run(input)
		if err != nil {
			fmt.Printf("❌ Error: %v\n\n", err)
			continue
		}
		fmt.Printf("🤖 Agent: %s\n\n", resp)
	}

	fmt.Printf("\n👋 Demo 结束 | SessionID: %s | Total messages: %d\n",
		agent.GetSessionID(), len(agent.GetHistory()))
}
