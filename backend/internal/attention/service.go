package attention

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func DefaultPolicy(spaceID string) BudgetPolicy {
	now := time.Now().UTC()
	return BudgetPolicy{
		SpaceID:                   spaceID,
		Enabled:                   true,
		ShadowMode:                true,
		MaxInterruptionsPerDay:    24,
		MaxRepeatedConfirmations:  3,
		MaxConsecutiveFollowups:   2,
		MaxUnchangedNotifications: 1,
		DedupeWindowMinutes:       30,
		Revision:                  1,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
}

func normalizePolicy(policy BudgetPolicy) BudgetPolicy {
	defaults := DefaultPolicy(policy.SpaceID)
	if policy.MaxInterruptionsPerDay <= 0 {
		policy.MaxInterruptionsPerDay = defaults.MaxInterruptionsPerDay
	}
	if policy.MaxRepeatedConfirmations <= 0 {
		policy.MaxRepeatedConfirmations = defaults.MaxRepeatedConfirmations
	}
	if policy.MaxConsecutiveFollowups <= 0 {
		policy.MaxConsecutiveFollowups = defaults.MaxConsecutiveFollowups
	}
	if policy.MaxUnchangedNotifications <= 0 {
		policy.MaxUnchangedNotifications = defaults.MaxUnchangedNotifications
	}
	if policy.DedupeWindowMinutes <= 0 {
		policy.DedupeWindowMinutes = defaults.DedupeWindowMinutes
	}
	return policy
}

func (s *Service) GetPolicy(ctx context.Context, spaceID string) (BudgetPolicy, error) {
	if s == nil || s.db == nil {
		return BudgetPolicy{}, errors.New("注意力预算服务不可用")
	}
	var policy BudgetPolicy
	err := s.db.WithContext(ctx).Where("space_id = ?", strings.TrimSpace(spaceID)).First(&policy).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return DefaultPolicy(strings.TrimSpace(spaceID)), nil
	}
	if err != nil {
		return BudgetPolicy{}, fmt.Errorf("读取注意力预算: %w", err)
	}
	return policy, nil
}

func (s *Service) PutPolicy(ctx context.Context, policy BudgetPolicy) (BudgetPolicy, error) {
	if s == nil || s.db == nil {
		return BudgetPolicy{}, errors.New("注意力预算服务不可用")
	}
	policy.SpaceID = strings.TrimSpace(policy.SpaceID)
	if policy.SpaceID == "" {
		return BudgetPolicy{}, errors.New("注意力预算缺少空间标识")
	}
	policy = normalizePolicy(policy)
	now := time.Now().UTC()
	var existing BudgetPolicy
	err := s.db.WithContext(ctx).Where("space_id = ?", policy.SpaceID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		policy.Revision = 1
		policy.CreatedAt = now
		policy.UpdatedAt = now
		if err := s.db.WithContext(ctx).Create(&policy).Error; err != nil {
			return BudgetPolicy{}, fmt.Errorf("创建注意力预算: %w", err)
		}
		return policy, nil
	}
	if err != nil {
		return BudgetPolicy{}, fmt.Errorf("读取注意力预算: %w", err)
	}
	if policy.Revision != 0 && policy.Revision != existing.Revision {
		return BudgetPolicy{}, fmt.Errorf("注意力预算版本冲突: expected %d got %d", existing.Revision, policy.Revision)
	}
	policy.Revision = existing.Revision + 1
	policy.CreatedAt = existing.CreatedAt
	policy.UpdatedAt = now
	result := s.db.WithContext(ctx).Model(&BudgetPolicy{}).Where("space_id = ? AND revision = ?", existing.SpaceID, existing.Revision).Updates(map[string]any{
		"enabled": policy.Enabled, "shadow_mode": policy.ShadowMode,
		"max_interruptions_per_day":   policy.MaxInterruptionsPerDay,
		"max_repeated_confirmations":  policy.MaxRepeatedConfirmations,
		"max_consecutive_followups":   policy.MaxConsecutiveFollowups,
		"max_unchanged_notifications": policy.MaxUnchangedNotifications,
		"dedupe_window_minutes":       policy.DedupeWindowMinutes,
		"revision":                    policy.Revision, "updated_at": now,
	})
	if result.Error != nil {
		return BudgetPolicy{}, fmt.Errorf("更新注意力预算: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return BudgetPolicy{}, errors.New("注意力预算版本冲突")
	}
	return policy, nil
}

func (s *Service) HasEvent(ctx context.Context, spaceID, idempotencyKey string) (bool, error) {
	if s == nil || s.db == nil {
		return false, errors.New("注意力预算服务不可用")
	}
	spaceID = strings.TrimSpace(spaceID)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if spaceID == "" || idempotencyKey == "" {
		return false, errors.New("注意力事件缺少空间或幂等标识")
	}
	var count int64
	if err := s.db.WithContext(ctx).Model(&Event{}).
		Where("space_id = ? AND idempotency_key = ?", spaceID, idempotencyKey).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("查询注意力事件: %w", err)
	}
	return count > 0, nil
}

