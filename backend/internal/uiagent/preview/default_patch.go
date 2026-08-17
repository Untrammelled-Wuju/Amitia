package preview

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/uiagent/schema"
)

type defaultPatchGenerator struct {
	validator schema.SchemaValidator
}

func NewDefaultPatchGenerator(v schema.SchemaValidator) PatchGenerator {
	return &defaultPatchGenerator{validator: v}
}

func (g *defaultPatchGenerator) GeneratePatch(ctx context.Context, obs *ObservationResult) (*Patch, error) {
	if obs == nil || len(obs.Errors) == 0 {
		return nil, nil
	}
	if g.validator == nil {
		return nil, fmt.Errorf("patch generator: validator not configured")
	}
	var fixes []Fix
	for _, err := range obs.Errors {
		fixes = append(fixes, Fix{
			Path:    "schema.root",
			Type:    "fix",
			Content: err,
		})
	}
	targetPaths := obs.ChangedPaths
	if len(targetPaths) == 0 {
		targetPaths = []string{"schema.root"}
	}
	return &Patch{
		SessionID:   obs.SessionID,
		Fixes:       fixes,
		TargetPaths: targetPaths,
	}, nil
}

type defaultApplier struct {
	sessions SessionManager
}

func NewDefaultApplier(mgr SessionManager) Applier {
	return &defaultApplier{sessions: mgr}
}

func (a *defaultApplier) ApplyPatch(ctx context.Context, sessionID string, patch *Patch) (string, error) {
	if patch == nil {
		return "", fmt.Errorf("applier: nil patch")
	}
	if a.sessions == nil {
		return "", fmt.Errorf("applier: session manager not configured")
	}
	session, err := a.sessions.Get(sessionID)
	if err != nil {
		return "", fmt.Errorf("applier: get session: %w", err)
	}
	if session.Schema == nil {
		return "", fmt.Errorf("applier: session schema is nil")
	}
	for _, fix := range patch.Fixes {
		applySchemaFix(session.Schema, fix)
	}
	return fmt.Sprintf("tx_%s_%d", sessionID, len(patch.Fixes)), nil
}

func applySchemaFix(doc *schema.SchemaUIDocument, fix Fix) {
	if doc == nil {
		return
	}
	switch fix.Type {
	case "fix":
		if doc.Layout == nil {
			doc.Layout = map[string]any{}
		}
		doc.Layout["last_fix"] = fix.Content
	}
}
