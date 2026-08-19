// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetRuntimeProtocolV2Migration() Migration {
	return Migration{
		Version: "202607311001",
		Name:    "add_desktop_pet_runtime_protocol_v2_tables",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_runtime_sessions (
  runtime_instance_id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL DEFAULT '',
  device_id TEXT NOT NULL DEFAULT '',
  protocol_version TEXT NOT NULL DEFAULT '',
  client_version TEXT NOT NULL DEFAULT '',
  capabilities_json TEXT NOT NULL DEFAULT '{}',
  status TEXT NOT NULL DEFAULT '',
  connected_at TEXT NOT NULL DEFAULT '',
  disconnected_at TEXT NOT NULL DEFAULT '',
  superseded_by TEXT NOT NULL DEFAULT '',
  last_command_sequence INTEGER NOT NULL DEFAULT 0,
  last_event_sequence INTEGER NOT NULL DEFAULT 0
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_runtime_command_acks (
  ack_id TEXT PRIMARY KEY,
  command_id TEXT NOT NULL DEFAULT '',
  runtime_instance_id TEXT NOT NULL DEFAULT '',
  command_sequence INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT '',
  reject_reason TEXT NOT NULL DEFAULT '',
  received_at TEXT NOT NULL DEFAULT '',
  created_at TEXT DEFAULT (datetime('now')),
  UNIQUE(runtime_instance_id, command_id)
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_runtime_event_inbox (
  inbox_id TEXT PRIMARY KEY,
  event_id TEXT NOT NULL DEFAULT '',
  runtime_instance_id TEXT NOT NULL DEFAULT '',
  event_sequence INTEGER NOT NULL DEFAULT 0,
  event_type TEXT NOT NULL DEFAULT '',
  payload_hash TEXT NOT NULL DEFAULT '',
  processed_at TEXT NOT NULL DEFAULT '',
  created_at TEXT DEFAULT (datetime('now')),
  UNIQUE(runtime_instance_id, event_id),
  UNIQUE(runtime_instance_id, event_sequence)
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_runtime_protocol_errors (
  error_id TEXT PRIMARY KEY,
  runtime_instance_id TEXT NOT NULL DEFAULT '',
  error_code TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  component TEXT NOT NULL DEFAULT '',
  recoverable INTEGER NOT NULL DEFAULT 0,
  command_id TEXT NOT NULL DEFAULT '',
  playback_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  occurred_at TEXT NOT NULL DEFAULT '',
  created_at TEXT DEFAULT (datetime('now'))
)`)
			return nil
		},
	}
}
