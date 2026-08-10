package interaction

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/mindruntime"
)

const (
	MaxEvidenceEvents    = 64
	MaxEvidenceRelations = 32
	MaxEvidenceMemories  = 64
	MaxSeenObservation   = 256

	SelectorMaxEvents    = 12
	SelectorMaxRelations = 8
	SelectorMaxMemories  = 12
)

type ReflectionEvidenceWindow struct {
	Events    []mindruntime.VerifiedEvent
	Relations []mindruntime.VerifiedRelation
	Memories  []mindruntime.VerifiedMemory
}

func NewReflectionEvidenceWindow() *ReflectionEvidenceWindow {
	return &ReflectionEvidenceWindow{
		Events:    make([]mindruntime.VerifiedEvent, 0, MaxEvidenceEvents),
		Relations: make([]mindruntime.VerifiedRelation, 0, MaxEvidenceRelations),
		Memories:  make([]mindruntime.VerifiedMemory, 0, MaxEvidenceMemories),
	}
}

func (w *ReflectionEvidenceWindow) AddEvents(events ...mindruntime.VerifiedEvent) int {
	added := 0
	for _, e := range events {
		if strings.TrimSpace(e.ID) == "" {
			continue
		}
		w.Events = append(w.Events, e)
		added++
	}
	if len(w.Events) > MaxEvidenceEvents {
		w.Events = w.Events[len(w.Events)-MaxEvidenceEvents:]
	}
	return added
}

func (w *ReflectionEvidenceWindow) AddRelations(relations ...mindruntime.VerifiedRelation) int {
	added := 0
	for _, r := range relations {
		if strings.TrimSpace(r.ID) == "" {
			continue
		}
		w.Relations = append(w.Relations, r)
		added++
	}
	if len(w.Relations) > MaxEvidenceRelations {
		w.Relations = w.Relations[len(w.Relations)-MaxEvidenceRelations:]
	}
	return added
}

func (w *ReflectionEvidenceWindow) AddMemories(memories ...mindruntime.VerifiedMemory) int {
	added := 0
	for _, m := range memories {
		if strings.TrimSpace(m.ID) == "" {
			continue
		}
		w.Memories = append(w.Memories, m)
		added++
	}
	if len(w.Memories) > MaxEvidenceMemories {
		w.Memories = w.Memories[len(w.Memories)-MaxEvidenceMemories:]
	}
	return added
}

func (w *ReflectionEvidenceWindow) ToEvidence() mindruntime.ReflectionEvidence {
	events := make([]mindruntime.VerifiedEvent, len(w.Events))
	copy(events, w.Events)
	relations := make([]mindruntime.VerifiedRelation, len(w.Relations))
	copy(relations, w.Relations)
	memories := make([]mindruntime.VerifiedMemory, len(w.Memories))
	copy(memories, w.Memories)
	return mindruntime.ReflectionEvidence{
		Events:    events,
		Relations: relations,
		Memories:  memories,
	}
}

type boundedIDSet struct {
	ids    []string
	maxLen int
}

func newBoundedIDSet(maxLen int) *boundedIDSet {
	return &boundedIDSet{
		ids:    make([]string, 0, maxLen),
		maxLen: maxLen,
	}
}

func (s *boundedIDSet) Contains(id string) bool {
	for _, existing := range s.ids {
		if existing == id {
			return true
		}
	}
	return false
}

func (s *boundedIDSet) Add(id string) bool {
	if s.Contains(id) {
		return false
	}
	s.ids = append(s.ids, id)
	if len(s.ids) > s.maxLen {
		s.ids = s.ids[len(s.ids)-s.maxLen:]
	}
	return true
}

