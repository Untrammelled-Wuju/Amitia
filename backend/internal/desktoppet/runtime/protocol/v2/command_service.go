package v2

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CommandService interface {
	CreateDurableCommand(userID, deviceID, commandType, idempotencyKey, coalesceKey string, deviceSeq int64, payload RevisionPayload) (*RuntimeCommand, error)
	CreateEphemeralCommand(userID, deviceID, commandType, idempotencyKey string, payload []byte) (*RuntimeCommand, error)
	GetCommand(commandID string) (*RuntimeCommand, error)
	GetCommandByIdempotencyKey(idempotencyKey string) (*RuntimeCommand, error)
	UpdateStatus(commandID string, from, to CommandStatus, t time.Time) error
	MarkDispatching(commandID, runtimeID string, t time.Time) error
	MarkTransportDispatched(commandID, runtimeID string, t time.Time) error
	MarkRuntimeReceived(commandID, runtimeID, sessionID string, t time.Time) error
	MarkRuntimeAccepted(commandID, runtimeID, sessionID string, t time.Time) error
	MarkRendererAccepted(commandID, runtimeID, sessionID string, t time.Time) error
	MarkPlaybackStarted(commandID, playbackID string, t time.Time) error
	MarkCompleted(commandID, playbackID string, t time.Time) error
	MarkFailed(commandID, errCode, errMsg string, t time.Time) error
	MarkExpired(commandID string, t time.Time) error
	MarkCancelled(commandID string, t time.Time) error
	GetLatestCommand(userID, deviceID, commandType string) (*RuntimeCommand, error)
	ListCommandsToDispatch(limit int) ([]*RuntimeCommand, error)
	AllocateDeviceSequence(tx *gorm.DB, userID, deviceID string, t time.Time) (int64, error)
	GetResult(commandID, runtimeID string) (*CommandResult, error)
	SaveResult(result *CommandResult) error
	GetAttempt(attemptID string) (*CommandAttempt, error)
	SaveAttempt(attempt *CommandAttempt) error
	NakDedup(userID, deviceID, idempotencyKey string, nakTime time.Time) error
	QueryDedup(userID, deviceID, idempotencyKey string, since time.Time) (*CommandDedup, error)
	ListExpiredCommands(batchSize, timeoutSec int) ([]*RuntimeCommand, error)
	DB() *gorm.DB
}

type commandService struct {
	db *gorm.DB
}

func NewCommandService(db *gorm.DB) CommandService {
	return &commandService{db: db}
}

func (s *commandService) DB() *gorm.DB { return s.db }

