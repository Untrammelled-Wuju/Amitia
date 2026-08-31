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
				columns := idx.Columns
				// Preserve the historical 202607310008 operation/checksum. The
				// final tenant-scoped dedup index is installed by the forward-only
				// 202608310002 migration below.
				if idx.Name == "ux_desktop_pet_behavior_inbox_dedup" {
					columns = []string{"dedup_key"}
				}
				if err := s.CreateIndex(idx.Name, idx.Table, columns, idx.Unique); err != nil {
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
			columns := []struct {
				table, name, definition string
			}{
				{"desktop_pet_behavior_contexts", "desktop_gesture_json", "TEXT NOT NULL DEFAULT '{}'"},
				{"desktop_pet_behavior_contexts", "foreground_json", "TEXT NOT NULL DEFAULT '{}'"},
				{"desktop_pet_behavior_contexts", "cooldowns_json", "TEXT NOT NULL DEFAULT '{}'"},
				{"desktop_pet_behavior_contexts", "recent_semantics_json", "TEXT NOT NULL DEFAULT '[]'"},
				{"desktop_pet_behavior_contexts", "last_source_revisions_json", "TEXT NOT NULL DEFAULT '{}'"},
				{"desktop_pet_behavior_inbox", "conversation_id", "TEXT NOT NULL DEFAULT ''"},
				{"desktop_pet_behavior_inbox", "interaction_id", "TEXT NOT NULL DEFAULT ''"},
				{"desktop_pet_behavior_inbox", "session_id", "TEXT NOT NULL DEFAULT ''"},
				{"desktop_pet_behavior_inbox", "tool_operation_id", "TEXT NOT NULL DEFAULT ''"},
				{"desktop_pet_behavior_inbox", "installation_id", "TEXT NOT NULL DEFAULT ''"},
				{"desktop_pet_behavior_inbox", "pet_instance_id", "TEXT NOT NULL DEFAULT ''"},
				{"desktop_pet_behavior_inbox", "release_id", "TEXT NOT NULL DEFAULT ''"},
				{"desktop_pet_behavior_inbox", "event_envelope_json", "TEXT NOT NULL DEFAULT '{}'"},
				{"desktop_pet_behavior_inbox", "payload_hash", "TEXT NOT NULL DEFAULT ''"},
				{"desktop_pet_behavior_inbox", "lease_owner", "TEXT NOT NULL DEFAULT ''"},
				{"desktop_pet_behavior_inbox", "lease_expires_at", "TEXT NOT NULL DEFAULT ''"},
				{"desktop_pet_behavior_inbox", "heartbeat_at", "TEXT NOT NULL DEFAULT ''"},
				{"desktop_pet_behavior_inbox", "available_at", "TEXT NOT NULL DEFAULT ''"},
				{"desktop_pet_behavior_inbox", "last_error_message", "TEXT NOT NULL DEFAULT ''"},
				{"desktop_pet_behavior_decisions", "interrupt_policy", "TEXT NOT NULL DEFAULT ''"},
				{"desktop_pet_behavior_decisions", "minimum_play_ms", "INTEGER NOT NULL DEFAULT 0"},
				{"desktop_pet_behavior_decisions", "maximum_play_ms", "INTEGER NOT NULL DEFAULT 0"},
			}
			for _, column := range columns {
				if err := s.AddColumn(column.table, column.name, column.definition); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

// DesktopPetBehaviorDecisionRecoveryMigration preserves all fields required to
// resume a committed behavior decision after a process crash. It is forward-only
// so already-applied behavior migrations keep their released checksums intact.
func DesktopPetBehaviorDecisionRecoveryMigration() Migration {
	return Migration{
		Version: "202608290006",
		Name:    "finalize_desktop_pet_behavior_decision_recovery",
		Up: func(s *Step) error {
			if err := s.AddColumn("desktop_pet_behavior_decisions", "fallback_depth", "INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_behavior_decisions", "return_policy", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_behavior_decisions", "context_hash", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			for _, idx := range []persistence.BehaviorIndexDef{
				{Name: "idx_behavior_inbox_status_available", Table: "desktop_pet_behavior_inbox", Columns: []string{"status", "available_at", "occurred_at"}},
				{Name: "idx_behavior_inbox_status_lease", Table: "desktop_pet_behavior_inbox", Columns: []string{"status", "lease_expires_at"}},
				{Name: "idx_behavior_decisions_event", Table: "desktop_pet_behavior_decisions", Columns: []string{"event_id", "created_at"}},
			} {
				if err := s.CreateIndex(idx.Name, idx.Table, idx.Columns, idx.Unique); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

// DesktopPetBehaviorReducerDedupMigration persists a bounded reducer-level
// event identity window. Durable/recoverable events are already protected by
// the inbox unique key, while this context-local fence also covers ephemeral
// events that are dispatched directly to the coordinator.
func DesktopPetBehaviorReducerDedupMigration() Migration {
	return Migration{
		Version: "202608310001",
		Name:    "finalize_desktop_pet_behavior_reducer_dedup",
		Up: func(s *Step) error {
			return s.AddColumn("desktop_pet_behavior_contexts", "recent_event_keys_json", "TEXT NOT NULL DEFAULT '[]'")
		},
	}
}
