package toolManager

import (
	"fmt"
	"log"
	"sync"

	registry "github.com/sukasukasuka123/microHub/service_registry"
)

// SkillMeta 描述一个已注册 skill 的元信息
type SkillMeta struct {
	Name        string
	Description string
	Addr        string
	Schema      map[string]interface{} // JSON Schema，供 LLM function calling 使用
}

// skillRegistry 管理 suka-eva 侧的 skill 元信息视图
type skillRegistry struct {
	mu     sync.RWMutex
	skills map[string]*SkillMeta
}

func newSkillRegistry() *skillRegistry {
	return &skillRegistry{skills: make(map[string]*SkillMeta)}
}

func (r *skillRegistry) upsert(meta *SkillMeta) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills[meta.Name] = meta
	log.Printf("[Registry] upsert skill: %s @ %s", meta.Name, meta.Addr)
}

func (r *skillRegistry) remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.skills, name)
	log.Printf("[Registry] remove skill: %s", name)
}

func (r *skillRegistry) get(name string) (*SkillMeta, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.skills[name]
	return m, ok
}

func (r *skillRegistry) listAll() []*SkillMeta {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*SkillMeta, 0, len(r.skills))
	for _, m := range r.skills {
		cp := *m
		result = append(result, &cp)
	}
	return result
}

func (r *skillRegistry) has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.skills[name]
	return ok
}

// syncFromHub 从 microHub registry 拉取通过测试的 skill，同步到本地视图
// 保守策略：只增不删（已下线的 skill 由 process 状态机负责 Retire）
func (r *skillRegistry) syncFromHub() {
	passed := registry.GetPassedTools()
	for _, t := range passed {
		if r.has(t.Name) {
			continue
		}
		r.upsert(&SkillMeta{
			Name:        t.Name,
			Description: t.Method,
			Addr:        t.Addr,
			Schema:      defaultSchema(),
		})
	}
}

// watchHub 监听 microHub 变更，阻塞，需在 goroutine 中调用
func (r *skillRegistry) watchHub() {
	log.Println("[Registry] 开始监听 microHub 变更...")
	for range registry.ChangeCh() {
		log.Println("[Registry] 检测到变更，重新同步 skill")
		r.syncFromHub()
	}
}

// registerManual 手动注册（测试 / 本地覆盖用）
func (r *skillRegistry) registerManual(meta *SkillMeta) error {
	if meta.Name == "" {
		return fmt.Errorf("skill name cannot be empty")
	}
	r.upsert(meta)
	return nil
}

func defaultSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}
