package builtin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/uiagent"
	"github.com/u-ai/backend/internal/uiagent/schema"
	"github.com/u-ai/backend/internal/uiagent/source"
	"github.com/u-ai/backend/internal/workspace"
)

var (
	uiAgentInspector  source.SourceInspector
	uiAgentExecutor   *uiagent.UIExecutor
	uiAgentSchemaGen  *schema.AISchemaGenerator
	uiAgentPreviewMgr uiagent.PreviewManager
	uiAgentSourceEditor source.SourceEditor
)

func SetUIAgentInspector(inspector source.SourceInspector) {
	uiAgentInspector = inspector
}

func SetUIAgentExecutor(executor *uiagent.UIExecutor) {
	uiAgentExecutor = executor
}

func SetUIAgentSchemaGenerator(gen *schema.AISchemaGenerator) {
	uiAgentSchemaGen = gen
}

func SetUIAgentPreviewManager(mgr uiagent.PreviewManager) {
	uiAgentPreviewMgr = mgr
}

func SetUIAgentSourceEditor(editor source.SourceEditor) {
	uiAgentSourceEditor = editor
}

func SetUIAgentPreciseService(svc workspace.PreciseEditingService) {
	uiAgentSourceEditor = source.NewSourceEditor(svc)
}

func init() {
	tool.Register(tool.Tool{
		Type: "function",
		Function: tool.Function{
			Name:        "uiagent.inspect",
			Description: "Inspect UI workspace files, symbols, and structure.",
			Parameters: tool.Parameters{
				Type: "object",
				Properties: map[string]tool.Property{
					"workspaceId": {
						Type:        "string",
						Description: "The workspace ID to inspect.",
					},
				},
				Required: []string{"workspaceId"},
			},
		},
	}, uiagentInspectHandler)

	tool.Register(tool.Tool{
		Type: "function",
		Function: tool.Function{
			Name:        "uiagent.modify",
			Description: "Modify UI workspace files with transaction support.",
			Parameters: tool.Parameters{
				Type: "object",
				Properties: map[string]tool.Property{
					"workspaceId": {
						Type:        "string",
						Description: "The workspace ID to modify.",
					},
					"operations": {
						Type:        "array",
						Description: "The modification operations to apply.",
					},
				},
				Required: []string{"workspaceId", "operations"},
			},
		},
	}, uiagentModifyHandler)

	tool.Register(tool.Tool{
		Type: "function",
		Function: tool.Function{
			Name:        "uiagent.create",
			Description: "Create new UI workspace files or schema drafts.",
			Parameters: tool.Parameters{
				Type: "object",
				Properties: map[string]tool.Property{
					"workspaceId": {
						Type:        "string",
						Description: "The workspace ID to create in.",
					},
				},
				Required: []string{"workspaceId"},
			},
		},
	}, uiagentCreateHandler)
}

func uiagentInspectHandler(ctx context.Context, execCtx tool.ToolExecutionContext, args map[string]interface{}) tool.ToolCallResult {
	workspaceID, _ := args["workspaceId"].(string)
	if workspaceID == "" {
		return tool.ErrorResult("invalid_input", "workspaceId is required")
	}

	if uiAgentInspector == nil {
		return tool.ErrorResult("ui_agent_unavailable", "UI agent inspector not configured")
	}

	result, err := uiAgentInspector.Inspect(ctx, workspaceID, nil)
	if err != nil {
		return tool.ErrorResult("inspect_failed", fmt.Sprintf("inspection failed: %v", err))
	}

	resultJSON, _ := json.Marshal(result)
	return tool.ToolCallResult{
		Status:      tool.ToolStatusSuccess,
		Content:     string(resultJSON),
		VisibleText: fmt.Sprintf("Inspected workspace %s: %d files found", workspaceID, result.TotalFiles),
	}
}

