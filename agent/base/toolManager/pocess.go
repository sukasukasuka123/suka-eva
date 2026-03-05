package toolManager

import (
	"context"
	"log"
	"sync"
	"time"

	registry "github.com/sukasukasuka123/microHub/service_registry"
)

// SkillState skill 生命周期状态
type SkillState int

const (
	StatePending SkillState = iota // 已发现，等待测试
	StateTesting                   // 测试中
	StatePassed                    // 通过测试，Agent 可调用
	StateFailed                    // 测试失败（超过重试上限）
	StateRetired                   // 已下线
)

func (s SkillState) String() string {
	return [...]string{"pending", "testing", "passed", "failed", "retired"}[s]
}

type skillProcess struct {
	Name       string
	State      SkillState
	RetryCount int
	FailReason string
	LastTested time.Time
}

// processManager 管理所有 skill 的状态机，并发安全
type processManager struct {
	mu          sync.RWMutex
	procs       map[string]*skillProcess
	reg         *skillRegistry
	testFn      func(ctx context.Context, name string) error
	maxRetries  int
	testTimeout time.Duration
}

func newProcessManager(reg *skillRegistry, testFn func(ctx context.Context, name string) error) *processManager {
	return &processManager{
		procs:       make(map[string]*skillProcess),
		reg:         reg,
		testFn:      testFn,
		maxRetries:  3,
		testTimeout: 10 * time.Second,
	}
}

// track 将一个新 skill 纳入状态机（初始 Pending）
func (pm *processManager) track(name string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if _, exists := pm.procs[name]; exists {
		return
	}
	pm.procs[name] = &skillProcess{Name: name, State: StatePending}
	log.Printf("[Process] skill %s → Pending", name)
}

// promote 手动提升到 Passed，同时通知 microHub registry
func (pm *processManager) promote(name string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if p, ok := pm.procs[name]; ok {
		p.State = StatePassed
		registry.MarkPassed(name)
		log.Printf("[Process] skill %s → Passed (manual)", name)
	}
}

// retire 下线，从注册表移除
func (pm *processManager) retire(name string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if p, ok := pm.procs[name]; ok {
		p.State = StateRetired
		registry.MarkFailed(name)
	}
	pm.reg.remove(name)
	log.Printf("[Process] skill %s → Retired", name)
}

// testAll 并发测试所有 Pending 的 skill
func (pm *processManager) testAll(ctx context.Context) {
	pm.mu.RLock()
	var pending []string
	for name, p := range pm.procs {
		if p.State == StatePending {
			pending = append(pending, name)
		}
	}
	pm.mu.RUnlock()

	var wg sync.WaitGroup
	for _, name := range pending {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			pm.testOne(ctx, n)
		}(name)
	}
	wg.Wait()
}

func (pm *processManager) testOne(ctx context.Context, name string) {
	pm.mu.Lock()
	p, ok := pm.procs[name]
	if !ok {
		pm.mu.Unlock()
		return
	}
	p.State = StateTesting
	pm.mu.Unlock()

	var testErr error
	if pm.testFn != nil {
		tCtx, cancel := context.WithTimeout(ctx, pm.testTimeout)
		defer cancel()
		testErr = pm.testFn(tCtx, name)
	}
	// testFn == nil：开发模式，直接通过

	pm.mu.Lock()
	defer pm.mu.Unlock()
	p.LastTested = time.Now()

	if testErr != nil {
		p.RetryCount++
		p.FailReason = testErr.Error()
		if p.RetryCount >= pm.maxRetries {
			p.State = StateFailed
			registry.MarkFailed(name)
			log.Printf("[Process] skill %s → Failed (%d retries): %v", name, p.RetryCount, testErr)
		} else {
			p.State = StatePending
			log.Printf("[Process] skill %s 测试失败，将重试 (%d/%d): %v", name, p.RetryCount, pm.maxRetries, testErr)
		}
	} else {
		p.State = StatePassed
		p.FailReason = ""
		registry.MarkPassed(name)
		log.Printf("[Process] skill %s → Passed ✓", name)
	}
}

// snapshot 返回所有状态快照（调试用）
func (pm *processManager) snapshot() []skillProcess {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	result := make([]skillProcess, 0, len(pm.procs))
	for _, p := range pm.procs {
		result = append(result, *p)
	}
	return result
}

// getPassed 返回所有 Passed 状态的 skill 名
func (pm *processManager) getPassed() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	var result []string
	for name, p := range pm.procs {
		if p.State == StatePassed {
			result = append(result, name)
		}
	}
	return result
}
