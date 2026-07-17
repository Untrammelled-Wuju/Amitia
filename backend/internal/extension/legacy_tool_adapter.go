package extension

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/agent/tool"
)

var legacyNameNormalizer = regexp.MustCompile(`[^a-z0-9]+`)

type LegacyToolAdapter struct{}

func NewLegacyToolAdapter() *LegacyToolAdapter {
	return &LegacyToolAdapter{}
}

func (a *LegacyToolAdapter) RegisterAll(ctx context.Context, registry SkillRegistry) ([]string, error) {
	registered := make([]string, 0)
	for _, legacy := range tool.GetAll() {
		definition, handler, err := a.Adapt(legacy, false)
		if err != nil {
			return registered, err
		}
		if err := registry.Register(ctx, definition, handler); err != nil {
			return registered, err
		}
		registered = append(registered, definition.ID)
	}
	for _, legacy := range tool.GetMemoryTools() {
		definition, handler, err := a.Adapt(legacy, true)
		if err != nil {
			return registered, err
		}
		if err := registry.Register(ctx, definition, handler); err != nil {
			return registered, err
		}
		registered = append(registered, definition.ID)
	}
	return registered, nil
}

func (a *LegacyToolAdapter) Adapt(legacy tool.Tool, memoryTool bool) (SkillDefinition, SkillHandler, error) {
	id := "dev.amitia.skill." + normalizeLegacyName(legacy.Function.Name)
	inputSchema, err := legacyInputSchema(legacy.Function.Parameters)
	if err != nil {
		return SkillDefinition{}, nil, err
	}
	outputSchema := legacyOutputSchema()
	triggers := []SkillTrigger{TriggerManual}
	if !memoryTool {
		triggers = append([]SkillTrigger{TriggerLLM}, triggers...)
	}
	capabilities, sideEffects, idempotent := legacyCapabilities(legacy.Function.Name)
	manifest := Manifest{
		Schema: "https://schemas.amitia.dev/extensions/v1/manifest.schema.json", APIVersion: "extensions.amitia.dev/v1alpha1", Kind: "Skill",
		Metadata:      ManifestMetadata{ID: id, Name: legacy.Function.Name, Version: "1.0.0", Description: legacy.Function.Description, Author: "Amitia", License: "AGPL-3.0-only"},
		Compatibility: ManifestCompatibility{EngineMin: "1.0.0", EngineMaxExclusive: "2.0.0"},
		Entry:         SkillEntry{Kind: "legacy_tool", Name: legacy.Function.Name}, Capabilities: capabilities, Triggers: triggers,
		Execution:   ManifestExecution{TimeoutMS: 5000, HasSideEffects: sideEffects, Retryable: false, Idempotent: idempotent},
		InputSchema: inputSchema, OutputSchema: outputSchema,
		ConfigSchema:  json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{}}`),
		DefaultConfig: json.RawMessage(`{}`), Enabled: true, AllowLLM: !memoryTool, AllowManual: true,
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		return SkillDefinition{}, nil, err
	}
	definition := SkillDefinition{
		ID: id, ModelName: legacy.Function.Name, Name: legacy.Function.Name, Description: legacy.Function.Description, Version: "1.0.0",
		Source: SkillSourceLegacy, Entry: manifest.Entry, InputSchema: inputSchema, OutputSchema: outputSchema,
		ConfigSchema: manifest.ConfigSchema, DefaultConfig: manifest.DefaultConfig, Capabilities: capabilities, Triggers: triggers,
		Timeout: 5 * time.Second, TimeoutMS: 5000, HasSideEffects: sideEffects, Retryable: false, Idempotent: idempotent,
		Enabled: true, Compatible: true, Author: "Amitia", License: "AGPL-3.0-only", Manifest: manifestRaw,
	}
	handler := func(callCtx context.Context, request ExecuteSkillRequest) (SkillResult, error) {
		execCtx := tool.ToolExecutionContext{
			Context: callCtx, ConversationID: request.Scope.ConversationID, CharacterID: request.Scope.CharacterID,
			Channel: request.Scope.Channel, RequestID: request.Scope.RequestID, CorrelationID: request.Scope.CorrelationID,
			CausationID: request.Scope.CausationID, User: request.Scope.UserID, Path: "extension.skill.legacy_tool", ToolCallID: request.Scope.ToolCallID,
			IdempotencyKey: request.IdempotencyKey,
		}
		var result tool.ToolCallResult
		var ok bool
		if memoryTool {
			result, ok = tool.ExecuteMemoryWithContextAndCancel(callCtx, execCtx, legacy.Function.Name, string(request.Input))
		} else {
			result, ok = tool.ExecuteWithContextAndCancel(callCtx, execCtx, legacy.Function.Name, string(request.Input))
		}
		if !ok {
			return SkillResult{}, NewExtensionError(ErrSkillExecutionFailed, "Legacy tool execution failed", result.ErrorCode, false, nil)
		}
		output, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			return SkillResult{}, marshalErr
		}
		status := RunSucceeded
		switch result.Status {
		case tool.ToolStatusCancelled:
			status = RunCancelled
		case tool.ToolStatusFailed, tool.ToolStatusUnknown:
			status = RunFailed
		case tool.ToolStatusPartialSuccess:
			status = RunPartiallySucceeded
		}
		sideEffectRecords := make([]SideEffectRecord, 0, len(result.SideEffects))
		for _, item := range result.SideEffects {
			sideEffectRecords = append(sideEffectRecords, SideEffectRecord{Type: item.Type, TargetID: item.TargetID, Confirmed: item.Confirmed})
		}
		visibleText := result.VisibleText
		if visibleText == "" {
			visibleText = result.Content
		}
		skillResult := SkillResult{Status: status, Output: output, SideEffects: sideEffectRecords, VisibleText: visibleText, ForceVoice: result.ForceVoice}
		if status == RunFailed {
			skillResult.Error = NewExtensionError(ErrSkillExecutionFailed, "Legacy tool execution failed", result.ErrorCode, false, nil)
		}
		return skillResult, nil
	}
	return definition, handler, nil
}

func normalizeLegacyName(name string) string {
	normalized := legacyNameNormalizer.ReplaceAllString(strings.ToLower(strings.TrimSpace(name)), "-")
	normalized = strings.Trim(normalized, "-")
	if normalized == "" {
		return "unnamed"
	}
	return normalized
}

func legacyInputSchema(parameters tool.Parameters) (json.RawMessage, error) {
	properties := map[string]interface{}{}
	for name, property := range parameters.Properties {
		properties[name] = map[string]interface{}{"type": property.Type, "description": property.Description}
	}
	schema := map[string]interface{}{
		"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "properties": properties,
		"required": parameters.Required, "additionalProperties": false,
	}
	return json.Marshal(schema)
}

func legacyOutputSchema() json.RawMessage {
	return json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"required":["status","content","confidence","force_voice"],"properties":{"status":{"type":"string"},"content":{"type":"string"},"error_code":{"type":"string"},"visible_text":{"type":"string"},"side_effects":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["type","confirmed"],"properties":{"type":{"type":"string"},"target_id":{"type":"string"},"confirmed":{"type":"boolean"}}}},"external_operation_id":{"type":"string"},"idempotency_key":{"type":"string"},"audit":{"type":"object"},"confidence":{"type":"number"},"force_voice":{"type":"boolean"}}}`)
}

func legacyCapabilities(name string) ([]string, bool, bool) {
	switch name {
	case "get_current_time":
		return []string{"runtime.time.read"}, false, true
	case "create_schedule":
		return []string{"scheduler.own.manage"}, true, true
	case "force_voice_reply":
		return []string{"notification.send"}, true, true
	case "read_need_state":
		return []string{"runtime.character.read"}, false, true
	case "read_psyche_state":
		return []string{"runtime.emotion.read"}, false, true
	case "summarize_memories":
		return []string{"memory.read"}, false, true
	case "save_memory", "save_profile", "save_episodic_memory":
		return []string{"memory.candidate.write"}, true, true
	default:
		return []string{}, false, false
	}
}
