package temporal

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RelationshipTimeCoordinator struct {
	repo  *RelationshipTimeRepository
	clock Clock
}

func NewRelationshipTimeCoordinator(repo *RelationshipTimeRepository, clock Clock) *RelationshipTimeCoordinator {
	if clock == nil {
		clock = SystemClock{}
	}
	return &RelationshipTimeCoordinator{repo: repo, clock: clock}
}

func (c *RelationshipTimeCoordinator) PrepareInbound(ctx context.Context, input PrepareInboundInput) (result RelationshipTimeContext, err error) {
	if c == nil || c.repo == nil {
		return result, errors.New("relationship time repository is required")
	}
	input.UserID = strings.TrimSpace(input.UserID)
	input.CharacterID = strings.TrimSpace(input.CharacterID)
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.InteractionID = strings.TrimSpace(input.InteractionID)
	if input.UserID == "" || input.CharacterID == "" || input.InteractionID == "" {
		return result, errors.New("user id, character id and interaction id are required")
	}
	settings, settingsErr := c.repo.GetSettings(ctx, input.CharacterID)
	if settingsErr == nil && settings != nil && !settings.Enabled {
		return RelationshipTimeContext{Version: RelationshipTimeVersion, UserID: input.UserID, CharacterID: input.CharacterID}, nil
	}
	if input.RequestID == "" {
		input.RequestID = input.InteractionID
	}
	if input.ObservedAt.IsZero() {
		input.ObservedAt = c.clock.Now()
	}
	input.ObservedAt = input.ObservedAt.UTC()
	err = c.repo.WithTransaction(ctx, func(repo *RelationshipTimeRepository) error {
		receipt := &InteractionReceipt{
			ID:            uuid.NewString(),
			RequestID:     input.RequestID,
			InteractionID: input.InteractionID,
			UserID:        input.UserID,
			CharacterID:   input.CharacterID,
			Channel:       input.Channel,
			PeerID:        input.PeerID,
			ObservedAtUTC: FormatRelationshipTime(input.ObservedAt),
			Status:        InteractionReceiptObserved,
			CreatedAtUTC:  FormatRelationshipTime(input.ObservedAt),
			UpdatedAtUTC:  FormatRelationshipTime(input.ObservedAt),
		}
		created, createErr := repo.CreateReceipt(ctx, receipt)
		if createErr != nil {
			return createErr
		}
		if !created {
			existing, getErr := repo.GetReceipt(ctx, input.UserID, input.RequestID)
			if getErr != nil {
				return getErr
			}
			result, getErr = c.contextFromReceipt(ctx, repo, existing)
			return getErr
		}
		global, getErr := repo.GetGlobalPresence(ctx, input.UserID)
		if getErr != nil {
			return getErr
		}
		relationship, getErr := repo.GetRelationshipPresence(ctx, input.UserID, input.CharacterID)
		if getErr != nil {
			return getErr
		}
		previousGlobal := time.Time{}
		previousRelationship := time.Time{}
		if global != nil {
			previousGlobal = ParseRelationshipTime(global.LastCommittedUserInteractionAtUTC)
		}
		if relationship != nil {
			previousRelationship = ParseRelationshipTime(relationship.LastCommittedUserInteractionAtUTC)
		}
		if !input.IsInternal && !isProactiveSource(input.Source) {
			if saveErr := repo.SaveObservedPresence(ctx, ObservePresenceInput{UserID: input.UserID, CharacterID: input.CharacterID, Channel: input.Channel, ObservedAt: input.ObservedAt}); saveErr != nil {
				return saveErr
			}
		}
		cadence, cadenceErr := c.loadCadence(ctx, repo, input.UserID, input.CharacterID)
		if cadenceErr != nil {
			return cadenceErr
		}
		globalGap, _, _, globalDiagnostics := gapMetrics(input.ObservedAt, previousGlobal, cadence)
		relationshipGap, normalizedGap, deviation, relationshipDiagnostics := gapMetrics(input.ObservedAt, previousRelationship, cadence)
		result = c.baseContext(input.UserID, input.CharacterID, input.ObservedAt, global, relationship, globalGap, relationshipGap, normalizedGap, deviation, cadence)
		result.Diagnostics = append(globalDiagnostics, relationshipDiagnostics...)
		eligible := !previousRelationship.IsZero() && !input.IsInternal && !isProactiveSource(input.Source)
		if eligible && settings != nil && !settings.ReunionEnabled {
			eligible = false
		}
		level := reunionLevel(relationshipGap, normalizedGap)
		if eligible && level != ReunionLevelNone {
			lastAssistantContact := time.Time{}
			if relationship != nil {
				lastAssistantContact = ParseRelationshipTime(relationship.LastAssistantContactAtUTC)
			}
			kind := classifyReunionKind(globalGap, cadence.ExpectedGap.Seconds(), level, lastAssistantContact, input.ObservedAt)
			policy := DefaultRelationshipTimePolicy(level)
			policy = applySettingsToPolicy(policy, settings)
			shouldExpress := kind != ReunionKindReplyToProactive
			if !shouldExpress {
				policy.MentionMode = ReunionMentionNone
				policy.SuppressionReason = "recent_proactive_contact"
			}
			policyJSON, _ := json.Marshal(policy)
			episode, _, episodeErr := repo.CreateOrGetReunionEpisode(ctx, &ReunionEpisode{
				ID:                                   uuid.NewString(),
				UserID:                               input.UserID,
				CharacterID:                          input.CharacterID,
				ReunionKind:                          kind,
				ReunionLevel:                         level,
				Status:                               ReunionStatePending,
				PreviousRelationshipInteractionAtUTC: FormatRelationshipTime(previousRelationship),
				PreviousGlobalInteractionAtUTC:       FormatRelationshipTime(previousGlobal),
				DetectedAtUTC:                        FormatRelationshipTime(input.ObservedAt),
				RelationshipGapSeconds:               relationshipGap,
				GlobalGapSeconds:                     globalGap,
				ExpectedGapSeconds:                   cadence.ExpectedGap.Seconds(),
				NormalizedGap:                        normalizedGap,
				DeviationScore:                       deviation,
				ContinuityBefore:                     result.ContinuityScore,
				PolicyJSON:                           string(policyJSON),
				IdempotencyKey:                       reunionIdempotencyKey(input.UserID, input.CharacterID, FormatRelationshipTime(previousRelationship)),
				CreatedAtUTC:                         FormatRelationshipTime(input.ObservedAt),
				UpdatedAtUTC:                         FormatRelationshipTime(input.ObservedAt),
			})
			if episodeErr != nil {
				return episodeErr
			}
			claimed, didClaim, claimErr := repo.ClaimReunionEpisode(ctx, episode.ID, input.InteractionID)
			if claimErr != nil {
				return claimErr
			}
			if claimed != nil {
				episode = claimed
			}
			result.Reunion = reunionContextFromEpisode(episode, shouldExpress && didClaim)
			if didClaim {
				if updateErr := repo.db.WithContext(ctx).Model(&RelationshipPresenceState{}).Where("user_id = ? AND character_id = ?", input.UserID, input.CharacterID).Update("active_reunion_episode_id", episode.ID).Error; updateErr != nil {
					return updateErr
				}
			}
		}
		updateFields := map[string]interface{}{
			"previous_global_committed_at_utc":       FormatRelationshipTime(previousGlobal),
			"previous_relationship_committed_at_utc": FormatRelationshipTime(previousRelationship),
		}
		if result.Reunion != nil {
			updateFields["reunion_episode_id"] = result.Reunion.EpisodeID
		}
		if updateErr := repo.db.WithContext(ctx).Model(&InteractionReceipt{}).Where("user_id = ? AND request_id = ?", input.UserID, input.RequestID).Updates(updateFields).Error; updateErr != nil {
			return updateErr
		}
		return nil
	})
	return result, err
}

