package kernel

import (
	"context"
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/deepsearch"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

type DeepSearchDeps struct {
	TaskService     *task_runtime.TaskRuntimeService
	ToolRegistry    *capability.ToolRegistry
	Config          deepsearch.Config
	TaskEntry       string
	GeneralSearchID string
}

func RegisterDeepSearchSystemTask(ctx context.Context, svc *task_runtime.TaskRuntimeService, entry string) error {
	if svc == nil {
		return nil
	}
	def := deepsearch.BuildTaskDefinition(entry)
	def.DefinitionHash = deepsearch.DefinitionHash(def)
	existing, err := svc.GetTaskDefinition(ctx, def.TaskID)
	if err == nil && existing != nil {
		return nil
	}
	return svc.PutTaskDefinition(ctx, &def)
}

func RegisterDeepSearchTool(deps DeepSearchDeps) error {
	if deps.ToolRegistry == nil || deps.TaskService == nil {
		return nil
	}

	inputSchema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"required": ["query"],
		"properties": {
			"query": {"type": "string", "minLength": 1, "maxLength": 2048},
			"focusAreas": {
				"type": "array",
				"maxItems": 8,
				"items": {"type": "string", "maxLength": 256}
			},
			"maxRounds": {"type": "integer", "minimum": 1, "maximum": 5},
			"maxQueriesPerRound": {"type": "integer", "minimum": 1, "maximum": 6},
			"resultsPerQuery": {"type": "integer", "minimum": 1, "maximum": 20},
			"maxSources": {"type": "integer", "minimum": 1, "maximum": 100},
			"language": {"type": "string"},
			"country": {"type": "string"},
			"safeSearch": {"enum": ["off", "moderate", "strict"]}
		}
	}`)

	outputSchema := json.RawMessage(`{
		"type": "object",
		"additionalProperties": false,
		"required": ["accepted", "taskRunId", "taskDefId", "invocationId"],
		"properties": {
			"accepted": {"type": "boolean"},
			"taskRunId": {"type": "string"},
			"taskDefId": {"type": "string"},
			"invocationId": {"type": "string"}
		}
	}`)

	definition := capability.ToolDefinition{
		ID:           "internal/search/deep",
		ModelName:    "deep_search",
		Source:       capability.ToolSourceInternal,
		Name:         "Deep Search",
		Description:  "Run a multi-round web search that aggregates, deduplicates, and ranks results into a research dossier.",
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Permissions: []capability.PermissionRequirement{
			{Capability: "search.deep", Description: "Allows running deep multi-round web search"},
			{Capability: "network.request", Description: "Each child search calls the general search provider"},
		},
		RiskLevel:      capability.RiskMedium,
		SideEffect:     capability.SideEffectExternal,
		HasSideEffects: true,
		Idempotent:     false,
		Retryable:      false,
		Enabled:        true,
		TimeoutMS:      5000,
		Runtime: capability.RuntimeBinding{
			RuntimeType: "task",
			RuntimeID:   deepsearch.DeepSearchTaskID,
			HandlerName: deepsearch.DeepSearchTaskID,
		},
		ExecutionPolicy: capability.ToolExecutionPolicy{
			Timeout:    5 * time.Second,
			Idempotent: false,
		},
		ResultPolicy: capability.ToolResultPolicy{
			SanitizeError:  true,
			MaxOutputBytes: 8192,
			Streaming:      capability.ToolStreamingPolicy{Enabled: false},
		},
	}
	return deps.ToolRegistry.Replace(context.Background(), definition)
}
