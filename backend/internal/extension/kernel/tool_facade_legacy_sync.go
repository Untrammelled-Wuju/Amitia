package kernel

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type LegacyToolSyncResult struct {
	Registered int
	Skipped    int
	Removed    int
	Total      int
}

func (f *ToolFacade) SyncLegacyTools(ctx context.Context, scope LegacyScope) (*LegacyToolSyncResult, error) {
	if f.legacy == nil {
		return &LegacyToolSyncResult{}, nil
	}
	legacyTools, err := f.legacy.ModelTools(ctx, scope)
	if err != nil {
		return nil, err
	}
	result := &LegacyToolSyncResult{Total: len(legacyTools)}
	if len(legacyTools) == 0 {
		return result, nil
	}
	seen := make(map[string]bool, len(legacyTools))
	for _, t := range legacyTools {
		if t.Function.Name == "" {
			result.Skipped++
			continue
		}
		toolID := "legacy:" + t.Function.Name
		seen[toolID] = true
		def := capability.ToolDefinition{
			ID:          toolID,
			ModelName:   t.Function.Name,
			Source:      capability.ToolSourceLegacy,
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Enabled:     true,
			Compatible:  true,
			InputSchema: parametersToSchema(t.Function.Parameters),
			Runtime: capability.RuntimeBinding{
				RuntimeType: capability.RuntimeTypeLegacy,
				HandlerName: t.Function.Name,
			},
			State: capability.ToolState{
				Installed:         true,
				ModuleEnabled:     true,
				CapabilityEnabled: true,
				ScopeAllowed:      true,
				PermissionGranted: true,
				RuntimeReady:      true,
				DependencyReady:   true,
				Health:            capability.HealthReady,
			},
			ModelExposure: capability.ModelExposureRule{
				ExposedByDefault: true,
			},
		}
		if err := f.toolRegistry.Replace(ctx, def); err != nil {
			result.Skipped++
			continue
		}
		result.Registered++
	}
	return result, nil
}

func parametersToSchema(params interface{}) []byte {
	if params == nil {
		return []byte(`{"type":"object","properties":{}}`)
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return []byte(`{"type":"object","properties":{}}`)
	}
	if len(raw) == 0 {
		return []byte(`{"type":"object","properties":{}}`)
	}
	return raw
}