func uiagentModifyHandler(ctx context.Context, execCtx tool.ToolExecutionContext, args map[string]interface{}) tool.ToolCallResult {
	workspaceID, _ := args["workspaceId"].(string)
	if workspaceID == "" {
		return tool.ErrorResult("invalid_input", "workspaceId is required")
	}

	operationsRaw, _ := json.Marshal(args["operations"])
	var sourceOps []source.SourceEditOperation
	if err := json.Unmarshal(operationsRaw, &sourceOps); err != nil {
		return tool.ErrorResult("invalid_operations", fmt.Sprintf("invalid operations: %v", err))
	}

	srcReq := source.SourceEditRequest{
		WorkspaceID: workspaceID,
		Operations:  sourceOps,
		Transaction: true,
	}

	// Use the real source editor if available
	if uiAgentSourceEditor != nil {
		result, err := uiAgentSourceEditor.ApplyEdits(ctx, srcReq)
		if err != nil {
			return tool.ErrorResult("modify_failed", fmt.Sprintf("modification failed: %v", err))
		}

		// No real file changes = not a success
		if len(result.ChangedFiles) == 0 {
			return tool.ErrorResult("no_changes", "no files were modified")
		}

		resultJSON, _ := json.Marshal(result)
		return tool.ToolCallResult{
			Status:      tool.ToolStatusSuccess,
			Content:     string(resultJSON),
			VisibleText: fmt.Sprintf("Applied %d operations to workspace %s (%d files changed)", result.AppliedOperations, workspaceID, len(result.ChangedFiles)),
		}
	}

	// Fallback to UI executor
	if uiAgentExecutor == nil {
		return tool.ErrorResult("ui_agent_unavailable", "UI agent executor not configured")
	}

	result, err := uiAgentExecutor.ApplySourceEdits(ctx, uiagent.UIChangePlan{
		Intent: uiagent.UIIntent{
			Target: uiagent.UITarget{
				WorkspaceID: workspaceID,
			},
		},
		Operations: convertToUIOperations(sourceOps),
	})
	if err != nil {
		return tool.ErrorResult("modify_failed", fmt.Sprintf("modification failed: %v", err))
	}

	resultJSON, _ := json.Marshal(result)
	return tool.ToolCallResult{
		Status:      tool.ToolStatusSuccess,
		Content:     string(resultJSON),
		VisibleText: fmt.Sprintf("Applied %d operations to workspace %s", result.AppliedOperations, workspaceID),
	}
}

func uiagentCreateHandler(ctx context.Context, execCtx tool.ToolExecutionContext, args map[string]interface{}) tool.ToolCallResult {
	workspaceID, _ := args["workspaceId"].(string)
	if workspaceID == "" {
		return tool.ErrorResult("invalid_input", "workspaceId is required")
	}

	mode, _ := args["mode"].(string)
	description, _ := args["description"].(string)

	if mode == "" || mode == "source" {
		return tool.ErrorResult("unsupported_mode", "source mode requires a configured source editor")
	}

	if uiAgentSchemaGen == nil {
		return tool.ErrorResult("ui_agent_unavailable", "UI agent schema generator not configured")
	}

	doc, err := uiAgentSchemaGen.Generate(description, nil)
	if err != nil {
		return tool.ErrorResult("create_failed", fmt.Sprintf("schema generation failed: %v", err))
	}

	resultJSON, _ := json.Marshal(map[string]interface{}{
		"success":   true,
		"schemaId":  doc.Title,
		"createdBy": "uiagent.create",
	})
	return tool.ToolCallResult{
		Status:      tool.ToolStatusSuccess,
		Content:     string(resultJSON),
		VisibleText: fmt.Sprintf("Created schema %s in workspace %s", doc.Title, workspaceID),
	}
}

func convertToUIOperations(ops []source.SourceEditOperation) []uiagent.UIOperation {
	result := make([]uiagent.UIOperation, 0, len(ops))
	for _, op := range ops {
		payload, _ := json.Marshal(op)
		result = append(result, uiagent.UIOperation{
			Type:    "edit",
			Target:  op.Path,
			Payload: payload,
		})
	}
	return result
}
