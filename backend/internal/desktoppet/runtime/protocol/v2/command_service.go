package v2

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	defaultEphemeralCommandTTL = 30 * time.Second
	// Once Renderer has started playback, expiresAt is no longer applicable.
	// A missing physical terminal is cleaned by an independent liveness budget.
	// Commands with an explicit maximumPlayMs use that exact budget plus grace;
	// otherwise use a conservative ceiling so legitimate long actions are never
	// killed by the transport ACK timeout.
	defaultPlaybackLivenessCeiling = 24 * time.Hour
	minimumPlaybackTerminalGrace   = 60 * time.Second
	ephemeralExpiryReconcileGrace  = 5 * time.Second
	// Fixed-width UTC keeps the TEXT column chronologically sortable while
	// remaining valid RFC3339. RFC3339Nano is variable-width and is not safe for
	// lexical SQL comparisons around whole-second/fractional boundaries.
	runtimeCommandExpiryLayout = "2006-01-02T15:04:05.000000000Z"
)

type CommandService interface {
	CreateDurableCommand(userID, deviceID, commandType, idempotencyKey, coalesceKey string, deviceSeq int64, payload RevisionPayload) (*RuntimeCommand, error)
	CreateEphemeralCommand(userID, deviceID, commandType, idempotencyKey string, payload []byte) (*RuntimeCommand, error)
	CreateEphemeralCommandForSession(userID, deviceID, runtimeID, sessionID, installationID, commandType, idempotencyKey string, payload []byte) (*RuntimeCommand, error)
	GetCommand(commandID string) (*RuntimeCommand, error)
	GetCommandByIdempotencyKey(idempotencyKey string) (*RuntimeCommand, error)
	UpdateStatus(commandID string, from, to CommandStatus, t time.Time) error
	MarkDispatching(commandID, runtimeID, sessionID string, t time.Time) error
	MarkTransportDispatched(commandID, runtimeID string, t time.Time) error
	MarkRuntimeReceived(commandID, runtimeID, sessionID string, t time.Time) error
	MarkRuntimeAccepted(commandID, runtimeID, sessionID string, t time.Time) error
	MarkRendererAccepted(commandID, runtimeID, sessionID, playbackID string, t time.Time) error
	MarkPlaybackStarted(commandID, playbackID string, t time.Time) error
	MarkCompleted(commandID, playbackID string, t time.Time) error
	MarkFailed(commandID, errCode, errMsg string, t time.Time) error
	MarkFailedRetryable(commandID, errCode, errMsg string, t time.Time) error
	RequeueDurableCommand(commandID, runtimeID string, t time.Time) error
	MarkExpired(commandID string, t time.Time) error
	MarkCancelled(commandID string, t time.Time) error
	MarkSuperseded(commandID, reason string, t time.Time) error
	SupersedeEphemeralCommands(userID, deviceID, runtimeID, sessionID, reason string, t time.Time) error
	GetLatestCommand(userID, deviceID, commandType string) (*RuntimeCommand, error)
	ListCommandsToDispatch(limit int) ([]*RuntimeCommand, error)
	ListCommandsToDispatchForConnection(userID, deviceID, runtimeID string, limit int) ([]*RuntimeCommand, error)
	ReconcileDesiredStateOnHello(userID, deviceID, runtimeID string, clientAppliedRevision, connectionGeneration int64) (int64, error)
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
	if !CommandType(commandType).IsDurable() {
		return nil, fmt.Errorf("unsupported durable runtime command type %q", commandType)
	}
	now := time.Now().UTC()
	nowText := now.Format("2006-01-02 15:04:05")
	payloadBytes, err := marshalJSON(payload)
	if err != nil {
		return nil, err
	}
	hash := ComputePayloadHash(payloadBytes)

	cmd := &RuntimeCommand{
		ID:                   "rtcmdv2_" + uuid.NewString(),
		UserID:               userID,
		DeviceID:             deviceID,
		CommandType:          commandType,
		Durability:           "durable",
		Status:               string(CommandStatusQueued),
		DesiredRevision:      payload.GetRevision(),
		IdempotencyKey:       idempotencyKey,
		CoalesceKey:          coalesceKey,
		DeviceSequence:       deviceSeq,
		PayloadJSON:          string(payloadBytes),
		PayloadHash:          hash,
		PayloadSchemaVersion: 1,
		CreatedAt:            nowText,
		UpdatedAt:            nowText,
	}
	if syncPayload, ok := payload.(SyncDesiredStatePayload); ok {
		cmd.SettingsRevision = syncPayload.SettingsRevision
		cmd.InstallationID = syncPayload.InstallationID
		cmd.PetID = syncPayload.PetID
		cmd.ReleaseID = syncPayload.ReleaseID
	}

	var result *RuntimeCommand
	duplicate := false
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var existing RuntimeCommand
		lookupErr := tx.Where(
			"user_id = ? AND device_id = ? AND idempotency_key = ?",
			userID, deviceID, idempotencyKey,
		).Order("device_sequence DESC").First(&existing).Error
		if lookupErr == nil {
			if existing.PayloadHash != hash {
				return fmt.Errorf("durable command idempotency conflict: key=%s", idempotencyKey)
			}
			// An expired durable command represents transport uncertainty, not a
			// completed logical intent. Recovery may revive it only while it is
			// still the newest revision for the coalesced desired-state stream.
			// Otherwise a delayed outbox/recovery pass could resurrect stale state
			// after a newer revision has already been created.
			if existing.Status == string(CommandStatusExpired) {
				if coalesceKey != "" && existing.DesiredRevision > 0 {
					var newer RuntimeCommand
					newerErr := tx.Where(
						"user_id = ? AND device_id = ? AND coalesce_key = ? AND desired_revision > ?",
						userID, deviceID, coalesceKey, existing.DesiredRevision,
					).Order("desired_revision DESC, device_sequence DESC").First(&newer).Error
					if newerErr == nil {
						if err := tx.Model(&RuntimeCommand{}).Where(
							"id = ? AND status = ?", existing.ID, string(CommandStatusExpired),
						).Updates(map[string]interface{}{
							"status":                   string(CommandStatusSuperseded),
							"superseded_by_command_id": newer.ID,
							"completed_at":             nowText,
							"updated_at":               nowText,
						}).Error; err != nil {
							return err
						}
						existing.Status = string(CommandStatusSuperseded)
						existing.SupersededByCommandID = newer.ID
						existing.CompletedAt = nowText
						existing.UpdatedAt = nowText
						result = &existing
						duplicate = true
						return nil
					}
					if newerErr != nil && !errors.Is(newerErr, gorm.ErrRecordNotFound) {
						return newerErr
					}
				}

				updates := map[string]interface{}{
					"status":             string(CommandStatusQueued),
					"device_sequence":    deviceSeq,
					"runtime_session_id": "",
					"error_code":         "",
					"error_message":      "",
					"completed_at":       "",
					"updated_at":         nowText,
				}
				if err := tx.Model(&RuntimeCommand{}).Where("id = ? AND status = ?", existing.ID, string(CommandStatusExpired)).Updates(updates).Error; err != nil {
					return err
				}
				existing.Status = string(CommandStatusQueued)
				existing.DeviceSequence = deviceSeq
				existing.RuntimeSessionID = ""
				existing.ErrorCode = ""
				existing.ErrorMessage = ""
				existing.CompletedAt = ""
				existing.UpdatedAt = nowText
				result = &existing
				return nil
			}
			result = &existing
			duplicate = true
			return nil
		}
		if lookupErr != nil && !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return lookupErr
		}

		if coalesceKey != "" && cmd.DesiredRevision > 0 {
			terminal := []string{
				string(CommandStatusCompleted), string(CommandStatusFailedTerminal), string(CommandStatusExpired),
				string(CommandStatusCancelled), string(CommandStatusSuperseded),
			}
			if err := tx.Model(&RuntimeCommand{}).
				Where("user_id = ? AND device_id = ? AND coalesce_key = ? AND desired_revision < ? AND status NOT IN ?",
					userID, deviceID, coalesceKey, cmd.DesiredRevision, terminal).
				Updates(map[string]interface{}{
					"status":                   string(CommandStatusSuperseded),
					"superseded_by_command_id": cmd.ID,
					"completed_at":             nowText,
					"updated_at":               nowText,
				}).Error; err != nil {
				return err
			}
		}

		if err := tx.Create(cmd).Error; err != nil {
			return err
		}
		result = cmd
		return nil
	})
	if err != nil {
		return result, err
	}
	if duplicate {
		return result, ErrCommandDuplication
	}
	return result, nil
}

