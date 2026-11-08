package gamemode

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	StateInactive    = "inactive"
	StateStarting    = "starting"
	StateActive      = "active"
	StatePaused      = "paused"
	StateStopping    = "stopping"
	StateUnknown     = "unknown"
	StateReconciling = "reconciling"
)

var (
	ErrOccupied = errors.New("game mode is occupied by another device")
	ErrNotOwner = errors.New("game mode session belongs to another device")
	ErrRevision = errors.New("game mode session revision changed")
	ErrInvalid  = errors.New("invalid game mode request")
	ErrState    = errors.New("invalid game mode state transition")
)

type Session struct {
	SpaceID         string     `gorm:"column:space_id;primaryKey;size:128" json:"spaceId"`
	SessionID       string     `gorm:"column:session_id;not null;size:128" json:"sessionId"`
	DeviceID        string     `gorm:"column:device_id;not null;size:128" json:"deviceId"`
	State           string     `gorm:"column:state;not null;size:32" json:"state"`
	GameExtensionID string     `gorm:"column:game_extension_id;not null;default:'';size:256" json:"gameExtensionId"`
	GamePluginID    string     `gorm:"column:game_plugin_id;not null;default:'';size:256" json:"gamePluginId"`
	GameTitle       string     `gorm:"column:game_title;not null;default:'';size:256" json:"gameTitle"`
	RuntimeID       string     `gorm:"column:runtime_id;not null;default:'';size:256" json:"runtimeId"`
	Revision        int64      `gorm:"column:revision;not null;default:1" json:"revision"`
	StartedAt       time.Time  `gorm:"column:started_at;not null" json:"startedAt"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;not null" json:"updatedAt"`
	ReleasedAt      *time.Time `gorm:"column:released_at" json:"releasedAt,omitempty"`
}

func (Session) TableName() string { return "kernel_game_mode_sessions" }

type AcquireInput struct {
	SpaceID         string
	DeviceID        string
	GameExtensionID string
	GamePluginID    string
	GameTitle       string
	RuntimeID       string
}

type UpdateInput struct {
	State            string
	GameTitle        string
	RuntimeID        string
	ExpectedRevision int64
}

type Service struct {
	db *gorm.DB
	mu sync.Mutex
}

func NewService(db *gorm.DB) (*Service, error) {
	if db == nil {
		return nil, errors.New("game mode database is required")
	}
	if err := db.AutoMigrate(&Session{}); err != nil {
		return nil, fmt.Errorf("migrate game mode session: %w", err)
	}
	return &Service{db: db}, nil
}

func (s *Service) Get(ctx context.Context, spaceID string) (Session, error) {
	if s == nil || s.db == nil {
		return Session{}, errors.New("game mode service is unavailable")
	}
	spaceID = strings.TrimSpace(spaceID)
	if spaceID == "" {
		return Session{}, fmt.Errorf("%w: spaceId is required", ErrInvalid)
	}
	var session Session
	err := s.db.WithContext(ctx).Where("space_id = ?", spaceID).First(&session).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Session{SpaceID: spaceID, State: StateInactive, Revision: 0}, nil
	}
	if err != nil {
		return Session{}, fmt.Errorf("read game mode session: %w", err)
	}
	return session, nil
}