func (c *RelationshipTimeCoordinator) ReleaseClaim(ctx context.Context, interactionID, reason string) error {
	if c == nil || c.repo == nil {
		return errors.New("relationship time repository is required")
	}
	return c.repo.ReleaseClaim(ctx, interactionID, reason)
}

func (c *RelationshipTimeCoordinator) Resolve(ctx context.Context, input SnapshotInput, nowUTC time.Time) (*RelationshipTimeContext, error) {
	if c == nil || c.repo == nil || strings.TrimSpace(input.UserID) == "" || strings.TrimSpace(input.CharacterID) == "" {
		return nil, nil
	}
	if nowUTC.IsZero() {
		nowUTC = c.clock.Now()
	}
	global, err := c.repo.GetGlobalPresence(ctx, input.UserID)
	if err != nil {
		return nil, err
	}
	relationship, err := c.repo.GetRelationshipPresence(ctx, input.UserID, input.CharacterID)
	if err != nil {
		return nil, err
	}
	cadence, err := c.loadCadence(ctx, c.repo, input.UserID, input.CharacterID)
	if err != nil {
		return nil, err
	}
	previousGlobal := time.Time{}
	previousRelationship := time.Time{}
	if global != nil {
		previousGlobal = ParseRelationshipTime(global.LastCommittedUserInteractionAtUTC)
	}
	if relationship != nil {
		previousRelationship = ParseRelationshipTime(relationship.LastCommittedUserInteractionAtUTC)
	}
	globalGap, _, _, globalDiagnostics := gapMetrics(nowUTC, previousGlobal, cadence)
	relationshipGap, normalized, deviation, relationshipDiagnostics := gapMetrics(nowUTC, previousRelationship, cadence)
	result := c.baseContext(input.UserID, input.CharacterID, nowUTC, global, relationship, globalGap, relationshipGap, normalized, deviation, cadence)
	result.Diagnostics = append(globalDiagnostics, relationshipDiagnostics...)
	if relationship != nil && relationship.ActiveReunionEpisodeID != "" {
		episode, getErr := c.repo.GetReunionEpisode(ctx, relationship.ActiveReunionEpisodeID)
		if getErr != nil {
			return nil, getErr
		}
		if episode != nil && (episode.Status == ReunionStatePending || episode.Status == ReunionStateClaimed) {
			result.Reunion = reunionContextFromEpisode(episode, episode.Status == ReunionStateClaimed)
		}
	}
	return &result, nil
}

