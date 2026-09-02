package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func ComputeDefinitionHash(def WorkflowDefinition) string {
	payload := map[string]any{
		"schemaVersion":     def.SchemaVersion,
		"id":                def.ID,
		"extensionId":       def.ExtensionID,
		"moduleId":          def.ModuleID,
		"name":              def.Name,
		"description":       def.Description,
		"inputSchema":       def.InputSchema,
		"outputSchema":      def.OutputSchema,
		"nodes":             def.Nodes,
		"edges":             def.Edges,
		"triggers":          def.Triggers,
		"permissions":       def.Permissions,
		"scope":             def.Scope,
		"callableByAgent":   def.CallableByAgent,
		"agentTool":         def.AgentTool,
		"enabled":           def.Enabled,
		"hasSideEffects":    def.HasSideEffects,
		"idempotent":        def.Idempotent,
		"limits":            def.Limits,
		"concurrencyPolicy": def.ConcurrencyPolicy.Normalize(),
		"version":           def.Version,
		"source":            def.Source,
		"metadata":          def.Metadata,
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
