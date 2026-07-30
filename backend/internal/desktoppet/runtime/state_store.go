// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only

package runtime

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

type RuntimeClient struct {
	RuntimeID             string `gorm:"column:runtime_id;primaryKey;type:text"`
	DeviceID              string `gorm:"column:device_id;type:text"`
	UserID                string `gorm:"column:user_id;type:text"`
	DisplayName           string `gorm:"column:display_name;type:text"`
	Platform              string `gorm:"column:platform;type:text"`
	Arch                  string `gorm:"column:arch;type:text"`
	AppVersion            string `gorm:"column:app_version;type:text"`
	ProtocolVersion       string `gorm:"column:protocol_version;type:text"`
	CapabilitiesJSON      string `gorm:"column:capabilities_json;type:text"`
	LastProcessInstanceID string `gorm:"column:last_process_instance_id;type:text"`
	LastSessionID         string `gorm:"column:last_session_id;type:text"`
	LastSeenAt            string `gorm:"column:last_seen_at;type:text"`
	LastConnectedAt       string `gorm:"column:last_connected_at;type:text"`
	LastDisconnectedAt    string `gorm:"column:last_disconnected_at;type:text"`
	CreatedAt             string `gorm:"column:created_at;type:text"`
	UpdatedAt             string `gorm:"column:updated_at;type:text"`
}

func (RuntimeClient) TableName() string { return "desktop_pet_runtime_clients" }

type RuntimeActualState struct {
	RuntimeID               string  `gorm:"column:runtime_id;primaryKey;type:text"`
	InstallationID          string  `gorm:"column:installation_id;primaryKey;type:text"`
	PetInstanceID           string  `gorm:"column:pet_instance_id;type:text"`
	SessionID               string  `gorm:"column:session_id;type:text"`
	DesiredRevision         int64   `gorm:"column:desired_revision;type:integer"`
	AppliedSettingsRevision int64   `gorm:"column:applied_settings_revision;type:integer"`
	Visible                 int     `gorm:"column:visible;type:integer"`
	CurrentActionKey        string  `gorm:"column:current_action_key;type:text"`
	PositionX               int     `gorm:"column:position_x;type:integer"`
	PositionY               int     `gorm:"column:position_y;type:integer"`
	ScreenID                string  `gorm:"column:screen_id;type:text"`
	Scale                   float64 `gorm:"column:scale;type:real"`
	Health                  string  `gorm:"column:health;type:text"`
	StateJSON               string  `gorm:"column:state_json;type:text"`
	ObservedAt              string  `gorm:"column:observed_at;type:text"`
	UpdatedAt               string  `gorm:"column:updated_at;type:text"`
}

func (RuntimeActualState) TableName() string { return "desktop_pet_runtime_actual_states" }

type StateStore interface {
	UpsertClient(client *RuntimeClient) error
	GetClient(runtimeID string) (*RuntimeClient, error)
	UpdateClientSession(runtimeID, sessionID, processInstanceID string) error
	MarkClientDisconnected(runtimeID string) error
	ListClientsByUser(userID string) ([]*RuntimeClient, error)

	UpsertActualState(state *RuntimeActualState) error
	GetActualState(runtimeID, installationID string) (*RuntimeActualState, error)
	ListActualStatesByRuntime(runtimeID string) ([]*RuntimeActualState, error)
	DeleteActualState(runtimeID, installationID string) error
	UpdateActualStateHealth(runtimeID, health string) error

	DB() *gorm.DB
}

type stateStore struct {
	db *gorm.DB
}

func NewStateStore(db *gorm.DB) StateStore {
	return &stateStore{db: db}
}

func (s *stateStore) DB() *gorm.DB { return s.db }

func (s *stateStore) UpsertClient(client *RuntimeClient) error {
	now := time.Now().Format(runtimeTimeFormat)
	var existing RuntimeClient
	err := s.db.Where("runtime_id = ?", client.RuntimeID).First(&existing).Error
	if err == nil {
		client.CreatedAt = existing.CreatedAt
		client.UpdatedAt = now
		return s.db.Save(client).Error
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if client.CreatedAt == "" {
			client.CreatedAt = now
		}
		client.UpdatedAt = now
		return s.db.Create(client).Error
	}
	return err
}

func (s *stateStore) GetClient(runtimeID string) (*RuntimeClient, error) {
	var client RuntimeClient
	err := s.db.Where("runtime_id = ?", runtimeID).First(&client).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRuntimeOffline
		}
		return nil, err
	}
	return &client, nil
}

func (s *stateStore) UpdateClientSession(runtimeID, sessionID, processInstanceID string) error {
	now := time.Now().Format(runtimeTimeFormat)
	return s.db.Model(&RuntimeClient{}).Where("runtime_id = ?", runtimeID).
		Updates(map[string]interface{}{
			"last_session_id":          sessionID,
			"last_process_instance_id": processInstanceID,
			"last_connected_at":        now,
			"updated_at":               now,
		}).Error
}

func (s *stateStore) MarkClientDisconnected(runtimeID string) error {
	now := time.Now().Format(runtimeTimeFormat)
	return s.db.Model(&RuntimeClient{}).Where("runtime_id = ?", runtimeID).
		Updates(map[string]interface{}{
			"last_disconnected_at": now,
			"updated_at":           now,
		}).Error
}

func (s *stateStore) ListClientsByUser(userID string) ([]*RuntimeClient, error) {
	var clients []*RuntimeClient
	err := s.db.Where("user_id = ?", userID).
		Order("last_seen_at DESC").
		Find(&clients).Error
	if clients == nil {
		clients = []*RuntimeClient{}
	}
	return clients, err
}

func (s *stateStore) UpsertActualState(state *RuntimeActualState) error {
	now := time.Now().Format(runtimeTimeFormat)
	if state.ObservedAt == "" {
		state.ObservedAt = now
	}
	var existing RuntimeActualState
	err := s.db.Where("runtime_id = ? AND installation_id = ?", state.RuntimeID, state.InstallationID).First(&existing).Error
	if err == nil {
		state.UpdatedAt = now
		return s.db.Save(state).Error
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		state.UpdatedAt = now
		return s.db.Create(state).Error
	}
	return err
}

func (s *stateStore) GetActualState(runtimeID, installationID string) (*RuntimeActualState, error) {
	var state RuntimeActualState
	err := s.db.Where("runtime_id = ? AND installation_id = ?", runtimeID, installationID).First(&state).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRuntimeOffline
		}
		return nil, err
	}
	return &state, nil
}

func (s *stateStore) ListActualStatesByRuntime(runtimeID string) ([]*RuntimeActualState, error) {
	var states []*RuntimeActualState
	err := s.db.Where("runtime_id = ?", runtimeID).
		Order("observed_at DESC").
		Find(&states).Error
	if states == nil {
		states = []*RuntimeActualState{}
	}
	return states, err
}

func (s *stateStore) DeleteActualState(runtimeID, installationID string) error {
	return s.db.Where("runtime_id = ? AND installation_id = ?", runtimeID, installationID).
		Delete(&RuntimeActualState{}).Error
}

func (s *stateStore) UpdateActualStateHealth(runtimeID, health string) error {
	now := time.Now().Format(runtimeTimeFormat)
	return s.db.Model(&RuntimeActualState{}).Where("runtime_id = ?", runtimeID).
		Updates(map[string]interface{}{
			"health":     health,
			"updated_at": now,
		}).Error
}
