package adapters

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/uiagent/schema"
)

// DiagnosticRunner runs external diagnostic tools (dart analyze, flutter analyze, tsc, etc.)
type DiagnosticRunner interface {
	RunDiagnostics(ctx context.Context, workspaceID string, paths []string) ([]string, []string, error)
	Supports(platform string) bool
}

type defaultDiagnosticRunner struct{}

func NewDiagnosticRunner() DiagnosticRunner {
	return &defaultDiagnosticRunner{}
}

func (r *defaultDiagnosticRunner) RunDiagnostics(ctx context.Context, workspaceID string, paths []string) ([]string, []string, error) {
	return []string{"external diagnostic runner not configured"}, nil, nil
}

func (r *defaultDiagnosticRunner) Supports(platform string) bool {
	return false
}

type PreviewMode string

const (
	PreviewModeSchema  PreviewMode = "schema"
	PreviewModeWeb     PreviewMode = "web_source"
	PreviewModeFlutter PreviewMode = "flutter_source"
)

type PreviewDiagnostics struct {
	StructuralTree interface{} `json:"structuralTree,omitempty"`
	CompileErrors  []string    `json:"compileErrors,omitempty"`
	RuntimeErrors  []string    `json:"runtimeErrors,omitempty"`
	SchemaErrors   []string    `json:"schemaErrors,omitempty"`
	SchemaWarnings []string    `json:"schemaWarnings,omitempty"`
}

type PreviewRequest struct {
	Mode           PreviewMode             `json:"mode"`
	Document       *schema.SchemaUIDocument `json:"document,omitempty"`
	Target         *PreviewTarget          `json:"target,omitempty"`
	TransactionID  string                  `json:"transactionId,omitempty"`
	RootExecution  string                  `json:"rootExecution,omitempty"`
	Revision       string                  `json:"revision,omitempty"`
}

type PreviewTarget struct {
	WorkspaceID string `json:"workspaceId,omitempty"`
	Platform    string `json:"platform,omitempty"`
	SourceType  string `json:"sourceType,omitempty"`
}

type PreviewAdapter interface {
	Generate(ctx context.Context, req PreviewRequest) (*PreviewDiagnostics, error)
}

type schemaPreviewAdapter struct {
	compiler  schema.SchemaCompiler
	validator schema.SchemaValidator
}

func NewSchemaPreviewAdapter(c schema.SchemaCompiler, v schema.SchemaValidator) PreviewAdapter {
	return &schemaPreviewAdapter{compiler: c, validator: v}
}

func (a *schemaPreviewAdapter) Generate(ctx context.Context, req PreviewRequest) (*PreviewDiagnostics, error) {
	d := &PreviewDiagnostics{}
	if req.Document == nil {
		return d, fmt.Errorf("schema document required")
	}

	if a.validator != nil {
		result := a.validator.Validate(req.Document)
		if !result.Valid {
			d.SchemaErrors = result.Errors
		}
		d.SchemaWarnings = result.Warnings
	}

	if a.compiler != nil {
		compiled, err := a.compiler.Compile(ctx, req.Document)
		if err != nil {
			d.CompileErrors = append(d.CompileErrors, err.Error())
		} else if compiled != nil {
			d.StructuralTree = map[string]interface{}{
				"widgetCount": compiled.WidgetCount,
			}
		}
	}

	return d, nil
}

type webSourcePreviewAdapter struct {
	diagnosticRunner DiagnosticRunner
}

func NewWebSourcePreviewAdapter() PreviewAdapter {
	return &webSourcePreviewAdapter{diagnosticRunner: NewDiagnosticRunner()}
}

func NewWebSourcePreviewAdapterWithRunner(runner DiagnosticRunner) PreviewAdapter {
	return &webSourcePreviewAdapter{diagnosticRunner: runner}
}

func (a *webSourcePreviewAdapter) Generate(ctx context.Context, req PreviewRequest) (*PreviewDiagnostics, error) {
	d := &PreviewDiagnostics{}
	if a.diagnosticRunner != nil && a.diagnosticRunner.Supports("web") {
		compileErrors, warnings, err := a.diagnosticRunner.RunDiagnostics(ctx, req.Target.WorkspaceID, nil)
		if err != nil {
			return d, fmt.Errorf("web diagnostic failed: %w", err)
		}
		d.CompileErrors = compileErrors
		d.SchemaWarnings = warnings
	} else {
		d.CompileErrors = append(d.CompileErrors, "web diagnostic runner not available")
	}
	return d, nil
}

type flutterSourcePreviewAdapter struct {
	diagnosticRunner DiagnosticRunner
}

func NewFlutterSourcePreviewAdapter() PreviewAdapter {
	return &flutterSourcePreviewAdapter{diagnosticRunner: NewDiagnosticRunner()}
}

func NewFlutterSourcePreviewAdapterWithRunner(runner DiagnosticRunner) PreviewAdapter {
	return &flutterSourcePreviewAdapter{diagnosticRunner: runner}
}

func (a *flutterSourcePreviewAdapter) Generate(ctx context.Context, req PreviewRequest) (*PreviewDiagnostics, error) {
	d := &PreviewDiagnostics{}
	if a.diagnosticRunner != nil && a.diagnosticRunner.Supports("flutter") {
		compileErrors, warnings, err := a.diagnosticRunner.RunDiagnostics(ctx, req.Target.WorkspaceID, nil)
		if err != nil {
			return d, fmt.Errorf("flutter diagnostic failed: %w", err)
		}
		d.CompileErrors = compileErrors
		d.SchemaWarnings = warnings
	} else {
		d.CompileErrors = append(d.CompileErrors, "flutter diagnostic runner not available")
	}
	return d, nil
}