func (s *commandService) CreateDurableCommand(userID, deviceID, commandType, idempotencyKey, coalesceKey string, deviceSeq int64, payload RevisionPayload) (*RuntimeCommand, error) {
	now := time.Now().Format("2006-01-02 15:04:05")
	payloadBytes, err := marshalJSON(payload)
	if err != nil {
		return nil, err
	}
	hash := ComputePayloadHash(payloadBytes)

	cmd := &RuntimeCommand{
		ID:             "rtcmdv2_" + uuid.NewString(),
		UserID:         userID,
		DeviceID:       deviceID,
		CommandType:    commandType,
		Status:         string(CommandStatusQueued),
		DesiredRevision: payload.GetRevision(),
		IdempotencyKey: idempotencyKey,
		CoalesceKey:    coalesceKey,
		DeviceSequence: deviceSeq,
		PayloadJSON:    string(payloadBytes),
		PayloadHash:    hash,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	var existing RuntimeCommand
	err = s.db.Where(
		"user_id = ? AND device_id = ? AND idempotency_key = ?",
		userID, deviceID, idempotencyKey,
	).Order("device_sequence DESC").First(&existing).Error

	if err == nil {
		if existing.Status == string(CommandStatusCompleted) || existing.PayloadHash == hash {
			return &existing, ErrCommandDuplication
		}
	}

	if err := s.db.Create(cmd).Error; err != nil {
		return nil, err
	}
	return cmd, nil
}

func (s *commandService) CreateEphemeralCommand(userID, deviceID, commandType, idempotencyKey string, payload []byte) (*RuntimeCommand, error) {
	now := time.Now().Format("2006-01-02 15:04:05")
	hash := ComputePayloadHash(payload)
	cmd := &RuntimeCommand{
		ID:             "rtcmdv2_" + uuid.NewString(),
		UserID:         userID,
		DeviceID:       deviceID,
		CommandType:    commandType,
		Status:         string(CommandStatusQueued),
		IdempotencyKey: idempotencyKey,
		PayloadJSON:    string(payload),
		PayloadHash:    hash,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	var existing RuntimeCommand
	if idempotencyKey != "" {
		err := s.db.Where(
			"user_id = ? AND device_id = ? AND idempotency_key = ?",
			userID, deviceID, idempotencyKey,
		).Order("created_at DESC").First(&existing).Error
		if err == nil && existing.PayloadHash == hash {
			return &existing, ErrCommandDuplication
		}
	}

	if err := s.db.Create(cmd).Error; err != nil {
		return nil, err
	}
	return cmd, nil
}

func (s *commandService) GetCommand(commandID string) (*RuntimeCommand, error) {
	var cmd RuntimeCommand
	err := s.db.Where("id = ?", commandID).First(&cmd).Error
	if err != nil {
		return nil, err
	}
	return &cmd, nil
}

func (s *commandService) GetCommandByIdempotencyKey(idempotencyKey string) (*RuntimeCommand, error) {
	var cmd RuntimeCommand
	err := s.db.Where("idempotency_key = ?", idempotencyKey).Order("created_at DESC").First(&cmd).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &cmd, nil
}

func (s *commandService) UpdateStatus(commandID string, from, to CommandStatus, t time.Time) error {
	updates := map[string]interface{}{
		"status":     string(to),
		"updated_at": t,
	}
	result := s.db.Model(&RuntimeCommand{}).Where("id = ? AND status = ?", commandID, string(from)).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("command status transition failed: no rows updated")
	}
	return nil
}

func (s *commandService) MarkDispatching(commandID, runtimeID string, t time.Time) error {
	return s.db.Model(&RuntimeCommand{}).Where("id = ?", commandID).Updates(map[string]interface{}{
		"status":     string(CommandStatusDispatching),
		"runtime_id": runtimeID,
		"updated_at": t,
	}).Error
}

func (s *commandService) MarkTransportDispatched(commandID, runtimeID string, t time.Time) error {
	return s.db.Model(&RuntimeCommand{}).Where("id = ? AND runtime_id = ?", commandID, runtimeID).Updates(map[string]interface{}{
		"status":     string(CommandStatusTransportDispatched),
		"updated_at": t,
	}).Error
}

func (s *commandService) MarkRuntimeReceived(commandID, runtimeID, sessionID string, t time.Time) error {
	return s.db.Model(&RuntimeCommand{}).Where("id = ? AND runtime_id = ?", commandID, runtimeID).Updates(map[string]interface{}{
		"status":              string(CommandStatusRuntimeReceived),
		"runtime_session_id":  sessionID,
		"updated_at":          t,
	}).Error
}

func (s *commandService) MarkRuntimeAccepted(commandID, runtimeID, sessionID string, t time.Time) error {
	return s.db.Model(&RuntimeCommand{}).Where("id = ? AND runtime_id = ?", commandID, runtimeID).Updates(map[string]interface{}{
		"status":              string(CommandStatusRuntimeAccepted),
		"runtime_session_id":  sessionID,
		"updated_at":          t,
	}).Error
}

func (s *commandService) MarkRendererAccepted(commandID, runtimeID, sessionID string, t time.Time) error {
	return s.db.Model(&RuntimeCommand{}).Where("id = ? AND runtime_id = ?", commandID, runtimeID).Updates(map[string]interface{}{
		"status":              string(CommandStatusRendererAccepted),
		"runtime_session_id":  sessionID,
		"updated_at":          t,
	}).Error
}

func (s *commandService) MarkPlaybackStarted(commandID, playbackID string, t time.Time) error {
	return s.db.Model(&RuntimeCommand{}).Where("id = ?", commandID).Updates(map[string]interface{}{
		"status":            string(CommandStatusPlaybackStarted),
		"updated_at":        t,
	}).Error
}

func (s *commandService) MarkCompleted(commandID, playbackID string, t time.Time) error {
	return s.db.Model(&RuntimeCommand{}).Where("id = ?", commandID).Updates(map[string]interface{}{
		"status":        string(CommandStatusCompleted),
		"completed_at":  t,
		"updated_at":    t,
	}).Error
}

func (s *commandService) MarkFailed(commandID, errCode, errMsg string, t time.Time) error {
	return s.db.Model(&RuntimeCommand{}).Where("id = ?", commandID).Updates(map[string]interface{}{
		"status":        string(CommandStatusFailedTerminal),
		"error_code":    errCode,
		"error_message": errMsg,
		"completed_at":  t,
		"updated_at":    t,
	}).Error
}

func (s *commandService) MarkExpired(commandID string, t time.Time) error {
	return s.db.Model(&RuntimeCommand{}).Where("id = ? AND status NOT IN (?, ?, ?, ?, ?, ?, ?)",
		commandID,
		string(CommandStatusCompleted), string(CommandStatusFailedTerminal), string(CommandStatusFailedRetryable),
		string(CommandStatusExpired), string(CommandStatusCancelled), string(CommandStatusSuperseded),
	).Updates(map[string]interface{}{
		"status":       string(CommandStatusExpired),
		"completed_at": t,
		"updated_at":   t,
	}).Error
}

func (s *commandService) MarkCancelled(commandID string, t time.Time) error {
	return s.db.Model(&RuntimeCommand{}).Where("id = ? AND status NOT IN (?, ?, ?, ?, ?)",
		commandID,
		string(CommandStatusCompleted), string(CommandStatusFailedTerminal), string(CommandStatusExpired), string(CommandStatusCancelled),
	).Updates(map[string]interface{}{
		"status":       string(CommandStatusCancelled),
		"completed_at": t,
		"updated_at":   t,
	}).Error
}

func (s *commandService) GetLatestCommand(userID, deviceID, commandType string) (*RuntimeCommand, error) {
	var cmd RuntimeCommand
	err := s.db.Where(
		"user_id = ? AND device_id = ? AND command_type = ? AND status IN (?, ?, ?, ?, ?, ?)",
		userID, deviceID, commandType,
		string(CommandStatusCreated), string(CommandStatusQueued),
		string(CommandStatusDispatching), string(CommandStatusTransportDispatched),
		string(CommandStatusRuntimeReceived), string(CommandStatusRuntimeAccepted),
	).Order("device_sequence DESC").First(&cmd).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCommandNotFound
		}
		return nil, err
	}
	return &cmd, nil
}

