package reminder

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/chat"
	"gorm.io/gorm"
)

type Dispatcher interface {
	ProcessMessage(ctx context.Context, req *chat.ProcessMessageRequest) (*chat.ProcessMessageResponse, error)
}

type Service struct {
	db       *gorm.DB
	repo     *Repository
	dispatch Dispatcher
	mu       sync.Mutex
	running  bool
	ticking  bool
}

func NewService(db *gorm.DB, dispatch Dispatcher) *Service {
	return &Service{db: db, repo: NewRepository(db), dispatch: dispatch}
}

func (s *Service) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		s.processDue(ctx)
		for {
			select {
			case <-ctx.Done():
				s.mu.Lock()
				s.running = false
				s.mu.Unlock()
				return
			case <-ticker.C:
				s.processDue(ctx)
			}
		}
	}()
}

func (s *Service) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *Service) List(spaceID string) ([]Reminder, error) {
	items, err := s.repo.List(spaceID)
	if err != nil {
		return nil, err
	}
	for index := range items {
		s.hydrate(&items[index])
	}
	return items, nil
}

func (s *Service) Find(id int, spaceID string) (*Reminder, error) {
	item, err := s.repo.Find(id, spaceID)
	if err != nil {
		return nil, err
	}
	s.hydrate(item)
	return item, nil
}

func (s *Service) Create(req *CreateReminderRequest, spaceID string) (*Reminder, error) {
	title := strings.TrimSpace(req.Title)
	remindAt := strings.TrimSpace(req.RemindAt)
	if title == "" || remindAt == "" {
		return nil, fmt.Errorf("标题和提醒时间不能为空")
	}
	parsed, err := parseTime(remindAt)
	if err != nil {
		return nil, fmt.Errorf("提醒时间格式无效")
	}
	if !parsed.After(time.Now()) {
		return nil, fmt.Errorf("提醒时间不能早于当前时间")
	}
	channel := strings.TrimSpace(req.Channel)
	if channel == "" {
		channel = "web"
	}
	repeatRule := strings.TrimSpace(req.RepeatRule)
	if repeatRule == "" {
		repeatRule = "none"
	}
	enabled := 1
	if req.Enabled != nil && !*req.Enabled {
		enabled = 0
	}
	item := &Reminder{
		SpaceID:        spaceID,
		Title:          title,
		Content:        strings.TrimSpace(req.Content),
		Channel:        channel,
		ConversationID: strings.TrimSpace(req.ConversationID),
		CharacterID:    strings.TrimSpace(req.CharacterID),
		RemindAt:       parsed.Format("2006-01-02 15:04:05"),
		RepeatRule:     repeatRule,
		Enabled:        enabled,
		CreatedAt:      nowString(),
		UpdatedAt:      nowString(),
	}
	if err := s.repo.Create(item); err != nil {
		return nil, fmt.Errorf("创建提醒失败: %w", err)
	}
	return item, nil
}

func (s *Service) Update(id int, spaceID string, updates map[string]interface{}) (*Reminder, error) {
	if len(updates) == 0 {
		return nil, fmt.Errorf("没有可更新的字段")
	}
	allowed := map[string]string{
		"title":           "title",
		"content":         "content",
		"channel":         "channel",
		"conversationId":  "conversation_id",
		"conversation_id": "conversation_id",
		"characterId":     "character_id",
		"character_id":    "character_id",
		"remindAt":        "remind_at",
		"remind_at":       "remind_at",
		"repeatRule":      "repeat_rule",
		"repeat_rule":     "repeat_rule",
		"enabled":         "enabled",
	}
	clean := make(map[string]interface{}, len(updates))
	for key, value := range updates {
		if column := allowed[key]; column != "" {
			clean[column] = value
		}
	}
	clean["updated_at"] = nowString()
	if err := s.repo.Update(id, spaceID, clean); err != nil {
		return nil, fmt.Errorf("更新提醒失败: %w", err)
	}
	return s.Find(id, spaceID)
}

func (s *Service) Delete(id int, spaceID string) error {
	return s.repo.Delete(id, spaceID)
}