func (s *commandService) CreateEphemeralCommand(userID, deviceID, commandType, idempotencyKey string, payload []byte) (*RuntimeCommand, error) {
	// Ephemeral commands are physical effects owned by one concrete runtime
	// websocket session. Creating an unbound command would allow a reconnecting
	// runtime to inherit stale work, so fail closed and require the session-bound
	// constructor everywhere. Keep this method only for interface compatibility.
	return nil, errors.New("unbound ephemeral command creation is disabled; use CreateEphemeralCommandForSession")
}

func (s *commandService) CreateEphemeralCommandForSession(userID, deviceID, runtimeID, sessionID, installationID, commandType, idempotencyKey string, payload []byte) (*RuntimeCommand, error) {
	if !CommandType(commandType).IsEphemeral() {
		return nil, fmt.Errorf("unsupported ephemeral runtime command type %q", commandType)
	}
	if strings.TrimSpace(runtimeID) == "" || strings.TrimSpace(sessionID) == "" {
		return nil, errors.New("session-bound ephemeral command requires runtime and session ids")
	}
	seq, err := s.AllocateDeviceSequence(nil, userID, deviceID, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	return s.createEphemeralCommand(userID, deviceID, runtimeID, sessionID, installationID, commandType, idempotencyKey, seq, payload)
}

func (s *commandService) createEphemeralCommand(userID, deviceID, runtimeID, sessionID, installationID, commandType, idempotencyKey string, deviceSeq int64, payload []byte) (*RuntimeCommand, error) {
	nowTime := time.Now().UTC()
	now := nowTime.Format("2006-01-02 15:04:05")
	hash := ComputePayloadHash(payload)
	expiresAt := nowTime.Add(defaultEphemeralCommandTTL)
	if CommandType(commandType) == CommandTypePlayAction {
		var playPayload PlayActionPayload
		if err := json.Unmarshal(payload, &playPayload); err != nil {
			return nil, fmt.Errorf("invalid play_action payload: %w", err)
		}
		if strings.TrimSpace(playPayload.ActionKey) == "" {
			return nil, errors.New("play_action requires actionKey")
		}
		if strings.TrimSpace(playPayload.RuntimeID) != strings.TrimSpace(runtimeID) {
			return nil, errors.New("play_action runtimeId must match the bound runtime")
		}
		if strings.TrimSpace(playPayload.PetInstanceID) != strings.TrimSpace(runtimeID) {
			return nil, errors.New("play_action petInstanceId must match the bound runtime pet instance")
		}
		if strings.TrimSpace(playPayload.InstallationID) == "" || strings.TrimSpace(playPayload.InstallationID) != strings.TrimSpace(installationID) {
			return nil, errors.New("play_action installationId must match the bound installation")
		}
		if strings.TrimSpace(playPayload.CharacterID) == "" {
			return nil, errors.New("play_action requires characterId")
		}
		if strings.TrimSpace(playPayload.ExpiresAt) != "" {
			parsed, parseErr := time.Parse(time.RFC3339Nano, playPayload.ExpiresAt)
			if parseErr != nil {
				parsed, parseErr = time.Parse(time.RFC3339, playPayload.ExpiresAt)
			}
			if parseErr != nil {
				return nil, fmt.Errorf("invalid play_action expiresAt: %w", parseErr)
			}
			// The producer may shorten the admission window but can never extend
			// the server-owned Ephemeral lifetime. This prevents a stale behavior
			// candidate or compromised caller from turning play_action into durable
			// physical intent.
			if parsed.UTC().Before(expiresAt) {
				expiresAt = parsed.UTC()
			}
		}
	}
	if !expiresAt.After(nowTime) {
		return nil, errors.New("ephemeral command expiresAt must be in the future")
	}
	cmd := &RuntimeCommand{
		ID:                   "rtcmdv2_" + uuid.NewString(),
		UserID:               userID,
		DeviceID:             deviceID,
		RuntimeID:            runtimeID,
		RuntimeSessionID:     sessionID,
		InstallationID:       installationID,
		CommandType:          commandType,
		Durability:           "ephemeral",
		Status:               string(CommandStatusQueued),
		DeviceSequence:       deviceSeq,
		IdempotencyKey:       idempotencyKey,
		PayloadJSON:          string(payload),
		PayloadHash:          hash,
		PayloadSchemaVersion: 1,
		ExpiresAt:            expiresAt.UTC().Format(runtimeCommandExpiryLayout),
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	var existing RuntimeCommand
	if idempotencyKey != "" {
		err := s.db.Where(
			"user_id = ? AND device_id = ? AND idempotency_key = ?",
			userID, deviceID, idempotencyKey,
		).Order("created_at DESC").First(&existing).Error
		if err == nil {
			if existing.PayloadHash == hash {
				return &existing, ErrCommandDuplication
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
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

func isValidProgressTransition(cmd *RuntimeCommand, target CommandStatus) bool {
	if cmd == nil {
		return false
	}
	current := CommandStatus(cmd.Status)
	if current == target {
		return true
	}
	switch target {
	case CommandStatusTransportDispatched:
		return current == CommandStatusDispatching
	case CommandStatusRuntimeReceived:
		// The runtime can ACK immediately after the websocket write succeeds and
		// race the dispatcher persistence of transport_dispatched.
		return current == CommandStatusTransportDispatched || current == CommandStatusDispatching
	case CommandStatusRuntimeAccepted:
		return current == CommandStatusRuntimeReceived
	case CommandStatusRendererAccepted:
		return CommandType(cmd.CommandType) == CommandTypePlayAction && current == CommandStatusRuntimeAccepted
	case CommandStatusPlaybackStarted:
		return CommandType(cmd.CommandType) == CommandTypePlayAction && current == CommandStatusRendererAccepted
	case CommandStatusCompleted:
		switch CommandType(cmd.CommandType) {
		case CommandTypePlayAction:
			// already_satisfied can complete immediately after renderer acceptance;
			// every physically started policy completes from playback_started.
			return current == CommandStatusRendererAccepted || current == CommandStatusPlaybackStarted
		case CommandTypeSyncDesiredState, CommandTypeEnsureAbsent, CommandTypeReloadRelease:
			// Durable desired commands are completed only by desired_applied.
			return current == CommandStatusRuntimeAccepted
		default:
			// Runtime-only commands (recenter/stop/pause/resume) have no renderer
			// lifecycle and may complete after runtime validation/application.
			return current == CommandStatusRuntimeAccepted
		}
	default:
		return false
	}
}

func (s *commandService) advanceProgress(
	commandID string,
	target CommandStatus,
	runtimeID, sessionID string,
	t time.Time,
	extra map[string]interface{},
) error {
	if commandID == "" {
		return errors.New("command id is empty")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		var cmd RuntimeCommand
		if err := tx.Where("id = ?", commandID).Take(&cmd).Error; err != nil {
			return err
		}
		current := CommandStatus(cmd.Status)
		if current.IsTerminal() {
			if current == target || current == CommandStatusCompleted {
				return nil
			}
			return fmt.Errorf("command %s already terminal: %s", commandID, current)
		}
		if !isValidProgressTransition(&cmd, target) {
			return fmt.Errorf("illegal command progress transition %s -> %s for %s", current, target, commandID)
		}
		if current == target {
			return nil
		}
		updates := map[string]interface{}{
			"status":     string(target),
			"updated_at": t,
		}
		if runtimeID != "" {
			updates["runtime_id"] = runtimeID
		}
		if sessionID != "" {
			updates["runtime_session_id"] = sessionID
		}
		for key, value := range extra {
			updates[key] = value
		}
		result := tx.Model(&RuntimeCommand{}).Where("id = ? AND status = ?", commandID, cmd.Status).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("command progress transition lost concurrent CAS")
		}
		return nil
	})
}

func (s *commandService) MarkDispatching(commandID, runtimeID, sessionID string, t time.Time) error {
	if commandID == "" {
		return errors.New("command id is empty")
	}
	if strings.TrimSpace(sessionID) == "" {
		return errors.New("dispatching command requires runtime session id")
	}
	updates := map[string]interface{}{
		"status":             string(CommandStatusDispatching),
		"runtime_session_id": sessionID,
		"error_code":         "",
		"error_message":      "",
		"updated_at":         t,
	}
	if runtimeID != "" {
		updates["runtime_id"] = runtimeID
	}
	result := s.db.Model(&RuntimeCommand{}).Where(
		"id = ? AND status IN (?, ?, ?)",
		commandID,
		string(CommandStatusCreated),
		string(CommandStatusQueued),
		string(CommandStatusFailedRetryable),
	).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 1 {
		return nil
	}
	var cmd RuntimeCommand
	if err := s.db.Where("id = ?", commandID).Take(&cmd).Error; err != nil {
		return err
	}
	return fmt.Errorf("command %s is not dispatchable from status %s", commandID, cmd.Status)
}

func (s *commandService) updateLastAttempt(commandID string, fields map[string]interface{}, t time.Time) error {
	if commandID == "" || len(fields) == 0 {
		return nil
	}
	var cmd RuntimeCommand
	if err := s.db.Select("last_attempt_id").Where("id = ?", commandID).Take(&cmd).Error; err != nil {
		return err
	}
	if cmd.LastAttemptID == "" {
		return nil
	}
	updates := make(map[string]interface{}, len(fields)+1)
	for key, value := range fields {
		updates[key] = value
	}
	updates["updated_at"] = t.UTC().Format("2006-01-02 15:04:05")
	return s.db.Model(&CommandAttempt{}).Where("attempt_id = ?", cmd.LastAttemptID).Updates(updates).Error
}

func (s *commandService) MarkTransportDispatched(commandID, runtimeID string, t time.Time) error {
	// The websocket write and the peer ACK are concurrent. A very fast runtime can
	// legitimately move the command to runtime_received before the dispatcher
	// persists transport_dispatched. Never regress that state; only backfill the
	// attempt timestamp in that race.
	var cmd RuntimeCommand
	if err := s.db.Select("status").Where("id = ?", commandID).Take(&cmd).Error; err != nil {
		return err
	}
	switch CommandStatus(cmd.Status) {
	case CommandStatusRuntimeReceived, CommandStatusRuntimeAccepted, CommandStatusRendererAccepted,
		CommandStatusPlaybackStarted, CommandStatusCompleted, CommandStatusFailedTerminal,
		CommandStatusExpired, CommandStatusCancelled, CommandStatusSuperseded:
		return s.updateLastAttempt(commandID, map[string]interface{}{
			"dispatched_at": t.UTC().Format(time.RFC3339Nano),
		}, t)
	}
	if err := s.advanceProgress(commandID, CommandStatusTransportDispatched, runtimeID, "", t, nil); err != nil {
		return err
	}
	return s.updateLastAttempt(commandID, map[string]interface{}{
		"dispatched_at": t.UTC().Format(time.RFC3339Nano),
	}, t)
}

func (s *commandService) MarkRuntimeReceived(commandID, runtimeID, sessionID string, t time.Time) error {
	if err := s.advanceProgress(commandID, CommandStatusRuntimeReceived, runtimeID, sessionID, t, nil); err != nil {
		return err
	}
	return s.updateLastAttempt(commandID, map[string]interface{}{
		"runtime_received_at": t.UTC().Format(time.RFC3339Nano),
	}, t)
}

func (s *commandService) MarkRuntimeAccepted(commandID, runtimeID, sessionID string, t time.Time) error {
	if err := s.advanceProgress(commandID, CommandStatusRuntimeAccepted, runtimeID, sessionID, t, nil); err != nil {
		return err
	}
	return s.updateLastAttempt(commandID, map[string]interface{}{
		"runtime_accepted_at": t.UTC().Format(time.RFC3339Nano),
	}, t)
}

func (s *commandService) MarkRendererAccepted(commandID, runtimeID, sessionID, playbackID string, t time.Time) error {
	playbackID = strings.TrimSpace(playbackID)
	if playbackID == "" {
		return errors.New("renderer acceptance requires playback id")
	}
	if err := s.advanceProgress(commandID, CommandStatusRendererAccepted, runtimeID, sessionID, t, map[string]interface{}{
		"runtime_playback_id": playbackID,
	}); err != nil {
		return err
	}
	return s.updateLastAttempt(commandID, map[string]interface{}{
		"renderer_accepted_at": t.UTC().Format(time.RFC3339Nano),
	}, t)
}

func (s *commandService) MarkPlaybackStarted(commandID, playbackID string, t time.Time) error {
	playbackID = strings.TrimSpace(playbackID)
	if playbackID == "" {
		return errors.New("playback start requires playback id")
	}
	var cmd RuntimeCommand
	if err := s.db.Select("command_type", "runtime_playback_id").Where("id = ?", commandID).Take(&cmd).Error; err != nil {
		return err
	}
	if CommandType(cmd.CommandType) == CommandTypePlayAction && strings.TrimSpace(cmd.PlaybackRequestID) != playbackID {
		return errors.New("playback start identity does not match renderer-accepted playback")
	}
	if err := s.advanceProgress(commandID, CommandStatusPlaybackStarted, "", "", t, map[string]interface{}{
		"playback_started_at": t.UTC().Format(time.RFC3339Nano),
	}); err != nil {
		return err
	}
	return s.updateLastAttempt(commandID, map[string]interface{}{
		"playback_started_at": t.UTC().Format(time.RFC3339Nano),
	}, t)
}

func (s *commandService) MarkCompleted(commandID, playbackID string, t time.Time) error {
	playbackID = strings.TrimSpace(playbackID)
	var cmd RuntimeCommand
	if err := s.db.Select("command_type", "runtime_playback_id").Where("id = ?", commandID).Take(&cmd).Error; err != nil {
		return err
	}
	if CommandType(cmd.CommandType) == CommandTypePlayAction {
		if playbackID == "" || strings.TrimSpace(cmd.PlaybackRequestID) != playbackID {
			return errors.New("playback completion identity does not match renderer-accepted playback")
		}
	}
	if err := s.advanceProgress(commandID, CommandStatusCompleted, "", "", t, map[string]interface{}{
		"completed_at": t,
	}); err != nil {
		return err
	}
	return s.updateLastAttempt(commandID, map[string]interface{}{
		"finished_at": t.UTC().Format(time.RFC3339Nano),
	}, t)
}

func (s *commandService) MarkFailed(commandID, errCode, errMsg string, t time.Time) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var cmd RuntimeCommand
		if err := tx.Where("id = ?", commandID).Take(&cmd).Error; err != nil {
			return err
		}
		if CommandStatus(cmd.Status).IsTerminal() {
			if cmd.Status == string(CommandStatusFailedTerminal) {
				return nil
			}
			return fmt.Errorf("command %s already terminal: %s", commandID, cmd.Status)
		}
		result := tx.Model(&RuntimeCommand{}).Where("id = ? AND status = ?", commandID, cmd.Status).Updates(map[string]interface{}{
			"status":        string(CommandStatusFailedTerminal),
			"error_code":    errCode,
			"error_message": errMsg,
			"completed_at":  t,
			"updated_at":    t,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("command failure transition lost concurrent CAS")
		}
		return nil
	})
	if err != nil {
		return err
	}
	return s.updateLastAttempt(commandID, map[string]interface{}{
		"finished_at":   t.UTC().Format(time.RFC3339Nano),
		"error_code":    errCode,
		"error_message": errMsg,
	}, t)
}

func (s *commandService) MarkFailedRetryable(commandID, errCode, errMsg string, t time.Time) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var cmd RuntimeCommand
		if err := tx.Where("id = ?", commandID).Take(&cmd).Error; err != nil {
			return err
		}
		status := CommandStatus(cmd.Status)
		if status.IsTerminal() {
			// A late transport failure must never overwrite a terminal result.
			return nil
		}
		if !cmd.IsDurable() {
			return fmt.Errorf("command %s is not durable and cannot be retried", commandID)
		}
		result := tx.Model(&RuntimeCommand{}).Where("id = ? AND status = ?", commandID, cmd.Status).Updates(map[string]interface{}{
			"status":             string(CommandStatusFailedRetryable),
			"runtime_session_id": "",
			"error_code":         errCode,
			"error_message":      errMsg,
			"updated_at":         t,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("command retryable transition lost concurrent CAS")
		}
		return nil
	})
	if err != nil {
		return err
	}
	return s.updateLastAttempt(commandID, map[string]interface{}{
		"finished_at":   t.UTC().Format(time.RFC3339Nano),
		"error_code":    errCode,
		"error_message": errMsg,
	}, t)
}

