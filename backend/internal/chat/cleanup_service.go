package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/business"
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
	SpaceID         string
	ConversationIDs []string
	MessageCount    int64
	MemoryCount     int64
	IncludeMemories bool
	Memories        []business.MemoryConsumerEntry
	ExpiresAt       time.Time
}

func (s *service) PreviewCleanup(req CleanupRequest, spaceID string, contexts ...context.Context) (*CleanupPreview, error) {
	query := s.db.Table("conversations").Where("deleted_at IS NULL")
	query = applyConversationOwnerScope(query, spaceID)
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
	var memoryEntries []business.MemoryConsumerEntry
	ctx := context.Background()
	if len(contexts) > 0 {
		ctx = contexts[0]
	}
	if len(ids) > 0 {
		if err := s.db.Table("messages").Where("deleted_at IS NULL AND conversation_id IN ?", ids).Count(&messageCount).Error; err != nil {
			return nil, err
		}
		if req.IncludeMemories {
			if s.memoryConsumers == nil {
				return nil, fmt.Errorf("统一记忆服务尚未就绪")
			}
			entries, err := s.memoryConsumers.Read(ctx, "", "memory", true)
			if err != nil {
				return nil, err
			}
			for _, entry := range entries {
				var document map[string]json.RawMessage
				if err := json.Unmarshal(entry.Body, &document); err != nil {
					return nil, err
				}
				conversation := business.MemoryText(document, "conversationId")
				for _, id := range ids {
					if conversation == id {
						memoryEntries = append(memoryEntries, entry)
						break
					}
				}
			}
			memoryCount = int64(len(memoryEntries))
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
		SpaceID: spaceID, ConversationIDs: append([]string(nil), ids...), MessageCount: messageCount,
		Memories: memoryEntries, MemoryCount: memoryCount, IncludeMemories: req.IncludeMemories, ExpiresAt: expiresAt,
	}
	s.cleanupMu.Unlock()

	return &CleanupPreview{
		PreviewID: previewID, ConversationCount: len(ids), MessageCount: messageCount,
		MemoryCount: memoryCount, ConversationIDs: ids, ExpiresAt: expiresAt.Format(time.RFC3339),
	}, nil
}

func (s *service) ConfirmCleanup(previewID, confirmText, spaceID string, contexts ...context.Context) (*CleanupResult, error) {
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
	if strings.TrimSpace(plan.SpaceID) != strings.TrimSpace(spaceID) {
		return nil, fmt.Errorf("清理预览不属于当前用户")
	}

	deletedMemories := int64(0)
	if len(plan.Memories) > 0 {
		port, ok := s.memoryConsumers.(business.MemoryDeletionPort)
		if !ok {
			return nil, fmt.Errorf("统一记忆删除入口不可用")
		}
		ctx := context.Background()
		if len(contexts) > 0 {
			ctx = contexts[0]
		}
		var err error
		deletedMemories, err = port.DeleteEntries(ctx, plan.Memories, "chat-cleanup/"+previewID)
		if err != nil {
			return nil, fmt.Errorf("记忆清理失败（已清理 %d 条），请重新预览：%w", deletedMemories, err)
		}
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for _, id := range plan.ConversationIDs {
			if _, err := s.tombstoneConversationTx(tx, id, spaceID); err != nil {
				if err == gorm.ErrRecordNotFound {
					continue
				}
				return err
			}
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

func (s *service) SetMemoryConsumers(port business.MemoryConsumerPort) { s.memoryConsumers = port }
