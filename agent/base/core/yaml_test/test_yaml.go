// test_yaml.go
package main

import (
	"fmt"
	"suka-eva/agent/base/core"
)

func main() {
	path := "baseConfig.yaml"
	fmt.Printf("Loading: %s\n", path)

	config, err := core.LoadConfig(path)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	fmt.Printf("✅ Success:\n")
	fmt.Printf("  agent.ai_url:    %s\n", config.Agent.AI_URL)
	fmt.Printf("  agent.ai_name:   %s\n", config.Agent.AI_Name)
	fmt.Printf("  agent.ai_api_key: len=%d\n", len(config.Agent.AI_API_Key))
}
