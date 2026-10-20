package interaction

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/temporal"
)

type ownedRuntimeContextKey struct{}

func RuntimeFromContext(ctx context.Context) (*RuntimeAssembly, bool) {
	runtime, ok := ctx.Value(ownedRuntimeContextKey{}).(*RuntimeAssembly)
	return runtime, ok && runtime != nil
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
