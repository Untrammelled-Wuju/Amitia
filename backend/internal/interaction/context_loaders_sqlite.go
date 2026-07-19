package interaction

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/character"
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

type RoleRuntimeProfileContextLoader struct {
	repo character.Repository
}

func NewRoleRuntimeProfileContextLoader(repo character.Repository) *RoleRuntimeProfileContextLoader {
	return &RoleRuntimeProfileContextLoader{repo: repo}
}

func (l *RoleRuntimeProfileContextLoader) Name() string           { return "runtimeProfile" }
func (l *RoleRuntimeProfileContextLoader) IsRequired() bool       { return true }
func (l *RoleRuntimeProfileContextLoader) Timeout() time.Duration { return 800 * time.Millisecond }
func (l *RoleRuntimeProfileContextLoader) CacheKey(scope InteractionScope, version string) string {
	return version + ":runtimeProfile:" + scope.CharacterID
}
func (l *RoleRuntimeProfileContextLoader) Load(ctx context.Context, scope InteractionScope, version string) (SnapshotField[any], error) {
	if l.repo == nil {
		return FieldUnavailable[any](l.Name()), errors.New("runtime profile repository unavailable")
	}
	if err := ctx.Err(); err != nil {
		return FieldUnavailable[any](l.Name()), err
	}
	profile, err := l.repo.GetRuntimeProfile(scope.CharacterID)
	if err != nil {
		return FieldUnavailable[any](l.Name()), err
	}
	return FieldReady[any](runtimeProfileFromRole(profile), l.Name(), version), nil
}