func ObservationToVerifiedEvent(
	observation decision.Observation,
	progress decision.GoalProgressBatchResult,
	continuation decision.ContinuationDecision,
) (mindruntime.VerifiedEvent, bool) {
	if strings.TrimSpace(observation.ID) == "" {
		return mindruntime.VerifiedEvent{}, false
	}
	if observation.Kind == decision.ObservationKindNoAction {
		return mindruntime.VerifiedEvent{}, false
	}

	summary := buildStructuredSummary(observation, progress, continuation)
	tags := buildEventTags(observation, progress, continuation)
	importance := reflectionEventImportance(observation, progress)

	return mindruntime.VerifiedEvent{
		ID:         observation.ID,
		Kind:       string(observation.Kind),
		Summary:    summary,
		Timestamp:  observation.ObservedAt,
		Tags:       tags,
		Importance: importance,
	}, true
}

func buildStructuredSummary(
	observation decision.Observation,
	progress decision.GoalProgressBatchResult,
	continuation decision.ContinuationDecision,
) string {
	parts := make([]string, 0, 6)
	parts = append(parts, fmt.Sprintf("kind=%s", observation.Kind))
	parts = append(parts, fmt.Sprintf("outcome=%s", observation.Outcome))

	if observation.ToolID != "" {
		parts = append(parts, fmt.Sprintf("tool=%s", observation.ToolID))
	}
	if observation.Evidence.Error != nil && strings.TrimSpace(observation.Evidence.Error.Code) != "" {
		parts = append(parts, fmt.Sprintf("error=%s", observation.Evidence.Error.Code))
	}
	if len(progress.Results) > 0 {
		dispositions := make([]string, 0, len(progress.Results))
		for _, r := range progress.Results {
			dispositions = append(dispositions, string(r.Disposition))
		}
		parts = append(parts, fmt.Sprintf("goal_progress=%s", strings.Join(dispositions, ",")))
	}
	if continuation.Disposition != "" {
		parts = append(parts, fmt.Sprintf("continuation=%s", continuation.Disposition))
	}
	return strings.Join(parts, ";")
}

func buildEventTags(
	observation decision.Observation,
	progress decision.GoalProgressBatchResult,
	continuation decision.ContinuationDecision,
) []string {
	tags := make([]string, 0, 8)
	if observation.CharacterID != "" {
		tags = append(tags, "char:"+observation.CharacterID)
	}
	if observation.UserID != "" {
		tags = append(tags, "user:"+observation.UserID)
	}
	if observation.ConversationID != "" {
		tags = append(tags, "conv:"+observation.ConversationID)
	}
	for _, ref := range observation.GoalRefs {
		if strings.TrimSpace(ref.ID) != "" {
			tags = append(tags, "goal:"+ref.ID)
		}
	}
	for _, result := range progress.Results {
		if strings.TrimSpace(result.GoalID) != "" {
			tags = append(tags, "result:"+result.GoalID)
		}
	}
	if continuation.Disposition != "" {
		tags = append(tags, "continuation:"+string(continuation.Disposition))
	}
	sort.Strings(tags)
	return tags
}

func reflectionEventImportance(observation decision.Observation, progress decision.GoalProgressBatchResult) float64 {
	if observation.Kind == decision.ObservationKindNoAction {
		return 0
	}

	for _, result := range progress.Results {
		if result.Disposition == decision.GoalProgressAchieved {
			return 0.8
		}
	}

	switch observation.Outcome {
	case decision.ObservationOutcomeFailed,
		decision.ObservationOutcomeTimedOut,
		decision.ObservationOutcomeNotMaterialized,
		decision.ObservationOutcomeNotDispatched:
		return 0.6
	case decision.ObservationOutcomeCancelled:
		return 0.4
	case decision.ObservationOutcomeSucceeded:
		return 0.5
	}
	return 0.5
}

type ReflectionEvidenceSelector struct {
	MaxEvents    int
	MaxRelations int
	MaxMemories  int
}

func NewReflectionEvidenceSelector() *ReflectionEvidenceSelector {
	return &ReflectionEvidenceSelector{
		MaxEvents:    SelectorMaxEvents,
		MaxRelations: SelectorMaxRelations,
		MaxMemories:  SelectorMaxMemories,
	}
}

