package hermes

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/dop251/goja"
)

// HookManager 管理 JS hook 函数的生命周期，通过 goja VM 编译并执行用户自定义的
// onTaskCreate / onTaskFinish / onFilename 钩子。
type HookManager struct {
	vm              *goja.Runtime
	hasCreateHook   bool
	hasFinishHook   bool
	hasFilenameHook bool
}

// TaskInfo 是 hook 中暴露的任务信息。
type TaskInfo struct {
	Name     string         `json:"name"`
	SavePath string         `json:"save_path"`
	Config   map[string]any `json:"config"`
}

// ResourceInfo 是 hook 中暴露的资源信息。
type ResourceInfo struct {
	ID         int               `json:"id"`
	Name       string            `json:"name"`
	Kind       string            `json:"kind"`
	Size       int64             `json:"size"`
	UniqueID   string            `json:"unique_id"`
	Extra     map[string]string `json:"extra"`
	Endpoints []EndpointInfo    `json:"endpoints"`
}

// EndpointInfo 是 hook 中暴露的端点信息。
type EndpointInfo struct {
	Protocol string `json:"protocol"`
	URL      string `json:"url"`
}

// TaskInput 是 onTaskCreate hook 的输入/输出类型。
type TaskInput struct {
	Task      TaskInfo       `json:"task"`
	Config    map[string]any `json:"config"`
	Metadata  map[string]any `json:"metadata"`
	Resources []ResourceInfo `json:"resources"`
}

// FinishContext 是 onTaskFinish hook 的上下文。
type FinishContext struct {
	Task      TaskInfo       `json:"task"`
	Config    map[string]any `json:"config"`
	Metadata  map[string]any `json:"metadata"`
	Resources []ResourceInfo `json:"resources"`
	FilePaths []string       `json:"filePaths"`
	SavePath  string         `json:"savePath"`
}

// ResourceMeta 是 onFilename hook 第二个参数，由 resource.Extra 展开的扁平视频元数据。
// 不同平台提供的字段不同，均为可选。
type ResourceMeta struct {
	ID         string `json:"id"`          // 视频 id
	Title      string `json:"title"`       // 视频标题
	Spec       string `json:"spec"`        // 视频质量，如 "original"、"xWT111"
	CreatedAt  int64  `json:"created_at"`  // 视频发布时间（秒）
	DownloadAt int64  `json:"download_at"` // 下载时间（秒）
	Author     string `json:"author"`      // up主名称
	Platform   string `json:"platform"`    // 平台标识，如 "wx_channels"
}

// FilenameParams 是 onFilename hook 的参数。
// Hook 返回 string（文件名）或 null/空串（沿用默认逻辑）。
type FilenameParams struct {
	Meta   ResourceMeta  `json:"meta"`
	Task   TaskInfo      `json:"task"`
	Config map[string]any `json:"config"`
}

// NewHookManager 创建一个空的 HookManager。
func NewHookManager() *HookManager {
	return &HookManager{}
}

// HasCreateHook 返回是否注册了 onTaskCreate hook。
func (hm *HookManager) HasCreateHook() bool {
	return hm.hasCreateHook
}

// HasFinishHook 返回是否注册了 onTaskFinish hook。
func (hm *HookManager) HasFinishHook() bool {
	return hm.hasFinishHook
}

// HasFilenameHook 返回是否注册了 onFilename hook。
func (hm *HookManager) HasFilenameHook() bool {
	return hm.hasFilenameHook
}

// Load 读取并编译 JS hook 脚本，检测是否定义了 onTaskCreate / onTaskFinish。
func (hm *HookManager) Load(scriptPath string) error {
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		return fmt.Errorf("读取 hook 脚本 %s 失败: %w", scriptPath, err)
	}

	vm := goja.New()
	registerBuiltins(vm)

	if _, err := vm.RunString(string(data)); err != nil {
		return fmt.Errorf("执行 hook 脚本失败: %w", err)
	}

	hm.vm = vm
	hm.hasCreateHook = isDefinedFunction(vm, "onTaskCreate")
	hm.hasFinishHook = isDefinedFunction(vm, "onTaskFinish")
	hm.hasFilenameHook = isDefinedFunction(vm, "onFilename")

	return nil
}

