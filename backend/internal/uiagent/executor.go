package uiagent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/uiagent/preview"
	"github.com/u-ai/backend/internal/uiagent/schema"
)

// SourceEditOperation describes a single file edit.
type SourceEditOperation struct {
	Path          string `json:"path"`
	OldText       string `json:"oldText,omitempty"`
	NewText       string `json:"newText,omitempty"`
	Patch         string `json:"patch,omitempty"`
	BaseSHA256    string `json:"baseSha256,omitempty"`
	SearchMode    string `json:"searchMode"`
	ReplaceAll    bool   `json:"replaceAll"`
	ExpectedCount int    `json:"expectedCount"`
}

// SourceEditRequest is a batched source edit request.
type SourceEditRequest struct {
	WorkspaceID string                `json:"workspaceId"`
	Operations  []SourceEditOperation `json:"operations"`
	Transaction bool                  `json:"transaction"`
}

// SourceEditResult wraps the outcome of source edit operations.
type SourceEditResult struct {
	ChangedFiles      []string `json:"changedFiles"`
	Success           bool     `json:"success"`
	AppliedOperations int      `json:"appliedOperations"`
	TransactionToken  string   `json:"transactionToken,omitempty"`
}

// SchemaEditResult is the result of a schema-mode operation.
type SchemaEditResult struct {
	ChangedFiles []string `json:"changedFiles"`
	SchemaID     string   `json:"schemaId,omitempty"`
}

// SourceEditor applies source code edits to a workspace.
type SourceEditor interface {
	ApplyEdits(ctx context.Context, req SourceEditRequest) (*SourceEditResult, error)
}

// PreviewManager creates and manages preview sessions.
type PreviewManager interface {
	Create(workspaceID string, doc *schema.SchemaUIDocument) (PreviewSessionRef, error)
}

// PreviewSessionRef references a preview session.
type PreviewSessionRef struct {
	ID string `json:"id"`
}

// UIExecutor orchestrates the concrete operations behind a change plan.
type UIExecutor struct {
	sourceEditor  sourceEditorAdapter
	schemaGen     *schema.AISchemaGenerator
	renderer      *schema.FlutterSchemaProjection
	validator     schema.SchemaValidator
	previewMgr    PreviewManager
	policy        Policy
	observer      preview.Observer
	autoRefiner   preview.AutoRefiner
	maxRefineIter int
}

type sourceEditorAdapter struct {
	editor SourceEditor
}

func (a sourceEditorAdapter) call(ctx context.Context, req SourceEditRequest) (*SourceEditResult, error) {
	if a.editor == nil {
		return nil, fmt.Errorf("source editor not configured")
	}
	return a.editor.ApplyEdits(ctx, req)
}

// UIExecutorOption configures a UIExecutor.
type UIExecutorOption func(*UIExecutor)

// WithSourceEditor sets the source editor.
func WithSourceEditor(ed SourceEditor) UIExecutorOption {
	return func(e *UIExecutor) { e.sourceEditor = sourceEditorAdapter{editor: ed} }
}

// WithSchemaGenerator sets the schema generator.
func WithSchemaGenerator(gen *schema.AISchemaGenerator) UIExecutorOption {
	return func(e *UIExecutor) { e.schemaGen = gen }
}

// WithRenderer sets the Flutter schema projection.
func WithRenderer(r *schema.FlutterSchemaProjection) UIExecutorOption {
	return func(e *UIExecutor) { e.renderer = r }
}

// WithValidator sets the schema validator.
func WithValidator(v schema.SchemaValidator) UIExecutorOption {
	return func(e *UIExecutor) { e.validator = v }
}

// WithPreviewManager sets the preview session manager.
func WithPreviewManager(mgr PreviewManager) UIExecutorOption {
	return func(e *UIExecutor) { e.previewMgr = mgr }
}

// WithPolicy sets the policy.
func WithPolicy(p Policy) UIExecutorOption {
	return func(e *UIExecutor) { e.policy = p }
}

// WithObserver wires the preview observer for closed-loop validation.
func WithObserver(obs preview.Observer) UIExecutorOption {
	return func(e *UIExecutor) { e.observer = obs }
}

// WithAutoRefiner wires the auto refiner for preview issue resolution.
func WithAutoRefiner(r preview.AutoRefiner) UIExecutorOption {
	return func(e *UIExecutor) { e.autoRefiner = r }
}

// WithRefineIterations sets the maximum refine iterations (default 2).
func WithRefineIterations(n int) UIExecutorOption {
	return func(e *UIExecutor) {
		if n > 0 {
			e.maxRefineIter = n
		}
	}
}

