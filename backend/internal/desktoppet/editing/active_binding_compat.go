package editing

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// resolvedActiveRevisionBinding is the compatibility view used while the
// editing UI still addresses an action by processingTaskID/actionKey. The
// ActionStream binding is authoritative whenever it exists; the legacy
// per-processing-task binding is maintained only as a compatibility mirror for
// older editor/recovery code that has not yet moved to stream IDs.
type resolvedActiveRevisionBinding struct {
	RevisionID      string
	BindingRevision int64
	UserID          string
	CharacterID     string
	ActionStreamID  string
	Canonical       bool
}

func resolveActiveRevisionBinding(repo Repository, processingTaskID, actionKey string) (*resolvedActiveRevisionBinding, error) {
	canonical, err := repo.GetActiveActionRevisionBindingByTask(processingTaskID, actionKey)
	if err != nil {
		return nil, err
	}
	if canonical != nil {
		return &resolvedActiveRevisionBinding{
			RevisionID:      canonical.ActiveActionRevisionID,
			BindingRevision: canonical.BindingRevision,
			UserID:          canonical.UserID,
			CharacterID:     canonical.CharacterID,
			ActionStreamID:  canonical.ActionStreamID,
			Canonical:       true,
		}, nil
	}

	legacy, err := repo.GetActiveRevisionBinding(processingTaskID, actionKey)
	if err != nil {
		return nil, err
	}
	if legacy == nil {
		return nil, nil
	}
	revisionID := legacy.RevisionID
	if revisionID == "" {
		revisionID = legacy.ActiveActionRevisionID
	}
	revision := legacy.BindingVersion
	if legacy.BindingRevision > revision {
		revision = legacy.BindingRevision
	}
	return &resolvedActiveRevisionBinding{
		RevisionID:      revisionID,
		BindingRevision: revision,
		UserID:          legacy.UserID,
		CharacterID:     legacy.CharacterID,
		Canonical:       false,
	}, nil
}