// InvokeCreateHook 调用 onTaskCreate，传入原始 task/resources/config 并返回修改结果。
// 返回 nil 表示无修改，应当保持原 task 不变。
func (hm *HookManager) InvokeCreateHook(input *TaskInput) (*TaskInput, error) {
	if !hm.hasCreateHook || hm.vm == nil {
		return nil, nil
	}

	hm.vm.Set("__basePath", input.Task.SavePath)

	fn, ok := goja.AssertFunction(hm.vm.Get("onTaskCreate"))
	if !ok {
		return nil, fmt.Errorf("onTaskCreate 不是函数")
	}

	taskVal := hm.vm.ToValue(input.Task)
	resourcesVal := hm.vm.ToValue(input.Resources)
	configVal := hm.vm.ToValue(input.Config)

	result, err := fn(goja.Undefined(), taskVal, resourcesVal, configVal)
	if err != nil {
		return nil, fmt.Errorf("onTaskCreate 执行失败: %w", err)
	}

	if result == nil || goja.IsUndefined(result) || goja.IsNull(result) {
		return nil, nil
	}

	exported := result.Export()
	jsonBytes, err := json.Marshal(exported)
	if err != nil {
		return nil, fmt.Errorf("序列化 onTaskCreate 结果失败: %w", err)
	}

	var modified TaskInput
	if err := json.Unmarshal(jsonBytes, &modified); err != nil {
		return nil, fmt.Errorf("反序列化 onTaskCreate 结果失败: %w", err)
	}

	return &modified, nil
}

// InvokeFinishHook 调用 onTaskFinish 执行下载后处理（zip、清理等）。
func (hm *HookManager) InvokeFinishHook(ctx *FinishContext) error {
	if !hm.hasFinishHook || hm.vm == nil {
		return nil
	}

	hm.vm.Set("__basePath", ctx.SavePath)

	fn, ok := goja.AssertFunction(hm.vm.Get("onTaskFinish"))
	if !ok {
		return fmt.Errorf("onTaskFinish 不是函数")
	}

	ctxVal := hm.vm.ToValue(ctx)
	_, err := fn(goja.Undefined(), ctxVal)
	if err != nil {
		return fmt.Errorf("onTaskFinish 执行失败: %w", err)
	}

	return nil
}

// InvokeFilenameHook 调用 onFilename，返回生成的文件名。
// systemName 是系统默认文件名（经过模板处理后的结果），用户可基于它调整。
// 返回空字符串表示沿用默认逻辑。
func (hm *HookManager) InvokeFilenameHook(params *FilenameParams, systemName string) (string, error) {
	if !hm.hasFilenameHook || hm.vm == nil {
		return "", nil
	}

	hm.vm.Set("__basePath", params.Task.SavePath)

	fn, ok := goja.AssertFunction(hm.vm.Get("onFilename"))
	if !ok {
		return "", fmt.Errorf("onFilename 不是函数")
	}

	systemVal := hm.vm.ToValue(systemName)
	metaVal := hm.vm.ToValue(params.Meta)
	taskVal := hm.vm.ToValue(params.Task)
	configVal := hm.vm.ToValue(params.Config)

	result, err := fn(goja.Undefined(), systemVal, metaVal, taskVal, configVal)
	if err != nil {
		return "", fmt.Errorf("onFilename 执行失败: %w", err)
	}

	if result == nil || goja.IsUndefined(result) || goja.IsNull(result) {
		return "", nil
	}

	s := strings.TrimSpace(result.String())
	return s, nil
}

func isDefinedFunction(vm *goja.Runtime, name string) bool {
	val := vm.Get(name)
	if val == nil || goja.IsUndefined(val) || goja.IsNull(val) {
		return false
	}
	_, ok := goja.AssertFunction(val)
	return ok
}