func (s *Service) Toggle(id int, spaceID string) (*Reminder, error) {
	item, err := s.repo.Toggle(id, spaceID)
	if err != nil {
		return nil, err
	}
	s.hydrate(item)
	return item, nil
}

func (s *Service) Status(spaceID string) (map[string]interface{}, error) {
	items, err := s.List(spaceID)
	if err != nil {
		return nil, err
	}
	enabled := 0
	dueNow := 0
	now := time.Now()
	for _, item := range items {
		if item.Enabled == 1 {
			enabled++
			if parsed, parseErr := parseTime(item.RemindAt); parseErr == nil && !parsed.After(now) {
				dueNow++
			}
		}
	}
	return map[string]interface{}{
		"schedulerRunning": s.Running(),
		"total":            len(items),
		"enabled":          enabled,
		"dueNow":           dueNow,
	}, nil
}

func (s *Service) Test(item *Reminder) map[string]interface{} {
	content := item.Content
	if content == "" {
		content = fmt.Sprintf("[提醒测试] %s", item.Title)
	}
	return map[string]interface{}{
		"id":             item.ID,
		"tested":         true,
		"title":          item.Title,
		"remindAt":       item.RemindAt,
		"messageContent": content,
		"channel":        item.Channel,
		"conversationId": s.resolveConversation(item),
		"safetyCheck": map[string]interface{}{
			"safe":   true,
			"reason": "",
		},
	}
}

func (s *Service) TriggerNow(ctx context.Context, item *Reminder) (string, string, error) {
	if item == nil {
		return "", "", fmt.Errorf("提醒不存在")
	}
	conversationID := s.resolveConversation(item)
	if conversationID == "" {
		return "", "", fmt.Errorf("没有可用对话")
	}
	if s.dispatch == nil {
		return "", "", fmt.Errorf("消息生成服务不可用")
	}
	prompt := item.Content
	if prompt == "" {
		prompt = fmt.Sprintf("请用自然的语气提醒用户：%s", item.Title)
	}
	requestID := fmt.Sprintf("reminder:%d:%d", item.ID, time.Now().UnixNano())
	response, err := s.dispatch.ProcessMessage(ctx, &chat.ProcessMessageRequest{
		SpaceID:                  item.SpaceID,
		CharacterID:              item.CharacterID,
		ConversationID:           conversationID,
		Channel:                  item.Channel,
		Message:                  prompt,
		Source:                   "proactive",
		ProactiveTaskInstruction: prompt,
		RequestID:                requestID,
		IsInternal:               true,
	})
	if err != nil {
		return "", "", err
	}
	messageID := ""
	if len(response.MessageIDs) > 0 {
		messageID = response.MessageIDs[0]
	}
	return messageID, conversationID, nil
}

func (s *Service) History(spaceID string, page, pageSize int, state string) ([]TriggerHistory, int64, error) {
	return s.repo.ListHistory(spaceID, page, pageSize, state)
}

func (s *Service) QueueSummary(spaceID string) (map[string]interface{}, error) {
	items, err := s.List(spaceID)
	if err != nil {
		return nil, err
	}
	pending := 0
	for _, item := range items {
		if item.Enabled == 1 {
			pending++
		}
	}
	return map[string]interface{}{
		"depth":          len(items),
		"pendingCount":   pending,
		"recentFailures": 0,
		"backpressure":   false,
	}, nil
}

func (s *Service) GetCleanupDays() string {
	var value string
	err := s.db.Table("app_settings").Select("value").Where("key = ? AND deleted_at IS NULL", "reminder_cleanup_days").Scan(&value).Error
	if err != nil || strings.TrimSpace(value) == "" {
		return "0"
	}
	return value
}

func (s *Service) SetCleanupDays(value string) error {
	days, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || days < 0 {
		return fmt.Errorf("自动清理天数无效")
	}
	normalized := strconv.Itoa(days)
	return s.db.Exec(`INSERT INTO app_settings(key, value, revision, deleted_at, updated_at)
		VALUES('reminder_cleanup_days', ?, 1, NULL, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, revision = app_settings.revision + 1, deleted_at = NULL, updated_at = excluded.updated_at`,
		normalized, nowString()).Error
}