func (c *RelationshipTimeCoordinator) FinalizeInteractionTx(ctx context.Context, tx *gorm.DB, input FinalizeInteractionInput) error {
	if c == nil || c.repo == nil {
		return errors.New("relationship time repository is required")
	}
	return c.repo.FinalizeInteractionTx(ctx, tx, input)
}

func (c *RelationshipTimeCoordinator) GetSettings(ctx context.Context, characterID string) (*RelationshipTimeSettings, error) {
	if c == nil || c.repo == nil {
		return nil, errors.New("relationship time repository is required")
	}
	return c.repo.GetSettings(ctx, characterID)
}

func (c *RelationshipTimeCoordinator) SaveSettings(ctx context.Context, settings *RelationshipTimeSettings) error {
	if c == nil || c.repo == nil {
		return errors.New("relationship time repository is required")
	}
	return c.repo.SaveSettings(ctx, settings)
}

func (c *RelationshipTimeCoordinator) GetPresenceState(ctx context.Context, userID, characterID string) (*RelationshipPresenceState, error) {
	if c == nil || c.repo == nil {
		return nil, errors.New("relationship time repository is required")
	}
	return c.repo.GetRelationshipPresence(ctx, userID, characterID)
}

func (c *RelationshipTimeCoordinator) ListReunionEpisodes(ctx context.Context, userID, characterID string, limit int) ([]ReunionEpisode, error) {
	if c == nil || c.repo == nil {
		return nil, errors.New("relationship time repository is required")
	}
	return c.repo.ListReunionEpisodes(ctx, userID, characterID, limit)
}

