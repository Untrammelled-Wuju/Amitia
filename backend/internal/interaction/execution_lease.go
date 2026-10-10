package interaction

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type executionLeaseContextKey struct{}

type ExecutionLeaseIdentity struct {
	InteractionID   string
	OwnerInstanceID string
}

func ClaimedExecutionLease(ctx context.Context) (ExecutionLeaseIdentity, bool) {
	if ctx == nil {
		return ExecutionLeaseIdentity{}, false
	}
	claim, ok := ctx.Value(executionLeaseContextKey{}).(ExecutionLeaseIdentity)
	return claim, ok && claim.InteractionID != "" && claim.OwnerInstanceID != ""
}

const executionLeaseStaleAfter = 90 * time.Second
const executionLeaseHeartbeatInterval = 10 * time.Second

type executionLeaseTracker interface {
	ClaimExecution(ctx context.Context, id string, version int64, owner string, staleBefore time.Time) (bool, error)
	RenewExecution(ctx context.Context, id string, owner string, now time.Time) (bool, error)
	ReleaseExecution(ctx context.Context, id string, owner string) error
}

func (t *SQLiteInteractionTracker) ClaimExecution(ctx context.Context, id string, version int64, owner string, staleBefore time.Time) (bool, error) {
	if id == "" || owner == "" {
		return false, ErrInteractionCASConflict
	}
	now := t.now()
	res := t.db.WithContext(ctx).Model(&InteractionRecordModel{}).
		Where("id = ? AND status_version = ? AND status IN ? AND commit_id = '' AND superseded_by_id = '' AND cancel_reason = ''",
			id, version, []string{string(InteractionStatusProcessing), string(InteractionStatusContextReady)}).
		Where("(owner_instance_id = '' OR heartbeat_at < ? OR heartbeat_at IS NULL)", staleBefore).
		Updates(map[string]interface{}{
			"owner_instance_id": owner,
			"heartbeat_at":      now,
		})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

func (t *SQLiteInteractionTracker) RenewExecution(ctx context.Context, id string, owner string, now time.Time) (bool, error) {
	res := t.db.WithContext(ctx).Model(&InteractionRecordModel{}).
		Where("id = ? AND owner_instance_id = ? AND status NOT IN ?",
			id, owner, terminalStatusStrings()).
		Updates(map[string]interface{}{"heartbeat_at": now})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

func (t *SQLiteInteractionTracker) ReleaseExecution(ctx context.Context, id string, owner string) error {
	return t.db.WithContext(ctx).Model(&InteractionRecordModel{}).
		Where("id = ? AND owner_instance_id = ?", id, owner).
		Updates(map[string]interface{}{
			"owner_instance_id": "",
			"heartbeat_at":      time.Time{},
		}).Error
}

func (o *Orchestrator) claimExecution(ctx context.Context, record *InteractionRecord) (context.Context, func(), error) {
	leaser, ok := o.tracker.(executionLeaseTracker)
	if !ok {
		return ctx, func() {}, nil
	}
	owner := uuid.NewString()
	now := time.Now().UTC()
	claimed, err := leaser.ClaimExecution(ctx, record.ID, record.StatusVersion, owner, now.Add(-executionLeaseStaleAfter))
	if err != nil {
		return ctx, nil, err
	}
	if !claimed {
		return ctx, nil, ErrOrchestratorProcessing
	}
	leaseCtx, cancel := context.WithCancel(ctx)
	leaseCtx = context.WithValue(leaseCtx, executionLeaseContextKey{}, ExecutionLeaseIdentity{
		InteractionID: record.ID, OwnerInstanceID: owner,
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(executionLeaseHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-leaseCtx.Done():
				return
			case <-ticker.C:
				pingCtx, releasePing := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
				renewed, renewErr := leaser.RenewExecution(pingCtx, record.ID, owner, time.Now().UTC())
				releasePing()
				if renewErr != nil || !renewed {
					cancel()
					return
				}
			}
		}
	}()
	release := func() {
		cancel()
		<-done
		releaseCtx, cancelRelease := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelRelease()
		_ = leaser.ReleaseExecution(releaseCtx, record.ID, owner)
	}
	return leaseCtx, release, nil
}

var _ executionLeaseTracker = (*SQLiteInteractionTracker)(nil)
