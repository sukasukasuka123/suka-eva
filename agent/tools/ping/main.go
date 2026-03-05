// micro_tool/ping/main.go
// [TEST SKILL] ping —— 测试网络连通性
// 顺带验证 suka-eva util.RunCmd 工具链是否正常工作
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"runtime"
	"time"

	"suka-eva/agent/base/util"

	pb "github.com/sukasukasuka123/microHub/proto/gen/proto"
	tool "github.com/sukasukasuka123/microHub/root_class/tool"
)

type PingHandler struct{}

func (h *PingHandler) ServiceName() string { return "ping" }

func (h *PingHandler) Execute(req *pb.ToolRequest) ([]*pb.ToolResponse, error) {
	target := req.Params["target"]
	if target == "" {
		target = "127.0.0.1"
	}

	var args []string
	if runtime.GOOS == "windows" {
		args = []string{"ping", "-n", "1", target}
	} else {
		args = []string{"ping", "-c", "1", "-W", "2", target}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmdResult, err := util.RunCmd(ctx, 5*time.Second, args...)

	type pingResult struct {
		Target    string `json:"target"`
		Reachable bool   `json:"reachable"`
		ElapsedMs int64  `json:"elapsed_ms"`
		Error     string `json:"error,omitempty"`
	}

	pr := pingResult{
		Target:    target,
		ElapsedMs: cmdResult.Elapsed.Milliseconds(),
	}
	if err != nil {
		pr.Reachable = false
		pr.Error = err.Error()
	} else {
		pr.Reachable = cmdResult.ExitCode == 0
	}

	b, _ := json.Marshal(pr)
	status := "ok"
	if !pr.Reachable {
		status = "unreachable"
	}

	fmt.Printf("[ping] %s → reachable=%v %dms\n", target, pr.Reachable, pr.ElapsedMs)
	return []*pb.ToolResponse{{
		ServiceName: h.ServiceName(),
		Status:      status,
		Result:      string(b),
	}}, nil
}

func main() {
	log.Println("[ping] TEST SKILL 启动，监听 :50102")
	if err := tool.New(&PingHandler{}).Serve(":50102"); err != nil {
		log.Fatalf("%v", err)
	}
}
