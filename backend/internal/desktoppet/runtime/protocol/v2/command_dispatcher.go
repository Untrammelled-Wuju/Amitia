// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package v2

import (
	"context"
	"encoding/json"
	"time"

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
	if conn == nil || conn.SessionID == "" {
		return
	}
	if conn.GetState() != ConnStateConnected {
		return
	}

	cmds, err := d.commands.ListCommandsToDispatch(100)
	if err != nil {
		return
	}

	now := time.Now().Format("2006-01-02 15:04:05")

	for _, cmd := range cmds {
		if cmd.UserID != conn.UserID || cmd.DeviceID != conn.DeviceID {
			continue
		}

		if err := d.commands.MarkDispatching(cmd.ID, conn.RuntimeID, time.Now()); err != nil {
			continue
		}

		var payload interface{}
		if err := json.Unmarshal([]byte(cmd.PayloadJSON), &payload); err != nil {
			_ = d.commands.MarkFailed(cmd.ID, "PAYLOAD_INVALID", err.Error(), time.Now())
			continue
		}

		envelope, err := d.handler.CreateEnvelope(
			MessageTypeCommand,
			cmd.CommandType,
			conn.RuntimeID,
			conn.SessionID,
			map[string]interface{}{
				"commandId":          cmd.ID,
				"commandType":        cmd.CommandType,
				"commandSequence":    cmd.DeviceSequence,
				"desiredRevision":    cmd.DesiredRevision,
				"settingsRevision":   cmd.SettingsRevision,
				"installationId":     cmd.InstallationID,
				"petId":              cmd.PetID,
				"releaseId":          cmd.ReleaseID,
				"payload":            payload,
			},
			conn.UserID,
			conn.DeviceID,
		)
		if err != nil {
			log.Warn("[v2-dispatcher] create envelope failed: ", err)
			continue
		}

		envelope.MessageID = cmd.ID
		envelope.Sequence = conn.LastSeq + 1

		if err := send(envelope, now); err != nil {
			log.Warn("[v2-dispatcher] send failed: ", err)
			continue
		}

		_ = d.commands.MarkTransportDispatched(cmd.ID, conn.RuntimeID, time.Now())
	}
}