func (c *RelationshipTimeCoordinator) GetReunionEpisode(ctx context.Context, episodeID string) (*ReunionEpisode, error) {
	if c == nil || c.repo == nil {
		return nil, errors.New("relationship time repository is required")
	}
	return c.repo.GetReunionEpisode(ctx, episodeID)
}

func (c *RelationshipTimeCoordinator) GetState(ctx context.Context, userID, characterID string) (*RelationshipTimeContext, error) {
	if c == nil || c.repo == nil {
		return nil, errors.New("relationship time repository is required")
	}
	result, err := c.Resolve(ctx, SnapshotInput{UserID: userID, CharacterID: characterID}, c.clock.Now())
	return result, err
}

func (c *RelationshipTimeCoordinator) RecordAssistantContact(ctx context.Context, userID, characterID string, at time.Time) error {
	if c == nil || c.repo == nil {
		return errors.New("relationship time repository is required")
	}
	if strings.TrimSpace(userID) == "" {
		userID = "default"
	}
	if strings.TrimSpace(characterID) == "" {
		return errors.New("character id is required")
	}
	if at.IsZero() {
		at = c.clock.Now()
	}
	return c.repo.RecordAssistantContact(ctx, userID, characterID, at.UTC())
}

func (c *RelationshipTimeCoordinator) FinalizeCommittedTx(ctx context.Context, tx *gorm.DB, userID, characterID, interactionID string, relationshipTime *RelationshipTimeContext, suppress bool, reason string, assistantInitiated bool) error {
	if c == nil || c.repo == nil {
		return errors.New("relationship time repository is required")
	}
	repo := c.repo
	if tx != nil {
		repo = repo.WithDB(tx)
	}
	receipt, err := repo.GetReceiptByInteraction(ctx, interactionID)
	if err != nil || receipt == nil || receipt.Status == InteractionReceiptCommitted {
		return err
	}
	committedAt := c.clock.Now().UTC()
	previousRelationship := ParseRelationshipTime(receipt.PreviousRelationshipCommittedAtUTC)
	previousGlobal := ParseRelationshipTime(receipt.PreviousGlobalCommittedAtUTC)
	var relationshipSample *CadenceSample
	if gap := committedAt.Sub(previousRelationship); !previousRelationship.IsZero() && gap >= SessionBreakThreshold {
		relationshipSample = cadenceSample(userID, characterID, interactionID, "relationship", previousRelationship, committedAt)
	}
	if gap := committedAt.Sub(previousGlobal); !previousGlobal.IsZero() && gap >= SessionBreakThreshold {
		if _, err := repo.AddCadenceSample(ctx, cadenceSample(userID, "", interactionID, "global", previousGlobal, committedAt)); err != nil {
			return err
		}
	}
	relationshipSamples, err := repo.ListCadenceSamples(ctx, userID, characterID, "relationship", MaximumCadenceSamples)
	if err != nil {
		return err
	}
	if relationshipSample != nil {
		relationshipSamples = append(relationshipSamples, *relationshipSample)
	}
	globalSamples, err := repo.ListCadenceSamples(ctx, userID, "", "global", MaximumCadenceSamples)
	if err != nil {
		return err
	}
	cadence := selectCadence(relationshipSamples, globalSamples)
	input := FinalizeInteractionInput{UserID: userID, CharacterID: characterID, InteractionID: interactionID, CommittedAt: committedAt, ExpectedGapSeconds: cadence.ExpectedGap.Seconds(), GapMADSeconds: cadence.MAD.Seconds(), CadenceSample: relationshipSample, SuppressReunion: suppress, SuppressionReason: reason, AssistantInitiated: assistantInitiated}
	if relationshipTime != nil && relationshipTime.Reunion != nil {
		input.ReunionEpisodeID = relationshipTime.Reunion.EpisodeID
		input.ReacclimationTurns = reacclimationTurns(relationshipTime.Reunion.Level)
		if !relationshipTime.Reunion.ShouldExpress {
			input.SuppressReunion = true
			if input.SuppressionReason == "" {
				input.SuppressionReason = "policy_suppressed"
			}
		}
	}
	return repo.FinalizeInteractionTx(ctx, tx, input)
}

