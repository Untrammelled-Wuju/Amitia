package wiring

import (
	"context"
	"fmt"
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
	"delivery_pending", "delivered",
}

var toolActiveStatuses = []string{"PENDING", "RUNNING"}

func interactionStatusToBehaviorPhase(status string) string {
	switch status {
	case "received", "normalized", "queued":
		return "received"
	case "processing":
		return "context_loading"
	case "context_ready", "decided":
		return "response_started"
	case "generated", "committed", "delivery_pending", "delivered":
		return "response_ready"
	default:
		return ""
	}
}

func (q *AmitiaStateSourceQuery) QueryActiveInteractions(ctx context.Context, userID, characterID string) ([]behavior.InteractionSnapshot, error) {
	if q.db == nil {
		return nil, nil
	}
	type interactionRow struct {
		ID             string    `gorm:"column:id"`
		Status         string    `gorm:"column:status"`
		StatusVersion  int64     `gorm:"column:status_version"`
		ConversationID string    `gorm:"column:conversation_id"`
		CreatedAt      time.Time `gorm:"column:created_at"`
		UpdatedAt      time.Time `gorm:"column:updated_at"`
	}
	var rows []interactionRow
	err := q.db.WithContext(ctx).Table("interaction_records").
		Select("id, status, status_version, conversation_id, created_at, updated_at").
		Where("user_id = ? AND character_id = ? AND status IN ?", userID, characterID, interactionActiveStatuses).
		Order("created_at DESC, updated_at DESC, status_version DESC").
		Limit(10).
		Find(&rows).Error
	if err != nil {
		log.Logger.Warnf("state_source: query active interactions failed: %v", err)
		return nil, err
	}
	snapshots := make([]behavior.InteractionSnapshot, 0, len(rows))
	for _, r := range rows {
		phase := interactionStatusToBehaviorPhase(r.Status)
		if phase == "" {
			continue
		}
		snapshots = append(snapshots, behavior.InteractionSnapshot{
			InteractionID:  r.ID,
			Phase:          phase,
			StatusVersion:  r.StatusVersion,
			ConversationID: r.ConversationID,
			CreatedAt:      r.CreatedAt,
			UpdatedAt:      r.UpdatedAt,
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
	err := q.db.WithContext(ctx).Table("interaction_records").
		Select("id, session_id, status, status_version, updated_at").
		Where("user_id = ? AND character_id = ? AND source = ? AND status IN ?", userID, characterID, "voice", voiceActiveStatuses).
		Order("updated_at DESC").
		Limit(1).
		Find(&row).Error
	if err != nil {
		log.Logger.Warnf("state_source: query voice session failed: %v", err)
		return nil, err
	}
	if row.ID == "" {
		return nil, nil
	}
	if row.UpdatedAt.IsZero() {
		return nil, fmt.Errorf("state_source: voice session %s has invalid updated_at", row.ID)
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
		ID            string `gorm:"column:id"`
		InteractionID string `gorm:"column:interaction_id"`
		ToolName      string `gorm:"column:tool_name"`
		Status        string `gorm:"column:status"`
		CreatedAt     string `gorm:"column:created_at"`
		UpdatedAt     string `gorm:"column:updated_at"`
	}
	var rows []toolRow
	err := q.db.WithContext(ctx).Table("tool_call_intents").
		Select(`tool_call_intents.id, tool_call_intents.tool_name, tool_call_intents.status, tool_call_intents.created_at, tool_call_intents.updated_at,
			COALESCE((
				SELECT ir.id FROM interaction_records AS ir
				WHERE ir.user_id = ?
				  AND ir.character_id = tool_call_intents.character_id
				  AND (
					(tool_call_intents.request_id <> ''
					 AND ir.request_id = tool_call_intents.request_id
					 AND (tool_call_intents.conversation_id = '' OR ir.conversation_id = tool_call_intents.conversation_id)
					 AND (tool_call_intents.channel = '' OR ir.channel = tool_call_intents.channel))
					OR
					(tool_call_intents.request_id = ''
					 AND tool_call_intents.conversation_id <> ''
					 AND ir.conversation_id = tool_call_intents.conversation_id
					 AND (tool_call_intents.channel = '' OR ir.channel = tool_call_intents.channel))
				  )
				ORDER BY ir.created_at DESC, ir.updated_at DESC
				LIMIT 1
			), '') AS interaction_id`, userID).
		Where("character_id = ? AND status IN ?", characterID, toolActiveStatuses).
		Where(`EXISTS (
			SELECT 1 FROM interaction_records AS ir
			WHERE ir.user_id = ?
			  AND ir.character_id = tool_call_intents.character_id
			  AND (
				(tool_call_intents.request_id <> ''
				 AND ir.request_id = tool_call_intents.request_id
				 AND (tool_call_intents.conversation_id = '' OR ir.conversation_id = tool_call_intents.conversation_id)
				 AND (tool_call_intents.channel = '' OR ir.channel = tool_call_intents.channel))
				OR
				(tool_call_intents.request_id = ''
				 AND tool_call_intents.conversation_id <> ''
				 AND ir.conversation_id = tool_call_intents.conversation_id
				 AND (tool_call_intents.channel = '' OR ir.channel = tool_call_intents.channel))
			  )
		)`, userID).
		Where(`NOT EXISTS (
			SELECT 1 FROM interaction_records AS other
			WHERE other.user_id <> ?
			  AND other.character_id = tool_call_intents.character_id
			  AND (
				(tool_call_intents.request_id <> ''
				 AND other.request_id = tool_call_intents.request_id
				 AND (tool_call_intents.conversation_id = '' OR other.conversation_id = tool_call_intents.conversation_id)
				 AND (tool_call_intents.channel = '' OR other.channel = tool_call_intents.channel))
				OR
				(tool_call_intents.request_id = ''
				 AND tool_call_intents.conversation_id <> ''
				 AND other.conversation_id = tool_call_intents.conversation_id
				 AND (tool_call_intents.channel = '' OR other.channel = tool_call_intents.channel))
			  )
		)`, userID).
		Order("updated_at DESC").
		Limit(20).
		Find(&rows).Error
	if err != nil {
		log.Logger.Warnf("state_source: query active tools failed: %v", err)
		return nil, err
	}
	tools := make(map[string]behavior.ToolOperationState)
	for _, r := range rows {
		startedAt, err := parseStateSourceTime(r.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("state_source: tool %s created_at: %w", r.ID, err)
		}
		lastActivityAt, err := parseStateSourceTime(r.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("state_source: tool %s updated_at: %w", r.ID, err)
		}
		tools[r.ID] = behavior.ToolOperationState{
			OperationID:    r.ID,
			InteractionID:  r.InteractionID,
			ToolCategory:   r.ToolName,
			StartedAt:      startedAt,
			LastActivityAt: lastActivityAt,
			LeaseExpiresAt: lastActivityAt.Add(5 * time.Minute),
		}
	}
	return tools, nil
}

func parseStateSourceTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported timestamp %q", s)
}

var _ behavior.StateSourceQuery = (*AmitiaStateSourceQuery)(nil)
