package interaction

import (
	"context"
	"time"
)

const executionLeaseStaleAfter = 90 * time.Second

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

var _ executionLeaseTracker = (*SQLiteInteractionTracker)(nil)