// bindActiveRevision makes the ActionStream binding the sole write authority
// for canonical revisions and mirrors the result to the old task/action table
// in the same transaction. Legacy revisions without an ActionStream keep the
// old CAS path so existing migrated data remains editable.
func bindActiveRevision(repo Repository, processingTaskID, actionKey, revisionID string, expectedRevision int64, actor, reason string) (previousRevisionID string, newBindingRevision int64, err error) {
	rev, err := repo.GetActionRevision(revisionID)
	if err != nil {
		return "", 0, err
	}
	if rev == nil || rev.ActionKey != actionKey {
		return "", 0, ErrRevisionNotFound
	}

	if rev.ActionStreamID == "" {
		if rev.ProcessingTaskID != processingTaskID {
			return "", 0, ErrRevisionNotFound
		}
		legacy, err := repo.GetActiveRevisionBinding(processingTaskID, actionKey)
		if err != nil {
			return "", 0, err
		}
		if legacy == nil {
			if expectedRevision != 0 {
				return "", 0, ErrActiveBindingConflict
			}
			now := nowUTC()
			binding := &ActiveRevisionBinding{
				ProcessingTaskID:       processingTaskID,
				ActionKey:              actionKey,
				RevisionID:             revisionID,
				BindingVersion:         1,
				ActivatedBy:            actor,
				Reason:                 reason,
				CreatedAt:              now,
				UpdatedAt:              now,
				UserID:                 rev.UserID,
				CharacterID:            rev.CharacterID,
				ActiveActionRevisionID: revisionID,
				BindingRevision:        1,
				BoundReason:            reason,
				BoundBy:                actor,
				BoundAt:                now,
			}
			if err := repo.UpsertActiveRevisionBinding(binding); err != nil {
				return "", 0, err
			}
			return "", 1, nil
		}
		if legacy.BindingVersion != expectedRevision {
			return "", 0, ErrActiveBindingConflict
		}
		previousRevisionID = legacy.RevisionID
		ok, err := repo.CASUpdateActiveBinding(processingTaskID, actionKey, expectedRevision, revisionID, actor, reason)
		if err != nil {
			return "", 0, err
		}
		if !ok {
			return "", 0, ErrActiveBindingConflict
		}
		return previousRevisionID, expectedRevision + 1, nil
	}

	routeBinding, err := repo.GetActiveActionRevisionBindingByTask(processingTaskID, actionKey)
	if err != nil {
		return "", 0, err
	}
	if routeBinding == nil || routeBinding.ActionStreamID != rev.ActionStreamID {
		return "", 0, ErrRevisionNotFound
	}

	stream, err := repo.GetActionStreamByID(rev.ActionStreamID)
	if err != nil {
		return "", 0, err
	}
	if stream == nil {
		return "", 0, fmt.Errorf("action stream not found: %s", rev.ActionStreamID)
	}
	userID := rev.UserID
	if userID == "" {
		userID = stream.UserID
	}
	characterID := rev.CharacterID
	if characterID == "" {
		characterID = stream.CharacterID
	}
	if userID == "" || characterID == "" {
		return "", 0, fmt.Errorf("canonical revision missing ownership identity: %s", revisionID)
	}

	db := repo.DB()
	err = db.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC().Format(time.RFC3339)
		var current ActiveActionRevisionBinding
		findErr := tx.Where("action_stream_id = ?", rev.ActionStreamID).First(&current).Error
		switch {
		case errors.Is(findErr, gorm.ErrRecordNotFound):
			if expectedRevision != 0 {
				return ErrActiveBindingConflict
			}
			current = ActiveActionRevisionBinding{
				ID:                     "ab-" + uuid.NewString(),
				ActionStreamID:         rev.ActionStreamID,
				UserID:                 userID,
				CharacterID:            characterID,
				ActionKey:              actionKey,
				ActiveActionRevisionID: revisionID,
				BindingRevision:        1,
				BoundReason:            reason,
				BoundBy:                actor,
				BoundAt:                now,
				CreatedAt:              now,
				UpdatedAt:              now,
			}
			if createErr := tx.Create(&current).Error; createErr != nil {
				return createErr
			}
			newBindingRevision = 1
		case findErr != nil:
			return findErr
		default:
			if current.BindingRevision != expectedRevision {
				return ErrActiveBindingConflict
			}
			previousRevisionID = current.ActiveActionRevisionID
			newBindingRevision = current.BindingRevision + 1
			result := tx.Model(&ActiveActionRevisionBinding{}).
				Where("id = ? AND binding_revision = ?", current.ID, expectedRevision).
				Updates(map[string]any{
					"active_action_revision_id": revisionID,
					"binding_revision":          newBindingRevision,
					"bound_reason":              reason,
					"bound_by":                  actor,
					"bound_at":                  now,
					"updated_at":                now,
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrActiveBindingConflict
			}
		}

		history := &ActionRevisionBindingHistory{
			ID:                 "bh-" + uuid.NewString(),
			ActionStreamID:     rev.ActionStreamID,
			BindingRevision:    newBindingRevision,
			PreviousRevisionID: previousRevisionID,
			NewRevisionID:      revisionID,
			Reason:             reason,
			Actor:              actor,
			OccurredAt:         now,
			CorrelationID:      "editor-bind-" + uuid.NewString(),
		}
		if createErr := tx.Create(history).Error; createErr != nil {
			return createErr
		}

		// Compatibility mirror. This is not an independent authority: its
		// version is forced to the canonical binding revision.
		var legacy ActiveRevisionBinding
		legacyErr := tx.Where("processing_task_id = ? AND action_key = ?", processingTaskID, actionKey).First(&legacy).Error
		if legacyErr != nil && !errors.Is(legacyErr, gorm.ErrRecordNotFound) {
			return legacyErr
		}
		if errors.Is(legacyErr, gorm.ErrRecordNotFound) {
			legacy = ActiveRevisionBinding{
				ProcessingTaskID: processingTaskID,
				ActionKey:        actionKey,
				CreatedAt:        now,
			}
		}
		legacy.RevisionID = revisionID
		legacy.BindingVersion = newBindingRevision
		legacy.ActivatedBy = actor
		legacy.Reason = reason
		legacy.UpdatedAt = now
		legacy.UserID = userID
		legacy.CharacterID = characterID
		legacy.ActiveActionRevisionID = revisionID
		legacy.BindingRevision = newBindingRevision
		legacy.BoundReason = reason
		legacy.BoundBy = actor
		legacy.BoundAt = now
		if errors.Is(legacyErr, gorm.ErrRecordNotFound) {
			if createErr := tx.Create(&legacy).Error; createErr != nil {
				return createErr
			}
		} else if saveErr := tx.Save(&legacy).Error; saveErr != nil {
			return saveErr
		}

		// Every canonical activation is published through the same durable
		// ActionRevision outbox used by baseline commits. This keeps quality
		// evaluation/invalidation attached to the binding authority rather than
		// to individual editor endpoints.
		payload, marshalErr := json.Marshal(map[string]any{
			"actionRevisionId":   revisionID,
			"actionStreamId":     rev.ActionStreamID,
			"bindingRevision":    newBindingRevision,
			"previousRevisionId": previousRevisionID,
			"actionKey":          actionKey,
			"userId":             userID,
			"characterId":        characterID,
			"occurredAt":         now,
		})
		if marshalErr != nil {
			return marshalErr
		}
		event := &ActionRevisionEventOutboxRecord{
			ID:                   "eo-" + uuid.NewString(),
			EventID:              "evt-" + uuid.NewString(),
			EventType:            "desktop_pet.action_revision.activated",
			AggregateType:        "action_revision",
			AggregateID:          revisionID,
			AggregateSequence:    newBindingRevision,
			ActionStreamID:       rev.ActionStreamID,
			ActionRevisionID:     revisionID,
			PreviousRevisionID:   previousRevisionID,
			ProcessingRevisionID: rev.SourceProcessingRevisionID,
			PayloadJSON:          string(payload),
			Status:               "pending",
			AvailableAt:          now,
			CreatedAt:            now,
		}
		return tx.Create(event).Error
	})
	if err != nil {
		return "", 0, err
	}
	return previousRevisionID, newBindingRevision, nil
}
