package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/internal/agent/tool"
	"github.com/u-ai/backend/internal/uiagent"
	"github.com/u-ai/backend/internal/uiagent/preview"
	"github.com/u-ai/backend/internal/uiagent/preview/adapters"
	"github.com/u-ai/backend/internal/uiagent/schema"
	"github.com/u-ai/backend/internal/uiagent/source"
	"github.com/u-ai/backend/internal/workspace"
)

var (
	uiAgentInspector        source.SourceInspector
	uiAgentExecutor         *uiagent.UIExecutor
	uiAgentSchemaGen        *schema.SchemaUIGenerator
	uiAgentAISchemaGen      *schema.AISchemaGenerator
	uiAgentPreviewMgr       uiagent.PreviewManager
	uiAgentObserver         preview.Observer
	uiAgentRefiner          preview.AutoRefiner
	uiAgentSourceEditor     source.SourceEditor
	uiAgentDiagnosticRunner adapters.DiagnosticRunner
	uiAgentPublisher        UIAgentPublisher
	uiAgentClientRuntime    UIAgentClientRuntime
)

type UIAgentPublishResult struct {
	SchemaID       string `json:"schemaId"`
	ContributionID string `json:"contributionId"`
}

type UIAgentPublisher interface {
	Publish(ctx context.Context, workspaceID string, doc *schema.SchemaUIDocument) (UIAgentPublishResult, error)
}

type UIAgentClientRuntime interface {
	ExecuteClientRuntimeCommand(ctx context.Context, action string, payload map[string]interface{}) (map[string]interface{}, error)
}

func SetUIAgentInspector(inspector source.SourceInspector) {
	uiAgentInspector = inspector
}

func SetUIAgentExecutor(executor *uiagent.UIExecutor) {
	uiAgentExecutor = executor
}

func SetUIAgentSchemaGenerator(gen *schema.SchemaUIGenerator) {
	uiAgentSchemaGen = gen
}

func SetUIAgentAISchemaGenerator(gen *schema.AISchemaGenerator) {
	uiAgentAISchemaGen = gen
}

func SetUIAgentPreviewManager(mgr uiagent.PreviewManager) {
	uiAgentPreviewMgr = mgr
}

func SetUIAgentObserver(obs preview.Observer) {
	uiAgentObserver = obs
}

func SetUIAgentRefiner(rfn preview.AutoRefiner) {
	uiAgentRefiner = rfn
}

func SetUIAgentSourceEditor(editor source.SourceEditor) {
	uiAgentSourceEditor = editor
}

func SetUIAgentPreciseService(svc workspace.PreciseEditingService) {
	uiAgentSourceEditor = source.NewSourceEditor(svc)
}

func SetUIAgentDiagnosticRunner(runner adapters.DiagnosticRunner) {
	uiAgentDiagnosticRunner = runner
}

func SetUIAgentPublisher(publisher UIAgentPublisher) {
	uiAgentPublisher = publisher
}