func (s *Service) Check(ctx context.Context, input CheckInput) (CheckResult, error) {
	if s == nil || s.db == nil {
		return CheckResult{}, errors.New("注意力预算服务不可用")
	}
	policy, err := s.GetPolicy(ctx, input.SpaceID)
	if err != nil {
		return CheckResult{}, err
	}
	input.SourceKind = strings.TrimSpace(input.SourceKind)
	input.SourceID = strings.TrimSpace(input.SourceID)
	input.EventKind = strings.TrimSpace(input.EventKind)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if input.IdempotencyKey == "" {
		input.IdempotencyKey = input.SourceKind + ":" + input.SourceID + ":" + input.EventKind
	}
	if input.PayloadJSON == "" {
		input.PayloadJSON = "{}"
	} else if !json.Valid([]byte(input.PayloadJSON)) {
		return CheckResult{}, errors.New("注意力事件载荷不是有效 JSON")
	}
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var interruptions int64
	var repeated int64
	var followups int64
	var unchanged int64
	if err := s.db.WithContext(ctx).Model(&Event{}).Where("space_id = ? AND created_at >= ? AND delivery_state = ?", input.SpaceID, start, "sent").Count(&interruptions).Error; err != nil {
		return CheckResult{}, fmt.Errorf("统计今日打断次数: %w", err)
	}
	if err := s.db.WithContext(ctx).Model(&Event{}).Where("space_id = ? AND created_at >= ? AND event_kind = ?", input.SpaceID, start, "confirmation").Count(&repeated).Error; err != nil {
		return CheckResult{}, fmt.Errorf("统计重复确认: %w", err)
	}
	if err := s.db.WithContext(ctx).Model(&Event{}).Where("space_id = ? AND created_at >= ? AND event_kind = ?", input.SpaceID, start, "followup").Count(&followups).Error; err != nil {
		return CheckResult{}, fmt.Errorf("统计连续追问: %w", err)
	}
	if err := s.db.WithContext(ctx).Model(&Event{}).Where("space_id = ? AND created_at >= ? AND event_kind = ?", input.SpaceID, start, "unchanged").Count(&unchanged).Error; err != nil {
		return CheckResult{}, fmt.Errorf("统计无变化通知: %w", err)
	}
	reason := ""
	limited := false
	switch {
	case interruptions >= int64(policy.MaxInterruptionsPerDay):
		limited, reason = true, "已达到今日注意力预算"
	case input.EventKind == "confirmation" && repeated >= int64(policy.MaxRepeatedConfirmations):
		limited, reason = true, "已达到重复确认上限"
	case input.EventKind == "followup" && followups >= int64(policy.MaxConsecutiveFollowups):
		limited, reason = true, "已达到连续追问上限"
	case input.EventKind == "unchanged" && unchanged >= int64(policy.MaxUnchangedNotifications):
		limited, reason = true, "无变化通知已发送"
	}
	if policy.Enabled && policy.DedupeWindowMinutes > 0 {
		var duplicate int64
		cutoff := now.Add(-time.Duration(policy.DedupeWindowMinutes) * time.Minute)
		if err := s.db.WithContext(ctx).Model(&Event{}).Where("space_id = ? AND idempotency_key = ? AND created_at >= ? AND delivery_state IN ?", input.SpaceID, input.IdempotencyKey, cutoff, []string{"sent", "observed", "suppressed"}).Count(&duplicate).Error; err != nil {
			return CheckResult{}, fmt.Errorf("检查注意力事件去重: %w", err)
		}
		if duplicate > 0 {
			limited, reason = true, "去重窗口内已处理相同事件"
		}
	}
	allowed := !limited
	shadowed := limited && policy.ShadowMode
	delivery := "sent"
	if limited && !shadowed {
		delivery = "suppressed"
	}
	if shadowed {
		delivery = "observed"
		reason += "；影子模式仅记录"
	}
	if !policy.Enabled {
		allowed, shadowed, delivery, reason = true, false, "sent", "注意力预算未启用"
	} else if shadowed {
		allowed = true
	}
	event := Event{
		ID:             uuid.NewString(),
		SpaceID:        input.SpaceID,
		RoleID:         input.RoleID,
		ConversationID: input.ConversationID,
		SourceKind:     input.SourceKind,
		SourceID:       input.SourceID,
		EventKind:      input.EventKind,
		DeliveryState:  delivery,
		Reason:         reason,
		PayloadJSON:    input.PayloadJSON,
		IdempotencyKey: input.IdempotencyKey,
		CreatedAt:      now,
	}
	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&event).Error; err != nil {
		return CheckResult{}, fmt.Errorf("记录注意力事件: %w", err)
	}
	return CheckResult{Allowed: allowed, Shadowed: shadowed, Reason: reason, Event: event, Policy: policy, Interruptions: int(interruptions)}, nil
}

func (s *Service) Stats(ctx context.Context, spaceID string) (Stats, error) {
	policy, err := s.GetPolicy(ctx, spaceID)
	if err != nil {
		return Stats{}, err
	}
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var stats Stats
	stats.Policy = policy
	base := func(state string, dest *int64, kind string) error {
		query := s.db.WithContext(ctx).Model(&Event{}).Where("space_id = ? AND created_at >= ?", spaceID, start)
		if state != "" {
			query = query.Where("delivery_state = ?", state)
		}
		if kind != "" {
			query = query.Where("event_kind = ?", kind)
		}
		return query.Count(dest).Error
	}
	if err := base("sent", &stats.TodayInterruptions, ""); err != nil {
		return Stats{}, err
	}
	if err := base("suppressed", &stats.TodaySuppressed, ""); err != nil {
		return Stats{}, err
	}
	if err := base("observed", &stats.TodayObserved, ""); err != nil {
		return Stats{}, err
	}
	if err := base("", &stats.RepeatedConfirmations, "confirmation"); err != nil {
		return Stats{}, err
	}
	if err := base("", &stats.ConsecutiveFollowups, "followup"); err != nil {
		return Stats{}, err
	}
	if err := base("", &stats.UnchangedNotifications, "unchanged"); err != nil {
		return Stats{}, err
	}
	return stats, nil
}