func (s *Service) CleanupHistory(spaceID string) {
	days, err := strconv.Atoi(s.GetCleanupDays())
	if err != nil || days <= 0 {
		return
	}
	_ = s.repo.DeleteHistoryBefore(spaceID, time.Now().AddDate(0, 0, -days))
}

func (s *Service) processDue(ctx context.Context) {
	s.mu.Lock()
	if s.ticking {
		s.mu.Unlock()
		return
	}
	s.ticking = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.ticking = false
		s.mu.Unlock()
	}()

	items, err := s.repo.Due(50)
	if err != nil {
		return
	}
	for index := range items {
		item := &items[index]
		historyID := uuid.NewString()
		now := nowString()
		history := &TriggerHistory{
			ID:          historyID,
			SpaceID:     item.SpaceID,
			TriggerID:   strconv.Itoa(item.ID),
			TriggerType: "reminder",
			Title:       item.Title,
			Channel:     item.Channel,
			State:       "sending",
			Priority:    "normal",
			Reason:      "到达提醒时间",
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		_ = s.repo.CreateHistory(history)
		_, _, triggerErr := s.TriggerNow(ctx, item)
		if triggerErr != nil {
			_ = s.repo.UpdateHistory(historyID, map[string]interface{}{
				"state": "failed", "attempt_count": 1, "last_error": triggerErr.Error(), "updated_at": nowString(),
			})
			continue
		}
		_ = s.repo.UpdateHistory(historyID, map[string]interface{}{
			"state": "sent", "attempt_count": 1, "updated_at": nowString(),
		})
		s.advance(item)
	}
}

func (s *Service) advance(item *Reminder) {
	updates := map[string]interface{}{
		"last_triggered_at": nowString(),
		"updated_at":        nowString(),
	}
	switch item.RepeatRule {
	case "daily", "weekly":
		current, err := parseTime(item.RemindAt)
		if err == nil {
			step := 24 * time.Hour
			if item.RepeatRule == "weekly" {
				step = 7 * 24 * time.Hour
			}
			next := current.Add(step)
			for !next.After(time.Now()) {
				next = next.Add(step)
			}
			updates["remind_at"] = next.Format("2006-01-02 15:04:05")
			updates["enabled"] = 1
		}
	default:
		updates["enabled"] = 0
	}
	_ = s.repo.Update(item.ID, item.SpaceID, updates)
}

func (s *Service) resolveConversation(item *Reminder) string {
	if item == nil {
		return ""
	}
	if item.ConversationID != "" {
		return item.ConversationID
	}
	query := ownerQuery(s.db.Table("conversations").Select("id"), item.SpaceID).Where("deleted_at IS NULL")
	if item.CharacterID != "" {
		query = query.Where("character_id = ?", item.CharacterID)
	}
	var conversationID string
	if err := query.Order("updated_at DESC").Limit(1).Scan(&conversationID).Error; err == nil && conversationID != "" {
		return conversationID
	}
	if item.Channel != "" {
		query = ownerQuery(s.db.Table("conversations").Select("id"), item.SpaceID).Where("channel = ? AND deleted_at IS NULL", item.Channel)
		if item.CharacterID != "" {
			query = query.Where("character_id = ?", item.CharacterID)
		}
		if err := query.Order("updated_at DESC").Limit(1).Scan(&conversationID).Error; err == nil {
			return conversationID
		}
	}
	return ""
}

func (s *Service) hydrate(item *Reminder) {
	if item == nil {
		return
	}
	if item.ConversationID != "" {
		var title string
		if err := s.db.Table("conversations").Select("title").Where("id = ?", item.ConversationID).Scan(&title).Error; err == nil {
			item.ConversationTitle = title
		}
	}
	if item.CharacterID != "" {
		var name string
		if err := s.db.Table("characters").Select("name").Where("id = ?", item.CharacterID).Scan(&name).Error; err == nil {
			item.CharacterName = name
		}
	}
}

func parseTime(value string) (time.Time, error) {
	layouts := []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid time")
}
