package interaction

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/extensioncontext"
	"gorm.io/gorm"
	amitialog "log"
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
	query := l.db.WithContext(ctx).Table("conversations").Select("id, message_count, updated_at").Where("id = ?", scope.ConversationID)
	if strings.TrimSpace(scope.SpaceID) != "" {
		query = query.Where("space_id = ?", strings.TrimSpace(scope.SpaceID))
	}
	err := query.Take(&row).Error
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

type BeliefContextLoader struct {
	read business.MemoryConsumerReader
}

func NewBeliefContextLoader(read business.MemoryConsumerReader) *BeliefContextLoader {
	return &BeliefContextLoader{read: read}
}

func (l *BeliefContextLoader) Name() string           { return "beliefs" }
func (l *BeliefContextLoader) IsRequired() bool       { return false }
func (l *BeliefContextLoader) Timeout() time.Duration { return 800 * time.Millisecond }
func (l *BeliefContextLoader) CacheKey(scope InteractionScope, version string) string {
	return version + ":beliefs:" + scope.CharacterID
}
func (l *BeliefContextLoader) Load(ctx context.Context, scope InteractionScope, version string) (SnapshotField[any], error) {
	if l.read == nil {
		return FieldUnavailable[any](l.Name()), errors.New("统一记忆读取端口尚未就绪")
	}
	rows, err := l.read(ctx, scope.CharacterID, 5)
	if err != nil {
		return FieldUnavailable[any](l.Name()), err
	}
	beliefs := make([]ResolvedBelief, 0, len(rows))
	for _, row := range rows {
		document, content, err := business.MemoryContent(row)
		if err != nil {
			return FieldUnavailable[any](l.Name()), err
		}
		var confidence float64
		if err := json.Unmarshal(content["confidence"], &confidence); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		beliefs = append(beliefs, ResolvedBelief{Key: business.MemoryText(document, "key"), Value: business.MemoryValue(content), Confidence: confidence})
	}
	return FieldReady[any](BeliefSet{Beliefs: beliefs}, l.Name(), version), nil
}

type LifeContextLoader struct {
	provider extensioncontext.Provider
}

func NewLifeContextLoader(provider extensioncontext.Provider) *LifeContextLoader {
	return &LifeContextLoader{provider: provider}
}

func (l *LifeContextLoader) Name() string           { return "life" }
func (l *LifeContextLoader) IsRequired() bool       { return false }
func (l *LifeContextLoader) Timeout() time.Duration { return 800 * time.Millisecond }
func (l *LifeContextLoader) CacheKey(scope InteractionScope, version string) string {
	return version + ":life:" + scope.CharacterID
}
func (l *LifeContextLoader) Load(ctx context.Context, scope InteractionScope, version string) (SnapshotField[any], error) {
	if l.provider == nil {
		return FieldUnavailable[any](l.Name()), nil
	}
	raw, err := l.provider.Resolve(ctx, "chat.realtime.schedule", extensioncontext.Request{
		SpaceID:        scope.SpaceID,
		CharacterID:    scope.CharacterID,
		ConversationID: scope.ConversationID,
		At:             time.Now(),
	})
	if err != nil {
		return FieldUnavailable[any](l.Name()), err
	}
	snapshot, err := extensioncontext.Decode(raw)
	if err != nil {
		return FieldUnavailable[any](l.Name()), err
	}
	state := LifeState{}
	found := false
	for _, contribution := range snapshot.Contributions {
		if contribution.Error != "" {
			continue
		}
		var payload struct {
			StateLife struct {
				Mood            string  `json:"mood"`
				Energy          float64 `json:"energy"`
				Busy            bool    `json:"busy"`
				Available       bool    `json:"available"`
				CurrentState    string  `json:"currentState"`
				CurrentActivity string  `json:"currentActivity"`
				IdleDuration    float64 `json:"idleDuration"`
			} `json:"stateLife"`
		}
		if err := json.Unmarshal(contribution.Data, &payload); err != nil {
			continue
		}
		if payload.StateLife.CurrentState == "" && payload.StateLife.CurrentActivity == "" {
			continue
		}
		energy := payload.StateLife.Energy
		if energy > 1 {
			energy /= 100
		}
		state = LifeState{
			Mood:            payload.StateLife.Mood,
			Energy:          clamp01(energy),
			Busy:            payload.StateLife.Busy,
			Available:       payload.StateLife.Available,
			CurrentState:    payload.StateLife.CurrentState,
			CurrentActivity: payload.StateLife.CurrentActivity,
			IdleSeconds:     payload.StateLife.IdleDuration,
		}
		found = true
		break
	}
	if !found {
		return FieldUnavailable[any](l.Name()), nil
	}
	return FieldReady[any](state, "extension_context", version), nil
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
	return version + ":unresolvedThreads:" + scope.CharacterID + ":" + scope.SpaceID
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