func SetUIAgentClientRuntime(runtime UIAgentClientRuntime) {
	uiAgentClientRuntime = runtime
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
					"preview": {
						Type:        "boolean",
						Description: "Keep the validated preview session available after commit.",
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
					"mode": {
						Type:        "string",
						Description: "Creation mode. Use schema for generated Schema UI.",
						Enum:        []string{"schema"},
					},
					"description": {
						Type:        "string",
						Description: "Natural-language description of the UI to create.",
					},
				},
				Required: []string{"workspaceId", "mode", "description"},
			},
		},
	}, uiagentCreateHandler)

	tool.Register(tool.Tool{
		Type: "function",
		Function: tool.Function{
			Name:        "uiagent.preview",
			Description: "Observe and refine a preview session to validate UI changes.",
			Parameters: tool.Parameters{
				Type: "object",
				Properties: map[string]tool.Property{
					"sessionId": {
						Type:        "string",
						Description: "The preview session ID to observe/refine.",
					},
					"action": {
						Type:        "string",
						Description: "The action to perform: observe, refine.",
						Enum:        []string{"observe", "refine"},
					},
					"workspaceId": {
						Type:        "string",
						Description: "The workspace ID (for session lookup if sessionId not provided).",
					},
				},
				Required: []string{"action"},
			},
		},
	}, uiagentPreviewHandler)

	tool.Register(tool.Tool{
		Type: "function",
		Function: tool.Function{
			Name:        "uiagent.client_runtime",
			Description: "Inspect and control the session-owned UI runtime using immutable packages. Packages may compose published Schema UI/conversation projections or provide browser-only clientCode executed inside an isolated iframe sandbox with scripts enabled but no same-origin, network, host DOM, or host runtime privileges. Trusted host JavaScript is not accepted by this tool. Production activation requires user approval for the exact package version unless the user explicitly grants future-version approval for that plugin.",
			Parameters: tool.Parameters{
				Type: "object",
				Properties: map[string]tool.Property{
					"action": {
						Type:        "string",
						Description: "Runtime action: inspect, define, run, stop, rollback, or undefine.",
						Enum:        []string{"inspect", "define", "run", "stop", "rollback", "undefine"},
					},
					"package": {
						Type:        "object",
						Description: "Client package for define. Must contain id/version. Contributions may reference published Schema UI sources or provide sandboxed clientCode {html,css,script,minHeight,maxHeight}; conversation nodes may reference published projections.",
					},
					"id":      {Type: "string", Description: "Client package id for run/stop/rollback/undefine."},
					"version": {Type: "string", Description: "Optional package version for run/undefine."},
					"mode":    {Type: "string", Description: "Optional run mode: run for first/restart/same-current activation, update when switching current to a different package.", Enum: []string{"run", "update"}},
				},
				Required: []string{"action"},
			},
		},
	}, uiagentClientRuntimeHandler)
}

func uiagentClientRuntimeHandler(ctx context.Context, execCtx tool.ToolExecutionContext, args map[string]interface{}) tool.ToolCallResult {
	if uiAgentClientRuntime == nil {
		return tool.ErrorResult("ui_agent_unavailable", "client runtime command bridge not configured")
	}
	action, _ := args["action"].(string)
	action = strings.TrimSpace(action)
	switch action {
	case "inspect", "define", "run", "stop", "rollback", "undefine":
	default:
		return tool.ErrorResult("invalid_input", "action must be inspect, define, run, stop, rollback, or undefine")
	}
	payload := make(map[string]interface{}, len(args))
	for key, value := range args {
		if key == "action" {
			continue
		}
		payload[key] = value
	}
	if action == "define" {
		pkg, ok := payload["package"].(map[string]interface{})
		if !ok || runtimeArgString(pkg, "id") == "" || runtimeArgString(pkg, "version") == "" {
			return tool.ErrorResult("invalid_input", "define requires package.id and package.version")
		}
	}
	if action != "inspect" && action != "define" && runtimeArgString(payload, "id") == "" {
		return tool.ErrorResult("invalid_input", action+" requires id")
	}
	payload["_runtimeScope"] = map[string]interface{}{
		"userId":         strings.TrimSpace(execCtx.User),
		"conversationId": strings.TrimSpace(execCtx.ConversationID),
		"requestId":      strings.TrimSpace(execCtx.RequestID),
	}
	result, err := uiAgentClientRuntime.ExecuteClientRuntimeCommand(ctx, action, payload)
	if err != nil {
		return tool.ErrorResult("client_runtime_failed", err.Error())
	}
	data, _ := json.Marshal(result)
	return tool.ToolCallResult{
		Status:      tool.ToolStatusSuccess,
		Content:     string(data),
		VisibleText: fmt.Sprintf("Client runtime %s completed", action),
		Confidence:  1,
	}
}

