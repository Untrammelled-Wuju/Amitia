package interaction

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/temporal"
	"gorm.io/gorm"
)

type ownedRuntimeContextKey struct{}

func RuntimeFromContext(ctx context.Context) (*RuntimeAssembly, bool) {
	runtime, ok := ctx.Value(ownedRuntimeContextKey{}).(*RuntimeAssembly)
	return runtime, ok && runtime != nil
}

type relationshipTimeCommitter interface {
	FinalizeCommittedTx(context.Context, *gorm.DB, string, string, string, *temporal.RelationshipTimeContext, bool, string, bool) error
}

func (o *Orchestrator) PrepareOwnedRuntime(ctx context.Context, record *InteractionRecord, req *ProcessRequest) (context.Context, *RuntimeAssembly, func(context.Context, bool) error, error) {
	if o == nil || o.pipeline == nil || record == nil || req == nil || record.Status != InteractionStatusProcessing {
		return ctx, nil, nil, ErrOrchestratorInvalidScope
	}
	ctx = temporal.ContextWithDeviceTimezone(ctx, req.DeviceTimezone)
	var prepared *temporal.RelationshipTimeContext
	if _, ok := coordination.FromContext(ctx); !ok {
		var err error
		prepared, err = o.prepareRelationshipTime(ctx, record, req)
		if err != nil {
			return ctx, nil, nil, err
		}
	}
	fail := func(err error) (context.Context, *RuntimeAssembly, func(context.Context, bool) error, error) {
		o.releaseRelationshipClaimIfUncommitted(ctx, record.ID, "owned_runtime_failed")
		return ctx, nil, nil, err
	}
	runtime := o.pipeline.Assemble(ctx, record.Scope, req)
	o.attachRelationshipTime(&runtime, prepared)
	runtime.ExecutorID = "owned"
	if runtime.Safety.Blocked {
		return fail(fmt.Errorf("%w: %s", ErrOrchestratorSafetyBlocked, strings.Join(runtime.Safety.Reasons, ",")))
	}
	if _, err := o.tracker.UpdateMetadata(ctx, record.ID, metadataFromRuntime(runtime, ctx)); err != nil {
		return fail(err)
	}
	req.InteractionID, req.Runtime = record.ID, &runtime
	finish := func(current context.Context, committed bool) error {
		if !committed {
			o.releaseRelationshipClaimIfUncommitted(current, record.ID, "owned_turn_failed")
			return nil
		}
		if prepared != nil {
			committer, ok := o.relationshipTimeCoordinator().(relationshipTimeCommitter)
			if !ok {
				return errors.New("关系时间提交端口未就绪")
			}
			suppress, reason := false, ""
			if prepared.Policy != nil {
				policy := prepared.Policy
				suppress = policy.MentionMode == "none" || policy.MaxMentionSentences == 0
				reason = policy.SuppressionReason
			}
			if err := committer.FinalizeCommittedTx(current, nil, record.Scope.SpaceID, record.Scope.CharacterID, record.ID, prepared, suppress, reason, req.IsInternal); err != nil {
				return err
			}
		}
		o.forgetPreparedRelationshipClaim(record.ID)
		return nil
	}
	return context.WithValue(ctx, ownedRuntimeContextKey{}, &runtime), &runtime, finish, nil
}
