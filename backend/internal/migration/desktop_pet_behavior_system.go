package migration

import (
	"github.com/u-ai/backend/internal/desktoppet/behavior/persistence"
)

func DesktopPetBehaviorMigration() Migration {
	return Migration{
		Version:           "202607310008",
		Name:              "add_desktop_pet_behavior_system",
		AcceptedChecksums: []string{"0a271c28e730dfbb93cf406e7df7a8cf5b9102e9ad2cced882f6bbd50dcfb8ff"},
		Up: func(s *Step) error {
			for _, sql := range persistence.DesktopPetBehaviorTableSQL {
				s.CreateTable(sql)
			}
			for _, idx := range persistence.DesktopPetBehaviorIndexDefs {
				if err := s.CreateIndex(idx.Name, idx.Table, idx.Columns, idx.Unique); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func DesktopPetBehaviorV2ColumnsMigration() Migration {
	return Migration{
		Version:           "202608020001",
		Name:              "add_behavior_v2_columns",
		AcceptedChecksums: []string{"6df3d911385f6a9424fe92d55998c005189d475e5fdb98cc3b3b7c2d427fad58"},
		Up: func(s *Step) error {
			_ = s.AddColumn("desktop_pet_behavior_contexts", "desktop_gesture_json", "TEXT NOT NULL DEFAULT '{}'")
			_ = s.AddColumn("desktop_pet_behavior_contexts", "foreground_json", "TEXT NOT NULL DEFAULT '{}'")
			_ = s.AddColumn("desktop_pet_behavior_contexts", "cooldowns_json", "TEXT NOT NULL DEFAULT '{}'")
			_ = s.AddColumn("desktop_pet_behavior_contexts", "recent_semantics_json", "TEXT NOT NULL DEFAULT '[]'")
			_ = s.AddColumn("desktop_pet_behavior_contexts", "last_source_revisions_json", "TEXT NOT NULL DEFAULT '{}'")

			_ = s.AddColumn("desktop_pet_behavior_inbox", "conversation_id", "TEXT NOT NULL DEFAULT ''")
			_ = s.AddColumn("desktop_pet_behavior_inbox", "interaction_id", "TEXT NOT NULL DEFAULT ''")
			_ = s.AddColumn("desktop_pet_behavior_inbox", "session_id", "TEXT NOT NULL DEFAULT ''")
			_ = s.AddColumn("desktop_pet_behavior_inbox", "tool_operation_id", "TEXT NOT NULL DEFAULT ''")
			_ = s.AddColumn("desktop_pet_behavior_inbox", "installation_id", "TEXT NOT NULL DEFAULT ''")
			_ = s.AddColumn("desktop_pet_behavior_inbox", "pet_instance_id", "TEXT NOT NULL DEFAULT ''")
			_ = s.AddColumn("desktop_pet_behavior_inbox", "release_id", "TEXT NOT NULL DEFAULT ''")
			_ = s.AddColumn("desktop_pet_behavior_inbox", "event_envelope_json", "TEXT NOT NULL DEFAULT '{}'")
			_ = s.AddColumn("desktop_pet_behavior_inbox", "payload_hash", "TEXT NOT NULL DEFAULT ''")
			_ = s.AddColumn("desktop_pet_behavior_inbox", "lease_owner", "TEXT NOT NULL DEFAULT ''")
			_ = s.AddColumn("desktop_pet_behavior_inbox", "lease_expires_at", "TEXT NOT NULL DEFAULT ''")
			_ = s.AddColumn("desktop_pet_behavior_inbox", "heartbeat_at", "TEXT NOT NULL DEFAULT ''")
			_ = s.AddColumn("desktop_pet_behavior_inbox", "available_at", "TEXT NOT NULL DEFAULT ''")
			_ = s.AddColumn("desktop_pet_behavior_inbox", "last_error_message", "TEXT NOT NULL DEFAULT ''")

			_ = s.AddColumn("desktop_pet_behavior_decisions", "interrupt_policy", "TEXT NOT NULL DEFAULT ''")
			_ = s.AddColumn("desktop_pet_behavior_decisions", "minimum_play_ms", "INTEGER NOT NULL DEFAULT 0")
			_ = s.AddColumn("desktop_pet_behavior_decisions", "maximum_play_ms", "INTEGER NOT NULL DEFAULT 0")

			return nil
		},
	}
}
