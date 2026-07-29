package kernel

import (
	"context"
	"sync/atomic"
)

type LegacyCallCounter struct {
	pluginStart                        atomic.Int64
	pluginDispatch                     atomic.Int64
	toolExecute                        atomic.Int64
	packageInstall                     atomic.Int64
	skillExecute                       atomic.Int64
	mcpToolRegister                    atomic.Int64
	scheduleTick                       atomic.Int64
	toolExecuteCalls                   atomic.Int64
	modelToolsCalls                    atomic.Int64
	promptHookCalls                    atomic.Int64
	mcpExecuteCalls                    atomic.Int64
	duplicateMCPToolRegistrations      atomic.Int64
	packageWriteCalls                  atomic.Int64
	duplicateContributionRegistrations atomic.Int64
	orphanRuntimeInstances             atomic.Int64
	orphanUISessions                   atomic.Int64
	failedCleanupResources             atomic.Int64
	duplicateMCPFromRegistry           atomic.Int64
	store                              *LegacyCounterStore
}

func NewLegacyCallCounter() *LegacyCallCounter {
	return &LegacyCallCounter{}
}

func NewLegacyCallCounterWithStore(store *LegacyCounterStore) *LegacyCallCounter {
	return &LegacyCallCounter{store: store}
}

func (c *LegacyCallCounter) SetStore(store *LegacyCounterStore) {
	c.store = store
}

func (c *LegacyCallCounter) LoadFromStore(ctx context.Context) error {
	if c.store == nil {
		return nil
	}
	return c.store.LoadAll(ctx)
}

func (c *LegacyCallCounter) syncToStore(metricName string) {
	if c.store == nil {
		return
	}
	_, _ = c.store.Increment(context.Background(), metricName)
}

func (c *LegacyCallCounter) setStoreValue(metricName string, value int64) {
	if c.store == nil {
		return
	}
	_ = c.store.Set(context.Background(), metricName, value)
}

func (c *LegacyCallCounter) storeGet(metricName string) int64 {
	if c.store == nil {
		return 0
	}
	return c.store.Get(context.Background(), metricName)
}

func (c *LegacyCallCounter) IncPluginStart() {
	c.pluginStart.Add(1)
	c.syncToStore("legacy_plugin_start")
}

func (c *LegacyCallCounter) IncPluginDispatch() {
	c.pluginDispatch.Add(1)
	c.syncToStore("legacy_plugin_dispatch")
}

func (c *LegacyCallCounter) IncToolExecute() {
	c.toolExecute.Add(1)
	c.syncToStore("legacy_tool_execute")
}

func (c *LegacyCallCounter) IncPackageInstall() {
	c.packageInstall.Add(1)
	c.syncToStore("legacy_package_install")
}

func (c *LegacyCallCounter) IncSkillExecute() {
	c.skillExecute.Add(1)
	c.syncToStore("legacy_skill_execute")
}

func (c *LegacyCallCounter) IncMCPToolRegister() {
	c.mcpToolRegister.Add(1)
	c.syncToStore("legacy_mcp_tool_register")
}

func (c *LegacyCallCounter) IncScheduleTick() {
	c.scheduleTick.Add(1)
	c.syncToStore("legacy_schedule_tick")
}

func (c *LegacyCallCounter) IncToolExecuteCalls() {
	c.toolExecuteCalls.Add(1)
	c.syncToStore("legacy_tool_execute_calls")
}

func (c *LegacyCallCounter) IncModelToolsCalls() {
	c.modelToolsCalls.Add(1)
	c.syncToStore("legacy_model_tools_calls")
}

func (c *LegacyCallCounter) IncPromptHookCalls() {
	c.promptHookCalls.Add(1)
	c.syncToStore("legacy_prompt_hook_calls")
}

func (c *LegacyCallCounter) IncMCPExecute() {
	c.mcpExecuteCalls.Add(1)
	c.syncToStore("legacy_mcp_execute_calls")
}

func (c *LegacyCallCounter) IncDuplicateMCPToolRegistration() {
	c.duplicateMCPToolRegistrations.Add(1)
	c.syncToStore("duplicate_mcp_tool_registrations")
}

