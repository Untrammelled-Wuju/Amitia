package preview

import (
	"context"
	"fmt"
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
}

type Observer interface {
	Capture(ctx context.Context, sessionID string) (*ObservationResult, error)
}

type defaultObserver struct {
	sessions  SessionManager
	validator schema.SchemaValidator
}

func NewObserver(mgr SessionManager, v schema.SchemaValidator) Observer {
	return &defaultObserver{
		sessions:  mgr,
		validator: v,
	}
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

	if session.Schema == nil {
		result.Errors = append(result.Errors, "schema is nil")
		return result, nil
	}

	if o.validator != nil {
		validationResult := o.validator.Validate(session.Schema)
		if !validationResult.Valid {
			result.Errors = append(result.Errors, validationResult.Errors...)
		}
		result.Warnings = append(result.Warnings, validationResult.Warnings...)
	}

	result.ChangedPaths = detectSchemaIssues(session.Schema)
	result.CanRefine = len(result.Errors) > 0 && len(result.Errors) <= 5
	return result, nil
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
