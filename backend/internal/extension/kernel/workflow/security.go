package workflow

import (
	"context"
	"fmt"
)

type PermissionCheckFunc func(ctx context.Context, extensionID, moduleID string, permissions []string, background bool) error
type ScopeCheckFunc func(ctx context.Context, extensionID, moduleID, scopeName string, execution ExecutionContext) error
type GenerationCheckFunc func(ctx context.Context, extensionID string, generation int64) error

type SecurityGuard struct {
	PermissionCheck PermissionCheckFunc
	ScopeCheck      ScopeCheckFunc
	GenerationCheck GenerationCheckFunc
}

func (g *SecurityGuard) Check(ctx context.Context, definition WorkflowDefinition, node WorkflowNode, execution ExecutionContext) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if definition.ExtensionID != "" && execution.ExtensionID != definition.ExtensionID {
		return fmt.Errorf("%w: workflow owner mismatch", ErrScopeDenied)
	}
	if definition.ModuleID != "" && execution.ModuleID != definition.ModuleID {
		return fmt.Errorf("%w: workflow module mismatch", ErrScopeDenied)
	}
	permissions := make([]string, 0, len(definition.Permissions)+len(node.Permissions))
	permissions = append(permissions, definition.Permissions...)
	permissions = append(permissions, node.Permissions...)
	if len(permissions) > 0 {
		if g == nil || g.PermissionCheck == nil {
			return fmt.Errorf("%w: permission checker not configured", ErrPermissionDenied)
		}
		if err := g.PermissionCheck(ctx, execution.ExtensionID, execution.ModuleID, permissions, execution.ScheduleID != ""); err != nil {
			return fmt.Errorf("%w: %v", ErrPermissionDenied, err)
		}
	}
	scopeName := node.Scope
	if scopeName == "" {
		scopeName = definition.Scope
	}
	if scopeName != "" && scopeName != "global" {
		if g == nil || g.ScopeCheck == nil {
			return fmt.Errorf("%w: scope checker not configured", ErrScopeDenied)
		}
		if err := g.ScopeCheck(ctx, execution.ExtensionID, execution.ModuleID, scopeName, execution); err != nil {
			return fmt.Errorf("%w: %v", ErrScopeDenied, err)
		}
	}
	if execution.Generation > 0 {
		if g == nil || g.GenerationCheck == nil {
			return fmt.Errorf("%w: generation checker not configured", ErrGenerationMismatch)
		}
		if err := g.GenerationCheck(ctx, execution.ExtensionID, execution.Generation); err != nil {
			return fmt.Errorf("%w: %v", ErrGenerationMismatch, err)
		}
	}
	return nil
}
