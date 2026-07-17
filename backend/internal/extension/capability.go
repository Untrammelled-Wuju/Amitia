package extension

type CapabilityDefinition struct {
	Name        string `json:"name"`
	Risk        string `json:"risk"`
	Description string `json:"description"`
}

var capabilityCatalog = map[string]CapabilityDefinition{
	"runtime.time.read":         {Name: "runtime.time.read", Risk: "low", Description: "读取宿主当前时间"},
	"runtime.character.read":    {Name: "runtime.character.read", Risk: "low", Description: "读取当前角色只读视图"},
	"runtime.relationship.read": {Name: "runtime.relationship.read", Risk: "low", Description: "读取当前角色关系只读视图"},
	"runtime.emotion.read":      {Name: "runtime.emotion.read", Risk: "low", Description: "读取当前角色情绪只读视图"},
	"context.inject":            {Name: "context.inject", Risk: "high", Description: "向当前会话上下文注入内容"},
	"memory.read":               {Name: "memory.read", Risk: "low", Description: "读取当前角色授权记忆"},
	"memory.candidate.write":    {Name: "memory.candidate.write", Risk: "high", Description: "通过宿主服务写入记忆候选"},
	"network.https":             {Name: "network.https", Risk: "high", Description: "访问 HTTPS 网络资源"},
	"scheduler.own.manage":      {Name: "scheduler.own.manage", Risk: "medium", Description: "管理当前作用域的提醒"},
	"notification.send":         {Name: "notification.send", Risk: "medium", Description: "发送宿主通知"},
	"channel.message.send":      {Name: "channel.message.send", Risk: "high", Description: "向当前渠道发送消息"},
	"storage.own.read":          {Name: "storage.own.read", Risk: "low", Description: "读取技能自有存储"},
	"storage.own.write":         {Name: "storage.own.write", Risk: "medium", Description: "写入技能自有存储"},
	"surface.render":            {Name: "surface.render", Risk: "medium", Description: "渲染宿主允许的界面表面"},
	"event.own.emit":            {Name: "event.own.emit", Risk: "low", Description: "发送当前插件命名空间事件"},
	"clipboard.read":            {Name: "clipboard.read", Risk: "high", Description: "读取系统剪贴板"},
	"system.foreground.read":    {Name: "system.foreground.read", Risk: "high", Description: "读取前台应用信息"},
}

func Capability(name string) (CapabilityDefinition, bool) {
	item, ok := capabilityCatalog[name]
	return item, ok
}

func Capabilities() []CapabilityDefinition {
	items := make([]CapabilityDefinition, 0, len(capabilityCatalog))
	for _, item := range capabilityCatalog {
		items = append(items, item)
	}
	return items
}
