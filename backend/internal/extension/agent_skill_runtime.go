// Deprecated: Legacy extension architecture.
// Do not add new capabilities. This implementation is retained only for
// compatibility, maintenance, testing, and migration to Extension Kernel.

package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

const (
	agentSkillActivateID      = "dev.amitia.skill.agent-skill-activate"
	agentSkillListResourcesID = "dev.amitia.skill.agent-skill-list-resources"
	agentSkillReadResourceID  = "dev.amitia.skill.agent-skill-read-resource"
	agentSkillGetAssetID      = "dev.amitia.skill.agent-skill-get-asset"
)

func registerAgentSkillRuntime(ctx context.Context, registry *Registry, service *AgentSkillService) error {
	definitions := []struct {
		definition SkillDefinition
		handler    SkillHandler
	}{
		{internalAgentSkillDefinition(agentSkillActivateID, "agent_skill_activate", "Activate an installed Agent Skill for the current round", json.RawMessage(`{"type":"object","additionalProperties":false,"required":["agentSkill"],"properties":{"agentSkill":{"type":"string"}}}`)), func(ctx context.Context, request ExecuteSkillRequest) (SkillResult, error) {
			var input struct {
				AgentSkill string `json:"agentSkill"`
			}
			if err := json.Unmarshal(request.Input, &input); err != nil {
				return SkillResult{}, err
			}
			activated, err := service.Activate(ctx, ActivateAgentSkillRequest{Scope: request.Scope, NameOrID: input.AgentSkill, Explicit: false})
			if err != nil {
				return SkillResult{}, err
			}
			output, _ := json.Marshal(map[string]interface{}{"activationId": activated.ActivationID, "extensionId": activated.Definition.ExtensionID, "name": activated.Definition.Name, "source": activated.Definition.Source, "scope": activated.Definition.Scope, "compatibilityStatus": activated.Definition.CompatibilityStatus, "bodyTokens": activated.BodyTokens, "toolMappings": activated.Definition.ToolMappings, "prompt": activated.Prompt, "instructionPosition": "after_character_rules", "scriptsUsed": false, "status": "activated"})
			return SkillResult{Status: RunSucceeded, Output: output, VisibleText: "Agent Skill activated: " + activated.Definition.Name}, nil
		}},
		{internalAgentSkillDefinition(agentSkillListResourcesID, "agent_skill_list_resources", "List controlled resources of an active Agent Skill", json.RawMessage(`{"type":"object","additionalProperties":false,"required":["agentSkill"],"properties":{"agentSkill":{"type":"string"},"kind":{"type":"string","enum":["reference","asset","script","agent_metadata","other"]}}}`)), func(ctx context.Context, request ExecuteSkillRequest) (SkillResult, error) {
			var input struct {
				AgentSkill string                 `json:"agentSkill"`
				Kind       AgentSkillResourceKind `json:"kind"`
			}
			if err := json.Unmarshal(request.Input, &input); err != nil {
				return SkillResult{}, err
			}
			resources, err := service.ListResources(ctx, ListAgentSkillResourcesRequest{Scope: request.Scope, NameOrID: input.AgentSkill, Kind: input.Kind})
			if err != nil {
				return SkillResult{}, err
			}
			output, _ := json.Marshal(map[string]interface{}{"resources": resources})
			return SkillResult{Status: RunSucceeded, Output: output, VisibleText: string(output)}, nil
		}},
		{internalAgentSkillDefinition(agentSkillReadResourceID, "agent_skill_read_resource", "Read a text resource from an active Agent Skill", json.RawMessage(`{"type":"object","additionalProperties":false,"required":["agentSkill","path"],"properties":{"agentSkill":{"type":"string"},"path":{"type":"string"}}}`)), func(ctx context.Context, request ExecuteSkillRequest) (SkillResult, error) {
			var input struct {
				AgentSkill string `json:"agentSkill"`
				Path       string `json:"path"`
			}
			if err := json.Unmarshal(request.Input, &input); err != nil {
				return SkillResult{}, err
			}
			content, err := service.ReadResource(ctx, ReadAgentSkillResourceRequest{Scope: request.Scope, NameOrID: input.AgentSkill, Path: input.Path})
			if err != nil {
				return SkillResult{}, err
			}
			output, _ := json.Marshal(content)
			return SkillResult{Status: RunSucceeded, Output: output, VisibleText: content.Content}, nil
		}},
		{internalAgentSkillDefinition(agentSkillGetAssetID, "agent_skill_get_asset", "Get a safe handle for an asset of an active Agent Skill", json.RawMessage(`{"type":"object","additionalProperties":false,"required":["agentSkill","path"],"properties":{"agentSkill":{"type":"string"},"path":{"type":"string"}}}`)), func(ctx context.Context, request ExecuteSkillRequest) (SkillResult, error) {
			definition, err := service.activeDefinition(request.Scope, extractAgentSkillInput(request.Input, "agentSkill"))
			if err != nil {
				return SkillResult{}, err
			}
			assetPath := extractAgentSkillInput(request.Input, "path")
			clean, pathErr := validateAgentSkillRelativePath(assetPath, service.limits)
			if pathErr != nil {
				return SkillResult{}, pathErr
			}
			var resource *AgentSkillResource
			for i := range definition.Resources {
				if definition.Resources[i].Path == clean {
					resource = &definition.Resources[i]
					break
				}
			}
			if resource == nil || resource.Kind != AgentSkillResourceAsset {
				return SkillResult{}, NewExtensionError(ErrAgentSkillResourceDenied, "asset is unavailable", clean, false, nil)
			}
			if resource.MIMEType == "application/x-msdownload" || resource.MIMEType == "application/x-executable" {
				return SkillResult{}, NewExtensionError(ErrAgentSkillResourceDenied, "executable assets are unavailable", clean, false, nil)
			}
			query := url.Values{"path": []string{clean}, "channel": []string{request.Scope.Channel}}
			if request.Scope.CharacterID != "" {
				query.Set("characterId", request.Scope.CharacterID)
			}
			if request.Scope.ConversationID != "" {
				query.Set("conversationId", request.Scope.ConversationID)
			}
			handle := fmt.Sprintf("/api/extensions/agent-skills/%s/assets/content?%s", url.PathEscape(definition.ExtensionID), query.Encode())
			output, _ := json.Marshal(map[string]interface{}{"handle": handle, "path": clean, "mimeType": resource.MIMEType, "size": resource.Size, "executable": false})
			return SkillResult{Status: RunSucceeded, Output: output, VisibleText: string(output)}, nil
		}},
	}
	for _, item := range definitions {
		if err := registry.Register(ctx, item.definition, item.handler); err != nil {
			return err
		}
	}
	return nil
}

