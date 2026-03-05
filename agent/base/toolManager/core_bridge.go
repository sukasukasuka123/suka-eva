package toolManager

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	pb "github.com/sukasukasuka123/microHub/proto/gen/proto"
	hubbase "github.com/sukasukasuka123/microHub/root_class/hub"
	registry "github.com/sukasukasuka123/microHub/service_registry"
)

// SkillManager 是 toolManager 暴露给 core 的唯一入口
// core 只持有 *SkillManager，不感知 Hub / gRPC / registry 细节
type SkillManager struct {
	reg     *skillRegistry
	process *processManager
	hub     *hubbase.BaseHub // core 通过这里 Dispatch skill
}

// NewSkillManager 创建 SkillManager
//
//   - hub:    microHub 实例，不可为 nil（纯微服务模式）
//   - testFn: skill 测试函数，nil = 跳过测试直接 Passed（开发模式）
func NewSkillManager(
	hub *hubbase.BaseHub,
	testFn func(ctx context.Context, name string) error,
) *SkillManager {
	reg := newSkillRegistry()
	pm := newProcessManager(reg, testFn)

	sm := &SkillManager{reg: reg, process: pm, hub: hub}

	// 初始化时从 microHub 拉一次已通过测试的 skill
	reg.syncFromHub()

	// 启动热更新监听
	go reg.watchHub()

	return sm
}

// ── skill 注册 API（给 main / 初始化代码调用）────────────

// Register 注册一个 skill 并纳入状态机
// schema 描述 skill 的入参（JSON Schema），供 LLM function calling 使用
// 如果 testFn 为 nil，调用后再调 Promote 直接上线
func (sm *SkillManager) Register(name, description, addr string, schema map[string]interface{}) error {
	if schema == nil {
		schema = defaultSchema()
	}
	if err := sm.reg.registerManual(&SkillMeta{
		Name: name, Description: description, Addr: addr, Schema: schema,
	}); err != nil {
		return err
	}
	sm.process.track(name)
	return nil
}

// Promote 手动将 skill 提升为 Passed，无需经过测试流程
// 适合：开发调试、已知可信的 skill、CI 中跳过集成测试
func (sm *SkillManager) Promote(name string) error {
	if !sm.reg.has(name) {
		return fmt.Errorf("skill %q 未注册", name)
	}
	sm.process.promote(name)
	return nil
}

// Retire 下线 skill，Agent 下一轮对话起不再可见
func (sm *SkillManager) Retire(name string) {
	sm.process.retire(name)
}

// TestPending 并发测试所有 Pending 状态的 skill
func (sm *SkillManager) TestPending(ctx context.Context) {
	sm.process.testAll(ctx)
}

// ── core 调用接口（给 core.Agent 使用）────────────────────

// BuildChatTools 返回当前所有 Passed skill 的 OpenAI function calling 定义
// core.Agent 在每轮 LLM 调用前调用此方法，拿到最新 skill 列表
func (sm *SkillManager) BuildChatTools() []ChatToolDef {
	passed := sm.process.getPassed()
	result := make([]ChatToolDef, 0, len(passed))

	for _, name := range passed {
		meta, ok := sm.reg.get(name)
		if !ok {
			continue
		}
		result = append(result, ChatToolDef{
			Name:        meta.Name,
			Description: meta.Description,
			Schema:      meta.Schema,
		})
	}
	return result
}

// Dispatch 执行一次 skill 调用
// core.Agent 在收到 LLM 的 tool_call 时调用此方法
//
// name:   skill 名称（对应 tool_call.function.name）
// params: 解析后的参数（对应 tool_call.function.arguments）
//
// 返回：skill 执行结果字符串（直接作为 tool 消息的 content 回给 LLM）
func (sm *SkillManager) Dispatch(ctx context.Context, name string, params map[string]interface{}) (string, error) {
	// 检查 skill 是否处于 Passed 状态
	meta, ok := sm.reg.get(name)
	if !ok {
		return "", fmt.Errorf("skill %q 未注册", name)
	}

	// 检查 microHub 是否认识这个 skill（防止 registry 不一致）
	if _, found := registry.SelectToolByName(name); !found {
		// 本地测试 skill（没有进 microHub 的）也允许直接调用
		log.Printf("[SkillManager] skill %q 不在 microHub registry，尝试直接调用 addr=%s", name, meta.Addr)
	}

	// map[string]interface{} → map[string]string
	strParams := make(map[string]string, len(params))
	for k, v := range params {
		strParams[k] = fmt.Sprintf("%v", v)
	}

	start := time.Now()
	results := sm.hub.Dispatch(ctx, &pb.ToolRequest{
		From:        "suka-eva",
		ServiceName: name,
		Params:      strParams,
	})
	latency := time.Since(start).Milliseconds()

	if len(results) == 0 {
		return "", fmt.Errorf("skill %q 无响应（Hub 路由失败或超时）", name)
	}

	// 聚合所有响应（流式 skill 可能返回多条）
	var parts []string
	var errs []string
	for _, r := range results {
		if r.Err != nil {
			errs = append(errs, r.Err.Error())
			continue
		}
		for _, resp := range r.Responses {
			if resp.Status != "ok" {
				errs = append(errs, fmt.Sprintf("[%s] %s", resp.ServiceName, resp.Result))
			} else {
				parts = append(parts, resp.Result)
			}
		}
	}

	log.Printf("[SkillManager] skill=%s latency=%dms responses=%d errors=%d",
		name, latency, len(parts), len(errs))

	if len(errs) > 0 && len(parts) == 0 {
		return "", fmt.Errorf("skill %q 执行失败: %s", name, strings.Join(errs, "; "))
	}

	return strings.Join(parts, "\n"), nil
}

// ── 查询 / 调试 API ───────────────────────────────────────

// PrintStatus 打印所有 skill 当前状态（调试用）
func (sm *SkillManager) PrintStatus() {
	log.Println("=== suka-eva Skill Status ===")
	snaps := sm.process.snapshot()
	for _, s := range snaps {
		meta, _ := sm.reg.get(s.Name)
		addr := ""
		if meta != nil {
			addr = meta.Addr
		}
		log.Printf("  [%-8s] %-20s %s", s.State, s.Name, addr)
	}
	log.Printf("  total: %d", len(snaps))
	log.Println("=============================")
}

// ListSkills 返回当前所有 skill 的摘要（供 Agent system prompt 动态生成）
func (sm *SkillManager) ListSkills() []SkillSummary {
	snaps := sm.process.snapshot()
	result := make([]SkillSummary, 0, len(snaps))
	for _, s := range snaps {
		meta, _ := sm.reg.get(s.Name)
		desc := ""
		if meta != nil {
			desc = meta.Description
		}
		result = append(result, SkillSummary{
			Name:        s.Name,
			Description: desc,
			State:       s.State,
		})
	}
	return result
}

// ── 对外数据类型 ──────────────────────────────────────────

// ChatToolDef 对应 OpenAI function calling 的一条工具定义
// core.Agent 直接用这个构建 ChatTool 列表
type ChatToolDef struct {
	Name        string
	Description string
	Schema      map[string]interface{}
}

// SkillSummary skill 对外摘要
type SkillSummary struct {
	Name        string
	Description string
	State       SkillState
}
