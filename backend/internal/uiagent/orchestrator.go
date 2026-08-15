package uiagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

var (
	ErrIntentRequired   = errors.New("ui orchestrator: intent is required")
	ErrExecutorNotReady = errors.New("ui orchestrator: executor not ready")
	ErrUnsupportedMode  = errors.New("ui orchestrator: unsupported target mode")
)

// Execute executes a UI change plan from start to finish,
// coordinating between source editing, schema generation, and preview.
func Execute(ctx context.Context, plan UIChangePlan, executor *UIExecutor) (*UIResult, error) {
	if executor == nil {
		return nil, ErrExecutorNotReady
	}
	if err := executor.Validate(plan); err != nil {
		return nil, err
	}

	result := &UIResult{
		State: "started",
	}

	switch plan.Mode {
	case UITargetSource:
		sourceResult, err := executor.ApplySourceEdits(ctx, plan)
		if err != nil {
			result.State = "failed"
			result.Warnings = append(result.Warnings, err.Error())
			return result, err
		}
		result.ChangedFiles = sourceResult.ChangedFiles
		result.State = "completed"

	case UITargetSchema:
		schemaResult, err := executor.ApplySchema(ctx, plan)
		if err != nil {
			result.State = "failed"
			result.Warnings = append(result.Warnings, err.Error())
			return result, err
		}
		result.ChangedFiles = schemaResult.ChangedFiles
		result.State = "completed"

	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedMode, plan.Mode)
	}

	if plan.PreviewStrategy != "" && result.State == "completed" {
		previewToken, err := executor.CreatePreview(ctx, plan, result)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("preview: %v", err))
		} else {
			result.PreviewRefs = append(result.PreviewRefs, previewToken)
		}
	}

	return result, nil
}

// PlanIntent builds a UIChangePlan from a UIIntent by resolving the mode,
// inferring required capabilities, and classifying risk.
func PlanIntent(intent UIIntent, resolver ModeResolver, policy Policy) (*UIChangePlan, error) {
	if intent.Action == "" && intent.Description == "" {
		return nil, ErrIntentRequired
	}

	mode, err := resolver.Resolve(intent)
	if err != nil {
		return nil, fmt.Errorf("resolve mode: %w", err)
	}

	plan := UIChangePlan{
		Intent:               intent,
		Mode:                 mode,
		RequiredCapabilities: RequiredCapabilitiesForMode[mode],
		Risk:                 policy.ClassifyRisk(UIChangePlan{Mode: mode}),
		PreviewStrategy:      PreviewStructural,
		RollbackStrategy:     RollbackEditTransaction,
		ExecContext:          intent.ExecContext,
	}

	if intent.ExecutionID != "" {
		plan.ExecutionID = intent.ExecutionID
	}
	if intent.RootExecutionID != "" {
		plan.RootExecutionID = intent.RootExecutionID
	}

	if intent.Target.Platform != "" {
		plan.TargetRuntime = &DeploymentTarget{
			Placement: capability.ProviderPlacement(intent.Target.Platform),
		}
	}

	payload, _ := json.Marshal(map[string]string{
		"description": intent.Description,
		"action":      string(intent.Action),
	})

	plan.Operations = []UIOperation{
		{
			Type:    string(intent.Action),
			Target:  string(mode),
			Payload: payload,
		},
	}

	if err := policy.Validate(plan); err != nil {
		return nil, fmt.Errorf("policy violation: %w", err)
	}

	return &plan, nil
}

// PlanSummary provides a summary of a plan for logging/telemetry.
func PlanSummary(plan *UIChangePlan) map[string]interface{} {
	if plan == nil {
		return nil
	}
	return map[string]interface{}{
		"mode":           string(plan.Mode),
		"operations":     len(plan.Operations),
		"risk":           string(plan.Risk),
		"preview":        string(plan.PreviewStrategy),
		"rollback":       string(plan.RollbackStrategy),
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
		"hasExecContext": plan.ExecContext != nil,
	}
}
