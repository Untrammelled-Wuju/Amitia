package psyche

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func StateVersionV1() string { return fmt.Sprintf("psyche-state-%d", time.Now().UnixNano()) }

var ErrVersionConflict = errors.New("psyche: version conflict detected")
var ErrStateNotFound = errors.New("psyche: state not found")

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

type psycheStateRecord struct {
	CharacterID  string    `gorm:"primaryKey;column:character_id"`
	Version      string    `gorm:"column:version"`
	StateVersion int       `gorm:"column:state_version;default:0"`
	Emotion      string    `gorm:"column:emotion"`
	Mood         string    `gorm:"column:mood"`
	Stress       float64   `gorm:"column:stress"`
	Energy       float64   `gorm:"column:energy"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

type psycheEventRecord struct {
	ID          string    `gorm:"primaryKey;column:id"`
	CharacterID string    `gorm:"column:character_id"`
	EventType   EventType `gorm:"column:event_type"`
	EventData   string    `gorm:"column:event_data"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

type psycheSnapshotRecord struct {
	ID           string    `gorm:"primaryKey;column:id"`
	CharacterID  string    `gorm:"column:character_id"`
	SnapshotData string    `gorm:"column:snapshot_data"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

func (psycheStateRecord) TableName() string {
	return "psyche_states"
}

func (psycheEventRecord) TableName() string {
	return "psyche_events"
}

func (psycheSnapshotRecord) TableName() string {
	return "psyche_snapshots"
}

func NewSQLitePsycheStore(db *gorm.DB) *SQLitePsycheStore {
	return &SQLitePsycheStore{db: db}
}

func (s *SQLitePsycheStore) WithDB(db *gorm.DB) *SQLitePsycheStore {
	return &SQLitePsycheStore{db: db}
}

func (s *SQLitePsycheStore) InitSchema() error {
	return s.db.AutoMigrate(&psycheStateRecord{}, &psycheEventRecord{}, &psycheSnapshotRecord{})
}

func (s *SQLitePsycheStore) SaveState(state *PsycheState) error {
	if state.CharacterID == "" {
		return fmt.Errorf("psyche: character id is required")
	}
	if state.Version == "" {
		state.Version = StateVersionV1()
	}
	if state.StateVersion < 1 {
		state.StateVersion = 1
	}
	now := time.Now().UTC()
	if state.CreatedAt.IsZero() {
		state.CreatedAt = now
	}
	nextVersion := state.StateVersion + 1
	nextVersionID := StateVersionV1()
	emotionJSON, _ := json.Marshal(state.Emotion)
	moodJSON, _ := json.Marshal(state.Mood)
	result := s.db.Model(&psycheStateRecord{}).
		Where("character_id = ? AND state_version = ?", state.CharacterID, state.StateVersion).
		Updates(map[string]interface{}{
			"emotion":       string(emotionJSON),
			"mood":          string(moodJSON),
			"stress":        state.Stress,
			"energy":        state.Energy,
			"version":       nextVersionID,
			"state_version": nextVersion,
			"updated_at":    now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var latest psycheStateRecord
		err := s.db.Where("character_id = ?", state.CharacterID).First(&latest).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			record := psycheStateRecord{
				CharacterID:  state.CharacterID,
				Version:      state.Version,
				StateVersion: state.StateVersion,
				Emotion:      string(emotionJSON),
				Mood:         string(moodJSON),
				Stress:       state.Stress,
				Energy:       state.Energy,
				CreatedAt:    state.CreatedAt,
				UpdatedAt:    now,
			}
			if err := s.db.Create(&record).Error; err != nil {
				return err
			}
			state.UpdatedAt = now
			return nil
		}
		if err != nil {
			return err
		}
		state.StateVersion = latest.StateVersion
		state.Version = latest.Version
		state.Stress = latest.Stress
		state.Energy = latest.Energy
		state.UpdatedAt = latest.UpdatedAt
		return fmt.Errorf("%w: character %s version mismatch", ErrVersionConflict, state.CharacterID)
	}
	state.StateVersion = nextVersion
	state.Version = nextVersionID
	state.UpdatedAt = now
	return nil
}

func (s *SQLitePsycheStore) LoadState(characterID string) (*PsycheState, error) {
	var raw psycheStateRecord
	if err := s.db.Where("character_id = ?", characterID).Take(&raw).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: character %s", ErrStateNotFound, characterID)
		}
		return nil, err
	}
	state := &PsycheState{
		CharacterID:  raw.CharacterID,
		Version:      raw.Version,
		StateVersion: raw.StateVersion,
		Stress:       raw.Stress,
		Energy:       raw.Energy,
		CreatedAt:    raw.CreatedAt,
		UpdatedAt:    raw.UpdatedAt,
	}
	if raw.Emotion != "" {
		json.Unmarshal([]byte(raw.Emotion), &state.Emotion)
	}
	if raw.Mood != "" {
		json.Unmarshal([]byte(raw.Mood), &state.Mood)
	}
	return state, nil
}

func (s *SQLitePsycheStore) SaveSnapshot(snapshot *PsycheSnapshot) error {
	if snapshot.ID == "" {
		snapshot.ID = uuid.New().String()
	}
	if snapshot.Timestamp.IsZero() {
		snapshot.Timestamp = time.Now().UTC()
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	record := psycheSnapshotRecord{
		ID:           snapshot.ID,
		CharacterID:  snapshot.CharacterID,
		SnapshotData: string(raw),
		CreatedAt:    snapshot.Timestamp,
	}
	if err := s.db.Create(&record).Error; err != nil {
		return err
	}
	return nil
}

func (s *SQLitePsycheStore) LoadSnapshots(characterID string, limit int) ([]PsycheSnapshot, error) {
	var records []psycheSnapshotRecord
	q := s.db.Where("character_id = ?", characterID).Order("created_at desc")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&records).Error; err != nil {
		return nil, err
	}
	snaps := make([]PsycheSnapshot, 0, len(records))
	for _, record := range records {
		var snap PsycheSnapshot
		if record.SnapshotData != "" {
			if err := json.Unmarshal([]byte(record.SnapshotData), &snap); err != nil {
				return nil, err
			}
		}
		if snap.ID == "" {
			snap.ID = record.ID
		}
		if snap.CharacterID == "" {
			snap.CharacterID = record.CharacterID
		}
		if snap.Timestamp.IsZero() {
			snap.Timestamp = record.CreatedAt
		}
		snaps = append(snaps, snap)
	}
	return snaps, nil
}

func (s *SQLitePsycheStore) AppendEvent(event *PsycheEvent) error {
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	raw, err := json.Marshal(event)
	if err != nil {
		return err
	}
	record := psycheEventRecord{
		ID:          event.ID,
		CharacterID: event.CharacterID,
		EventType:   event.Type,
		EventData:   string(raw),
		CreatedAt:   event.Timestamp,
	}
	if err := s.db.Create(&record).Error; err != nil {
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
		CharacterID:  characterID,
		Version:      StateVersionV1(),
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