func (s *commandService) RequeueDurableCommand(commandID, runtimeID string, t time.Time) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var cmd RuntimeCommand
		if err := tx.Where("id = ?", commandID).Take(&cmd).Error; err != nil {
			return err
		}
		if !cmd.IsDurable() {
			return fmt.Errorf("command %s is not durable and cannot be requeued", commandID)
		}
		status := CommandStatus(cmd.Status)
		if status.IsTerminal() {
			return fmt.Errorf("command %s already terminal: %s", commandID, status)
		}
		updates := map[string]interface{}{
			"status":             string(CommandStatusQueued),
			"runtime_session_id": "",
			"error_code":         "",
			"error_message":      "",
			"updated_at":         t,
		}
		// Runtime IDs are connection incarnations. A durable desired-state command
		// that survived an old process/session must be retargeted to the runtime
		// which just authenticated, otherwise the connection-scoped dispatcher
		// will correctly filter it out forever.
		if runtimeID != "" {
			updates["runtime_id"] = runtimeID
		}
		result := tx.Model(&RuntimeCommand{}).Where("id = ? AND status = ?", commandID, cmd.Status).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("command requeue transition lost concurrent CAS")
		}
		return nil
	})
}

func (s *commandService) MarkExpired(commandID string, t time.Time) error {
	result := s.db.Model(&RuntimeCommand{}).Where("id = ? AND status NOT IN (?, ?, ?, ?, ?)",
		commandID,
		string(CommandStatusCompleted), string(CommandStatusFailedTerminal), string(CommandStatusExpired),
		string(CommandStatusCancelled), string(CommandStatusSuperseded),
	).Updates(map[string]interface{}{
		"status":       string(CommandStatusExpired),
		"completed_at": t,
		"updated_at":   t,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var cmd RuntimeCommand
		if err := s.db.Where("id = ?", commandID).Take(&cmd).Error; err != nil {
			return err
		}
		if CommandStatus(cmd.Status).IsTerminal() {
			return nil
		}
		return errors.New("command expiration transition updated no rows")
	}
	return s.updateLastAttempt(commandID, map[string]interface{}{
		"finished_at":   t.UTC().Format(time.RFC3339Nano),
		"error_code":    "COMMAND_EXPIRED",
		"error_message": "command expired",
	}, t)
}

func (s *commandService) MarkCancelled(commandID string, t time.Time) error {
	result := s.db.Model(&RuntimeCommand{}).Where("id = ? AND status NOT IN (?, ?, ?, ?, ?)",
		commandID,
		string(CommandStatusCompleted), string(CommandStatusFailedTerminal), string(CommandStatusExpired), string(CommandStatusCancelled), string(CommandStatusSuperseded),
	).Updates(map[string]interface{}{
		"status":       string(CommandStatusCancelled),
		"completed_at": t,
		"updated_at":   t,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var cmd RuntimeCommand
		if err := s.db.Where("id = ?", commandID).Take(&cmd).Error; err != nil {
			return err
		}
		if CommandStatus(cmd.Status).IsTerminal() {
			return nil
		}
		return errors.New("command cancellation transition updated no rows")
	}
	return s.updateLastAttempt(commandID, map[string]interface{}{
		"finished_at":   t.UTC().Format(time.RFC3339Nano),
		"error_code":    "COMMAND_CANCELLED",
		"error_message": "command cancelled",
	}, t)
}

func (s *commandService) MarkSuperseded(commandID, reason string, t time.Time) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "command superseded"
	}
	result := s.db.Model(&RuntimeCommand{}).Where(
		"id = ? AND status NOT IN (?, ?, ?, ?, ?)",
		commandID,
		string(CommandStatusCompleted), string(CommandStatusFailedTerminal), string(CommandStatusExpired),
		string(CommandStatusCancelled), string(CommandStatusSuperseded),
	).Updates(map[string]interface{}{
		"status":        string(CommandStatusSuperseded),
		"error_code":    "COMMAND_SUPERSEDED",
		"error_message": reason,
		"completed_at":  t,
		"updated_at":    t,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var cmd RuntimeCommand
		if err := s.db.Where("id = ?", commandID).Take(&cmd).Error; err != nil {
			return err
		}
		if CommandStatus(cmd.Status).IsTerminal() {
			return nil
		}
		return errors.New("command supersede transition updated no rows")
	}
	return s.updateLastAttempt(commandID, map[string]interface{}{
		"finished_at":   t.UTC().Format(time.RFC3339Nano),
		"error_code":    "COMMAND_SUPERSEDED",
		"error_message": reason,
	}, t)
}

func (s *commandService) SupersedeEphemeralCommands(userID, deviceID, runtimeID, sessionID, reason string, t time.Time) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		query := tx.Where(
			"user_id = ? AND device_id = ? AND durability = ? AND status NOT IN (?, ?, ?, ?, ?)",
			userID, deviceID, "ephemeral",
			string(CommandStatusCompleted), string(CommandStatusFailedTerminal), string(CommandStatusExpired),
			string(CommandStatusCancelled), string(CommandStatusSuperseded),
		)
		if runtimeID != "" {
			query = query.Where("runtime_id = '' OR runtime_id IS NULL OR runtime_id = ?", runtimeID)
		}
		if sessionID != "" {
			query = query.Where("runtime_session_id = '' OR runtime_session_id IS NULL OR runtime_session_id = ?", sessionID)
		}
		var cmds []RuntimeCommand
		if err := query.Find(&cmds).Error; err != nil {
			return err
		}
		for _, cmd := range cmds {
			// A reconnect is a hard ephemeral boundary. This includes rows which were
			// queued for the old connected runtime but had not yet created an Attempt;
			// replaying those on the new session would execute stale physical intent.
			result := tx.Model(&RuntimeCommand{}).Where(
				"id = ? AND status = ?", cmd.ID, cmd.Status,
			).Updates(map[string]interface{}{
				"status":        string(CommandStatusSuperseded),
				"error_code":    "CONNECTION_REPLACED",
				"error_message": reason,
				"completed_at":  t,
				"updated_at":    t,
			})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				continue
			}
			if cmd.LastAttemptID != "" {
				if err := tx.Model(&CommandAttempt{}).Where("attempt_id = ?", cmd.LastAttemptID).Updates(map[string]interface{}{
					"finished_at":   t.UTC().Format(time.RFC3339Nano),
					"error_code":    "CONNECTION_REPLACED",
					"error_message": reason,
					"updated_at":    t.UTC().Format("2006-01-02 15:04:05"),
				}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
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
	if limit <= 0 {
		limit = 100
	}
	now := time.Now().UTC()
	retryBefore := now.Add(-time.Second).Format("2006-01-02 15:04:05")
	nowRFC3339 := now.Format(runtimeCommandExpiryLayout)
	err := s.db.Where(
		"(status IN (?, ?) OR (status = ? AND datetime(updated_at) <= datetime(?))) AND (expires_at = '' OR expires_at IS NULL OR expires_at > ?)",
		string(CommandStatusCreated), string(CommandStatusQueued),
		string(CommandStatusFailedRetryable), retryBefore, nowRFC3339,
	).Order("device_sequence ASC, created_at ASC").Limit(limit).Find(&cmds).Error
	if err != nil {
		return nil, err
	}
	return cmds, nil
}

func (s *commandService) ListCommandsToDispatchForConnection(userID, deviceID, runtimeID string, limit int) ([]*RuntimeCommand, error) {
	var cmds []*RuntimeCommand
	if limit <= 0 {
		limit = 100
	}
	now := time.Now().UTC()
	retryBefore := now.Add(-time.Second).Format("2006-01-02 15:04:05")
	nowRFC3339 := now.Format(runtimeCommandExpiryLayout)
	query := s.db.Where(
		"user_id = ? AND device_id = ? AND (status IN (?, ?) OR (status = ? AND datetime(updated_at) <= datetime(?))) AND (expires_at = '' OR expires_at IS NULL OR expires_at > ?)",
		userID, deviceID,
		string(CommandStatusCreated), string(CommandStatusQueued),
		string(CommandStatusFailedRetryable), retryBefore, nowRFC3339,
	)
	if runtimeID != "" {
		query = query.Where("runtime_id = '' OR runtime_id IS NULL OR runtime_id = ?", runtimeID)
	}
	err := query.Order("device_sequence ASC, created_at ASC").Limit(limit).Find(&cmds).Error
	if err != nil {
		return nil, err
	}
	return cmds, nil
}

type authoritativeDesiredStateRow struct {
	UserID               string `gorm:"column:user_id"`
	DeviceID             string `gorm:"column:device_id"`
	RuntimeID            string `gorm:"column:runtime_id"`
	InstallationID       string `gorm:"column:installation_id"`
	PetID                string `gorm:"column:pet_id"`
	ReleaseID            string `gorm:"column:release_id"`
	DesiredEnabled       bool   `gorm:"column:desired_enabled"`
	DesiredVisible       bool   `gorm:"column:desired_visible"`
	DesiredActionKey     string `gorm:"column:desired_action_key"`
	SettingsSnapshotJSON string `gorm:"column:settings_snapshot_json"`
	SettingsRevision     int64  `gorm:"column:settings_revision"`
	DesiredRevision      int64  `gorm:"column:desired_revision"`
	DesiredHash          string `gorm:"column:desired_hash"`
}

func (s *commandService) ReconcileDesiredStateOnHello(userID, deviceID, runtimeID string, clientAppliedRevision, connectionGeneration int64) (int64, error) {
	const desiredTable = "desktop_pet_runtime_desired_states"
	// A few isolated protocol unit tests intentionally migrate only the runtime
	// tables. Production startup migrates the desired-state table before the
	// facade is exposed, so absence here means there is no authority to reconcile.
	if !s.db.Migrator().HasTable(desiredTable) {
		return 0, nil
	}

	var state authoritativeDesiredStateRow
	query := s.db.Table(desiredTable).Where("user_id = ? AND device_id = ?", userID, deviceID)
	if err := query.Take(&state).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}
	if state.DesiredRevision <= 0 {
		return 0, nil
	}
	// Desired state is device-scoped. runtime_id is routing metadata from the
	// last publisher, not a second authority boundary. A regenerated runtime ID
	// on the same authenticated device must still converge to the device's
	// persisted desired state. The command below is retargeted to the current
	// connected runtime.
	coalesceKey := fmt.Sprintf("desired:%s", deviceID)
	var existing RuntimeCommand
	err := s.db.Where(
		"user_id = ? AND device_id = ? AND coalesce_key = ? AND desired_revision = ?",
		userID, deviceID, coalesceKey, state.DesiredRevision,
	).Order("device_sequence DESC").First(&existing).Error
	if err == nil {
		status := CommandStatus(existing.Status)
		switch {
		case status == CommandStatusCreated || status == CommandStatusQueued || status == CommandStatusFailedRetryable:
			// Even an already-queued command can belong to a dead runtime incarnation.
			// Requeueing is also the retarget operation and keeps the dispatcher
			// scoped without making old runtime IDs a permanent routing tombstone.
			if runtimeID != "" && existing.RuntimeID != runtimeID {
				if err := s.RequeueDurableCommand(existing.ID, runtimeID, time.Now().UTC()); err != nil {
					return 0, err
				}
			}
			return state.DesiredRevision, nil
		case !status.IsTerminal():
			// The old websocket/session is being replaced. Anything that was in
			// flight on it is transport-uncertain and must be dispatched again.
			if err := s.RequeueDurableCommand(existing.ID, runtimeID, time.Now().UTC()); err != nil {
				return 0, err
			}
			return state.DesiredRevision, nil
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}

	// A client reporting the desired revision does not prove that the server
	// received the durable desired_applied event. If a same-revision command was
	// still non-terminal above, it has already been requeued onto this fresh
	// session. Only short-circuit after that recovery check.
	if clientAppliedRevision >= state.DesiredRevision {
		return state.DesiredRevision, nil
	}

	var settingsSnapshot json.RawMessage
	if state.SettingsSnapshotJSON != "" {
		settingsSnapshot = json.RawMessage(state.SettingsSnapshotJSON)
		if !json.Valid(settingsSnapshot) {
			return 0, errors.New("authoritative desired settings snapshot is invalid JSON")
		}
	}
	payload := SyncDesiredStatePayload{
		DesiredRevision:        state.DesiredRevision,
		DesiredHash:            state.DesiredHash,
		EnsureAbsent:           !state.DesiredEnabled && !state.DesiredVisible,
		InstallationID:         state.InstallationID,
		PetID:                  state.PetID,
		ReleaseID:              state.ReleaseID,
		RuntimeContractVersion: CurrentSchemaVersion,
		DefaultActionKey:       state.DesiredActionKey,
		SettingsRevision:       state.SettingsRevision,
		SettingsSnapshot:       settingsSnapshot,
	}
	commandType := CommandTypeSyncDesiredState
	if payload.EnsureAbsent {
		commandType = CommandTypeEnsureAbsent
	}
	seq, err := s.AllocateDeviceSequence(nil, userID, deviceID, time.Now().UTC())
	if err != nil {
		return 0, err
	}
	if connectionGeneration < 1 {
		connectionGeneration = 1
	}
	idempotencyKey := fmt.Sprintf("reconcile:%s:%d:%d", deviceID, state.DesiredRevision, connectionGeneration)
	cmd, err := s.CreateDurableCommand(userID, deviceID, string(commandType), idempotencyKey, coalesceKey, seq, payload)
	if err != nil && !errors.Is(err, ErrCommandDuplication) {
		return 0, err
	}
	if cmd != nil && runtimeID != "" && cmd.RuntimeID == "" {
		if err := s.db.Model(&RuntimeCommand{}).Where("id = ?", cmd.ID).Update("runtime_id", runtimeID).Error; err != nil {
			return 0, err
		}
	}
	return state.DesiredRevision, nil
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
		"sequence":         newSeq,
		"last_reserved_at": now,
		"updated_at":       now,
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

func parseRuntimeCommandTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, runtimeCommandExpiryLayout, "2006-01-02 15:04:05"} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

func playbackLivenessDeadline(cmd *RuntimeCommand, timeoutSec int) (time.Time, bool) {
	if cmd == nil || CommandType(cmd.CommandType) != CommandTypePlayAction {
		return time.Time{}, false
	}
	startedAt, ok := parseRuntimeCommandTime(cmd.PlaybackStartedAt)
	if !ok {
		// Legacy rows may predate command-level playback_started_at persistence.
		// updated_at is the transition timestamp for playback_started and is a safe
		// conservative fallback for those rows.
		startedAt, ok = parseRuntimeCommandTime(cmd.UpdatedAt)
		if !ok {
			return time.Time{}, false
		}
	}

	grace := time.Duration(timeoutSec) * time.Second
	if grace < minimumPlaybackTerminalGrace {
		grace = minimumPlaybackTerminalGrace
	}
	budget := defaultPlaybackLivenessCeiling
	var payload PlayActionPayload
	if err := json.Unmarshal([]byte(cmd.PayloadJSON), &payload); err == nil && payload.MaximumPlayMs > 0 {
		budget = time.Duration(payload.MaximumPlayMs)*time.Millisecond + grace
	}
	return startedAt.Add(budget), true
}

func (s *commandService) ListExpiredCommands(batchSize, timeoutSec int) ([]*RuntimeCommand, error) {
	if batchSize <= 0 {
		return []*RuntimeCommand{}, nil
	}
	var cmds []*RuntimeCommand
	now := time.Now().UTC()
	ackExpiredBefore := now.Add(-time.Duration(timeoutSec) * time.Second).Format("2006-01-02 15:04:05")
	// Backend cleanup intentionally trails the physical admission deadline by a
	// small delivery grace. Renderer/Main/Scheduler enforce the exact deadline;
	// this grace prevents an action_started/command_accepted event that occurred
	// just before expiry from losing a race against the asynchronous reconciler.
	expiryReconcileBefore := now.Add(-ephemeralExpiryReconcileGrace).Format(runtimeCommandExpiryLayout)

	// Admission/transport timeout applies only before physical playback starts.
	if err := s.db.Where(
		`status IN (?, ?, ?, ?, ?, ?, ?) AND (
			(durability = 'ephemeral' AND expires_at <> '' AND expires_at IS NOT NULL AND expires_at <= ?) OR
			(durability = 'ephemeral' AND (expires_at = '' OR expires_at IS NULL) AND datetime(updated_at) < datetime(?)) OR
			(durability <> 'ephemeral' AND datetime(updated_at) < datetime(?))
		)`,
		string(CommandStatusCreated), string(CommandStatusQueued),
		string(CommandStatusDispatching), string(CommandStatusTransportDispatched),
		string(CommandStatusRuntimeReceived), string(CommandStatusRuntimeAccepted),
		string(CommandStatusRendererAccepted),
		expiryReconcileBefore, ackExpiredBefore, ackExpiredBefore,
	).Order("updated_at ASC").Limit(batchSize).Find(&cmds).Error; err != nil {
		return nil, err
	}
	if len(cmds) >= batchSize {
		return cmds, nil
	}

	// Started playback uses a distinct liveness budget. Do not reuse command TTL
	// or a fixed transport timeout: either would kill legitimate long actions.
	var started []*RuntimeCommand
	if err := s.db.Where(
		"status = ? AND durability = 'ephemeral'",
		string(CommandStatusPlaybackStarted),
	).Order("updated_at ASC").Find(&started).Error; err != nil {
		return nil, err
	}
	for _, cmd := range started {
		deadline, ok := playbackLivenessDeadline(cmd, timeoutSec)
		if !ok || now.Before(deadline) {
			continue
		}
		cmds = append(cmds, cmd)
		if len(cmds) >= batchSize {
			break
		}
	}
	return cmds, nil
}
