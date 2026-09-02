package kernel

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type builtinMemorySandboxToolSpec struct {
	id, modelName, name, description, input string
	permissions                             []capability.PermissionRequirement
	risk                                    capability.RiskLevel
	side                                    capability.SideEffectLevel
	approval, background, idempotent        bool
	timeout                                 time.Duration
	maxOutput                               int
}

func registerBuiltinMemorySandboxTools(ctx context.Context, registry *capability.ToolRegistry) error {
	if registry == nil {
		return nil
	}
	object := `{"type":"object","additionalProperties":true}`
	specs := []builtinMemorySandboxToolSpec{
		{id: "builtin.memory.query", modelName: "query_memory", name: "Query Memory", description: "Hybrid semantic/keyword memory retrieval with type, layer, importance and temporal filters in the current character scope.", input: `{"type":"object","required":["query"],"additionalProperties":false,"properties":{"query":{"type":"string","minLength":1,"maxLength":4096},"mode":{"type":"string","enum":["hybrid","keyword","vector"]},"limit":{"type":"integer","minimum":1,"maximum":50},"min_importance":{"type":"integer","minimum":0,"maximum":10},"types":{"type":"array","maxItems":32,"items":{"type":"string"}},"layers":{"type":"array","maxItems":6,"items":{"type":"string","enum":["fact","profile","episodic","working","worldbook","graph"]}},"time_basis":{"type":"string","enum":["occurred","validity","created","updated","last_used"]},"from":{"type":"string"},"to":{"type":"string"},"at":{"type":"string"},"local_date_from":{"type":"string"},"local_date_to":{"type":"string"},"include_unknown_time":{"type":"boolean"}}}`, permissions: []capability.PermissionRequirement{{Capability: "memory.read", Risk: string(capability.RiskMedium)}}, risk: capability.RiskMedium, side: capability.SideEffectReadOnly, background: true, idempotent: true, timeout: 30 * time.Second, maxOutput: 256 * 1024},
		{id: "builtin.memory.get_by_title", modelName: "get_memory_by_title", name: "Get Memory By Title", description: "Read exact memory title/key content with bounded chunking and optional in-document query.", input: `{"type":"object","required":["title"],"additionalProperties":false,"properties":{"title":{"type":"string","minLength":1,"maxLength":512},"query":{"type":"string","maxLength":2048},"offset":{"type":"integer","minimum":0,"maximum":10000000},"max_chars":{"type":"integer","minimum":1,"maximum":50000}}}`, permissions: []capability.PermissionRequirement{{Capability: "memory.read", Risk: string(capability.RiskMedium)}}, risk: capability.RiskMedium, side: capability.SideEffectReadOnly, background: true, idempotent: true, timeout: 15 * time.Second, maxOutput: 128 * 1024},
		{id: "builtin.sandbox.environment.read", modelName: "read_environment_variable", name: "Read Sandbox Environment Variable", description: "Read one sandbox-package environment variable or list names in the current scope. Does not expose the backend process environment.", input: `{"type":"object","additionalProperties":false,"properties":{"name":{"type":"string","pattern":"^[A-Za-z_][A-Za-z0-9_]{0,127}$"}}}`, permissions: []capability.PermissionRequirement{{Capability: "secrets.read", Risk: string(capability.RiskHigh)}}, risk: capability.RiskHigh, side: capability.SideEffectReadOnly, approval: true, background: false, idempotent: true, timeout: 5 * time.Second, maxOutput: 128 * 1024},
		{id: "builtin.sandbox.environment.write", modelName: "write_environment_variable", name: "Write Sandbox Environment Variable", description: "Create, update, or delete one scoped sandbox-package environment variable without mutating the backend process environment.", input: `{"type":"object","required":["name"],"additionalProperties":false,"properties":{"name":{"type":"string","pattern":"^[A-Za-z_][A-Za-z0-9_]{0,127}$"},"value":{"type":"string","maxLength":65536},"delete":{"type":"boolean"}}}`, permissions: []capability.PermissionRequirement{{Capability: "secrets.write", Risk: string(capability.RiskHigh)}}, risk: capability.RiskHigh, side: capability.SideEffectSystem, approval: true, background: false, idempotent: true, timeout: 5 * time.Second, maxOutput: 4096},
		{id: "builtin.sandbox.script.direct", modelName: "execute_sandbox_script_direct", name: "Execute Sandbox Script Direct", description: "Execute inline JavaScript in a restricted Node VM with no require, process, filesystem, network, child-process, WASM, eval or Function access.", input: `{"type":"object","required":["script"],"additionalProperties":false,"properties":{"script":{"type":"string","minLength":1,"maxLength":131072},"input":{},"timeout_ms":{"type":"integer","minimum":100,"maximum":30000}}}`, permissions: []capability.PermissionRequirement{{Capability: "service.runtime.execute", Risk: string(capability.RiskHigh)}}, risk: capability.RiskHigh, side: capability.SideEffectSystem, approval: true, background: false, idempotent: false, timeout: 32 * time.Second, maxOutput: 1024 * 1024},
	}
	for _, spec := range specs {
		def := capability.ToolDefinition{
			ID: spec.id, ModelName: spec.modelName, Source: capability.ToolSourceBuiltin, Name: spec.name, Description: spec.description,
			InputSchema: json.RawMessage(spec.input), OutputSchema: json.RawMessage(object), Permissions: spec.permissions,
			RiskLevel: spec.risk, SideEffect: spec.side, HasSideEffects: spec.side != capability.SideEffectReadOnly && spec.side != capability.SideEffectNone,
			Idempotent: spec.idempotent, Retryable: spec.idempotent, TimeoutMS: spec.timeout.Milliseconds(), Enabled: true, Compatible: true,
			ToolVersion: capability.ToolVersion{SchemaVersion: 1, Revision: "builtin-v1"}, ModelExposure: capability.ModelExposureRule{ExposedByDefault: true, Categories: []string{"memory", "sandbox", "automation"}, Priority: 44},
			ExecutionPolicy: capability.ToolExecutionPolicy{Timeout: spec.timeout, MaxConcurrency: 2, Idempotent: spec.idempotent, ApprovalRequired: spec.approval, AllowBackground: spec.background},
			ResultPolicy:    capability.ToolResultPolicy{SanitizeError: true, MaxOutputBytes: spec.maxOutput}, Runtime: capability.RuntimeBinding{RuntimeType: capability.RuntimeTypeBuiltin, RuntimeID: "builtin", HandlerName: spec.modelName},
		}
		if err := registry.Register(ctx, def); err != nil {
			return fmt.Errorf("register builtin memory/sandbox tool %s: %w", spec.modelName, err)
		}
	}
	return nil
}
