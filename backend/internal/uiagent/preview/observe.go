package preview

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/uiagent/schema"
)

type ObservationResult struct {
	SessionID      string      `json:"sessionId"`
	PreviewToken   string      `json:"previewToken,omitempty"`
	StructuralRepr interface{} `json:"structuralRepr,omitempty"`
	ScreenshotRef  string      `json:"screenshotRef,omitempty"`
	WidgetTree     interface{} `json:"widgetTree,omitempty"`
	ChangedPaths   []string    `json:"changedPaths,omitempty"`
	Raw            interface{} `json:"raw,omitempty"`
	CapturedAt     time.Time   `json:"capturedAt"`
	CanRefine      bool        `json:"canRefine"`
	Errors         []string    `json:"errors,omitempty"`
	Warnings       []string    `json:"warnings,omitempty"`
	CompileErrors  []string    `json:"compileErrors,omitempty"`
	RuntimeErrors  []string    `json:"runtimeErrors,omitempty"`
	OverflowErrors []string    `json:"overflowErrors,omitempty"`
	BindingErrors  []string    `json:"bindingErrors,omitempty"`
	ActionErrors   []string    `json:"actionErrors,omitempty"`
	Platform       string      `json:"platform,omitempty"`
}

func (r *ObservationResult) AllErrors() []string {
	var all []string
	all = append(all, r.Errors...)
	all = append(all, r.CompileErrors...)
	all = append(all, r.RuntimeErrors...)
	all = append(all, r.OverflowErrors...)
	all = append(all, r.BindingErrors...)
	all = append(all, r.ActionErrors...)
	return all
}

func ShouldBlockCommit(obs *ObservationResult) bool {
	if obs == nil {
		return false
	}
	if len(obs.Errors) > 0 {
		return true
	}
	if len(obs.CompileErrors) > 0 {
		return true
	}
	if len(obs.RuntimeErrors) > 0 {
		return true
	}
	if len(obs.BindingErrors) > 0 {
		return true
	}
	if len(obs.ActionErrors) > 0 {
		return true
	}
	if len(obs.OverflowErrors) > 0 {
		return true
	}
	return false
}

type Observer interface {
	Capture(ctx context.Context, sessionID string) (*ObservationResult, error)
}

type ObserverOption func(*defaultObserver)

func WithDiagnosticRunner(runner DiagnosticRunner) ObserverOption {
	return func(o *defaultObserver) {
		o.diagnosticRunner = runner
	}
}

type DiagnosticRunner interface {
	RunDiagnostics(ctx context.Context, workspaceID string, paths []string) ([]string, []string, error)
	Supports(platform string) bool
}

type PlatformDiagnosticRunner interface {
	DiagnosticRunner
	RunPlatformDiagnostics(ctx context.Context, platform string, workspaceID string, paths []string) ([]string, []string, error)
}

type defaultObserver struct {
	sessions         SessionManager
	validator        schema.SchemaValidator
	diagnosticRunner DiagnosticRunner
}