func runtimeArgString(values map[string]interface{}, key string) string {
	value, ok := values[key]
	if !ok || value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
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

	wantPreview, _ := args["preview"].(bool)

	srcReq := source.SourceEditRequest{
		WorkspaceID: workspaceID,
		Operations:  sourceOps,
		Transaction: true,
		AutoCommit:  false,
	}

	// Use the real source editor if available
	if uiAgentSourceEditor != nil {
		tx, err := uiAgentSourceEditor.BeginTransaction(ctx, workspaceID)
		if err != nil {
			return tool.ErrorResult("modify_failed", fmt.Sprintf("begin transaction failed: %v", err))
		}

		result, err := uiAgentSourceEditor.ApplyPatchesTx(ctx, tx, srcReq)
		if err != nil {
			_ = uiAgentSourceEditor.RollbackTx(ctx, tx)
			return tool.ErrorResult("modify_failed", fmt.Sprintf("modification failed: %v", err))
		}

		if len(result.ChangedFiles) == 0 {
			_ = uiAgentSourceEditor.RollbackTx(ctx, tx)
			return tool.ErrorResult("no_changes", "no files were modified")
		}

		diffResult, err := uiAgentSourceEditor.PreviewDiff(ctx, tx)
		if err != nil {
			_ = uiAgentSourceEditor.RollbackTx(ctx, tx)
			return tool.ErrorResult("preview_failed", fmt.Sprintf("preview diff failed: %v", err))
		}
		result.DiffPreview = diffResult.UnifiedDiff

		previewEditor, ok := uiAgentSourceEditor.(source.PreviewTransactionEditor)
		if !ok {
			_ = uiAgentSourceEditor.RollbackTx(ctx, tx)
			return tool.ErrorResult("preview_unavailable", "source editor does not support pre-commit preview transactions")
		}
		if err := previewEditor.MaterializePreviewTx(ctx, tx); err != nil {
			_ = uiAgentSourceEditor.RollbackTx(ctx, tx)
			return tool.ErrorResult("preview_failed", fmt.Sprintf("failed to materialize transaction for preview: %v", err))
		}

		if uiAgentPreviewMgr == nil || uiAgentObserver == nil {
			_ = uiAgentSourceEditor.RollbackTx(ctx, tx)
			return tool.ErrorResult("preview_unavailable", "source validation requires preview manager and observer")
		}
		platform := detectSourcePreviewPlatform(result.ChangedFiles)
		if platform == "" {
			_ = uiAgentSourceEditor.RollbackTx(ctx, tx)
			return tool.ErrorResult("preview_unavailable", "unable to determine source preview platform")
		}
		previewSession, previewErr := uiAgentPreviewMgr.Create(workspaceID, nil)
		if previewErr != nil {
			_ = uiAgentSourceEditor.RollbackTx(ctx, tx)
			return tool.ErrorResult("preview_failed", fmt.Sprintf("preview session creation failed: %v", previewErr))
		}
		previewSession.Target = &preview.PreviewTarget{WorkspaceID: workspaceID, Platform: platform, SourceType: "source"}
		previewSession.TransactionID = tx.ID
		obs, obsErr := uiAgentObserver.Capture(ctx, previewSession.ID)
		if obsErr != nil {
			_ = uiAgentSourceEditor.RollbackTx(ctx, tx)
			_ = uiAgentPreviewMgr.Terminate(previewSession.ID)
			return tool.ErrorResult("preview_failed", fmt.Sprintf("preview observation failed: %v", obsErr))
		}
		if preview.ShouldBlockCommit(obs) {
			_ = uiAgentSourceEditor.RollbackTx(ctx, tx)
			if !wantPreview {
				_ = uiAgentPreviewMgr.Terminate(previewSession.ID)
			}
			return tool.ErrorResult("preview_validation_failed", fmt.Sprintf("preview blocked commit: %s", strings.Join(obs.AllErrors(), "; ")))
		}
		if err := previewEditor.FinalizePreviewTx(ctx, tx); err != nil {
			_ = uiAgentSourceEditor.RollbackTx(ctx, tx)
			return tool.ErrorResult("commit_failed", fmt.Sprintf("commit failed after preview: %v", err))
		}

		response := map[string]interface{}{
			"success":           result.Success,
			"appliedOperations": result.AppliedOperations,
			"transactionToken":  result.TransactionToken,
			"changedFiles":      result.ChangedFiles,
			"diffPreview":       result.DiffPreview,
		}

		if wantPreview {
			response["previewRef"] = previewSession.ID
		} else {
			_ = uiAgentPreviewMgr.Terminate(previewSession.ID)
		}

		resultJSON, _ := json.Marshal(response)
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

	response := map[string]interface{}{
		"success":           true,
		"appliedOperations": result.AppliedOperations,
		"changedFiles":      result.ChangedFiles,
	}

	if wantPreview && uiAgentPreviewMgr != nil {
		previewSession, previewErr := uiAgentPreviewMgr.Create(workspaceID, nil)
		if previewErr != nil {
			return tool.ErrorResult("preview_failed", fmt.Sprintf("preview session creation failed: %v", previewErr))
		}
		response["previewRef"] = previewSession.ID
	}

	resultJSON, _ := json.Marshal(response)
	return tool.ToolCallResult{
		Status:      tool.ToolStatusSuccess,
		Content:     string(resultJSON),
		VisibleText: fmt.Sprintf("Applied %d operations to workspace %s", result.AppliedOperations, workspaceID),
	}
}

func detectSourcePreviewPlatform(paths []string) string {
	hasWeb := false
	for _, p := range paths {
		switch strings.ToLower(filepath.Ext(p)) {
		case ".dart":
			return "flutter"
		case ".vue", ".tsx", ".jsx", ".ts", ".js", ".svelte", ".html", ".css", ".scss":
			hasWeb = true
		}
	}
	if hasWeb {
		return "web"
	}
	return ""
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

	if uiAgentAISchemaGen == nil {
		return tool.ErrorResult("ui_agent_unavailable", "UI agent AI schema generator not configured")
	}

	if !uiAgentAISchemaGen.HasLLMCallFunc() {
		return tool.ErrorResult("ui_agent_unavailable", "UI agent AI schema generator has no LLM function configured")
	}

	doc, err := uiAgentAISchemaGen.Generate(description, nil)
	if err != nil {
		return tool.ErrorResult("create_failed", fmt.Sprintf("AI schema generation failed: %v", err))
	}

	if uiAgentPreviewMgr == nil {
		return tool.ErrorResult("ui_agent_unavailable", "UI agent preview manager not configured")
	}

	session, previewErr := uiAgentPreviewMgr.Create(workspaceID, doc)
	if previewErr != nil {
		return tool.ErrorResult("preview_failed", fmt.Sprintf("preview session creation failed: %v", previewErr))
	}

	if uiAgentObserver != nil {
		obsResult, obsErr := uiAgentObserver.Capture(ctx, session.ID)
		if obsErr != nil {
			return tool.ErrorResult("preview_observe_failed", fmt.Sprintf("preview observation failed: %v", obsErr))
		}
		if preview.ShouldBlockCommit(obsResult) && obsResult.CanRefine && uiAgentRefiner != nil {
			refined, refineErr := uiAgentRefiner.Refine(ctx, preview.RefineRequest{
				SessionID:     session.ID,
				Observation:   obsResult,
				Target:        session.Target,
				MaxIterations: preview.MaxRefineIterations,
			})
			if refineErr == nil && refined != nil && refined.Observation != nil {
				obsResult = refined.Observation
			}
		}
		if preview.ShouldBlockCommit(obsResult) {
			allErrors := obsResult.AllErrors()
			response := map[string]interface{}{
				"success":    false,
				"schemaId":   doc.Title,
				"previewRef": session.ID,
				"errors":     allErrors,
				"state":      "needs_manual",
			}
			resultJSON, _ := json.Marshal(response)
			return tool.ToolCallResult{
				Status:      tool.ToolStatusSuccess,
				Content:     string(resultJSON),
				VisibleText: fmt.Sprintf("Schema %s generated but has %d issues requiring manual review", doc.Title, len(allErrors)),
			}
		}
	}
	if uiAgentPublisher == nil {
		return tool.ErrorResult("ui_agent_unavailable", "UI agent publisher not configured")
	}
	published, publishErr := uiAgentPublisher.Publish(ctx, workspaceID, doc)
	if publishErr != nil {
		return tool.ErrorResult("publish_failed", fmt.Sprintf("schema publish failed: %v", publishErr))
	}

	response := map[string]interface{}{
		"success":        true,
		"schemaId":       published.SchemaID,
		"contributionId": published.ContributionID,
		"previewRef":     session.ID,
		"createdBy":      "uiagent.create",
		"state":          "published",
	}

	resultJSON, _ := json.Marshal(response)
	return tool.ToolCallResult{
		Status:      tool.ToolStatusSuccess,
		Content:     string(resultJSON),
		VisibleText: fmt.Sprintf("Created and published schema %s in workspace %s", doc.Title, workspaceID),
	}
}

func uiagentPreviewHandler(ctx context.Context, execCtx tool.ToolExecutionContext, args map[string]interface{}) tool.ToolCallResult {
	action, _ := args["action"].(string)
	if action == "" {
		return tool.ErrorResult("invalid_input", "action is required (observe, refine)")
	}

	sessionID, _ := args["sessionId"].(string)
	workspaceID, _ := args["workspaceId"].(string)

	if sessionID == "" {
		return tool.ErrorResult("invalid_input", "sessionId is required")
	}

	switch action {
	case "observe":
		if uiAgentObserver == nil {
			return tool.ErrorResult("ui_agent_unavailable", "UI agent preview observer not configured")
		}
		obsResult, err := uiAgentObserver.Capture(ctx, sessionID)
		if err != nil {
			return tool.ErrorResult("preview_observe_failed", fmt.Sprintf("observe failed: %v", err))
		}
		blockCommit := preview.ShouldBlockCommit(obsResult)
		response := map[string]interface{}{
			"sessionId":      obsResult.SessionID,
			"errors":         obsResult.AllErrors(),
			"warnings":       obsResult.Warnings,
			"compileErrors":  obsResult.CompileErrors,
			"runtimeErrors":  obsResult.RuntimeErrors,
			"bindingErrors":  obsResult.BindingErrors,
			"actionErrors":   obsResult.ActionErrors,
			"overflowErrors": obsResult.OverflowErrors,
			"canRefine":      obsResult.CanRefine,
			"blockCommit":    blockCommit,
		}
		resultJSON, _ := json.Marshal(response)
		if blockCommit {
			return tool.ToolCallResult{
				Status:      tool.ToolStatusSuccess,
				Content:     string(resultJSON),
				VisibleText: fmt.Sprintf("Preview observe session %s: %d issues found, commit/publish blocked", sessionID, len(obsResult.AllErrors())),
			}
		}
		return tool.ToolCallResult{
			Status:      tool.ToolStatusSuccess,
			Content:     string(resultJSON),
			VisibleText: fmt.Sprintf("Preview observe session %s: clean, ready to commit/publish", sessionID),
		}

	case "refine":
		if uiAgentRefiner == nil {
			return tool.ErrorResult("ui_agent_unavailable", "UI agent preview refiner not configured")
		}
		refineReq := preview.RefineRequest{
			SessionID:     sessionID,
			MaxIterations: preview.MaxRefineIterations,
		}
		if workspaceID != "" {
			refineReq.Target = &preview.PreviewTarget{
				WorkspaceID: workspaceID,
			}
		}
		refineResult, err := uiAgentRefiner.Refine(ctx, refineReq)
		if err != nil {
			return tool.ErrorResult("preview_refine_failed", fmt.Sprintf("refine failed: %v", err))
		}
		resultJSON, _ := json.Marshal(refineResult)
		return tool.ToolCallResult{
			Status:      tool.ToolStatusSuccess,
			Content:     string(resultJSON),
			VisibleText: fmt.Sprintf("Preview refine session %s: state=%s, iterations=%d", sessionID, refineResult.State, refineResult.Iterations),
		}

	default:
		return tool.ErrorResult("unsupported_action", fmt.Sprintf("unsupported preview action: %s", action))
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
