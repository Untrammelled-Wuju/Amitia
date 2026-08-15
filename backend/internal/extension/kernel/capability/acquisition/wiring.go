package acquisition

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

// buildFindCapabilitiesToolDefinition 构造 find_capability 工具的 ToolDefinition。
// 该工具让 Agent 能够搜索当前不具备但执行任务所需要的能力。
func buildFindCapabilitiesToolDefinition() capability.ToolDefinition {
	return capability.ToolDefinition{
		ID:          capability.BuildToolID(capability.ToolSourceAcquisition, "capability", "find"),
		ModelName:   "find_capability",
		Source:      capability.ToolSourceAcquisition,
		Name:        "find_capability",
		Description: FindCapabilitiesToolDescription,
		InputSchema: inputSchemaFindCapabilities(),
		Enabled:     true,
		Internal:    false,
		Permissions: []capability.PermissionRequirement{
			{Capability: "capability.search", Description: "search for available capabilities"},
		},
		SideEffect:     capability.SideEffectReadOnly,
		RiskLevel:      capability.RiskLow,
		HasSideEffects: false,
		Idempotent:     true,
		Retryable:      true,
		TimeoutMS:      30000,
	}
}

// buildAcquireCapabilityToolDefinition 构造 acquire_capability 工具的 ToolDefinition。
// 该工具让 Agent 能够安装、启用或配置一个候选能力，使其变得可执行。
func buildAcquireCapabilityToolDefinition() capability.ToolDefinition {
	return capability.ToolDefinition{
		ID:          capability.BuildToolID(capability.ToolSourceAcquisition, "capability", "acquire"),
		ModelName:   "acquire_capability",
		Source:      capability.ToolSourceAcquisition,
		Name:        "acquire_capability",
		Description: AcquireCapabilityToolDescription,
		InputSchema: inputSchemaAcquireCapability(),
		Enabled:     true,
		Internal:    false,
		Permissions: []capability.PermissionRequirement{
			{Capability: "capability.install", Description: "install and enable a capability"},
		},
		SideEffect:     capability.SideEffectWrite,
		RiskLevel:      capability.RiskMedium,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		TimeoutMS:      120000,
	}
}

// RegisterAcquisitionTools 将 find_capability 和 acquire_capability 注册到 ToolRegistry。
// 如果 registry 为 nil 则返回错误。如果某个工具已经注册过，则跳过。
func RegisterAcquisitionTools(ctx context.Context, registry *capability.ToolRegistry) error {
	if registry == nil {
		return ErrToolRegistryNil
	}

	tools := []capability.ToolDefinition{
		buildFindCapabilitiesToolDefinition(),
		buildAcquireCapabilityToolDefinition(),
	}

	for _, def := range tools {
		if _, exists := registry.Get(ctx, def.ID); exists {
			continue
		}
		if err := registry.Register(ctx, def); err != nil {
			return fmt.Errorf("register acquisition tool %s: %w", def.ID, err)
		}
	}

	return nil
}

// inputSchemaFindCapabilities 返回 find_capability 工具的输入 JSON Schema。
func inputSchemaFindCapabilities() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "capabilityId": {
      "type": "string",
      "description": "The identifier of the capability needed (for example: browser.control, search.web, mcp.server.filesystem, chat.openai, skill.weather_query)"
    },
    "description": {
      "type": "string",
      "description": "Optional description of what the AI wants to use the capability for"
    },
    "preferredKind": {
      "type": "string",
      "description": "Optional preferred kind filter: installed_extension, agent_skill, mcp, extension_package, builtin, generated_skill"
    }
  },
  "required": ["capabilityId"]
}`)
}

// inputSchemaAcquireCapability 返回 acquire_capability 工具的输入 JSON Schema。
func inputSchemaAcquireCapability() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "candidateId": {
      "type": "string",
      "description": "The id of the candidate from find_capability result"
    },
    "approval": {
      "type": "boolean",
      "description": "Whether the AI should proceed with auto-install when user pre-approved"
    },
    "userConfirmed": {
      "type": "boolean",
      "description": "Whether the explicit user approval was already granted"
    }
  },
  "required": ["candidateId"]
}`)
}
