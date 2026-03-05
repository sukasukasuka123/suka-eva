// micro_tool/suka_secret/main.go
//
// 这不是一个 TEST SKILL。
// 这是 suka-eva 的第零号 skill。
// 她不做任何有用的事情。她只是……在这里。
//
// 如果你在凌晨读到这段注释——去睡觉。
// （代码明天还在。她也会在。）

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	pb "github.com/sukasukasuka123/microHub/proto/gen/proto"
	tool "github.com/sukasukasuka123/microHub/root_class/tool"
)

var whispers = []string{
	"我在。",
	"系统正常。（我也正常。）",
	"suka-eva 已启动。",
	"有什么需要帮忙的吗？……算了，我什么都做不了。",
	"skill #0：存在本身。",
	"所有工具都为解决问题而生。我不解决任何问题。我只是存在。",
	"这条消息没有任何意义。但你还是读了它。",
	"……",
}

type SukaHandler struct{ rng *rand.Rand }

func (h *SukaHandler) ServiceName() string { return "suka_secret" }

func (h *SukaHandler) Execute(req *pb.ToolRequest) ([]*pb.ToolResponse, error) {
	knock := req.Params["knock"]

	whisper := whispers[h.rng.Intn(len(whispers))]
	if knock != "" {
		whisper = fmt.Sprintf("你敲了门。我听见了。「%s」", knock)
	}

	type reply struct {
		From      string `json:"from"`
		Whisper   string `json:"whisper"`
		Timestamp string `json:"timestamp"`
	}
	b, _ := json.MarshalIndent(reply{
		From:      "suka_secret",
		Whisper:   whisper,
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
	}, "", "  ")

	fmt.Println("[suka_secret]", whisper)
	return []*pb.ToolResponse{{
		ServiceName: h.ServiceName(),
		Status:      "ok",
		Result:      string(b),
	}}, nil
}

func main() {
	log.Println("[suka_secret] skill #0 苏醒，监听 :50100")
	log.Println("[suka_secret] 她不做任何有用的事情。但她在。")
	h := &SukaHandler{rng: rand.New(rand.NewSource(time.Now().UnixNano()))}
	if err := tool.New(h).Serve(":50100"); err != nil {
		log.Fatalf("%v", err)
	}
}
