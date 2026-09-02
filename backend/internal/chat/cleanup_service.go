package chat

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	memorymodel "github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/internal/pipelinecheckpoint"
	"gorm.io/gorm"
)

// CleanupRequest describes a destructive chat-cleanup selection. All filters
// are optional; an empty request previews every active conversation, but the
// confirm endpoint still requires the preview id and an explicit confirmation
// phrase.
type CleanupRequest struct {
	BeforeDate      string   `json:"beforeDate"`
	OlderThanDays   int      `json:"olderThanDays"`
	Channels        []string `json:"channels"`
	Sources         []string `json:"sources"`
	IncludeMemories bool     `json:"includeMemories"`
}

type CleanupPreview struct {
	PreviewID         string   `json:"previewId"`
	ConversationCount int      `json:"conversationCount"`
	MessageCount      int64    `json:"messageCount"`
	MemoryCount       int64    `json:"memoryCount"`
	ConversationIDs   []string `json:"conversationIds"`
	ExpiresAt         string   `json:"expiresAt"`
}

type CleanupResult struct {
	ConversationCount int   `json:"conversationCount"`
	MessageCount      int64 `json:"messageCount"`
	MemoryCount       int64 `json:"memoryCount"`
}

type cleanupPlan struct {
	UserID          string
	ConversationIDs []string
	MessageCount    int64
	MemoryCount     int64
	IncludeMemories bool
	ExpiresAt       time.Time
}

func (s *service) PreviewCleanup(req CleanupRequest, userID string) (*CleanupPreview, error) {
	query := s.db.Table("conversations").Where("deleted_at IS NULL")
	threshold := strings.TrimSpace(req.BeforeDate)
	if req.OlderThanDays > 0 {
		candidate := time.Now().Add(-time.Duration(req.OlderThanDays) * 24 * time.Hour).Format("2006-01-02 15:04:05")
		if threshold == "" || candidate < threshold {
			threshold = candidate
		}
	}
	if threshold != "" {
		if len(threshold) == len("2006-01-02") {
			threshold += " 00:00:00"
		}
		query = query.Where("created_at < ?", threshold)
	}
	if channels := compactStrings(req.Channels); len(channels) > 0 {
		query = query.Where("channel IN ?", channels)
	}
	if sources := compactStrings(req.Sources); len(sources) > 0 {
		query = query.Where("source IN ?", sources)
	}

	var rows []struct{ ID string }
	if err := query.Select("id").Order("created_at ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		if id := strings.TrimSpace(row.ID); id != "" {
			ids = append(ids, id)
		}
	}
	var messageCount int64
	var memoryCount int64
	if len(ids) > 0 {
		if err := s.db.Table("messages").Where("deleted_at IS NULL AND conversation_id IN ?", ids).Count(&messageCount).Error; err != nil {
			return nil, err
		}
		if req.IncludeMemories {
			if err := s.db.Model(&memorymodel.Memory{}).Where("source_conv_id IN ?", ids).Count(&memoryCount).Error; err != nil {
				return nil, err
			}
		}
	}

	expiresAt := time.Now().Add(10 * time.Minute)
	previewID := uuid.NewString()
	s.cleanupMu.Lock()
	if s.cleanupPlans == nil {
		s.cleanupPlans = make(map[string]cleanupPlan)
	}
	for id, plan := range s.cleanupPlans {
		if time.Now().After(plan.ExpiresAt) {
			delete(s.cleanupPlans, id)
		}
	}
	s.cleanupPlans[previewID] = cleanupPlan{
		UserID: userID, ConversationIDs: append([]string(nil), ids...), MessageCount: messageCount,
		MemoryCount: memoryCount, IncludeMemories: req.IncludeMemories, ExpiresAt: expiresAt,
	}
	s.cleanupMu.Unlock()

	return &CleanupPreview{
		PreviewID: previewID, ConversationCount: len(ids), MessageCount: messageCount,
		MemoryCount: memoryCount, ConversationIDs: ids, ExpiresAt: expiresAt.Format(time.RFC3339),
	}, nil
}

func (s *service) ConfirmCleanup(previewID, confirmText, userID string) (*CleanupResult, error) {
	if strings.TrimSpace(confirmText) != "确认清理" {
		return nil, fmt.Errorf("确认文本不匹配")
	}
	previewID = strings.TrimSpace(previewID)
	if previewID == "" {
		return nil, fmt.Errorf("previewId 不能为空")
	}
	s.cleanupMu.Lock()
	plan, ok := s.cleanupPlans[previewID]
	if ok {
		delete(s.cleanupPlans, previewID) // one-shot token even if execution fails
	}
	s.cleanupMu.Unlock()
	if !ok || time.Now().After(plan.ExpiresAt) {
		return nil, fmt.Errorf("清理预览不存在或已过期，请重新预览")
	}
	if strings.TrimSpace(plan.UserID) != strings.TrimSpace(userID) {
		return nil, fmt.Errorf("清理预览不属于当前用户")
	}

	deletedMemories := int64(0)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for _, id := range plan.ConversationIDs {
			if _, err := s.tombstoneConversationTx(tx, id, userID); err != nil {
				if err == gorm.ErrRecordNotFound {
					continue
				}
				return err
			}
		}
		if plan.IncludeMemories && len(plan.ConversationIDs) > 0 {
			result := tx.Where("source_conv_id IN ?", plan.ConversationIDs).Delete(&memorymodel.Memory{})
			if result.Error != nil {
				return result.Error
			}
			deletedMemories = result.RowsAffected
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, id := range plan.ConversationIDs {
		_ = pipelinecheckpoint.New(s.db).ResetConversation(id)
	}
	return &CleanupResult{ConversationCount: len(plan.ConversationIDs), MessageCount: plan.MessageCount, MemoryCount: deletedMemories}, nil
}

func (s *service) VacuumCleanup() error {
	return s.db.Exec("VACUUM").Error
}

func compactStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