func (c *RelationshipTimeCoordinator) loadCadence(ctx context.Context, repo *RelationshipTimeRepository, userID, characterID string) (cadenceEstimate, error) {
	relationship, err := repo.ListCadenceSamples(ctx, userID, characterID, "relationship", MaximumCadenceSamples)
	if err != nil {
		return cadenceEstimate{}, err
	}
	global, err := repo.ListCadenceSamples(ctx, userID, "", "global", MaximumCadenceSamples)
	if err != nil {
		return cadenceEstimate{}, err
	}
	return selectCadence(relationship, global), nil
}

func (c *RelationshipTimeCoordinator) contextFromReceipt(ctx context.Context, repo *RelationshipTimeRepository, receipt *InteractionReceipt) (RelationshipTimeContext, error) {
	global, err := repo.GetGlobalPresence(ctx, receipt.UserID)
	if err != nil {
		return RelationshipTimeContext{}, err
	}
	relationship, err := repo.GetRelationshipPresence(ctx, receipt.UserID, receipt.CharacterID)
	if err != nil {
		return RelationshipTimeContext{}, err
	}
	cadence, err := c.loadCadence(ctx, repo, receipt.UserID, receipt.CharacterID)
	if err != nil {
		return RelationshipTimeContext{}, err
	}
	now := ParseRelationshipTime(receipt.ObservedAtUTC)
	globalGap, _, _, globalDiagnostics := gapMetrics(now, ParseRelationshipTime(receipt.PreviousGlobalCommittedAtUTC), cadence)
	relationshipGap, normalized, deviation, relationshipDiagnostics := gapMetrics(now, ParseRelationshipTime(receipt.PreviousRelationshipCommittedAtUTC), cadence)
	result := c.baseContext(receipt.UserID, receipt.CharacterID, now, global, relationship, globalGap, relationshipGap, normalized, deviation, cadence)
	result.Diagnostics = append(globalDiagnostics, relationshipDiagnostics...)
	if receipt.ReunionEpisodeID != "" {
		episode, getErr := repo.GetReunionEpisode(ctx, receipt.ReunionEpisodeID)
		if getErr != nil {
			return RelationshipTimeContext{}, getErr
		}
		if episode != nil {
			result.Reunion = reunionContextFromEpisode(episode, episode.ClaimInteractionID == receipt.InteractionID)
		}
	}
	return result, nil
}

func (c *RelationshipTimeCoordinator) baseContext(userID, characterID string, now time.Time, global *GlobalPresenceState, relationship *RelationshipPresenceState, globalGap, relationshipGap, normalized, deviation float64, cadence cadenceEstimate) RelationshipTimeContext {
	result := RelationshipTimeContext{Version: RelationshipTimeVersion, UserID: userID, CharacterID: characterID, NowUTC: now.UTC(), GlobalGapSeconds: globalGap, RelationshipGapSeconds: relationshipGap, ExpectedGapSeconds: roundFinite(cadence.ExpectedGap.Seconds()), GapDeviationScore: deviation, NormalizedGap: normalized, ContinuityScore: continuityScore(relationshipGap, cadence.ExpectedGap.Seconds())}
	if global != nil {
		result.GlobalLastCommittedAt = ParseRelationshipTime(global.LastCommittedUserInteractionAtUTC)
	}
	if relationship != nil {
		result.FirstInteractionAt = ParseRelationshipTime(relationship.FirstInteractionAtUTC)
		result.RelationshipLastCommittedAt = ParseRelationshipTime(relationship.LastCommittedUserInteractionAtUTC)
		result.LastSuccessfulExchangeAt = ParseRelationshipTime(relationship.LastSuccessfulExchangeAtUTC)
		result.LastAssistantContactAt = ParseRelationshipTime(relationship.LastAssistantContactAtUTC)
		result.InteractionCount = relationship.InteractionCount
		result.SessionCount = relationship.SessionCount
		result.ReacclimationTurnsLeft = relationship.ReacclimationTurnsRemaining
		result.StoredTension = 0
		result.EffectiveTension = result.StoredTension
		result.HasRecentAssistantContact = !result.LastAssistantContactAt.IsZero() && now.Sub(result.LastAssistantContactAt) >= 0 && now.Sub(result.LastAssistantContactAt) <= 72*time.Hour
		if !result.FirstInteractionAt.IsZero() && now.After(result.FirstInteractionAt) {
			result.RelationshipAgeDays = int(now.Sub(result.FirstInteractionAt).Hours() / 24)
		}
	}
	return result
}

