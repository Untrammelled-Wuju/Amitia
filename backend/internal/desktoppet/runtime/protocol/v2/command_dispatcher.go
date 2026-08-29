// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package v2

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
	"github.com/u-ai/backend/log"
	"gorm.io/gorm"
)

type ConnectionCommandDispatcher struct {
	commands CommandService
	handler  *Handler
}

func NewConnectionCommandDispatcher(commands CommandService, handler *Handler) *ConnectionCommandDispatcher {
	return &ConnectionCommandDispatcher{
		commands: commands,
		handler:  handler,
	}
}

func (d *ConnectionCommandDispatcher) Run(
	ctx context.Context,
	conn *Connection,
	send func(*Envelope, string) error,
) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.dispatchOnce(conn, send)
		}
	}
}

func (d *ConnectionCommandDispatcher) dispatchOnce(conn *Connection, send func(*Envelope, string) error) {
	if conn == nil {
		return
	}

	// Hold the same reconnect fence used by inbound mutations. A replacement
	// websocket cannot supersede this connection between the state/generation
	// snapshot and command delivery.
	conn.fenceMu.RLock()
	defer conn.fenceMu.RUnlock()
	if conn.GetState() != ConnStateConnected || !conn.IsHandshakeAcked() {
		return
	}
	sessionID, generation := conn.SessionSnapshot()
	if sessionID == "" || generation <= 0 {
		return
	}

	cmds, err := d.commands.ListCommandsToDispatchForConnection(
		string(conn.UserID),
		string(conn.DeviceID),
		string(conn.RuntimeID),
		100,
	)
	if err != nil {
		log.Warn("[v2-dispatcher] list commands failed: ", err)
		return
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	for _, cmd := range cmds {
		if !cmd.HasValidClassification() {
			if markErr := d.commands.MarkFailed(cmd.ID, "COMMAND_CLASSIFICATION_INVALID", "stored runtime command type/durability classification is invalid", time.Now().UTC()); markErr != nil {
				log.Warn("[v2-dispatcher] mark invalid command classification failed: ", markErr)
			}
			continue
		}
		if !cmd.IsDurable() && cmd.RuntimeSessionID != sessionID {
			if markErr := d.commands.MarkSuperseded(cmd.ID, "ephemeral command is not bound to the active runtime session", time.Now().UTC()); markErr != nil {
				log.Warn("[v2-dispatcher] supersede stale ephemeral command failed: ", markErr)
			}
			continue
		}
		if !connectionSupportsCommand(conn, CommandType(cmd.CommandType)) {
			if markErr := d.commands.MarkFailed(cmd.ID, "RUNTIME_CAPABILITY_UNSUPPORTED", "connected runtime does not advertise required command capability", time.Now().UTC()); markErr != nil {
				log.Warn("[v2-dispatcher] mark unsupported command failed: ", markErr)
			}
			continue
		}
		if cmd.ExpiresAt != "" {
			expiresAt, parseErr := time.Parse(time.RFC3339Nano, cmd.ExpiresAt)
			if parseErr != nil {
				expiresAt, parseErr = time.Parse(time.RFC3339, cmd.ExpiresAt)
			}
			if parseErr != nil || !expiresAt.After(time.Now().UTC()) {
				if markErr := d.commands.MarkExpired(cmd.ID, time.Now().UTC()); markErr != nil {
					log.Warn("[v2-dispatcher] mark expired command failed: ", markErr)
				}
				continue
			}
		}
		targetRuntimeID := cmd.RuntimeID
		if targetRuntimeID == "" && cmd.CommandType == string(CommandTypePlayAction) {
			var playPayload PlayActionPayload
			if err := json.Unmarshal([]byte(cmd.PayloadJSON), &playPayload); err == nil {
				targetRuntimeID = playPayload.RuntimeID
			}
		}
		if targetRuntimeID != "" && targetRuntimeID != string(conn.RuntimeID) {
			continue
		}

		if err := d.commands.MarkDispatching(cmd.ID, string(conn.RuntimeID), sessionID, time.Now()); err != nil {
			continue
		}
		attemptID := "attempt_" + cmd.ID + "_" + fmt.Sprint(generation) + "_" + fmt.Sprint(time.Now().UTC().UnixNano())
		attemptNow := time.Now().UTC().Format("2006-01-02 15:04:05")
		attempt := &CommandAttempt{
			AttemptID: attemptID, CommandID: cmd.ID, RuntimeSessionID: sessionID,
			ConnectionGeneration: generation, InsertedAt: attemptNow, UpdatedAt: attemptNow,
		}
		if err := d.commands.DB().Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(attempt).Error; err != nil {
				return err
			}
			result := tx.Model(&RuntimeCommand{}).Where(
				"id = ? AND status = ?", cmd.ID, string(CommandStatusDispatching),
			).Update("last_attempt_id", attemptID)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return fmt.Errorf("bind command attempt lost dispatch CAS")
			}
			return nil
		}); err != nil {
			log.Warn("[v2-dispatcher] persist dispatch attempt failed: ", err)
			if cmd.IsDurable() {
				if markErr := d.commands.MarkFailedRetryable(cmd.ID, "ATTEMPT_PERSIST_FAILED", err.Error(), time.Now().UTC()); markErr != nil {
					log.Warn("[v2-dispatcher] mark attempt persistence retryable failure failed: ", markErr)
				}
			} else if markErr := d.commands.MarkFailed(cmd.ID, "ATTEMPT_PERSIST_FAILED", err.Error(), time.Now().UTC()); markErr != nil {
				log.Warn("[v2-dispatcher] mark attempt persistence terminal failure failed: ", markErr)
			}
			continue
		}

		var payload interface{}
		if err := json.Unmarshal([]byte(cmd.PayloadJSON), &payload); err != nil {
			if markErr := d.commands.MarkFailed(cmd.ID, "PAYLOAD_INVALID", err.Error(), time.Now()); markErr != nil {
				log.Warn("[v2-dispatcher] mark invalid payload failed: ", markErr)
			}
			continue
		}

		rawPayload, err := valueToRawMessage(payload)
		if err != nil {
			if markErr := d.commands.MarkFailed(cmd.ID, "PAYLOAD_ENCODE_FAILED", err.Error(), time.Now()); markErr != nil {
				log.Warn("[v2-dispatcher] mark payload encode failure failed: ", markErr)
			}
			continue
		}

		envelope, err := d.handler.CreateEnvelope(
			MessageTypeCommand,
			cmd.CommandType,
			conn.RuntimeID,
			runtimeidentity.ParseRuntimeSessionID(sessionID),
			CommandDispatchPayload{
				CommandID:        cmd.ID,
				CommandType:      cmd.CommandType,
				CommandSequence:  cmd.DeviceSequence,
				DesiredRevision:  cmd.DesiredRevision,
				SettingsRevision: cmd.SettingsRevision,
				InstallationID:   cmd.InstallationID,
				PetID:            cmd.PetID,
				ReleaseID:        cmd.ReleaseID,
				ExpiresAt:        cmd.ExpiresAt,
				Payload:          rawPayload,
			},
			conn.UserID,
			conn.DeviceID,
		)
		if err != nil {
			log.Warn("[v2-dispatcher] create envelope failed: ", err)
			if cmd.IsDurable() {
				if markErr := d.commands.MarkFailedRetryable(cmd.ID, "ENVELOPE_CREATE_FAILED", err.Error(), time.Now().UTC()); markErr != nil {
					log.Warn("[v2-dispatcher] mark retryable envelope failure failed: ", markErr)
				}
			} else if markErr := d.commands.MarkFailed(cmd.ID, "ENVELOPE_CREATE_FAILED", err.Error(), time.Now().UTC()); markErr != nil {
				log.Warn("[v2-dispatcher] mark terminal envelope failure failed: ", markErr)
			}
			continue
		}

		envelope.MessageID = cmd.ID
		envelope.ConnectionGeneration = generation
		envelope.Sequence = conn.NextOutboundSequence()

		if err := send(envelope, now); err != nil {
			log.Warn("[v2-dispatcher] send failed: ", err)
			if cmd.IsDurable() {
				if markErr := d.commands.MarkFailedRetryable(cmd.ID, "TRANSPORT_WRITE_FAILED", err.Error(), time.Now().UTC()); markErr != nil {
					log.Warn("[v2-dispatcher] mark retryable transport failure failed: ", markErr)
				}
			} else if markErr := d.commands.MarkFailed(cmd.ID, "TRANSPORT_WRITE_FAILED", err.Error(), time.Now().UTC()); markErr != nil {
				log.Warn("[v2-dispatcher] mark terminal transport failure failed: ", markErr)
			}
			continue
		}

		if err := d.commands.MarkTransportDispatched(cmd.ID, string(conn.RuntimeID), time.Now().UTC()); err != nil {
			log.Warn("[v2-dispatcher] mark transport dispatched failed: ", err)
		}
	}
}

func connectionSupportsCommand(conn *Connection, commandType CommandType) bool {
	if conn == nil {
		return false
	}
	switch commandType {
	case CommandTypeSyncDesiredState, CommandTypeEnsureAbsent, CommandTypeReloadRelease:
		return conn.HasCapability(CapabilitySyncDesiredV2)
	case CommandTypePlayAction:
		return conn.HasCapability(CapabilityPlayActionV2) &&
			conn.HasCapability(CapabilityRendererAckV2) &&
			conn.HasCapability(CapabilityExpiryRFC3339)
	case CommandTypeStopAction, CommandTypePauseAction, CommandTypeResumeAction, CommandTypeRecenterOnce:
		return conn.HasCapability(CapabilityExpiryRFC3339)
	default:
		return false
	}
}

func valueToRawMessage(v interface{}) (json.RawMessage, error) {
	if raw, ok := v.(json.RawMessage); ok {
		return raw, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("encode command payload: %w", err)
	}
	return b, nil
}
