// micro_tool/example/main.go
// [TEST SKILL] echo —— 原样返回输入
// 用途：验证 microHub → SkillManager → core.Agent 整条调用链
package main

import (
	"encoding/json"
	"fmt"
	"log"

	pb "github.com/sukasukasuka123/microHub/proto/gen/proto"
	tool "github.com/sukasukasuka123/microHub/root_class/tool"
)

type EchoHandler struct{}

func (h *EchoHandler) ServiceName() string { return "echo" }

func (h *EchoHandler) Execute(req *pb.ToolRequest) ([]*pb.ToolResponse, error) {
	msg := req.Params["message"]
	if msg == "" {
		msg = "(empty)"
	}

	type result struct {
		Echo   string `json:"echo"`
		From   string `json:"from"`
		Status string `json:"status"`
	}
	b, _ := json.Marshal(result{Echo: msg, From: "echo", Status: "ok"})

	fmt.Printf("[echo] → %s\n", string(b))
	return []*pb.ToolResponse{{
		ServiceName: h.ServiceName(),
		Status:      "ok",
		Result:      string(b),
	}}, nil
}

func main() {
	log.Println("[echo] TEST SKILL 启动，监听 :50101")
	if err := tool.New(&EchoHandler{}).Serve(":50101"); err != nil {
		log.Fatalf("%v", err)
	}
}