func (s *ReflectionEvidenceSelector) Select(window *ReflectionEvidenceWindow) mindruntime.ReflectionEvidence {
	events := make([]mindruntime.VerifiedEvent, len(window.Events))
	copy(events, window.Events)
	relations := make([]mindruntime.VerifiedRelation, len(window.Relations))
	copy(relations, window.Relations)
	memories := make([]mindruntime.VerifiedMemory, len(window.Memories))
	copy(memories, window.Memories)

	sort.SliceStable(events, func(i, j int) bool {
		if events[i].Importance != events[j].Importance {
			return events[i].Importance > events[j].Importance
		}
		if !events[i].Timestamp.Equal(events[j].Timestamp) {
			return events[i].Timestamp.After(events[j].Timestamp)
		}
		return events[i].ID < events[j].ID
	})

	sort.SliceStable(relations, func(i, j int) bool {
		if !relations[i].LastUpdated.Equal(relations[j].LastUpdated) {
			return relations[i].LastUpdated.After(relations[j].LastUpdated)
		}
		return relations[i].ID < relations[j].ID
	})

	sort.SliceStable(memories, func(i, j int) bool {
		if memories[i].Importance != memories[j].Importance {
			return memories[i].Importance > memories[j].Importance
		}
		if !memories[i].CreatedAt.Equal(memories[j].CreatedAt) {
			return memories[i].CreatedAt.After(memories[j].CreatedAt)
		}
		return memories[i].ID < memories[j].ID
	})

	if len(events) > s.MaxEvents {
		events = events[:s.MaxEvents]
	}
	if len(relations) > s.MaxRelations {
		relations = relations[:s.MaxRelations]
	}
	if len(memories) > s.MaxMemories {
		memories = memories[:s.MaxMemories]
	}

	return mindruntime.ReflectionEvidence{
		Events:    events,
		Relations: relations,
		Memories:  memories,
	}
}

type InteractionScopeKey struct {
	UserID         string
	CharacterID    string
	ConversationID string
	RequestID      string
}

func (k InteractionScopeKey) ToReflectionScopeKey() ReflectionScopeKey {
	return ReflectionScopeKey{
		UserID:         k.UserID,
		CharacterID:    k.CharacterID,
		ConversationID: k.ConversationID,
	}
}

type ReflectionScopeKey struct {
	UserID         string
	CharacterID    string
	ConversationID string
}

func (k ReflectionScopeKey) Normalize() ReflectionScopeKey {
	return ReflectionScopeKey{
		UserID:         strings.TrimSpace(k.UserID),
		CharacterID:    strings.TrimSpace(k.CharacterID),
		ConversationID: strings.TrimSpace(k.ConversationID),
	}
}

func (k ReflectionScopeKey) IsZero() bool {
	return k.UserID == "" && k.CharacterID == "" && k.ConversationID == ""
}

func (k ReflectionScopeKey) String() string {
	return fmt.Sprintf("%s|%s|%s", k.UserID, k.CharacterID, k.ConversationID)
}

func NewReflectionScopeKey(scope InteractionScope) ReflectionScopeKey {
	return ReflectionScopeKey{
		UserID:         strings.TrimSpace(scope.UserID),
		CharacterID:    strings.TrimSpace(scope.CharacterID),
		ConversationID: strings.TrimSpace(scope.ConversationID),
	}
}

type ReflectionExternalEvidence struct {
	Relations          []mindruntime.VerifiedRelation
	Memories           []mindruntime.VerifiedMemory
	RelationChangeCount int
	AnomalyScores      []float64
}

type ReflectionEvidenceReader interface {
	LoadVerifiedEvidence(
		ctx context.Context,
		scope ReflectionScopeKey,
		cutoff time.Time,
		limit int,
	) (ReflectionExternalEvidence, error)
}