// NewUIExecutor creates a UIExecutor with the given options.
func NewUIExecutor(opts ...UIExecutorOption) *UIExecutor {
	e := &UIExecutor{
		policy:        DefaultPolicy(),
		maxRefineIter: preview.MaxRefineIterations,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Validate checks the plan against the policy.
func (e *UIExecutor) Validate(plan UIChangePlan) error {
	return e.policy.Validate(plan)
}

// ApplySourceEdits executes source-editing operations.
func (e *UIExecutor) ApplySourceEdits(ctx context.Context, plan UIChangePlan) (*SourceEditResult, error) {
	srcReq := SourceEditRequest{
		WorkspaceID: plan.Intent.Target.WorkspaceID,
		Transaction: plan.RollbackStrategy == RollbackEditTransaction,
	}

	for _, op := range plan.Operations {
		srcOp := SourceEditOperation{
			Path: op.Target,
		}
		if len(op.Payload) > 0 {
			var payload struct {
				Patch      string `json:"patch"`
				OldText    string `json:"oldText"`
				NewText    string `json:"newText"`
				BaseSHA256 string `json:"baseSha256"`
				SearchMode string `json:"searchMode"`
				ReplaceAll bool   `json:"replaceAll"`
				ExpectCount int   `json:"expectCount"`
			}
			if err := json.Unmarshal(op.Payload, &payload); err == nil {
				srcOp.Patch = payload.Patch
				srcOp.OldText = payload.OldText
				srcOp.NewText = payload.NewText
				srcOp.BaseSHA256 = payload.BaseSHA256
				srcOp.SearchMode = payload.SearchMode
				srcOp.ReplaceAll = payload.ReplaceAll
				srcOp.ExpectedCount = payload.ExpectCount
			}
		}
		srcReq.Operations = append(srcReq.Operations, srcOp)
	}

	if len(srcReq.Operations) == 0 {
		return nil, fmt.Errorf("no source operations to apply")
	}

	result, err := e.sourceEditor.call(ctx, srcReq)
	if err != nil {
		return nil, fmt.Errorf("apply source edits: %w", err)
	}

	return result, nil
}

// ApplySchema executes schema generation and rendering.
func (e *UIExecutor) ApplySchema(ctx context.Context, plan UIChangePlan) (*SchemaEditResult, error) {
	if e.schemaGen == nil {
		return nil, fmt.Errorf("schema generator not configured")
	}

	doc, err := e.schemaGen.Generate(plan.Intent.Description, nil)
	if err != nil {
		return nil, fmt.Errorf("generate schema: %w", err)
	}

	if e.validator != nil {
		validation := e.validator.Validate(doc)
		if !validation.Valid {
			return nil, fmt.Errorf("schema validation failed: %v", validation.Errors)
		}
	}

	if e.renderer != nil {
		_, renderErr := e.renderer.Project(ctx, doc, schema.RenderSchemaRequest{
			Document: doc,
			Platform: plan.Intent.Target.Platform,
		})
		if renderErr != nil {
			return nil, fmt.Errorf("render schema: %w", renderErr)
		}
	}

	return &SchemaEditResult{
		ChangedFiles: []string{"schema://" + doc.Title},
		SchemaID:     doc.Title,
	}, nil
}

// CreatePreview creates a preview session and runs the closed-loop
// Observe → Refine cycle to validate and repair schema issues.
func (e *UIExecutor) CreatePreview(ctx context.Context, plan UIChangePlan, result *UIResult) (string, error) {
	if e.previewMgr == nil {
		return "", fmt.Errorf("preview manager not configured")
	}
	session, err := e.previewMgr.Create(plan.Intent.Target.WorkspaceID, nil)
	if err != nil {
		return "", err
	}

	if e.observer != nil {
		obsResult, obsErr := e.observer.Capture(ctx, session.ID)
		if obsErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("preview observe: %v", obsErr))
		} else {
			result.ObserveIssues = obsResult.Errors
			if len(obsResult.Errors) == 0 {
				result.PreviewState = "clean"
			} else if obsResult.CanRefine && e.autoRefiner != nil {
				result.PreviewState = "refining"
				refineReq := preview.RefineRequest{
					SessionID:     session.ID,
					Observation:   obsResult,
					ChangedPaths:  obsResult.ChangedPaths,
					MaxIterations: e.maxRefineIter,
					Revision:      plan.ExecutionID,
				}
				if plan.Intent.Target.WorkspaceID != "" {
					refineReq.Target = &preview.PreviewTarget{
						WorkspaceID: plan.Intent.Target.WorkspaceID,
						Platform:    plan.Intent.Target.Platform,
						SourceType:  string(plan.Intent.Target.Type),
					}
				}
				refineResult, refineErr := e.autoRefiner.Refine(ctx, refineReq)
				if refineErr != nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("preview refine: %v", refineErr))
					result.PreviewState = "needs_manual"
				} else {
					result.RefineResult = refineResult
					result.PreviewState = refineResult.State
					if refineResult.RollbackToken != "" {
						result.RollbackToken = refineResult.RollbackToken
					}
				}
			} else {
				result.PreviewState = "needs_manual"
			}
		}
	}

	return session.ID, nil
}