func (c *LegacyCallCounter) IncPackageWriteCalls() {
	c.packageWriteCalls.Add(1)
	c.syncToStore("legacy_package_write_calls")
}

func (c *LegacyCallCounter) IncDuplicateContributionRegistration() {
	c.duplicateContributionRegistrations.Add(1)
	c.syncToStore("duplicate_contribution_registrations")
}

func (c *LegacyCallCounter) IncOrphanRuntimeInstance() {
	c.orphanRuntimeInstances.Add(1)
	c.syncToStore("orphan_runtime_instances")
}

func (c *LegacyCallCounter) SetOrphanRuntimeInstances(n int64) {
	c.orphanRuntimeInstances.Store(n)
	c.setStoreValue("orphan_runtime_instances", n)
}

func (c *LegacyCallCounter) IncOrphanUISession() {
	c.orphanUISessions.Add(1)
	c.syncToStore("orphan_ui_sessions")
}

func (c *LegacyCallCounter) IncFailedCleanupResource() {
	c.failedCleanupResources.Add(1)
	c.syncToStore("failed_cleanup_resources")
}

func (c *LegacyCallCounter) ToolExecuteCalls() int64 {
	if c.store != nil {
		return c.storeGet("legacy_tool_execute_calls")
	}
	return c.toolExecuteCalls.Load()
}

func (c *LegacyCallCounter) ModelToolsCalls() int64 {
	if c.store != nil {
		return c.storeGet("legacy_model_tools_calls")
	}
	return c.modelToolsCalls.Load()
}

func (c *LegacyCallCounter) PromptHookCalls() int64 {
	if c.store != nil {
		return c.storeGet("legacy_prompt_hook_calls")
	}
	return c.promptHookCalls.Load()
}

func (c *LegacyCallCounter) MCPExecuteTotal() int64 {
	if c.store != nil {
		return c.storeGet("legacy_mcp_execute_calls")
	}
	return c.mcpExecuteCalls.Load()
}

func (c *LegacyCallCounter) DuplicateMCPToolRegistrations() int64 {
	if c.store != nil {
		return c.storeGet("duplicate_mcp_tool_registrations")
	}
	return c.duplicateMCPToolRegistrations.Load()
}

func (c *LegacyCallCounter) PackageWriteCalls() int64 {
	if c.store != nil {
		return c.storeGet("legacy_package_write_calls")
	}
	return c.packageWriteCalls.Load()
}

func (c *LegacyCallCounter) DuplicateContributionRegistrations() int64 {
	if c.store != nil {
		return c.storeGet("duplicate_contribution_registrations")
	}
	return c.duplicateContributionRegistrations.Load()
}

func (c *LegacyCallCounter) OrphanRuntimeInstances() int64 {
	if c.store != nil {
		return c.storeGet("orphan_runtime_instances")
	}
	return c.orphanRuntimeInstances.Load()
}

func (c *LegacyCallCounter) OrphanUISessions() int64 {
	if c.store != nil {
		return c.storeGet("orphan_ui_sessions")
	}
	return c.orphanUISessions.Load()
}

func (c *LegacyCallCounter) FailedCleanupResources() int64 {
	if c.store != nil {
		return c.storeGet("failed_cleanup_resources")
	}
	return c.failedCleanupResources.Load()
}

func (c *LegacyCallCounter) SetDuplicateMCPFromRegistry(n int64) {
	c.duplicateMCPFromRegistry.Store(n)
	c.setStoreValue("duplicate_mcp_from_registry", n)
}

func (c *LegacyCallCounter) DuplicateMCPFromRegistry() int64 {
	if c.store != nil {
		return c.storeGet("duplicate_mcp_from_registry")
	}
	return c.duplicateMCPFromRegistry.Load()
}

