package extension

type CapabilityDefinition struct {
	Name        string `json:"name"`
	Risk        string `json:"risk"`
	Description string `json:"description"`
}

var capabilityCatalog = map[string]CapabilityDefinition{
	"runtime.time.read":          {Name: "runtime.time.read", Risk: "low", Description: "读取宿主当前时间"},
	"runtime.character.read":     {Name: "runtime.character.read", Risk: "low", Description: "读取当前角色只读视图"},
	"runtime.relationship.read":  {Name: "runtime.relationship.read", Risk: "low", Description: "读取当前角色关系只读视图"},
	"runtime.emotion.read":       {Name: "runtime.emotion.read", Risk: "low", Description: "读取当前角色情绪只读视图"},
	"context.inject":             {Name: "context.inject", Risk: "high", Description: "向当前会话上下文注入内容"},
	"memory.read":                {Name: "memory.read", Risk: "low", Description: "读取当前角色授权记忆"},
	"memory.candidate.write":     {Name: "memory.candidate.write", Risk: "high", Description: "通过宿主服务写入记忆候选"},
	"temporal.context.read":      {Name: "temporal.context.read", Risk: "low", Description: "读取当前作用域的时间上下文"},
	"temporal.anchor.read":       {Name: "temporal.anchor.read", Risk: "low", Description: "读取当前作用域的时间锚点"},
	"temporal.anchor.manage":     {Name: "temporal.anchor.manage", Risk: "high", Description: "创建和管理候选时间锚点"},
	"temporal.event.subscribe":   {Name: "temporal.event.subscribe", Risk: "medium", Description: "订阅当前作用域的时间事件"},
	"temporal.proactive.request": {Name: "temporal.proactive.request", Risk: "high", Description: "请求时间相关主动消息候选"},
	"network.https":              {Name: "network.https", Risk: "high", Description: "访问 HTTPS 网络资源"},
	"scheduler.own.manage":       {Name: "scheduler.own.manage", Risk: "medium", Description: "管理当前作用域的提醒"},
	"notification.send":          {Name: "notification.send", Risk: "medium", Description: "发送宿主通知"},
	"channel.message.send":       {Name: "channel.message.send", Risk: "high", Description: "向当前渠道发送消息"},
	"storage.own.read":           {Name: "storage.own.read", Risk: "low", Description: "读取技能自有存储"},
	"storage.own.write":          {Name: "storage.own.write", Risk: "medium", Description: "写入技能自有存储"},
	"surface.render":             {Name: "surface.render", Risk: "medium", Description: "渲染宿主允许的界面表面"},
	"event.own.emit":             {Name: "event.own.emit", Risk: "low", Description: "发送当前插件命名空间事件"},
	"clipboard.read":             {Name: "clipboard.read", Risk: "high", Description: "读取系统剪贴板"},
	"clipboard.write":            {Name: "clipboard.write", Risk: "medium", Description: "写入系统剪贴板"},
	"system.foreground.read":     {Name: "system.foreground.read", Risk: "high", Description: "读取前台应用信息"},
	"mcp.invoke":                 {Name: "mcp.invoke", Risk: "high", Description: "调用外部 MCP 工具"},
	"network.remote":             {Name: "network.remote", Risk: "high", Description: "访问远程服务"},
	"filesystem.read":            {Name: "filesystem.read", Risk: "high", Description: "读取文件系统"},
	"filesystem.write":           {Name: "filesystem.write", Risk: "high", Description: "写入文件系统"},
	"external.account.read":      {Name: "external.account.read", Risk: "high", Description: "读取外部账户数据"},
	"external.account.write":     {Name: "external.account.write", Risk: "high", Description: "修改外部账户数据"},
	"message.send":               {Name: "message.send", Risk: "high", Description: "通过外部服务发送消息"},
	"data.delete":                {Name: "data.delete", Risk: "high", Description: "删除外部数据"},
	"financial.action":           {Name: "financial.action", Risk: "high", Description: "执行金融操作"},
}

func Capability(name string) (CapabilityDefinition, bool) {
	item, ok := capabilityCatalog[name]
	if !ok && len(name) > len("mcp.server.") && name[:len("mcp.server.")] == "mcp.server." {
		return CapabilityDefinition{Name: name, Risk: "high", Description: "访问指定 MCP Server"}, true
	}
	if !ok && len(name) > len("mcp.tool.") && name[:len("mcp.tool.")] == "mcp.tool." {
		return CapabilityDefinition{Name: name, Risk: "high", Description: "调用指定 MCP Tool"}, true
	}
	return item, ok
}

func Capabilities() []CapabilityDefinition {
	items := make([]CapabilityDefinition, 0, len(capabilityCatalog))
	for _, item := range capabilityCatalog {
		items = append(items, item)
	}
	return items
}
