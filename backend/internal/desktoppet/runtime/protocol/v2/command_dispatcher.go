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
	if conn.GetState() != ConnStateConnected {
		return
	}
	sessionID, generation := conn.SessionSnapshot()
	if sessionID == "" || generation <= 0 {
		return
	}

	cmds, err := d.commands.ListCommandsToDispatch(100)
	if err != nil {
		log.Warn("[v2-dispatcher] list commands failed: ", err)
		return
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	for _, cmd := range cmds {
		if cmd.UserID != string(conn.UserID) || cmd.DeviceID != string(conn.DeviceID) {
			continue
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

		if err := d.commands.MarkDispatching(cmd.ID, string(conn.RuntimeID), time.Now()); err != nil {
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
				Payload:          rawPayload,
			},
			conn.UserID,
			conn.DeviceID,
		)
		if err != nil {
			log.Warn("[v2-dispatcher] create envelope failed: ", err)
			continue
		}

		envelope.MessageID = cmd.ID
		envelope.ConnectionGeneration = generation
		envelope.Sequence = conn.NextOutboundSequence()

		if err := send(envelope, now); err != nil {
			log.Warn("[v2-dispatcher] send failed: ", err)
			continue
		}

		if err := d.commands.MarkTransportDispatched(cmd.ID, string(conn.RuntimeID), time.Now()); err != nil {
			log.Warn("[v2-dispatcher] mark transport dispatched failed: ", err)
		}
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
