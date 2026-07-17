package extension

import "sync"

const (
	agentSkillMetricImportTotal         = "agent_skill_import_total"
	agentSkillMetricImportFailure       = "agent_skill_import_failure_total"
	agentSkillMetricEnabled             = "agent_skill_enabled_total"
	agentSkillMetricActivation          = "agent_skill_activation_total"
	agentSkillMetricActivationFailure   = "agent_skill_activation_failure_total"
	agentSkillMetricResourceRead        = "agent_skill_resource_read_total"
	agentSkillMetricResourceReadFailure = "agent_skill_resource_read_failure_total"
	agentSkillMetricScriptDetected      = "agent_skill_script_detected_total"
	agentSkillMetricBlocked             = "agent_skill_blocked_total"
	agentSkillMetricPromptTokens        = "agent_skill_prompt_tokens"
	agentSkillMetricCatalogTokens       = "agent_skill_catalog_tokens"
	agentSkillMetricUnsupportedTool     = "agent_skill_tool_mapping_unsupported_total"
)

var agentSkillMetricNames = []string{agentSkillMetricImportTotal, agentSkillMetricImportFailure, agentSkillMetricEnabled, agentSkillMetricActivation, agentSkillMetricActivationFailure, agentSkillMetricResourceRead, agentSkillMetricResourceReadFailure, agentSkillMetricScriptDetected, agentSkillMetricBlocked, agentSkillMetricPromptTokens, agentSkillMetricCatalogTokens, agentSkillMetricUnsupportedTool}

var defaultAgentSkillMetrics = struct {
	sync.RWMutex
	values map[string]uint64
}{values: map[string]uint64{}}

func addAgentSkillMetric(name string, value uint64) {
	defaultAgentSkillMetrics.Lock()
	defaultAgentSkillMetrics.values[name] += value
	defaultAgentSkillMetrics.Unlock()
}

func agentSkillMetricsSnapshot() map[string]uint64 {
	defaultAgentSkillMetrics.RLock()
	defer defaultAgentSkillMetrics.RUnlock()
	result := make(map[string]uint64, len(agentSkillMetricNames))
	for _, name := range agentSkillMetricNames {
		result[name] = defaultAgentSkillMetrics.values[name]
	}
	return result
}