func (s *commandService) ListCommandsToDispatch(limit int) ([]*RuntimeCommand, error) {
	var cmds []*RuntimeCommand
	err := s.db.Where(
		"status IN (?, ?)",
		string(CommandStatusCreated), string(CommandStatusQueued),
	).Order("created_at ASC").Limit(limit).Find(&cmds).Error
	if err != nil {
		return nil, err
	}
	return cmds, nil
}

func (s *commandService) AllocateDeviceSequence(tx *gorm.DB, userID, deviceID string, t time.Time) (int64, error) {
	db := s.db
	if tx != nil {
		db = tx
	}

	var seq DeviceCommandSequence
	err := db.Where(
		"user_id = ? AND device_id = ?", userID, deviceID,
	).First(&seq).Error

	now := time.Now().Format("2006-01-02 15:04:05")
	if errors.Is(err, gorm.ErrRecordNotFound) {
		seq = DeviceCommandSequence{
			UserID:         userID,
			DeviceID:       deviceID,
			Sequence:       1,
			LastReservedAt: now,
			InsertedAt:     now,
			UpdatedAt:      now,
		}
		if err := db.Create(&seq).Error; err != nil {
			return 0, err
		}
		return 1, nil
	}
	if err != nil {
		return 0, err
	}

	newSeq := seq.Sequence + 1
	result := db.Model(&DeviceCommandSequence{}).Where(
		"user_id = ? AND device_id = ? AND sequence = ?",
		userID, deviceID, seq.Sequence,
	).Updates(map[string]interface{}{
		"sequence":          newSeq,
		"last_reserved_at":  now,
		"updated_at":        now,
	})
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected == 0 {
		return s.AllocateDeviceSequence(tx, userID, deviceID, t)
	}
	return newSeq, nil
}

