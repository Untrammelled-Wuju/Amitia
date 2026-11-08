package capabilities

import (
	"sort"
	"strings"
)

type Definition struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

var definitions = []Definition{
	{ID: "ai.chat", Label: "文本与对话", Description: "允许统筹中心路由文本对话和统筹数据请求"},
	{ID: "ai.vision", Label: "识图", Description: "允许统筹中心调用设备识图相关能力"},
	{ID: "ai.speech", Label: "语音", Description: "允许统筹中心调用设备语音识别与语音合成能力"},
	{ID: "ai.image", Label: "图像生成", Description: "允许统筹中心调用设备图像生成能力"},
	{ID: "task.execute", Label: "主动任务", Description: "允许统筹中心调度设备任务"},
	{ID: "mcp.invoke", Label: "MCP", Description: "允许统筹中心调用设备 MCP 能力"},
	{ID: "skill.invoke", Label: "Skill", Description: "允许统筹中心调用设备 Skill"},
	{ID: "workflow.execute", Label: "工作流", Description: "允许统筹中心调用设备工作流"},
	{ID: "extension.invoke", Label: "扩展能力", Description: "允许统筹中心调用设备扩展包能力"},
	{ID: "browser.invoke", Label: "浏览器", Description: "允许统筹中心读取和操作设备浏览器"},
	{ID: "desktop.pet", Label: "桌宠", Description: "允许统筹中心调用设备桌宠运行时"},
	{ID: "game.execute", Label: "游戏模式", Description: "允许统筹中心调用设备游戏运行时"},
}

var definitionsByID = func() map[string]Definition {
	byID := make(map[string]Definition, len(definitions))
	for _, item := range definitions {
		byID[item.ID] = item
	}
	return byID
}()

func All() []Definition {
	result := make([]Definition, len(definitions))
	copy(result, definitions)
	return result
}

func IDs() []string {
	result := make([]string, 0, len(definitions))
	for _, item := range definitions {
		result = append(result, item.ID)
	}
	return result
}

func Lookup(id string) (Definition, bool) {
	item, ok := definitionsByID[strings.TrimSpace(id)]
	return item, ok
}

func Valid(id string) bool {
	_, ok := Lookup(id)
	return ok
}

func Normalize(ids []string) ([]string, error) {
	seen := make(map[string]struct{}, len(ids))
	result := make([]string, 0, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if !Valid(id) {
			return nil, &InvalidCapabilityError{ID: raw}
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	sort.Strings(result)
	return result, nil
}

type InvalidCapabilityError struct {
	ID string
}

func (e *InvalidCapabilityError) Error() string {
	return "不支持的能力标识: " + e.ID
}

func ForHandler(runtimeType, handler string) string {
	handler = strings.TrimSpace(handler)
	switch {
	case strings.HasPrefix(handler, "workflow."):
		return "workflow.execute"
	case strings.HasPrefix(handler, "desktop_pet."):
		return "desktop.pet"
	case strings.HasPrefix(handler, "gamehost."):
		return "game.execute"
	case handler == "workbench.browser.observe":
		return "browser.invoke"
	case handler == "task.host.source-event":
		return "task.execute"
	case runtimeType == "game_host":
		return "game.execute"
	case runtimeType == "task":
		return "task.execute"
	case runtimeType == "workflow":
		return "workflow.execute"
	case runtimeType == "mcp":
		return "mcp.invoke"
	case runtimeType == "browser":
		return "browser.invoke"
	case runtimeType == "plugin_js", runtimeType == "plugin_service", runtimeType == "javascript",
		runtimeType == "wasm", runtimeType == "trusted_service", runtimeType == "desktop_extension",
		runtimeType == "workspace", runtimeType == "channel", runtimeType == "android_native",
		runtimeType == "android_linux", runtimeType == "ios_native", runtimeType == "background_removal",
		runtimeType == "media":
		return "extension.invoke"
	}
	return "ai.chat"
}