func reunionContextFromEpisode(episode *ReunionEpisode, shouldExpress bool) *ReunionContext {
	if episode == nil {
		return nil
	}
	return &ReunionContext{EpisodeID: episode.ID, Kind: episode.ReunionKind, Level: episode.ReunionLevel, State: episode.Status, RelationshipGapSeconds: episode.RelationshipGapSeconds, GlobalGapSeconds: episode.GlobalGapSeconds, ExpectedGapSeconds: episode.ExpectedGapSeconds, NormalizedGap: episode.NormalizedGap, ClaimedByInteractionID: episode.ClaimInteractionID, ClaimExpiresAt: ParseRelationshipTime(episode.ClaimExpiresAtUTC), ShouldExpress: shouldExpress && episode.Status == ReunionStateClaimed}
}

func cadenceSample(userID, characterID, interactionID, kind string, previous, current time.Time) *CadenceSample {
	return &CadenceSample{ID: uuid.NewString(), UserID: userID, CharacterID: characterID, InteractionID: interactionID, PreviousInteractionAtUTC: FormatRelationshipTime(previous), CurrentInteractionAtUTC: FormatRelationshipTime(current), GapSeconds: roundFinite(current.Sub(previous).Seconds()), SampleKind: kind, Included: true, CreatedAtUTC: FormatRelationshipTime(current)}
}

func isProactiveSource(source string) bool {
	value := strings.ToLower(strings.TrimSpace(source))
	return strings.Contains(value, "proactive") || strings.Contains(value, "active_message") || strings.Contains(value, "internal")
}

func applySettingsToPolicy(policy RelationshipTimePolicy, settings *RelationshipTimeSettings) RelationshipTimePolicy {
	if settings == nil {
		return policy
	}
	if !settings.AllowReunionMention {
		policy.MentionMode = ReunionMentionNone
		policy.MaxMentionSentences = 0
		return policy
	}
	if !settings.AllowMemoryRecall {
		policy.MemoryRecallBudget = 0
		policy.RestorePreviousTopic = false
	}
	if !settings.AllowRelationshipAge {
		policy.UseRelationshipAge = false
	}
	if !settings.AllowProactiveReference {
		policy.RestorePreviousTopic = false
	}
	if settings.MaxMentionSentences > 0 && settings.MaxMentionSentences < policy.MaxMentionSentences {
		policy.MaxMentionSentences = settings.MaxMentionSentences
	}
	switch settings.Sensitivity {
	case "conservative":
		if policy.MaxMentionSentences > 0 {
			policy.MaxMentionSentences = 0
			policy.MentionMode = ReunionMentionNone
			policy.MemoryRecallBudget = 0
		}
	case "expressive":
		if policy.MentionMode != ReunionMentionNone && policy.MaxMentionSentences < 2 {
			policy.MaxMentionSentences = 2
		}
	}
	return policy
}
