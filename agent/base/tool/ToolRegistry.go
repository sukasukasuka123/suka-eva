package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
)

// ToolRegistry 工具注册中心
type ToolRegistry struct {
	Tools map[string]any // 存储 *Tool[I,O]，key=工具名
}

func NewRegistry() *ToolRegistry {
	return &ToolRegistry{
		Tools: make(map[string]any),
	}
}

// Register 注册泛型工具
func Register[I any, O any](r ToolRegistry, t *Tool[I, O]) {
	if t == nil {
		panic(fmt.Sprintf("cannot register nil tool"))
	}
	if t.Name == "" {
		panic("tool name cannot be empty")
	}
	r.Tools[t.Name] = t
}

// ==================== 对外统一接口（Agent 调用这个） ====================

// Call 统一调用入口：Agent 只依赖这个签名
func (r *ToolRegistry) Call(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
	raw, ok := r.Tools[name]
	if !ok {
		return nil, fmt.Errorf("tool %q not registered", name)
	}

	// 🔍 类型断言：先试"非泛型兼容模式"（Tool[map,interface]）
	if tool, ok := raw.(*Tool[map[string]interface{}, interface{}]); ok {
		return tool.Execute(ctx, args)
	}

	// 🔄 否则走泛型转发（反射 + JSON 编解码）
	return r.callGeneric(ctx, raw, args)
}

// GetSchema 获取工具的 JSON Schema（供 LLM 使用）
func (r *ToolRegistry) GetSchema(name string) (map[string]interface{}, error) {
	raw, ok := r.Tools[name]
	if !ok {
		return nil, fmt.Errorf("tool %q not found", name)
	}
	// 用反射统一获取 Parameters 字段
	v := reflect.ValueOf(raw)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	params := v.FieldByName("Parameters")
	if !params.IsValid() {
		return nil, fmt.Errorf("tool %q has no Parameters field", name)
	}
	return params.Interface().(map[string]interface{}), nil
}

// ==================== 泛型转发核心（反射 + JSON） ====================

func (r *ToolRegistry) callGeneric(ctx context.Context, raw any, args map[string]interface{}) (interface{}, error) {
	// 反射获取 *Tool[I,O]
	rv := reflect.ValueOf(raw)
	if rv.Kind() != reflect.Ptr || rv.Elem().Type().Name() != "Tool" {
		return nil, fmt.Errorf("invalid tool type: %T", raw)
	}
	toolVal := rv.Elem()

	// 获取泛型参数类型 I 和 O
	toolType := toolVal.Type() // type Tool[I, O]
	if toolType.NumIn() != 2 { // I, O 是两个类型参数
		return nil, fmt.Errorf("tool must have 2 type params, got %d", toolType.NumIn())
	}
	// 关键：通过 Field 获取 Execute 函数，再解析其类型
	exeField := toolVal.FieldByName("Execute")
	if !exeField.IsValid() || exeField.Kind() != reflect.Func {
		return nil, fmt.Errorf("tool has no valid Execute function")
	}
	exeType := exeField.Type() // func(context.Context, I) (O, error)

	// 解析入参类型 I（exeType.In(1)）
	inputType := exeType.In(1)
	inputPtr := reflect.New(inputType) // *I

	// JSON 编解码：map → I（万能但有一次反射开销）
	jsonData, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("marshal args failed: %w", err)
	}
	if err := json.Unmarshal(jsonData, inputPtr.Interface()); err != nil {
		return nil, fmt.Errorf("unmarshal to %s failed: %w", inputType, err)
	}

	// 调用 Execute(ctx, *I)
	results := exeField.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		inputPtr.Elem(), // 传 I 值（非指针）
	})

	// 解析返回：(O, error)
	if len(results) != 2 {
		return nil, fmt.Errorf("Execute must return (O, error), got %d values", len(results))
	}
	errVal := results[1]
	if !errVal.IsNil() {
		return nil, errVal.Interface().(error)
	}

	// O → map[string]interface{}（方便 Agent 统一处理）
	output := results[0].Interface() // O
	jsonOut, err := json.Marshal(output)
	if err != nil {
		return nil, fmt.Errorf("marshal output failed: %w", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(jsonOut, &result); err != nil {
		// 如果 O 不是 map/struct，直接返回原始值
		return output, nil
	}
	return result, nil
}
