package psyche

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const StateVersionV1 = "psyche-state-v1"

var ErrVersionConflict = errors.New("psyche: version conflict detected")

type PsycheStore interface {
	SaveState(state *PsycheState) error
	LoadState(characterID string) (*PsycheState, error)
	SaveSnapshot(snapshot *PsycheSnapshot) error
	LoadSnapshots(characterID string, limit int) ([]PsycheSnapshot, error)
	AppendEvent(event *PsycheEvent) error
	GetForUpdateSnapshot(characterID string) (*PsycheState, error)
}

type InMemoryPsycheStore struct {
	mu        sync.RWMutex
	states    map[string]PsycheState
	snapshots map[string][]PsycheSnapshot
	events    []PsycheEvent
}

func NewInMemoryPsycheStore() *InMemoryPsycheStore {
	return &InMemoryPsycheStore{
		states:    make(map[string]PsycheState),
		snapshots: make(map[string][]PsycheSnapshot),
	}
}

func (s *InMemoryPsycheStore) SaveState(state *PsycheState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	state.StateVersion++
	s.states[state.CharacterID] = *state
	return nil
}

func (s *InMemoryPsycheStore) LoadState(characterID string) (*PsycheState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.states[characterID]
	if !ok {
		return nil, fmt.Errorf("state not found for character %s", characterID)
	}
	return &state, nil
}

func (s *InMemoryPsycheStore) SaveSnapshot(snapshot *PsycheSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots[snapshot.CharacterID] = append(s.snapshots[snapshot.CharacterID], *snapshot)
	return nil
}

func (s *InMemoryPsycheStore) LoadSnapshots(characterID string, limit int) ([]PsycheSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snaps, ok := s.snapshots[characterID]
	if !ok {
		return []PsycheSnapshot{}, nil
	}
	if limit > 0 && len(snaps) > limit {
		snaps = snaps[len(snaps)-limit:]
	}
	cp := make([]PsycheSnapshot, len(snaps))
	copy(cp, snaps)
	return cp, nil
}

func (s *InMemoryPsycheStore) AppendEvent(event *PsycheEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, *event)
	return nil
}

func (s *InMemoryPsycheStore) GetForUpdateSnapshot(characterID string) (*PsycheState, error) {
	return s.LoadState(characterID)
}

type SQLitePsycheStore struct {
	db *gorm.DB
}

func NewSQLitePsycheStore(db *gorm.DB) *SQLitePsycheStore {
	return &SQLitePsycheStore{db: db}
}

func (s *SQLitePsycheStore) SaveState(state *PsycheState) error {
	nextVersion := state.StateVersion + 1
	result := s.db.Model(&PsycheState{}).
		Where("character_id = ? AND state_version = ?", state.CharacterID, state.StateVersion).
		Updates(map[string]interface{}{
			"emotion":      state.Emotion,
			"mood":         state.Mood,
			"stress":       state.Stress,
			"energy":       state.Energy,
			"version":      StateVersionV1,
			"state_version": nextVersion,
			"updated_at":   time.Now().UTC(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var latest PsycheState
		if err := s.db.Where("character_id = ?", state.CharacterID).First(&latest).Error; err != nil {
			return fmt.Errorf("%w: failed to load latest state: %v", ErrVersionConflict, err)
		}
		state.StateVersion = latest.StateVersion
		state.Emotion = latest.Emotion
		state.Mood = latest.Mood
		state.Stress = latest.Stress
		state.Energy = latest.Energy
		state.UpdatedAt = latest.UpdatedAt
		return fmt.Errorf("%w: character %s version mismatch", ErrVersionConflict, state.CharacterID)
	}
	state.StateVersion = nextVersion
	state.Version = StateVersionV1
	return nil
}

func (s *SQLitePsycheStore) LoadState(characterID string) (*PsycheState, error) {
	var state PsycheState
	if err := s.db.Where("character_id = ?", characterID).First(&state).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("state not found for character %s", characterID)
		}
		return nil, err
	}
	return &state, nil
}

func (s *SQLitePsycheStore) SaveSnapshot(snapshot *PsycheSnapshot) error {
	if err := s.db.Create(snapshot).Error; err != nil {
		return err
	}
	return nil
}

func (s *SQLitePsycheStore) LoadSnapshots(characterID string, limit int) ([]PsycheSnapshot, error) {
	var snaps []PsycheSnapshot
	q := s.db.Where("character_id = ?", characterID).Order("timestamp desc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&snaps).Error; err != nil {
		return nil, err
	}
	return snaps, nil
}

func (s *SQLitePsycheStore) AppendEvent(event *PsycheEvent) error {
	if err := s.db.Create(event).Error; err != nil {
		return err
	}
	return nil
}

func (s *SQLitePsycheStore) GetForUpdateSnapshot(characterID string) (*PsycheState, error) {
	return s.LoadState(characterID)
}

func NewPsycheState(characterID string) PsycheState {
	now := time.Now().UTC()
	return PsycheState{
		CharacterID: characterID,
		Version:     StateVersionV1,
		StateVersion: 1,
		Emotion: EmotionDimensions{
			Valence:   0.5,
			Arousal:   0.5,
			Dominance: 0.5,
		},
		Mood: MoodDimensions{
			MoodValence: 0.5,
			MoodArousal: 0.5,
		},
		Stress:    0,
		Energy:    0.7,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func ApplyEvent(state PsycheState, event PsycheEvent) PsycheState {
	state.Emotion.Valence = clampSignedDelta(state.Emotion.Valence, event.ValenceDelta)
	state.Emotion.Arousal = clampSignedDelta(state.Emotion.Arousal, event.ArousalDelta)
	state.Emotion.Dominance = clampSignedDelta(state.Emotion.Dominance, event.DominanceDelta)

	moodTransfer := math.Abs(event.ValenceDelta) * 0.1
	if event.ValenceDelta > 0 {
		state.Mood.MoodValence = clampSignedDelta(state.Mood.MoodValence, moodTransfer)
	} else if event.ValenceDelta < 0 {
		state.Mood.MoodValence = clampSignedDelta(state.Mood.MoodValence, -moodTransfer)
	}

	moodArousalTransfer := math.Abs(event.ArousalDelta) * 0.08
	if event.ArousalDelta > 0 {
		state.Mood.MoodArousal = clampSignedDelta(state.Mood.MoodArousal, moodArousalTransfer)
	} else if event.ArousalDelta < 0 {
		state.Mood.MoodArousal = clampSignedDelta(state.Mood.MoodArousal, -moodArousalTransfer)
	}

	state.Stress = clampSignedDelta(state.Stress, event.StressDelta)
	state.Energy = clampSignedDelta(state.Energy, event.EnergyDelta)
	state.UpdatedAt = event.Timestamp

	return state
}

func CreateSnapshot(state PsycheState) PsycheSnapshot {
	return PsycheSnapshot{
		ID:               uuid.New().String(),
		CharacterID:      state.CharacterID,
		Version:          state.Version,
		Timestamp:        state.UpdatedAt,
		EmotionValence:   state.Emotion.Valence,
		EmotionArousal:   state.Emotion.Arousal,
		EmotionDominance: state.Emotion.Dominance,
		MoodValence:      state.Mood.MoodValence,
		MoodArousal:      state.Mood.MoodArousal,
		Stress:           state.Stress,
		Energy:           state.Energy,
	}
}

func clampSignedDelta(current, delta float64) float64 {
	result := current + delta
	if result < 0 {
		return 0
	}
	if result > 1 {
		return 1
	}
	return math.Round(result*10000) / 10000
}
