package temporal

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RelationshipTimeRepository struct {
	db    *gorm.DB
	clock Clock
}

func NewRelationshipTimeRepository(db *gorm.DB, clock Clock) *RelationshipTimeRepository {
	if clock == nil {
		clock = SystemClock{}
	}
	return &RelationshipTimeRepository{db: db, clock: clock}
}

func (r *RelationshipTimeRepository) WithDB(db *gorm.DB) *RelationshipTimeRepository {
	return &RelationshipTimeRepository{db: db, clock: r.clock}
}

func (r *RelationshipTimeRepository) WithTransaction(ctx context.Context, fn func(*RelationshipTimeRepository) error) error {
	if r == nil || r.db == nil {
		return errors.New("relationship time database is required")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(r.WithDB(tx))
	})
}

func (r *RelationshipTimeRepository) GetGlobalPresence(ctx context.Context, userID string) (*GlobalPresenceState, error) {
	var state GlobalPresenceState
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &state, err
}

func (r *RelationshipTimeRepository) SaveGlobalPresence(ctx context.Context, state *GlobalPresenceState) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(state).Error
}

func (r *RelationshipTimeRepository) GetRelationshipPresence(ctx context.Context, userID, characterID string) (*RelationshipPresenceState, error) {
	var state RelationshipPresenceState
	err := r.db.WithContext(ctx).Where("user_id = ? AND character_id = ?", userID, characterID).First(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &state, err
}

func (r *RelationshipTimeRepository) GetSettings(ctx context.Context, characterID string) (*RelationshipTimeSettings, error) {
	var settings RelationshipTimeSettings
	err := r.db.WithContext(ctx).Where("character_id = ?", characterID).First(&settings).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &settings, err
}

func (r *RelationshipTimeRepository) SaveSettings(ctx context.Context, settings *RelationshipTimeSettings) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "character_id"}}, UpdateAll: true}).Create(settings).Error
}

func (r *RelationshipTimeRepository) SaveRelationshipPresence(ctx context.Context, state *RelationshipPresenceState) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "character_id"}},
		UpdateAll: true,
	}).Create(state).Error
}

func (r *RelationshipTimeRepository) SaveObservedPresence(ctx context.Context, input ObservePresenceInput) error {
	if strings.TrimSpace(input.UserID) == "" || strings.TrimSpace(input.CharacterID) == "" {
		return errors.New("user id and character id are required")
	}
	now := input.ObservedAt
	if now.IsZero() {
		now = r.clock.Now()
	}
	nowText := FormatRelationshipTime(now)
	global := GlobalPresenceState{
		UserID:                        input.UserID,
		FirstUserActivityAtUTC:        nowText,
		LastObservedUserActivityAtUTC: nowText,
		LastChannel:                   input.Channel,
		LastCharacterID:               input.CharacterID,
		StateVersion:                  1,
		CreatedAtUTC:                  nowText,
		UpdatedAtUTC:                  nowText,
	}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"last_observed_user_activity_at_utc": gorm.Expr("CASE WHEN last_observed_user_activity_at_utc = '' OR julianday(last_observed_user_activity_at_utc) < julianday(?) THEN ? ELSE last_observed_user_activity_at_utc END", nowText, nowText),
			"last_channel":                       input.Channel,
			"last_character_id":                  input.CharacterID,
			"state_version":                      gorm.Expr("state_version + 1"),
			"updated_at_utc":                     nowText,
		}),
	}).Create(&global).Error; err != nil {
		return err
	}
	relationship := RelationshipPresenceState{
		ID:                            relationshipPresenceID(input.UserID, input.CharacterID),
		UserID:                        input.UserID,
		CharacterID:                   input.CharacterID,
		LastObservedUserActivityAtUTC: nowText,
		ExpectedGapSeconds:            DefaultExpectedGap.Seconds(),
		ContinuityScore:               1,
		StateVersion:                  1,
		CreatedAtUTC:                  nowText,
		UpdatedAtUTC:                  nowText,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "character_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"last_observed_user_activity_at_utc": gorm.Expr("CASE WHEN last_observed_user_activity_at_utc = '' OR julianday(last_observed_user_activity_at_utc) < julianday(?) THEN ? ELSE last_observed_user_activity_at_utc END", nowText, nowText),
			"state_version":                      gorm.Expr("state_version + 1"),
			"updated_at_utc":                     nowText,
		}),
	}).Create(&relationship).Error
}