func (s *Service) Acquire(ctx context.Context, input AcquireInput) (Session, error) {
	if s == nil || s.db == nil {
		return Session{}, errors.New("game mode service is unavailable")
	}
	spaceID := strings.TrimSpace(input.SpaceID)
	deviceID := strings.TrimSpace(input.DeviceID)
	if spaceID == "" || deviceID == "" {
		return Session{}, fmt.Errorf("%w: spaceId and deviceId are required", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	var result Session
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current Session
		err := tx.Where("space_id = ?", spaceID).First(&current).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("read current game mode: %w", err)
		}
		if err == nil && current.State != StateInactive && current.DeviceID != deviceID {
			return fmt.Errorf("%w: device=%s state=%s", ErrOccupied, current.DeviceID, current.State)
		}
		if err == nil && current.State != StateInactive && current.DeviceID == deviceID {
			result = current
			return nil
		}
		result = Session{
			SpaceID: spaceID, SessionID: "game_" + uuid.NewString(), DeviceID: deviceID,
			State: StateActive, GameExtensionID: strings.TrimSpace(input.GameExtensionID),
			GamePluginID: strings.TrimSpace(input.GamePluginID), GameTitle: strings.TrimSpace(input.GameTitle),
			RuntimeID: strings.TrimSpace(input.RuntimeID), Revision: 1, StartedAt: now, UpdatedAt: now,
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if createErr := tx.Create(&result).Error; createErr != nil {
				return fmt.Errorf("create game mode session: %w", createErr)
			}
			return nil
		}
		result.Revision = current.Revision + 1
		update := tx.Model(&Session{}).Where("space_id = ? AND revision = ?", spaceID, current.Revision).Updates(map[string]any{
			"session_id": result.SessionID, "device_id": result.DeviceID, "state": result.State,
			"game_extension_id": result.GameExtensionID, "game_plugin_id": result.GamePluginID,
			"game_title": result.GameTitle, "runtime_id": result.RuntimeID, "revision": result.Revision,
			"started_at": result.StartedAt, "updated_at": result.UpdatedAt, "released_at": nil,
		})
		if update.Error != nil {
			return fmt.Errorf("claim game mode session: %w", update.Error)
		}
		if update.RowsAffected != 1 {
			return ErrRevision
		}
		return nil
	})
	if err != nil {
		return Session{}, err
	}
	return result, nil
}

