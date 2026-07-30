package wiring

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/behavior"
	"github.com/u-ai/backend/log"

	"gorm.io/gorm"
)

type AmitiaStateSourceQuery struct {
	db *gorm.DB
}

func NewAmitiaStateSourceQuery(db *gorm.DB) *AmitiaStateSourceQuery {
	return &AmitiaStateSourceQuery{db: db}
}

var interactionActiveStatuses = []string{
	"received", "normalized", "queued", "processing",
	"context_ready", "decided", "generated", "committed",
	"delivery_pending", "delivered",
}

var voiceActiveStatuses = []string{
	"received", "normalized", "queued", "processing",
	"context_ready", "decided", "generated", "committed",
}

var toolActiveStatuses = []string{"PENDING", "RUNNING"}

func (q *AmitiaStateSourceQuery) QueryActiveInteractions(ctx context.Context, userID, characterID string) ([]behavior.InteractionSnapshot, error) {
	if q.db == nil {
		return nil, nil
	}
	type interactionRow struct {
		ID             string `gorm:"column:id"`
		Status         string `gorm:"column:status"`
		StatusVersion  int64  `gorm:"column:status_version"`
		ConversationID string `gorm:"column:conversation_id"`
	}
	var rows []interactionRow
	err := q.db.Table("interaction_records").
		Select("id, status, status_version, conversation_id").
		Where("user_id = ? AND character_id = ? AND status IN ?", userID, characterID, interactionActiveStatuses).
		Order("updated_at DESC").
		Limit(10).
		Find(&rows).Error
	if err != nil {
		log.Logger.Warnf("state_source: query active interactions failed: %v", err)
		return nil, nil
	}
	snapshots := make([]behavior.InteractionSnapshot, 0, len(rows))
	for _, r := range rows {
		snapshots = append(snapshots, behavior.InteractionSnapshot{
			InteractionID:  r.ID,
			Phase:          r.Status,
			StatusVersion:  r.StatusVersion,
			ConversationID: r.ConversationID,
		})
	}
	return snapshots, nil
}

func (q *AmitiaStateSourceQuery) QueryVoiceSession(ctx context.Context, userID, characterID string) (*behavior.VoiceBehaviorState, error) {
	if q.db == nil {
		return nil, nil
	}
	type voiceRow struct {
		ID            string    `gorm:"column:id"`
		SessionID     string    `gorm:"column:session_id"`
		Status        string    `gorm:"column:status"`
		StatusVersion int64     `gorm:"column:status_version"`
		UpdatedAt     time.Time `gorm:"column:updated_at"`
	}
	var row voiceRow
	err := q.db.Table("interaction_records").
		Select("id, session_id, status, status_version, updated_at").
		Where("character_id = ? AND source = ? AND status IN ?", characterID, "voice", voiceActiveStatuses).
		Order("updated_at DESC").
		Limit(1).
		Find(&row).Error
	if err != nil {
		log.Logger.Warnf("state_source: query voice session failed: %v", err)
		return nil, nil
	}
	if row.ID == "" {
		return nil, nil
	}
	state := "listening"
	if row.Status == "generated" || row.Status == "committed" || row.Status == "delivery_pending" || row.Status == "delivered" {
		state = "speaking"
	}
	return &behavior.VoiceBehaviorState{
		SessionID:      row.SessionID,
		State:          state,
		StateVersion:   row.StatusVersion,
		LeaseExpiresAt: row.UpdatedAt.Add(30 * time.Second),
	}, nil
}

func (q *AmitiaStateSourceQuery) QueryActiveTools(ctx context.Context, userID, characterID string) (map[string]behavior.ToolOperationState, error) {
	if q.db == nil {
		return nil, nil
	}
	type toolRow struct {
		ID        string `gorm:"column:id"`
		ToolName  string `gorm:"column:tool_name"`
		Status    string `gorm:"column:status"`
		CreatedAt string `gorm:"column:created_at"`
		UpdatedAt string `gorm:"column:updated_at"`
	}
	var rows []toolRow
	err := q.db.Table("tool_call_intents").
		Select("id, tool_name, status, created_at, updated_at").
		Where("character_id = ? AND status IN ?", characterID, toolActiveStatuses).
		Order("updated_at DESC").
		Limit(20).
		Find(&rows).Error
	if err != nil {
		log.Logger.Warnf("state_source: query active tools failed: %v", err)
		return nil, nil
	}
	tools := make(map[string]behavior.ToolOperationState)
	for _, r := range rows {
		startedAt := parseStateSourceTime(r.CreatedAt)
		lastActivityAt := parseStateSourceTime(r.UpdatedAt)
		tools[r.ID] = behavior.ToolOperationState{
			OperationID:    r.ID,
			ToolCategory:   r.ToolName,
			StartedAt:      startedAt,
			LastActivityAt: lastActivityAt,
			LeaseExpiresAt: lastActivityAt.Add(5 * time.Minute),
		}
	}
	return tools, nil
}

func parseStateSourceTime(s string) time.Time {
	if s == "" {
		return time.Now()
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Now()
}

var _ behavior.StateSourceQuery = (*AmitiaStateSourceQuery)(nil)
