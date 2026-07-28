package kernel

import (
	"sync/atomic"
)

type LegacyCallCounter struct {
	pluginStart      atomic.Int64
	pluginDispatch   atomic.Int64
	toolExecute      atomic.Int64
	packageInstall   atomic.Int64
	skillExecute     atomic.Int64
	mcpToolRegister  atomic.Int64
	scheduleTick     atomic.Int64
	toolExecuteCalls atomic.Int64
	modelToolsCalls  atomic.Int64
	promptHookCalls  atomic.Int64
	mcpExecuteCalls  atomic.Int64
}

func NewLegacyCallCounter() *LegacyCallCounter {
	return &LegacyCallCounter{}
}

func (c *LegacyCallCounter) IncPluginStart() {
	c.pluginStart.Add(1)
}

func (c *LegacyCallCounter) IncPluginDispatch() {
	c.pluginDispatch.Add(1)
}

func (c *LegacyCallCounter) IncToolExecute() {
	c.toolExecute.Add(1)
}

func (c *LegacyCallCounter) IncPackageInstall() {
	c.packageInstall.Add(1)
}

func (c *LegacyCallCounter) IncSkillExecute() {
	c.skillExecute.Add(1)
}

func (c *LegacyCallCounter) IncMCPToolRegister() {
	c.mcpToolRegister.Add(1)
}

func (c *LegacyCallCounter) IncScheduleTick() {
	c.scheduleTick.Add(1)
}

func (c *LegacyCallCounter) IncToolExecuteCalls() {
	c.toolExecuteCalls.Add(1)
}

func (c *LegacyCallCounter) IncModelToolsCalls() {
	c.modelToolsCalls.Add(1)
}

func (c *LegacyCallCounter) IncPromptHookCalls() {
	c.promptHookCalls.Add(1)
}

func (c *LegacyCallCounter) IncMCPExecute() {
	c.mcpExecuteCalls.Add(1)
}

func (c *LegacyCallCounter) ToolExecuteCalls() int64 {
	return c.toolExecuteCalls.Load()
}

func (c *LegacyCallCounter) ModelToolsCalls() int64 {
	return c.modelToolsCalls.Load()
}

func (c *LegacyCallCounter) PromptHookCalls() int64 {
	return c.promptHookCalls.Load()
}

func (c *LegacyCallCounter) MCPExecuteTotal() int64 {
	return c.mcpExecuteCalls.Load()
}

func (c *LegacyCallCounter) LegacyFallbackTotal() int64 {
	return c.toolExecuteCalls.Load() +
		c.modelToolsCalls.Load() +
		c.promptHookCalls.Load()
}

func (c *LegacyCallCounter) Total() int64 {
	return c.pluginStart.Load() +
		c.pluginDispatch.Load() +
		c.toolExecute.Load() +
		c.packageInstall.Load() +
		c.skillExecute.Load() +
		c.mcpToolRegister.Load() +
		c.scheduleTick.Load() +
		c.toolExecuteCalls.Load() +
		c.modelToolsCalls.Load() +
		c.promptHookCalls.Load() +
		c.mcpExecuteCalls.Load()
}

func (c *LegacyCallCounter) Snapshot() map[string]int64 {
	return map[string]int64{
		"legacy_plugin_start":         c.pluginStart.Load(),
		"legacy_plugin_dispatch":      c.pluginDispatch.Load(),
		"legacy_tool_execute":         c.toolExecute.Load(),
		"legacy_package_install":      c.packageInstall.Load(),
		"legacy_skill_execute":        c.skillExecute.Load(),
		"legacy_mcp_tool_register":    c.mcpToolRegister.Load(),
		"legacy_schedule_tick":        c.scheduleTick.Load(),
		"legacy_tool_execute_calls":   c.toolExecuteCalls.Load(),
		"legacy_model_tools_calls":    c.modelToolsCalls.Load(),
		"legacy_prompt_hook_calls":    c.promptHookCalls.Load(),
		"legacy_mcp_execute_calls":    c.mcpExecuteCalls.Load(),
		"legacy_fallback_total":       c.LegacyFallbackTotal(),
		"legacy_total":                c.Total(),
	}
}

var globalLegacyCallCounter = NewLegacyCallCounter()

func GlobalLegacyCallCounter() *LegacyCallCounter {
	return globalLegacyCallCounter
}