func (r *RelationshipTimeRepository) GetReceipt(ctx context.Context, userID, requestID string) (*InteractionReceipt, error) {
	var receipt InteractionReceipt
	err := r.db.WithContext(ctx).Where("user_id = ? AND request_id = ?", userID, requestID).First(&receipt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &receipt, err
}

func (r *RelationshipTimeRepository) GetReceiptByInteraction(ctx context.Context, interactionID string) (*InteractionReceipt, error) {
	var receipt InteractionReceipt
	err := r.db.WithContext(ctx).Where("interaction_id = ?", interactionID).First(&receipt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &receipt, err
}

func (r *RelationshipTimeRepository) CreateReceipt(ctx context.Context, receipt *InteractionReceipt) (bool, error) {
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(receipt)
	return result.RowsAffected > 0, result.Error
}

func (r *RelationshipTimeRepository) SaveReceipt(ctx context.Context, receipt *InteractionReceipt) error {
	return r.db.WithContext(ctx).Save(receipt).Error
}

func (r *RelationshipTimeRepository) GetReunionEpisode(ctx context.Context, episodeID string) (*ReunionEpisode, error) {
	var episode ReunionEpisode
	err := r.db.WithContext(ctx).Where("id = ?", episodeID).First(&episode).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &episode, err
}

func (r *RelationshipTimeRepository) GetReunionEpisodeByIdempotencyKey(ctx context.Context, idempotencyKey string) (*ReunionEpisode, error) {
	var episode ReunionEpisode
	err := r.db.WithContext(ctx).Where("idempotency_key = ?", idempotencyKey).First(&episode).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &episode, err
}

func (r *RelationshipTimeRepository) CreateOrGetReunionEpisode(ctx context.Context, episode *ReunionEpisode) (*ReunionEpisode, bool, error) {
	if episode.ID == "" {
		episode.ID = uuid.NewString()
	}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "idempotency_key"}},
		DoNothing: true,
	}).Create(episode)
	if result.Error != nil {
		return nil, false, result.Error
	}
	stored, err := r.GetReunionEpisodeByIdempotencyKey(ctx, episode.IdempotencyKey)
	return stored, result.RowsAffected > 0, err
}

func (r *RelationshipTimeRepository) ClaimReunionEpisode(ctx context.Context, episodeID, interactionID string) (*ReunionEpisode, bool, error) {
	if episodeID == "" || interactionID == "" {
		return nil, false, errors.New("episode id and interaction id are required")
	}
	now := r.clock.Now()
	nowText := FormatRelationshipTime(now)
	expiresText := FormatRelationshipTime(now.Add(ReunionClaimTTL))
	result := r.db.WithContext(ctx).Model(&ReunionEpisode{}).
		Where("id = ? AND status IN ? AND (claim_interaction_id = '' OR claim_interaction_id = ? OR claim_expires_at_utc = '' OR julianday(claim_expires_at_utc) <= julianday(?))", episodeID, []ReunionState{ReunionStatePending, ReunionStateClaimed}, interactionID, nowText).
		Updates(map[string]interface{}{
			"status":               ReunionStateClaimed,
			"claim_interaction_id": interactionID,
			"claim_expires_at_utc": expiresText,
			"updated_at_utc":       nowText,
		})
	if result.Error != nil {
		return nil, false, result.Error
	}
	episode, err := r.GetReunionEpisode(ctx, episodeID)
	return episode, result.RowsAffected > 0 && episode != nil && episode.ClaimInteractionID == interactionID, err
}

func (r *RelationshipTimeRepository) ReleaseClaim(ctx context.Context, interactionID, reason string) error {
	_ = reason
	nowText := FormatRelationshipTime(r.clock.Now())
	return r.db.WithContext(ctx).Model(&ReunionEpisode{}).
		Where("claim_interaction_id = ? AND status = ?", interactionID, ReunionStateClaimed).
		Updates(map[string]interface{}{
			"status":               ReunionStatePending,
			"claim_interaction_id": "",
			"claim_expires_at_utc": "",
			"updated_at_utc":       nowText,
		}).Error
}

func (r *RelationshipTimeRepository) AddCadenceSample(ctx context.Context, sample *CadenceSample) (bool, error) {
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "interaction_id"}, {Name: "sample_kind"}},
		DoNothing: true,
	}).Create(sample)
	if result.Error != nil || result.RowsAffected == 0 {
		return result.RowsAffected > 0, result.Error
	}
	if err := r.trimCadenceSamples(ctx, sample.UserID, sample.CharacterID, sample.SampleKind); err != nil {
		return true, err
	}
	return true, nil
}

