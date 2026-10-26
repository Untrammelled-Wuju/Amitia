package interaction

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/safety"
	"github.com/u-ai/backend/internal/temporal"
)

type ownedRuntimeContextKey struct{}

func RuntimeFromContext(ctx context.Context) (*RuntimeAssembly, bool) {
	runtime, ok := ctx.Value(ownedRuntimeContextKey{}).(*RuntimeAssembly)
	return runtime, ok && runtime != nil
}

func (r *RuntimeAssembly) EvaluateGeneratedText(ctx context.Context, text string) (safety.PostGenOutput, error) {
	result := r.CheckGeneratedText(text)
	if !result.Audited {
		return result, nil
	}
	if r.safetyRecorder == nil {
		return safety.PostGenOutput{}, fmt.Errorf("%w: safety_incident_store_unavailable", ErrOrchestratorSafetyBlocked)
	}
	incident := r.safetyIncident
	incident.Description = strings.Join(result.Reasons, ",")
	incident.Status = "open"
	incident.PolicyRevision = r.SafetyPolicy.Revision
	details, err := json.Marshal(map[string]any{"action": result.Action, "reasons": result.Reasons, "replacement": result.Replacement, "bdiRevision": r.SafetyPolicy.BDI.Revision, "copingStrategy": copingStrategyOf(r.ExpressionPlan), "emotionIntensity": emotionIntensityOf(r.ExpressionPlan)})
	if err != nil {
		return safety.PostGenOutput{}, err
	}
	incident.DetailsJSON = string(details)
	if err := r.safetyRecorder.RecordIncident(ctx, incident); err != nil {
		return safety.PostGenOutput{}, err
	}
	return result, nil
}

func copingStrategyOf(plan *decision.ExpressionPlan) string {
	if plan == nil {
		return ""
	}
	return string(plan.CopingStrategy)
}

func emotionIntensityOf(plan *decision.ExpressionPlan) float64 {
	if plan == nil {
		return 0
	}
	return plan.EmotionIntensity
}

func (r *RuntimeAssembly) CheckGeneratedText(text string) safety.PostGenOutput {
	if r == nil || r.safetyGovernor == nil || strings.TrimSpace(text) == "" {
		return safety.PostGenOutput{Allowed: true, Cleaned: text}
	}
	return r.safetyGovernor.CheckPostGen(safety.PostGenInput{GeneratedText: text})
}

func (o *RuntimeContextService) PrepareOwnedRuntime(ctx context.Context, record *InteractionRecord, req *ProcessRequest) (context.Context, *RuntimeAssembly, func(context.Context, bool) error, error) {
	if o == nil || o.pipeline == nil || record == nil || req == nil || record.Status != InteractionStatusProcessing {
		return ctx, nil, nil, ErrOrchestratorInvalidScope
	}
	scope, owned := coordination.FromContext(ctx)
	_, hasSnapshot := ownedContextFromContext(ctx)
	if !owned || !hasSnapshot || scope.RoleID != record.Scope.CharacterID || scope.SpaceID != record.Scope.SpaceID || req.CharacterID != scope.RoleID || req.ConversationID != record.Scope.ConversationID {
		return ctx, nil, nil, coordination.ErrWrongOwner
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return ctx, nil, nil, err
	}
	select {
	case o.slots <- struct{}{}:
	case <-ctx.Done():
		return ctx, nil, nil, context.Cause(ctx)
	}
	var once sync.Once
	release := func() { once.Do(func() { <-o.slots }) }
	preparedOK := false
	defer func() {
		if !preparedOK {
			release()
		}
	}()
	ctx = temporal.ContextWithDeviceTimezone(ctx, req.DeviceTimezone)
	runtime := o.pipeline.Assemble(ctx, record.Scope, req)
	runtime.ExecutorID = "owned"
	if runtime.Safety.Blocked {
		return ctx, nil, nil, fmt.Errorf("%w: %s", ErrOrchestratorSafetyBlocked, strings.Join(runtime.Safety.Reasons, ","))
	}
	if _, err := o.tracker.UpdateMetadata(ctx, record.ID, metadataFromRuntime(runtime, ctx)); err != nil {
		return ctx, nil, nil, err
	}
	req.InteractionID, req.Runtime = record.ID, &runtime
	finish := func(context.Context, bool) error { release(); return nil }
	preparedOK = true
	return context.WithValue(ctx, ownedRuntimeContextKey{}, &runtime), &runtime, finish, nil
}