func (c *LegacyCallCounter) FinalGateMetrics() map[string]int64 {
	if c.store != nil {
		return map[string]int64{
			"legacy_tool_execute_calls":            c.storeGet("legacy_tool_execute_calls"),
			"legacy_mcp_execute_calls":             c.storeGet("legacy_mcp_execute_calls"),
			"legacy_package_write_calls":           c.storeGet("legacy_package_write_calls"),
			"legacy_package_read_calls":            globalLegacyReadCounter.PackageReadCallsFallbacks(),
			"duplicate_contribution_registrations": c.storeGet("duplicate_contribution_registrations"),
			"orphan_runtime_instances":             c.storeGet("orphan_runtime_instances"),
			"orphan_ui_sessions":                   c.storeGet("orphan_ui_sessions"),
			"failed_cleanup_resources":             c.storeGet("failed_cleanup_resources"),
			"duplicate_mcp_tool_registrations":     c.storeGet("duplicate_mcp_from_registry"),
		}
	}
	return map[string]int64{
		"legacy_tool_execute_calls":            c.toolExecuteCalls.Load(),
		"legacy_mcp_execute_calls":             c.mcpExecuteCalls.Load(),
		"legacy_package_write_calls":           c.packageWriteCalls.Load(),
		"legacy_package_read_calls":            globalLegacyReadCounter.PackageReadCallsFallbacks(),
		"duplicate_contribution_registrations": c.duplicateContributionRegistrations.Load(),
		"orphan_runtime_instances":             c.orphanRuntimeInstances.Load(),
		"orphan_ui_sessions":                   c.orphanUISessions.Load(),
		"failed_cleanup_resources":             c.failedCleanupResources.Load(),
		"duplicate_mcp_tool_registrations":     c.duplicateMCPFromRegistry.Load(),
	}
}

func (c *LegacyCallCounter) FinalGatePassed() bool {
	for _, v := range c.FinalGateMetrics() {
		if v != 0 {
			return false
		}
	}
	return true
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
		c.mcpExecuteCalls.Load() +
		c.duplicateMCPToolRegistrations.Load() +
		c.packageWriteCalls.Load() +
		c.duplicateContributionRegistrations.Load() +
		c.orphanRuntimeInstances.Load() +
		c.orphanUISessions.Load() +
		c.failedCleanupResources.Load()
}

func (c *LegacyCallCounter) Snapshot() map[string]int64 {
	if c.store != nil {
		return c.store.Snapshot(context.Background())
	}
	return map[string]int64{
		"legacy_plugin_start":                  c.pluginStart.Load(),
		"legacy_plugin_dispatch":               c.pluginDispatch.Load(),
		"legacy_tool_execute":                  c.toolExecute.Load(),
		"legacy_package_install":               c.packageInstall.Load(),
		"legacy_skill_execute":                 c.skillExecute.Load(),
		"legacy_mcp_tool_register":             c.mcpToolRegister.Load(),
		"legacy_schedule_tick":                 c.scheduleTick.Load(),
		"legacy_tool_execute_calls":            c.toolExecuteCalls.Load(),
		"legacy_model_tools_calls":             c.modelToolsCalls.Load(),
		"legacy_prompt_hook_calls":             c.promptHookCalls.Load(),
		"legacy_mcp_execute_calls":             c.mcpExecuteCalls.Load(),
		"duplicate_mcp_tool_registrations":     c.duplicateMCPToolRegistrations.Load(),
		"legacy_package_write_calls":           c.packageWriteCalls.Load(),
		"legacy_package_read_calls":            globalLegacyReadCounter.PackageReadCallsFallbacks(),
		"duplicate_contribution_registrations": c.duplicateContributionRegistrations.Load(),
		"orphan_runtime_instances":             c.orphanRuntimeInstances.Load(),
		"orphan_ui_sessions":                   c.orphanUISessions.Load(),
		"failed_cleanup_resources":             c.failedCleanupResources.Load(),
		"legacy_fallback_total":                c.LegacyFallbackTotal(),
		"legacy_total":                         c.Total(),
	}
}

var globalLegacyCallCounter = NewLegacyCallCounter()

func GlobalLegacyCallCounter() *LegacyCallCounter {
	return globalLegacyCallCounter
}