func (r *RelationshipTimeRepository) trimCadenceSamples(ctx context.Context, userID, characterID, sampleKind string) error {
	query := `DELETE FROM temporal_cadence_samples
WHERE id IN (
    SELECT id FROM temporal_cadence_samples
    WHERE user_id = ? AND character_id = ? AND sample_kind = ? AND included = 1
    ORDER BY julianday(current_interaction_at_utc) DESC, julianday(created_at_utc) DESC
    LIMIT -1 OFFSET ?
)`
	return r.db.WithContext(ctx).Exec(query, userID, characterID, sampleKind, MaximumCadenceSamples).Error
}

func (r *RelationshipTimeRepository) ListCadenceSamples(ctx context.Context, userID, characterID, sampleKind string, limit int) ([]CadenceSample, error) {
	if limit <= 0 || limit > MaximumCadenceSamples {
		limit = MaximumCadenceSamples
	}
	query := r.db.WithContext(ctx).Where("user_id = ? AND character_id = ? AND included = 1", userID, characterID)
	if sampleKind != "" {
		query = query.Where("sample_kind = ?", sampleKind)
	}
	var samples []CadenceSample
	err := query.Order("julianday(current_interaction_at_utc) DESC, julianday(created_at_utc) DESC").Limit(limit).Find(&samples).Error
	return samples, err
}

func (r *RelationshipTimeRepository) AddEffect(ctx context.Context, effect *TemporalEffectLedgerEntry) (bool, error) {
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "effect_key"}},
		DoNothing: true,
	}).Create(effect)
	return result.RowsAffected > 0, result.Error
}

func (r *RelationshipTimeRepository) FinalizeInteractionTx(ctx context.Context, tx *gorm.DB, input FinalizeInteractionInput) error {
	repository := r
	if tx != nil {
		repository = r.WithDB(tx)
	}
	receipt, err := repository.GetReceiptByInteraction(ctx, input.InteractionID)
	if err != nil {
		return err
	}
	if receipt == nil {
		return fmt.Errorf("relationship time receipt not found: %s", input.InteractionID)
	}
	if receipt.Status == InteractionReceiptCommitted {
		return nil
	}
	committedAt := input.CommittedAt
	if committedAt.IsZero() {
		committedAt = repository.clock.Now()
	}
	committedText := FormatRelationshipTime(committedAt)
	global, err := repository.GetGlobalPresence(ctx, input.UserID)
	if err != nil {
		return err
	}
	if global == nil {
		global = &GlobalPresenceState{UserID: input.UserID, CreatedAtUTC: committedText}
	}
	globalSessionBreak := isSessionBreak(global.LastCommittedUserInteractionAtUTC, committedAt)
	if global.FirstUserActivityAtUTC == "" {
		global.FirstUserActivityAtUTC = committedText
	}
	global.LastCommittedUserInteractionAtUTC = committedText
	global.LastCharacterID = input.CharacterID
	global.InteractionCount++
	if globalSessionBreak || global.SessionCount == 0 {
		global.SessionCount++
	}
	global.StateVersion++
	global.UpdatedAtUTC = committedText
	if err := repository.SaveGlobalPresence(ctx, global); err != nil {
		return err
	}
	relationship, err := repository.GetRelationshipPresence(ctx, input.UserID, input.CharacterID)
	if err != nil {
		return err
	}
	if relationship == nil {
		relationship = &RelationshipPresenceState{
			ID:                 relationshipPresenceID(input.UserID, input.CharacterID),
			UserID:             input.UserID,
			CharacterID:        input.CharacterID,
			ExpectedGapSeconds: DefaultExpectedGap.Seconds(),
			ContinuityScore:    1,
			CreatedAtUTC:       committedText,
		}
	}
	relationshipSessionBreak := isSessionBreak(relationship.LastCommittedUserInteractionAtUTC, committedAt)
	if relationship.FirstInteractionAtUTC == "" {
		relationship.FirstInteractionAtUTC = committedText
	}
	relationship.LastCommittedUserInteractionAtUTC = committedText
	relationship.LastSuccessfulExchangeAtUTC = committedText
	relationship.InteractionCount++
	if relationshipSessionBreak || relationship.SessionCount == 0 {
		relationship.SessionCount++
	}
	if input.ExpectedGapSeconds > 0 {
		relationship.ExpectedGapSeconds = input.ExpectedGapSeconds
	}
	if input.GapMADSeconds >= 0 {
		relationship.GapMADSeconds = input.GapMADSeconds
	}
	if relationship.ReacclimationTurnsRemaining > 0 {
		relationship.ReacclimationTurnsRemaining--
	}
	if input.CadenceSample != nil {
		created, addErr := repository.AddCadenceSample(ctx, input.CadenceSample)
		if addErr != nil {
			return addErr
		}
		if created {
			if relationship.CadenceSampleCount < MaximumCadenceSamples {
				relationship.CadenceSampleCount++
			}
		}
	}
	if input.ReunionEpisodeID != "" {
		if err := repository.finalizeReunion(ctx, relationship, input, committedText); err != nil {
			return err
		}
	}
	relationship.StateVersion++
	relationship.UpdatedAtUTC = committedText
	if err := repository.SaveRelationshipPresence(ctx, relationship); err != nil {
		return err
	}
	for index := range input.EffectLedgerEntries {
		effect := &input.EffectLedgerEntries[index]
		if effect.ID == "" {
			effect.ID = uuid.NewString()
		}
		if effect.AppliedAtUTC == "" {
			effect.AppliedAtUTC = committedText
		}
		if _, err := repository.AddEffect(ctx, effect); err != nil {
			return err
		}
	}
	receipt.Status = InteractionReceiptCommitted
	receipt.UpdatedAtUTC = committedText
	return repository.SaveReceipt(ctx, receipt)
}