func internalAgentSkillDefinition(id, modelName, description string, input json.RawMessage) SkillDefinition {
	output := json.RawMessage(`{"type":"object","additionalProperties":true}`)
	metadata := ManifestMetadata{ID: id, Name: modelName, Version: "1.0.0", Description: description, Author: "Amitia", License: "AGPL-3.0-only"}
	manifest := Manifest{Schema: "https://schemas.amitia.dev/extensions/v1/manifest.schema.json", APIVersion: "extensions.amitia.dev/v1alpha1", Kind: "Skill", Metadata: metadata, Compatibility: ManifestCompatibility{EngineMin: "1.0.0"}, Entry: SkillEntry{Kind: "builtin", Name: modelName}, Capabilities: []string{}, Triggers: []SkillTrigger{TriggerLLM}, Execution: ManifestExecution{TimeoutMS: 5000, Idempotent: false}, InputSchema: input, OutputSchema: output, Enabled: true, AllowLLM: true, AllowManual: false}
	raw, _ := json.Marshal(manifest)
	return SkillDefinition{ID: id, ModelName: modelName, Name: modelName, Description: description, Version: "1.0.0", Source: SkillSourceBuiltin, Entry: manifest.Entry, InputSchema: input, OutputSchema: output, Capabilities: []string{}, Triggers: manifest.Triggers, TimeoutMS: 5000, Enabled: true, Compatible: true, Author: metadata.Author, License: metadata.License, Manifest: raw, Internal: true}
}
func extractAgentSkillInput(raw json.RawMessage, key string) string {
	var input map[string]interface{}
	_ = json.Unmarshal(raw, &input)
	value, _ := input[key].(string)
	return value
}

func (r *Runtime) PrepareAgentSkillPrompt(ctx context.Context, scope ExecutionScope, message string) (string, []ActivatedAgentSkill, []string) {
	if r == nil || r.AgentSkills == nil {
		return "", nil, nil
	}
	return r.AgentSkills.PreparePrompt(ctx, scope, message)
}
func (r *Runtime) EndAgentSkillRound(scope ExecutionScope) {
	if r != nil && r.AgentSkills != nil {
		r.AgentSkills.EndRound(scope)
	}
}