func (s *commandService) GetResult(commandID, runtimeID string) (*CommandResult, error) {
	var result CommandResult
	err := s.db.Where("command_id = ? AND runtime_id = ?", commandID, runtimeID).First(&result).Error
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *commandService) SaveResult(result *CommandResult) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	var existing CommandResult
	err := s.db.Where("command_id = ? AND runtime_id = ?", result.CommandID, result.RuntimeID).First(&existing).Error
	if err == nil {
		result.ID = existing.ID
		result.InsertedAt = existing.InsertedAt
		result.UpdatedAt = now
		return s.db.Save(result).Error
	}
	if result.InsertedAt == "" {
		result.InsertedAt = now
	}
	result.UpdatedAt = now
	return s.db.Create(result).Error
}

func (s *commandService) GetAttempt(attemptID string) (*CommandAttempt, error) {
	var attempt CommandAttempt
	err := s.db.Where("attempt_id = ?", attemptID).First(&attempt).Error
	if err != nil {
		return nil, err
	}
	return &attempt, nil
}

func (s *commandService) SaveAttempt(attempt *CommandAttempt) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	var existing CommandAttempt
	err := s.db.Where("attempt_id = ?", attempt.AttemptID).First(&existing).Error
	if err == nil {
		attempt.InsertedAt = existing.InsertedAt
		attempt.UpdatedAt = now
		return s.db.Save(attempt).Error
	}
	if attempt.InsertedAt == "" {
		attempt.InsertedAt = now
	}
	attempt.UpdatedAt = now
	return s.db.Create(attempt).Error
}

func (s *commandService) NakDedup(userID, deviceID, idempotencyKey string, nakTime time.Time) error {
	now := nakTime.Format("2006-01-02 15:04:05")
	var dedup CommandDedup
	err := s.db.Where(
		"user_id = ? AND device_id = ? AND idempotency_key = ?",
		userID, deviceID, idempotencyKey,
	).First(&dedup).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		dedup = CommandDedup{
			ID:             "dedup_" + uuid.NewString(),
			UserID:         userID,
			DeviceID:       deviceID,
			IdempotencyKey: idempotencyKey,
			NakCount:       1,
			LastNakAt:      now,
			InsertedAt:     now,
			UpdatedAt:      now,
		}
		return s.db.Create(&dedup).Error
	}
	if err != nil {
		return err
	}

	return s.db.Model(&CommandDedup{}).Where("id = ?", dedup.ID).Updates(map[string]interface{}{
		"nak_count":   gorm.Expr("nak_count + 1"),
		"last_nak_at": now,
		"updated_at":  now,
	}).Error
}

func (s *commandService) QueryDedup(userID, deviceID, idempotencyKey string, since time.Time) (*CommandDedup, error) {
	var dedup CommandDedup
	err := s.db.Where(
		"user_id = ? AND device_id = ? AND idempotency_key = ? AND last_nak_at >= ?",
		userID, deviceID, idempotencyKey, since,
	).First(&dedup).Error
	if err != nil {
		return nil, err
	}
	return &dedup, nil
}

func (s *commandService) ListExpiredCommands(batchSize, timeoutSec int) ([]*RuntimeCommand, error) {
	var cmds []*RuntimeCommand
	expiredBefore := time.Now().Add(-time.Duration(timeoutSec) * time.Second).Format("2006-01-02 15:04:05")
	err := s.db.Where(
		"status IN (?, ?, ?, ?, ?, ?, ?) AND updated_at < ?",
		string(CommandStatusCreated), string(CommandStatusQueued),
		string(CommandStatusDispatching), string(CommandStatusTransportDispatched),
		string(CommandStatusRuntimeReceived), string(CommandStatusRuntimeAccepted),
		string(CommandStatusRendererAccepted),
		expiredBefore,
	).Limit(batchSize).Find(&cmds).Error
	if err != nil {
		return nil, err
	}
	return cmds, nil
}
