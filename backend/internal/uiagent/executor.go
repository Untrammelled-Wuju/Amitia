package uiagent

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/uiagent/schema"
)

// SourceEditOperation describes a single file edit.
type SourceEditOperation struct {
	Path          string `json:"path"`
	OldText       string `json:"oldText,omitempty"`
	NewText       string `json:"newText,omitempty"`
	Patch         string `json:"patch,omitempty"`
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

// UIExecutor orchestrates the concrete operations behind a UI change plan.
type UIExecutor struct {
	sourceEditor sourceEditorAdapter
	schemaGen    *schema.AISchemaGenerator
	renderer     *schema.FlutterRenderer
	validator    schema.SchemaValidator
	previewMgr   PreviewManager
	policy       Policy
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

// WithRenderer sets the Flutter renderer.
func WithRenderer(r *schema.FlutterRenderer) UIExecutorOption {
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

// NewUIExecutor creates a UIExecutor with the given options.
func NewUIExecutor(opts ...UIExecutorOption) *UIExecutor {
	e := &UIExecutor{
		policy: DefaultPolicy(),
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
		srcReq.Operations = append(srcReq.Operations, SourceEditOperation{
			Path: op.Target,
		})
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
		_, renderErr := e.renderer.Render(ctx, doc, schema.RenderSchemaRequest{
			Document: doc,
			Platform: plan.Intent.Target.Platform,
		})
		if renderErr != nil {
			return nil, fmt.Errorf("render schema: %w", renderErr)
		}
	}

	return &SchemaEditResult{
		ChangedFiles: []string{"schema://" + doc.SchemaID},
		SchemaID:     doc.SchemaID,
	}, nil
}

// CreatePreview creates a preview session for the result.
func (e *UIExecutor) CreatePreview(ctx context.Context, plan UIChangePlan, result *UIResult) (string, error) {
	if e.previewMgr == nil {
		return "", fmt.Errorf("preview manager not configured")
	}
	session, err := e.previewMgr.Create(plan.Intent.Target.WorkspaceID, nil)
	if err != nil {
		return "", err
	}
	return session.ID, nil
}