func NewObserver(mgr SessionManager, v schema.SchemaValidator, opts ...ObserverOption) Observer {
	o := &defaultObserver{
		sessions:  mgr,
		validator: v,
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func (o *defaultObserver) Capture(ctx context.Context, sessionID string) (*ObservationResult, error) {
	result := &ObservationResult{
		SessionID:  sessionID,
		CapturedAt: time.Now(),
		CanRefine:  false,
	}

	if o.sessions == nil {
		return result, nil
	}

	session, err := o.sessions.Get(sessionID)
	if err != nil {
		return result, fmt.Errorf("get session: %w", err)
	}

	if session.Schema == nil && (session.Target == nil || session.Target.SourceType != "source") {
		result.Errors = append(result.Errors, "schema is nil")
		return result, nil
	}

	if session.Schema != nil && o.validator != nil {
		validationResult := o.validator.Validate(session.Schema)
		if !validationResult.Valid {
			for _, e := range validationResult.Errors {
				categorizeSchemaError(e, result)
			}
		}
		result.Warnings = append(result.Warnings, validationResult.Warnings...)
	}

	if session.Schema != nil {
		result.ChangedPaths = detectSchemaIssues(session.Schema)
	}

	if session.Target != nil && session.Target.Platform != "" && o.diagnosticRunner != nil {
		result.Platform = session.Target.Platform
		if o.diagnosticRunner.Supports(session.Target.Platform) {
			var compileErrors, warnings []string
			var diagErr error
			if platformRunner, ok := o.diagnosticRunner.(PlatformDiagnosticRunner); ok {
				compileErrors, warnings, diagErr = platformRunner.RunPlatformDiagnostics(ctx, session.Target.Platform, session.WorkspaceID, nil)
			} else {
				compileErrors, warnings, diagErr = o.diagnosticRunner.RunDiagnostics(ctx, session.WorkspaceID, nil)
			}
			if diagErr != nil {
				result.RuntimeErrors = append(result.RuntimeErrors, fmt.Sprintf("diagnostic error: %v", diagErr))
			}
			for _, ce := range compileErrors {
				categorizeDiagnosticError(ce, result)
			}
			result.Warnings = append(result.Warnings, warnings...)
		}
	}

	result.CanRefine = len(result.AllErrors()) > 0 && len(result.AllErrors()) <= 5
	return result, nil
}

func categorizeSchemaError(err string, result *ObservationResult) {
	lower := strings.ToLower(err)
	if strings.Contains(lower, "missing required property") {
		result.Errors = append(result.Errors, err)
		return
	}
	if strings.Contains(lower, "does not allow child type") || strings.Contains(lower, "unknown component type") {
		result.OverflowErrors = append(result.OverflowErrors, err)
		return
	}
	if strings.Contains(lower, "binding") && strings.Contains(lower, "dangerous") {
		result.BindingErrors = append(result.BindingErrors, err)
		return
	}
	if strings.Contains(lower, "action") {
		result.ActionErrors = append(result.ActionErrors, err)
		return
	}
	result.Errors = append(result.Errors, err)
}

func categorizeDiagnosticError(line string, result *ObservationResult) {
	lower := strings.ToLower(line)
	if strings.Contains(lower, "error") || strings.Contains(lower, "exception") {
		if strings.Contains(lower, "runtime") || strings.Contains(lower, "null") || strings.Contains(lower, "undefined") {
			result.RuntimeErrors = append(result.RuntimeErrors, line)
		} else {
			result.CompileErrors = append(result.CompileErrors, line)
		}
		return
	}
	if strings.Contains(lower, "overflow") || strings.Contains(lower, "rangeerror") {
		result.OverflowErrors = append(result.OverflowErrors, line)
		return
	}
	if strings.Contains(lower, "binding") {
		result.BindingErrors = append(result.BindingErrors, line)
		return
	}
	if strings.Contains(lower, "action") || strings.Contains(lower, "event") {
		result.ActionErrors = append(result.ActionErrors, line)
		return
	}
}

func detectSchemaIssues(doc *schema.SchemaUIDocument) []string {
	var paths []string
	for i, child := range doc.Children {
		detectNodeIssues(&child, fmt.Sprintf("children[%d]", i), &paths)
	}
	return paths
}

func detectNodeIssues(node *schema.SchemaUINode, path string, paths *[]string) {
	hasActionTarget := false
	for _, action := range node.Actions {
		if action.Target != "" {
			hasActionTarget = true
			break
		}
	}
	if len(node.Actions) > 0 && !hasActionTarget {
		*paths = append(*paths, path+".actions: missing target")
	}

	for i, binding := range node.Bindings {
		if binding.Source == "" {
			*paths = append(*paths, fmt.Sprintf("%s.bindings[%d]: empty source", path, i))
		}
	}

	for i, child := range node.Children {
		detectNodeIssues(&child, fmt.Sprintf("%s.children[%d]", path, i), paths)
	}
}
