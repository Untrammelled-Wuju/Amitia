package feedback

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

var ErrRequestConflict = errors.New("反馈请求编号不能用于不同内容")

type Scope struct {
	SpaceID string
	OwnerID string
	CoreID  string
	RoleID  string
}

type ScopedRepository struct {
	db *gorm.DB
}

func NewScopedRepository(db *gorm.DB) *ScopedRepository {
	return &ScopedRepository{db: db}
}

func (r *ScopedRepository) WithDB(db *gorm.DB) *ScopedRepository {
	return &ScopedRepository{db: db}
}

func (s Scope) Validate() error {
	if strings.TrimSpace(s.SpaceID) == "" || strings.TrimSpace(s.OwnerID) == "" || strings.TrimSpace(s.CoreID) == "" || strings.TrimSpace(s.RoleID) == "" {
		return errors.New("反馈数据归属无效")
	}
	return nil
}

func (r *ScopedRepository) query(ctx context.Context, scope Scope) *gorm.DB {
	return r.db.WithContext(ctx).Model(&MessageFeedback{}).Where("space_id=? AND owner_id=? AND core_id=? AND role_id=? AND deleted_at IS NULL", scope.SpaceID, scope.OwnerID, scope.CoreID, scope.RoleID)
}

func (r *ScopedRepository) Create(ctx context.Context, scope Scope, item MessageFeedback) (*MessageFeedback, bool, error) {
	if err := scope.Validate(); err != nil {
		return nil, false, err
	}
	if item.MessageID == "" || item.ConversationID == "" || item.RequestID == "" || !ValidFeedbackTypes[item.FeedbackType] || len(item.Reason) > 8192 || strings.ContainsRune(item.Reason, 0) {
		return nil, false, errors.New("反馈参数无效")
	}
	item.SpaceID, item.OwnerID, item.CoreID, item.RoleID = scope.SpaceID, scope.OwnerID, scope.CoreID, scope.RoleID
	item.Revision = 1
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if item.CreatedAt == "" {
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	var existing MessageFeedback
	err := r.query(ctx, scope).Where("request_id=?", item.RequestID).Take(&existing).Error
	if err == nil {
		if existing.MessageID != item.MessageID || existing.FeedbackType != item.FeedbackType || existing.Reason != item.Reason {
			return nil, false, ErrRequestConflict
		}
		return &existing, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}
	if err := r.db.WithContext(ctx).Create(&item).Error; err != nil {
		if replayErr := r.query(ctx, scope).Where("request_id=?", item.RequestID).Take(&existing).Error; replayErr == nil {
			if existing.MessageID != item.MessageID || existing.FeedbackType != item.FeedbackType || existing.Reason != item.Reason {
				return nil, false, ErrRequestConflict
			}
			return &existing, false, nil
		}
		return nil, false, fmt.Errorf("创建反馈失败: %w", err)
	}
	return &item, true, nil
}

func (r *ScopedRepository) ListMessage(ctx context.Context, scope Scope, messageID string) ([]MessageFeedback, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(messageID) == "" {
		return nil, errors.New("消息编号不能为空")
	}
	items := []MessageFeedback{}
	err := r.query(ctx, scope).Where("message_id=?", messageID).Order("created_at DESC,id DESC").Find(&items).Error
	return items, err
}

func (r *ScopedRepository) Stats(ctx context.Context, scope Scope, messageIDs []string) (map[string]any, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	items := []MessageFeedback{}
	if len(messageIDs) == 0 {
		return map[string]any{"total": 0, "byType": map[string]int64{}, "recent": items}, nil
	}
	if err := r.query(ctx, scope).Where("message_id IN ?", messageIDs).Order("created_at DESC,id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	byType := map[string]int64{}
	for _, item := range items {
		byType[item.FeedbackType]++
	}
	recent := append([]MessageFeedback(nil), items...)
	sort.SliceStable(recent, func(i, j int) bool {
		if recent[i].CreatedAt == recent[j].CreatedAt {
			return recent[i].ID > recent[j].ID
		}
		return recent[i].CreatedAt > recent[j].CreatedAt
	})
	if len(recent) > 10 {
		recent = recent[:10]
	}
	if recent == nil {
		recent = []MessageFeedback{}
	}
	return map[string]any{"total": len(items), "byType": byType, "recent": recent}, nil
}

func (r *ScopedRepository) Delete(ctx context.Context, scope Scope, id int) (*MessageFeedback, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if id < 1 {
		return nil, errors.New("反馈编号无效")
	}
	var item MessageFeedback
	if err := r.query(ctx, scope).Where("id=?", id).Take(&item).Error; err != nil {
		return nil, errors.New("反馈不存在或不属于当前角色")
	}
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&MessageFeedback{}).
		Where("id=? AND revision=? AND deleted_at IS NULL", item.ID, item.Revision).
		Updates(map[string]any{"deleted_at": now, "updated_at": now.Format(time.RFC3339Nano), "revision": item.Revision + 1})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected != 1 {
		return nil, errors.New("反馈数据版本已变化，请重新加载")
	}
	item.Revision++
	item.UpdatedAt = now.Format(time.RFC3339Nano)
	item.DeletedAt = gorm.DeletedAt{Time: now, Valid: true}
	return &item, nil
}
