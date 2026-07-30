// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only

package runtime

import (
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/contracts"
	"gorm.io/gorm"
)

const runtimeTimeFormat = "2006-01-02 15:04:05"

type RuntimeCommand struct {
	ID               string `gorm:"column:id;primaryKey;type:text"`
	RuntimeID        string `gorm:"column:runtime_id;type:text"`
	UserID           string `gorm:"column:user_id;type:text"`
	InstallationID   string `gorm:"column:installation_id;type:text"`
	PetInstanceID    string `gorm:"column:pet_instance_id;type:text"`
	Name             string `gorm:"column:name;type:text"`
	PayloadJSON      string `gorm:"column:payload_json;type:text"`
	Durability       string `gorm:"column:durability;type:text"`
	CoalesceKey      string `gorm:"column:coalesce_key;type:text"`
	IdempotencyKey   string `gorm:"column:idempotency_key;type:text"`
	DesiredRevision  int64  `gorm:"column:desired_revision;type:integer"`
	Status           string `gorm:"column:status;type:text"`
	AttemptCount     int    `gorm:"column:attempt_count;type:integer"`
	MaxAttempts      int    `gorm:"column:max_attempts;type:integer"`
	NextAttemptAt    string `gorm:"column:next_attempt_at;type:text"`
	DeadlineAt       string `gorm:"column:deadline_at;type:text"`
	LastSessionID    string `gorm:"column:last_session_id;type:text"`
	LastErrorCode    string `gorm:"column:last_error_code;type:text"`
	LastErrorMessage string `gorm:"column:last_error_message;type:text"`
	ResultJSON       string `gorm:"column:result_json;type:text"`
	CreatedAt        string `gorm:"column:created_at;type:text"`
	UpdatedAt        string `gorm:"column:updated_at;type:text"`
	CompletedAt      string `gorm:"column:completed_at;type:text"`
}

func (RuntimeCommand) TableName() string { return "desktop_pet_runtime_commands" }

type CommandStore interface {
	Create(cmd *RuntimeCommand) error
	GetByID(id string) (*RuntimeCommand, error)
	GetByIdempotencyKey(runtimeID, idempotencyKey string) (*RuntimeCommand, error)
	UpdateStatusCAS(id string, fromStatus []string, toStatus string, updates map[string]interface{}) error
	UpdateResult(id string, status string, resultJSON string, errorCode, errorMessage string) error
	ListPendingByRuntime(runtimeID string) ([]*RuntimeCommand, error)
	ListDispatchable(runtimeID string, limit int) ([]*RuntimeCommand, error)
	MarkSuperseded(runtimeID, coalesceKey string, belowRevision int64) error
	DeleteCompletedBefore(cutoffTime string) (int64, error)
	DeleteByRuntime(runtimeID string) error
	DB() *gorm.DB
}

type commandStore struct {
	db *gorm.DB
}

func NewCommandStore(db *gorm.DB) CommandStore {
	return &commandStore{db: db}
}

func (s *commandStore) DB() *gorm.DB { return s.db }

func (s *commandStore) Create(cmd *RuntimeCommand) error {
	if cmd.ID == "" {
		cmd.ID = uuid.NewString()
	}
	now := time.Now().Format(runtimeTimeFormat)
	if cmd.CreatedAt == "" {
		cmd.CreatedAt = now
	}
	if cmd.UpdatedAt == "" {
		cmd.UpdatedAt = now
	}
	return s.db.Create(cmd).Error
}

func (s *commandStore) GetByID(id string) (*RuntimeCommand, error) {
	var cmd RuntimeCommand
	err := s.db.Where("id = ?", id).First(&cmd).Error
	if err != nil {
		return nil, err
	}
	return &cmd, nil
}

func (s *commandStore) GetByIdempotencyKey(runtimeID, idempotencyKey string) (*RuntimeCommand, error) {
	var cmd RuntimeCommand
	err := s.db.Where("runtime_id = ? AND idempotency_key = ?", runtimeID, idempotencyKey).First(&cmd).Error
	if err != nil {
		return nil, err
	}
	return &cmd, nil
}

func (s *commandStore) UpdateStatusCAS(id string, fromStatus []string, toStatus string, updates map[string]interface{}) error {
	if updates == nil {
		updates = map[string]interface{}{}
	}
	updates["status"] = toStatus
	updates["updated_at"] = time.Now().Format(runtimeTimeFormat)
	result := s.db.Model(&RuntimeCommand{}).
		Where("id = ? AND status IN ?", id, fromStatus).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrRuntimeCommandStoreFailed
	}
	return nil
}

func (s *commandStore) UpdateResult(id string, status string, resultJSON string, errorCode, errorMessage string) error {
	now := time.Now().Format(runtimeTimeFormat)
	updates := map[string]interface{}{
		"status":             status,
		"result_json":        resultJSON,
		"last_error_code":    errorCode,
		"last_error_message": errorMessage,
		"updated_at":         now,
	}
	return s.db.Model(&RuntimeCommand{}).Where("id = ?", id).Updates(updates).Error
}

func (s *commandStore) ListPendingByRuntime(runtimeID string) ([]*RuntimeCommand, error) {
	var cmds []*RuntimeCommand
	err := s.db.Where("runtime_id = ? AND status = ?", runtimeID, contracts.CmdPending).
		Order("created_at ASC").
		Find(&cmds).Error
	if cmds == nil {
		cmds = []*RuntimeCommand{}
	}
	return cmds, err
}

func (s *commandStore) ListDispatchable(runtimeID string, limit int) ([]*RuntimeCommand, error) {
	now := time.Now().Format(runtimeTimeFormat)
	var cmds []*RuntimeCommand
	q := s.db.Where(
		"runtime_id = ? AND status = ? AND (max_attempts <= 0 OR attempt_count < max_attempts) AND (next_attempt_at = '' OR next_attempt_at <= ?) AND (deadline_at = '' OR deadline_at > ?)",
		runtimeID, contracts.CmdPending, now, now,
	).Order("created_at ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&cmds).Error
	if cmds == nil {
		cmds = []*RuntimeCommand{}
	}
	return cmds, err
}

func (s *commandStore) MarkSuperseded(runtimeID, coalesceKey string, belowRevision int64) error {
	now := time.Now().Format(runtimeTimeFormat)
	return s.db.Model(&RuntimeCommand{}).
		Where("runtime_id = ? AND coalesce_key = ? AND desired_revision < ? AND status = ?",
			runtimeID, coalesceKey, belowRevision, contracts.CmdPending).
		Updates(map[string]interface{}{
			"status":     contracts.CmdSuperseded,
			"updated_at": now,
		}).Error
}

func (s *commandStore) DeleteCompletedBefore(cutoffTime string) (int64, error) {
	result := s.db.Where("completed_at != '' AND completed_at < ?", cutoffTime).
		Delete(&RuntimeCommand{})
	return result.RowsAffected, result.Error
}

func (s *commandStore) DeleteByRuntime(runtimeID string) error {
	return s.db.Where("runtime_id = ?", runtimeID).Delete(&RuntimeCommand{}).Error
}