func (r *RelationshipTimeRepository) finalizeReunion(ctx context.Context, relationship *RelationshipPresenceState, input FinalizeInteractionInput, committedText string) error {
	episode, err := r.GetReunionEpisode(ctx, input.ReunionEpisodeID)
	if err != nil {
		return err
	}
	if episode == nil {
		return fmt.Errorf("reunion episode not found: %s", input.ReunionEpisodeID)
	}
	if episode.Status == ReunionStateHandled || episode.Status == ReunionStateSuppressed {
		return nil
	}
	if episode.Status != ReunionStateClaimed || episode.ClaimInteractionID != input.InteractionID {
		return errors.New("reunion episode is not claimed by interaction")
	}
	status := ReunionStateHandled
	if input.SuppressReunion {
		status = ReunionStateSuppressed
	}
	episode.Status = status
	episode.HandledInteractionID = input.InteractionID
	episode.HandledAtUTC = committedText
	episode.SuppressionReason = input.SuppressionReason
	episode.ClaimInteractionID = ""
	episode.ClaimExpiresAtUTC = ""
	episode.UpdatedAtUTC = committedText
	if err := r.db.WithContext(ctx).Save(episode).Error; err != nil {
		return err
	}
	effectType := "reunion_handled"
	if status == ReunionStateSuppressed {
		effectType = "reunion_suppressed"
	}
	if _, err := r.AddEffect(ctx, &TemporalEffectLedgerEntry{
		ID:               uuid.NewString(),
		EffectKey:        "relationship-time:" + episode.ID + ":finalized:v1",
		EffectType:       effectType,
		UserID:           input.UserID,
		CharacterID:      input.CharacterID,
		ReunionEpisodeID: episode.ID,
		InteractionID:    input.InteractionID,
		PayloadJSON:      "{}",
		AppliedAtUTC:     committedText,
	}); err != nil {
		return err
	}
	if relationship.ActiveReunionEpisodeID == episode.ID {
		relationship.ActiveReunionEpisodeID = ""
	}
	if input.ReacclimationTurns > relationship.ReacclimationTurnsRemaining {
		relationship.ReacclimationTurnsRemaining = input.ReacclimationTurns
	}
	return nil
}

func (r *RelationshipTimeRepository) ListReunionEpisodes(ctx context.Context, userID, characterID string, limit int) ([]ReunionEpisode, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if characterID != "" {
		query = query.Where("character_id = ?", characterID)
	}
	var episodes []ReunionEpisode
	err := query.Order("detected_at_utc DESC").Limit(limit).Find(&episodes).Error
	return episodes, err
}

func (r *RelationshipTimeRepository) ListEffectLedger(ctx context.Context, userID, characterID string, limit int) ([]TemporalEffectLedgerEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if characterID != "" {
		query = query.Where("character_id = ?", characterID)
	}
	var entries []TemporalEffectLedgerEntry
	err := query.Order("applied_at_utc DESC").Limit(limit).Find(&entries).Error
	return entries, err
}

func relationshipPresenceID(userID, characterID string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(userID+"\x00"+characterID)).String()
}

func isSessionBreak(previous string, current time.Time) bool {
	previousTime := ParseRelationshipTime(previous)
	return previousTime.IsZero() || current.Sub(previousTime) >= SessionBreakThreshold
}