func runtimeProfileFromRole(profile *character.RoleRuntimeProfile) RuntimeProfile {
	if profile == nil {
		return RuntimeProfile{}
	}
	return RuntimeProfile{
		PersonalitySource:   "role_runtime_profile",
		CharacterID:         profile.CharacterID,
		Name:                profile.Name,
		Identity:            profile.Identity,
		Personality:         profile.Personality,
		SpeakingStyle:       profile.SpeakingStyle,
		RelationshipStyle:   profile.RelationshipStyle,
		CharacterBase:       profile.CharacterBase,
		BoundaryRules:       profile.BoundaryRules,
		PersonalitySliders:  profile.PersonalitySliders,
		BasePrompt:          profile.BasePrompt,
		GeneratedPrompt:     profile.GeneratedPrompt,
		PersonalityConfig:   profile.PersonalityConfig,
		ChatStyleConfig:     profile.ChatStyleConfig,
		SceneRules:          profile.SceneRules,
		Gender:              profile.Gender,
		GenderLabel:         profile.GenderLabel,
		Pronoun:             profile.Pronoun,
		SelfReference:       profile.SelfReference,
		UserAddressingStyle: profile.UserAddressingStyle,
		GenderExpression:    profile.GenderExpression,
		LifeIdentity:        profile.LifeIdentity,
		Diagnostics:         profile.Diagnostics,
	}
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
		Emotion string
		Mood    string
		Stress  float64
		Energy  float64
	}
	err := l.db.WithContext(ctx).Table("psyche_states").Select("emotion, mood, stress, energy").Where("character_id = ?", scope.CharacterID).Take(&row).Error
	if err != nil {
		return FieldUnavailable[any](l.Name()), err
	}
	state := PsycheState{
		Stress:  row.Stress,
		Fatigue: clamp01(1 - row.Energy),
		Arousal: clamp01(0.5 + row.Stress/2),
	}
	if row.Emotion != "" && row.Emotion != "{}" {
		var emo struct {
			Valence   float64 `json:"valence"`
			Arousal   float64 `json:"arousal"`
			Dominance float64 `json:"dominance"`
		}
		if json.Unmarshal([]byte(row.Emotion), &emo) == nil {
			state.Valence = emo.Valence
			state.Dominance = emo.Dominance
			if emo.Arousal > 0 {
				state.Arousal = emo.Arousal
			}
		}
	}
	if row.Mood != "" && row.Mood != "{}" {
		var m struct {
			MoodValence float64 `json:"moodValence"`
			MoodArousal float64 `json:"moodArousal"`
		}
		if json.Unmarshal([]byte(row.Mood), &m) == nil {
			state.MoodValence = m.MoodValence
			state.MoodArousal = m.MoodArousal
		}
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
	userID := scope.UserID
	if userID == "" {
		userID = "default"
	}
	err := l.db.WithContext(ctx).Table("relationship_states").Select("relation_data").Where("character_id = ? AND user_id = ?", scope.CharacterID, userID).Order("CASE WHEN channel = '*' AND relation_type = 'user_character' THEN 0 ELSE 1 END, updated_at DESC").Take(&row).Error
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

type LifeContextLoader struct {
	db *gorm.DB
}

func NewLifeContextLoader(db *gorm.DB) *LifeContextLoader {
	return &LifeContextLoader{db: db}
}

func (l *LifeContextLoader) Name() string           { return "life" }
func (l *LifeContextLoader) IsRequired() bool       { return false }
func (l *LifeContextLoader) Timeout() time.Duration { return 800 * time.Millisecond }
func (l *LifeContextLoader) CacheKey(scope InteractionScope, version string) string {
	return version + ":life:" + scope.CharacterID
}
func (l *LifeContextLoader) Load(ctx context.Context, scope InteractionScope, version string) (SnapshotField[any], error) {
	if l.db == nil {
		return FieldUnavailable[any](l.Name()), errors.New("database unavailable")
	}
	state := LifeState{
		Mood:            "neutral",
		Energy:          0.5,
		Available:       true,
		CurrentState:    "IDLE",
		CurrentActivity: "空闲中",
	}
	if l.db.Migrator().HasTable("moods") {
		var row struct {
			Mood      string
			MoodValue string
		}
		err := l.db.WithContext(ctx).Table("moods").Select("mood, mood_value").Where("character_id = ?", scope.CharacterID).Order("created_at DESC").Limit(1).Scan(&row).Error
		if err != nil {
			return FieldUnavailable[any](l.Name()), err
		}
		if row.Mood != "" {
			state.Mood = row.Mood
		}
		if row.MoodValue != "" {
			state.Mood = row.MoodValue
		}
	}
	if l.db.Migrator().HasTable("psyche_states") {
		var row struct {
			Energy float64
		}
		err := l.db.WithContext(ctx).Table("psyche_states").Select("energy").Where("character_id = ?", scope.CharacterID).Order("updated_at DESC").Limit(1).Scan(&row).Error
		if err != nil {
			return FieldUnavailable[any](l.Name()), err
		}
		if row.Energy != 0 {
			state.Energy = clamp01(row.Energy)
		}
	}
	needs, err := loadNeedSummaries(ctx, l.db, scope.CharacterID)
	if err != nil {
		return FieldUnavailable[any](l.Name()), err
	}
	state.Needs = needs
	return FieldReady[any](state, l.Name(), version), nil
}

type NeedContextLoader struct {
	db *gorm.DB
}

func NewNeedContextLoader(db *gorm.DB) *NeedContextLoader {
	return &NeedContextLoader{db: db}
}

func (l *NeedContextLoader) Name() string           { return "needs" }
func (l *NeedContextLoader) IsRequired() bool       { return false }
func (l *NeedContextLoader) Timeout() time.Duration { return 800 * time.Millisecond }
func (l *NeedContextLoader) CacheKey(scope InteractionScope, version string) string {
	return version + ":needs:" + scope.CharacterID
}
func (l *NeedContextLoader) Load(ctx context.Context, scope InteractionScope, version string) (SnapshotField[any], error) {
	if l.db == nil {
		return FieldUnavailable[any](l.Name()), errors.New("database unavailable")
	}
	needs, err := loadNeedSummaries(ctx, l.db, scope.CharacterID)
	if err != nil {
		return FieldUnavailable[any](l.Name()), err
	}
	return FieldReady[any](NeedState{Needs: needs, Count: len(needs)}, l.Name(), version), nil
}

type UnresolvedThreadContextLoader struct {
	db *gorm.DB
}

func NewUnresolvedThreadContextLoader(db *gorm.DB) *UnresolvedThreadContextLoader {
	return &UnresolvedThreadContextLoader{db: db}
}

func (l *UnresolvedThreadContextLoader) Name() string           { return "unresolvedThreads" }
func (l *UnresolvedThreadContextLoader) IsRequired() bool       { return false }
func (l *UnresolvedThreadContextLoader) Timeout() time.Duration { return 800 * time.Millisecond }
func (l *UnresolvedThreadContextLoader) CacheKey(scope InteractionScope, version string) string {
	return version + ":unresolvedThreads:" + scope.CharacterID + ":" + scope.UserID
}
func (l *UnresolvedThreadContextLoader) Load(ctx context.Context, scope InteractionScope, version string) (SnapshotField[any], error) {
	if l.db == nil {
		return FieldUnavailable[any](l.Name()), errors.New("database unavailable")
	}
	if !l.db.Migrator().HasTable("unresolved_threads") {
		return FieldReady[any](UnresolvedThreadSet{}, l.Name(), version), nil
	}
	var rows []struct {
		ID              string
		Topic           string
		Reason          string
		Severity        float64
		EscalationLevel int
		CreatedAt       string
	}
	err := l.db.WithContext(ctx).Table("unresolved_threads").
		Select("id, topic, reason, severity, escalation_level, created_at").
		Where("character_id = ? AND resolved_at IS NULL", scope.CharacterID).
		Order("severity DESC, created_at ASC").
		Limit(5).
		Scan(&rows).Error
	if err != nil {
		return FieldUnavailable[any](l.Name()), err
	}
	threads := make([]UnresolvedThreadSummary, 0, len(rows))
	for _, row := range rows {
		thread := UnresolvedThreadSummary{
			ID:              row.ID,
			Topic:           row.Topic,
			Reason:          row.Reason,
			Severity:        row.Severity,
			EscalationLevel: row.EscalationLevel,
		}
		if parsed, ok := parseSQLiteTime(row.CreatedAt); ok {
			thread.CreatedAt = parsed
		}
		threads = append(threads, thread)
	}
	return FieldReady[any](UnresolvedThreadSet{Threads: threads, Count: len(threads)}, l.Name(), version), nil
}

func loadNeedSummaries(ctx context.Context, db *gorm.DB, characterID string) ([]NeedSummary, error) {
	if !db.Migrator().HasTable("need_states") {
		return nil, nil
	}
	var rows []struct {
		NeedKey      string
		CurrentValue float64
		Baseline     float64
		UpdatedAt    string
	}
	err := db.WithContext(ctx).Table("need_states").Select("need_key, current_value, baseline, updated_at").Where("character_id = ?", characterID).Order("need_key").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	needs := make([]NeedSummary, 0, len(rows))
	for _, row := range rows {
		needs = append(needs, NeedSummary{
			Kind:      row.NeedKey,
			Level:     row.CurrentValue,
			Baseline:  row.Baseline,
			UpdatedAt: row.UpdatedAt,
		})
	}
	return needs, nil
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
