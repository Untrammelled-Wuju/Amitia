package interaction

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type ChannelContextLoader struct{}

func NewChannelContextLoader() *ChannelContextLoader {
	return &ChannelContextLoader{}
}

func (l *ChannelContextLoader) Name() string           { return "channel" }
func (l *ChannelContextLoader) IsRequired() bool       { return true }
func (l *ChannelContextLoader) Timeout() time.Duration { return 500 * time.Millisecond }
func (l *ChannelContextLoader) CacheKey(scope InteractionScope, version string) string {
	return version + ":channel:" + scope.Channel
}
func (l *ChannelContextLoader) Load(ctx context.Context, scope InteractionScope, version string) (SnapshotField[any], error) {
	caps := ChannelCapabilities{
		Channel:      scope.Channel,
		SupportsText: true,
	}
	switch scope.Channel {
	case "web":
		caps.SupportsImage = true
		caps.SupportsVoice = true
	case "wechat", "qq":
		caps.SupportsImage = true
	}
	return FieldReady[any](caps, l.Name(), version), ctx.Err()
}

type ConversationContextLoader struct {
	db *gorm.DB
}

func NewConversationContextLoader(db *gorm.DB) *ConversationContextLoader {
	return &ConversationContextLoader{db: db}
}

func (l *ConversationContextLoader) Name() string           { return "conversation" }
func (l *ConversationContextLoader) IsRequired() bool       { return false }
func (l *ConversationContextLoader) Timeout() time.Duration { return 800 * time.Millisecond }
func (l *ConversationContextLoader) CacheKey(scope InteractionScope, version string) string {
	return version + ":conversation:" + scope.ConversationID
}
func (l *ConversationContextLoader) Load(ctx context.Context, scope InteractionScope, version string) (SnapshotField[any], error) {
	var row struct {
		ID           string
		MessageCount int
		UpdatedAt    string
	}
	err := l.db.WithContext(ctx).Table("conversations").Select("id, message_count, updated_at").Where("id = ?", scope.ConversationID).Take(&row).Error
	if err != nil {
		return FieldUnavailable[any](l.Name()), err
	}
	state := ConversationState{
		ConversationID: row.ID,
		MessageCount:   row.MessageCount,
		StateVersion:   version,
		Scope:          &scope,
	}
	if parsed, ok := parseSQLiteTime(row.UpdatedAt); ok {
		state.LastMessageAt = parsed
	}
	return FieldReady[any](state, l.Name(), version), nil
}

type PsycheContextLoader struct {
	db *gorm.DB
}

func NewPsycheContextLoader(db *gorm.DB) *PsycheContextLoader {
	return &PsycheContextLoader{db: db}
}

func (l *PsycheContextLoader) Name() string           { return "psyche" }
func (l *PsycheContextLoader) IsRequired() bool       { return false }
func (l *PsycheContextLoader) Timeout() time.Duration { return 800 * time.Millisecond }
func (l *PsycheContextLoader) CacheKey(scope InteractionScope, version string) string {
	return version + ":psyche:" + scope.CharacterID
}
func (l *PsycheContextLoader) Load(ctx context.Context, scope InteractionScope, version string) (SnapshotField[any], error) {
	var row struct {
		Stress float64
		Energy float64
	}
	err := l.db.WithContext(ctx).Table("psyche_states").Select("stress, energy").Where("character_id = ?", scope.CharacterID).Take(&row).Error
	if err != nil {
		return FieldUnavailable[any](l.Name()), err
	}
	state := PsycheState{
		Stress:  row.Stress,
		Fatigue: clamp01(1 - row.Energy),
		Arousal: clamp01(0.5 + row.Stress/2),
	}
	return FieldReady[any](state, l.Name(), version), nil
}

type RelationshipContextLoader struct {
	db *gorm.DB
}

func NewRelationshipContextLoader(db *gorm.DB) *RelationshipContextLoader {
	return &RelationshipContextLoader{db: db}
}

func (l *RelationshipContextLoader) Name() string           { return "relationship" }
func (l *RelationshipContextLoader) IsRequired() bool       { return false }
func (l *RelationshipContextLoader) Timeout() time.Duration { return 800 * time.Millisecond }
func (l *RelationshipContextLoader) CacheKey(scope InteractionScope, version string) string {
	return version + ":relationship:" + scope.CharacterID + ":" + scope.UserID
}
func (l *RelationshipContextLoader) Load(ctx context.Context, scope InteractionScope, version string) (SnapshotField[any], error) {
	var row struct {
		RelationData string
	}
	err := l.db.WithContext(ctx).Table("relationship_states").Select("relation_data").Where("character_id = ?", scope.CharacterID).Order("updated_at DESC").Take(&row).Error
	if err != nil {
		return FieldUnavailable[any](l.Name()), err
	}
	var raw map[string]float64
	json.Unmarshal([]byte(row.RelationData), &raw)
	state := RelationshipState{
		Trust:            raw["trust"],
		Familiarity:      raw["familiarity"],
		Security:         raw["security"],
		Tension:          raw["tension"],
		RepairConfidence: raw["repairConfidence"],
		Boundary:         raw["boundary"],
	}
	return FieldReady[any](state, l.Name(), version), nil
}

type BeliefContextLoader struct {
	db *gorm.DB
}

func NewBeliefContextLoader(db *gorm.DB) *BeliefContextLoader {
	return &BeliefContextLoader{db: db}
}

func (l *BeliefContextLoader) Name() string           { return "beliefs" }
func (l *BeliefContextLoader) IsRequired() bool       { return false }
func (l *BeliefContextLoader) Timeout() time.Duration { return 800 * time.Millisecond }
func (l *BeliefContextLoader) CacheKey(scope InteractionScope, version string) string {
	return version + ":beliefs:" + scope.CharacterID
}
func (l *BeliefContextLoader) Load(ctx context.Context, scope InteractionScope, version string) (SnapshotField[any], error) {
	var rows []struct {
		Key        string
		Value      string
		Confidence float64
	}
	err := l.db.WithContext(ctx).Table("memories").Select("key, value, confidence").Where("character_id = ?", scope.CharacterID).Order("importance DESC, updated_at DESC").Limit(5).Scan(&rows).Error
	if err != nil {
		return FieldUnavailable[any](l.Name()), err
	}
	beliefs := make([]ResolvedBelief, 0, len(rows))
	for _, row := range rows {
		beliefs = append(beliefs, ResolvedBelief{Key: row.Key, Value: row.Value, Confidence: row.Confidence})
	}
	return FieldReady[any](BeliefSet{Beliefs: beliefs}, l.Name(), version), nil
}

func parseSQLiteTime(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	layouts := []string{time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02T15:04:05Z07:00"}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}