func (s *Service) Update(ctx context.Context, spaceID, deviceID, sessionID string, input UpdateInput) (Session, error) {
	if s == nil || s.db == nil {
		return Session{}, errors.New("game mode service is unavailable")
	}
	state := strings.TrimSpace(input.State)
	if !validState(state) {
		return Session{}, fmt.Errorf("%w: unsupported state %q", ErrState, state)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var current Session
	if err := s.db.WithContext(ctx).Where("space_id = ?", strings.TrimSpace(spaceID)).First(&current).Error; err != nil {
		return Session{}, fmt.Errorf("read game mode session: %w", err)
	}
	if current.State == StateInactive || current.SessionID != sessionID || current.DeviceID != deviceID {
		return Session{}, ErrNotOwner
	}
	if input.ExpectedRevision > 0 && current.Revision != input.ExpectedRevision {
		return Session{}, ErrRevision
	}
	if !validTransition(current.State, state) {
		return Session{}, fmt.Errorf("%w: %s -> %s", ErrState, current.State, state)
	}
	current.State = state
	if strings.TrimSpace(input.GameTitle) != "" {
		current.GameTitle = strings.TrimSpace(input.GameTitle)
	}
	if strings.TrimSpace(input.RuntimeID) != "" {
		current.RuntimeID = strings.TrimSpace(input.RuntimeID)
	}
	current.UpdatedAt = time.Now().UTC()
	current.Revision++
	update := s.db.WithContext(ctx).Model(&Session{}).Where("space_id = ? AND revision = ?", current.SpaceID, current.Revision-1).Updates(map[string]any{
		"state": current.State, "game_title": current.GameTitle, "runtime_id": current.RuntimeID,
		"updated_at": current.UpdatedAt, "revision": current.Revision,
	})
	if update.Error != nil {
		return Session{}, fmt.Errorf("update game mode session: %w", update.Error)
	}
	if update.RowsAffected != 1 {
		return Session{}, ErrRevision
	}
	return current, nil
}

func (s *Service) Release(ctx context.Context, spaceID, deviceID, sessionID string, expectedRevision int64, force bool) (Session, error) {
	if s == nil || s.db == nil {
		return Session{}, errors.New("game mode service is unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var current Session
	if err := s.db.WithContext(ctx).Where("space_id = ?", strings.TrimSpace(spaceID)).First(&current).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Session{SpaceID: spaceID, State: StateInactive}, nil
		}
		return Session{}, fmt.Errorf("read game mode session: %w", err)
	}
	if current.State == StateInactive {
		return current, nil
	}
	owner := current.SessionID == sessionID && current.DeviceID == deviceID
	if !force && !owner {
		return Session{}, ErrNotOwner
	}
	if force && expectedRevision > 0 && current.Revision != expectedRevision {
		return Session{}, ErrRevision
	}
	now := time.Now().UTC()
	current.State = StateInactive
	current.Revision++
	current.UpdatedAt = now
	current.ReleasedAt = &now
	update := s.db.WithContext(ctx).Model(&Session{}).Where("space_id = ? AND revision = ?", current.SpaceID, current.Revision-1).Updates(map[string]any{
		"state": current.State, "revision": current.Revision, "updated_at": now, "released_at": now,
	})
	if update.Error != nil {
		return Session{}, fmt.Errorf("release game mode session: %w", update.Error)
	}
	if update.RowsAffected != 1 {
		return Session{}, ErrRevision
	}
	return current, nil
}

func (s *Service) MarkUnknown(ctx context.Context, spaceID, deviceID string) (Session, error) {
	if s == nil || s.db == nil {
		return Session{}, errors.New("game mode service is unavailable")
	}
	spaceID = strings.TrimSpace(spaceID)
	deviceID = strings.TrimSpace(deviceID)
	if spaceID == "" || deviceID == "" {
		return Session{}, fmt.Errorf("%w: spaceId and deviceId are required", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var current Session
	if err := s.db.WithContext(ctx).Where("space_id = ?", spaceID).First(&current).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Session{SpaceID: spaceID, State: StateInactive, Revision: 0}, nil
		}
		return Session{}, fmt.Errorf("read game mode session: %w", err)
	}
	if current.State == StateInactive || current.DeviceID != deviceID || current.State == StateUnknown {
		return current, nil
	}
	now := time.Now().UTC()
	current.State = StateUnknown
	current.UpdatedAt = now
	current.Revision++
	update := s.db.WithContext(ctx).Model(&Session{}).Where("space_id = ? AND revision = ?", spaceID, current.Revision-1).Updates(map[string]any{
		"state": current.State, "updated_at": current.UpdatedAt, "revision": current.Revision,
	})
	if update.Error != nil {
		return Session{}, fmt.Errorf("mark game mode unknown: %w", update.Error)
	}
	if update.RowsAffected != 1 {
		return Session{}, ErrRevision
	}
	return current, nil
}

func (s *Service) MarkReady(ctx context.Context, spaceID, deviceID string) (Session, error) {
	if s == nil || s.db == nil {
		return Session{}, errors.New("game mode service is unavailable")
	}
	spaceID = strings.TrimSpace(spaceID)
	deviceID = strings.TrimSpace(deviceID)
	if spaceID == "" || deviceID == "" {
		return Session{}, fmt.Errorf("%w: spaceId and deviceId are required", ErrInvalid)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var current Session
	if err := s.db.WithContext(ctx).Where("space_id = ?", spaceID).First(&current).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Session{SpaceID: spaceID, State: StateInactive, Revision: 0}, nil
		}
		return Session{}, fmt.Errorf("read game mode session: %w", err)
	}
	if current.State != StateUnknown || current.DeviceID != deviceID {
		return current, nil
	}
	now := time.Now().UTC()
	current.State = StateReconciling
	current.UpdatedAt = now
	current.Revision++
	update := s.db.WithContext(ctx).Model(&Session{}).Where("space_id = ? AND revision = ?", spaceID, current.Revision-1).Updates(map[string]any{
		"state": current.State, "updated_at": current.UpdatedAt, "revision": current.Revision,
	})
	if update.Error != nil {
		return Session{}, fmt.Errorf("reconcile game mode session: %w", update.Error)
	}
	if update.RowsAffected != 1 {
		return Session{}, ErrRevision
	}
	return current, nil
}

func validState(state string) bool {
	switch state {
	case StateStarting, StateActive, StatePaused, StateStopping, StateUnknown, StateReconciling:
		return true
	default:
		return false
	}
}

func validTransition(from, to string) bool {
	if from == to {
		return true
	}
	switch from {
	case StateStarting:
		return to == StateActive || to == StateUnknown || to == StateStopping
	case StateActive:
		return to == StatePaused || to == StateUnknown || to == StateStopping
	case StatePaused:
		return to == StateActive || to == StateUnknown || to == StateStopping
	case StateUnknown:
		return to == StateReconciling || to == StateActive || to == StateStopping
	case StateReconciling:
		return to == StateActive || to == StatePaused || to == StateUnknown || to == StateStopping
	case StateStopping:
		return to == StateInactive
	default:
		return false
	}
}
