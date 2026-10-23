package importbatch

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

var (
	ErrNotFound         = errors.New("chat import batch not found")
	ErrRoleMismatch     = errors.New("chat import batch role mismatch")
	ErrAlreadyPromoted  = errors.New("chat import batch already promoted")
	ErrDeleteNotAllowed = errors.New("promoted chat import batch cannot be deleted")
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateOrGet(ctx context.Context, batch *Batch, items []Item) (*Batch, []Item, bool, error) {
	if r == nil || r.db == nil {
		return nil, nil, false, errors.New("chat import repository unavailable")
	}
	var existing Batch
	err := r.db.WithContext(ctx).
		Where("space_id = ? AND device_id = ? AND idempotency_key = ? AND idempotency_key <> ''", batch.SpaceID, batch.DeviceID, batch.IdempotencyKey).
		First(&existing).Error
	if err == nil {
		existingItems, loadErr := r.items(ctx, existing.ID)
		return &existing, existingItems, true, loadErr
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, false, err
	}
	batch.MessageCount = len(items)
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(batch).Error; err != nil {
			return err
		}
		if len(items) > 0 {
			if err := tx.CreateInBatches(items, 128).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		var raced Batch
		lookupErr := r.db.WithContext(ctx).
			Where("space_id = ? AND device_id = ? AND idempotency_key = ? AND idempotency_key <> ''", batch.SpaceID, batch.DeviceID, batch.IdempotencyKey).
			First(&raced).Error
		if lookupErr == nil {
			racedItems, itemsErr := r.items(ctx, raced.ID)
			return &raced, racedItems, true, itemsErr
		}
		return nil, nil, false, err
	}
	return batch, items, false, nil
}

func (r *Repository) Get(ctx context.Context, spaceID, batchID string) (*Batch, []Item, error) {
	if r == nil || r.db == nil {
		return nil, nil, errors.New("chat import repository unavailable")
	}
	var batch Batch
	err := r.db.WithContext(ctx).
		Where("space_id = ? AND id = ?", strings.TrimSpace(spaceID), strings.TrimSpace(batchID)).
		First(&batch).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	items, err := r.items(ctx, batch.ID)
	if err != nil {
		return nil, nil, err
	}
	return &batch, items, nil
}

func (r *Repository) List(ctx context.Context, spaceID string, limit int) ([]Batch, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("chat import repository unavailable")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var batches []Batch
	if err := r.db.WithContext(ctx).
		Where("space_id = ?", strings.TrimSpace(spaceID)).
		Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&batches).Error; err != nil {
		return nil, err
	}
	if batches == nil {
		batches = []Batch{}
	}
	return batches, nil
}

func (r *Repository) DeleteReady(ctx context.Context, spaceID, batchID string) error {
	if r == nil || r.db == nil {
		return errors.New("chat import repository unavailable")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var batch Batch
		err := tx.Where("space_id = ? AND id = ?", strings.TrimSpace(spaceID), strings.TrimSpace(batchID)).First(&batch).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if batch.Status == StatusPromoted {
			return ErrDeleteNotAllowed
		}
		if err := tx.Where("batch_id = ?", batch.ID).Delete(&Item{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", batch.ID).Delete(&Batch{}).Error
	})
}

func (r *Repository) SetSummary(ctx context.Context, spaceID, batchID, summary string) error {
	return r.db.WithContext(ctx).Model(&Batch{}).
		Where("space_id = ? AND id = ?", strings.TrimSpace(spaceID), strings.TrimSpace(batchID)).
		Updates(map[string]any{"summary_text": strings.TrimSpace(summary), "updated_at": now()}).Error
}

func (r *Repository) MarkPromoted(ctx context.Context, spaceID, batchID, roleID, conversationID, ownerID, coreID string) error {
	if r == nil || r.db == nil {
		return errors.New("chat import repository unavailable")
	}
	if strings.TrimSpace(conversationID) == "" {
		return errors.New("promoted conversation id is required")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var batch Batch
		err := tx.Where("space_id = ? AND id = ?", strings.TrimSpace(spaceID), strings.TrimSpace(batchID)).First(&batch).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if batch.RoleID != "" && batch.RoleID != roleID {
			return ErrRoleMismatch
		}
		if batch.Status == StatusPromoted {
			if batch.PromotedConversationID == conversationID {
				return nil
			}
			return ErrAlreadyPromoted
		}
		result := tx.Model(&Batch{}).
			Where("space_id = ? AND id = ? AND status IN ?", strings.TrimSpace(spaceID), strings.TrimSpace(batchID), []string{StatusReady, StatusFailed}).
			Updates(map[string]any{
				"role_id":                        roleID,
				"status":                         StatusPromoted,
				"promoted_conversation_id":       conversationID,
				"promoted_conversation_owner_id": ownerID,
				"promoted_core_id":               coreID,
				"error_code":                     "",
				"error_message":                  "",
				"promoted_at":                    now(),
				"updated_at":                     now(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("chat import promotion state conflict")
		}
		return nil
	})
}

func (r *Repository) MarkFailed(ctx context.Context, spaceID, batchID, code, message string) error {
	return r.db.WithContext(ctx).Model(&Batch{}).
		Where("space_id = ? AND id = ? AND status <> ?", strings.TrimSpace(spaceID), strings.TrimSpace(batchID), StatusPromoted).
		Updates(map[string]any{
			"status":        StatusFailed,
			"error_code":    strings.TrimSpace(code),
			"error_message": strings.TrimSpace(message),
			"updated_at":    now(),
		}).Error
}

func (r *Repository) items(ctx context.Context, batchID string) ([]Item, error) {
	var items []Item
	if err := r.db.WithContext(ctx).
		Where("batch_id = ?", batchID).
		Order("sequence ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, err
	}
	if items == nil {
		items = []Item{}
	}
	return items, nil
}

func PromotionArchive(batch *Batch, items []Item, sourceOwnerID string) map[string]any {
	messages := make([]map[string]any, 0, len(items))
	for _, item := range items {
		createdAt := item.Timestamp
		if strings.TrimSpace(createdAt) == "" {
			createdAt = item.CreatedAt
		}
		messages = append(messages, map[string]any{
			"id":             item.ID,
			"conversationId": batch.ID,
			"characterId":    batch.RoleID,
			"sequence":       item.Sequence,
			"role":           item.Role,
			"content":        item.Content,
			"createdAt":      createdAt,
			"source":         "import",
		})
	}
	return map[string]any{
		"importBatchId": batch.ID,
		"sourceOwnerId": sourceOwnerID,
		"sourceRoleId":  batch.RoleID,
		"conversation": map[string]any{
			"id":        batch.ID,
			"title":     batch.Title,
			"source":    "import",
			"channel":   "web",
			"createdAt": batch.CreatedAt,
			"updatedAt": batch.UpdatedAt,
			"roleId":    batch.RoleID,
		},
		"messages": messages,
	}
}

func now() string {
	return timeNow().Format("2006-01-02 15:04:05")
}

var timeNow = func() time.Time {
	return time.Now()
}
