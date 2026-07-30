// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

func DesktopPetStateMachineMigration() Migration {
	return Migration{
		Version: "202607300007",
		Name:    "add_desktop_pet_state_machine_fields_and_audit",
		Up: func(s *Step) error {
			if err := addStateMachineFieldsToGenerationTask(s); err != nil {
				return err
			}
			if err := addStateMachineFieldsToGenerationAction(s); err != nil {
				return err
			}
			if err := addStateMachineFieldsToGenerationFrame(s); err != nil {
				return err
			}
			if err := addStateMachineFieldsToProcessingTask(s); err != nil {
				return err
			}
			if err := addStateMachineFieldsToProcessingAction(s); err != nil {
				return err
			}
			if err := addStateMachineFieldsToProcessedFrame(s); err != nil {
				return err
			}
			if err := addStateMachineFieldsToPackage(s); err != nil {
				return err
			}
			if err := createDesktopPetStateTransitionsTable(s); err != nil {
				return err
			}
			if err := normalizeHistoricalStates(s); err != nil {
				return err
			}
			return nil
		},
	}
}

func addStateMachineFieldsToGenerationTask(s *Step) error {
	cols := [][2]string{
		{"row_version", "INTEGER NOT NULL DEFAULT 0"},
		{"status_reason", "TEXT NOT NULL DEFAULT ''"},
		{"failure_stage", "TEXT NOT NULL DEFAULT ''"},
		{"last_transition_at", "TEXT NOT NULL DEFAULT ''"},
		{"submitted_at", "TEXT NOT NULL DEFAULT ''"},
		{"cancelling_at", "TEXT NOT NULL DEFAULT ''"},
		{"cancelled_at", "TEXT NOT NULL DEFAULT ''"},
	}
	for _, c := range cols {
		if err := s.AddColumn("desktop_pet_generation_tasks", c[0], c[1]); err != nil {
			return err
		}
	}
	return nil
}

func addStateMachineFieldsToGenerationAction(s *Step) error {
	cols := [][2]string{
		{"row_version", "INTEGER NOT NULL DEFAULT 0"},
		{"current_stage", "TEXT NOT NULL DEFAULT 'created'"},
		{"status_reason", "TEXT NOT NULL DEFAULT ''"},
		{"failure_stage", "TEXT NOT NULL DEFAULT ''"},
		{"last_transition_at", "TEXT NOT NULL DEFAULT ''"},
		{"execution_id", "TEXT NOT NULL DEFAULT ''"},
		{"worker_id", "TEXT NOT NULL DEFAULT ''"},
	}
	for _, c := range cols {
		if err := s.AddColumn("desktop_pet_generation_task_actions", c[0], c[1]); err != nil {
			return err
		}
	}
	return nil
}

func addStateMachineFieldsToGenerationFrame(s *Step) error {
	cols := [][2]string{
		{"row_version", "INTEGER NOT NULL DEFAULT 0"},
		{"current_stage", "TEXT NOT NULL DEFAULT 'created'"},
		{"status_reason", "TEXT NOT NULL DEFAULT ''"},
		{"failure_stage", "TEXT NOT NULL DEFAULT ''"},
		{"last_transition_at", "TEXT NOT NULL DEFAULT ''"},
	}
	for _, c := range cols {
		if err := s.AddColumn("desktop_pet_generation_frames", c[0], c[1]); err != nil {
			return err
		}
	}
	return nil
}

func addStateMachineFieldsToProcessingTask(s *Step) error {
	cols := [][2]string{
		{"row_version", "INTEGER NOT NULL DEFAULT 0"},
		{"status_reason", "TEXT NOT NULL DEFAULT ''"},
		{"failure_stage", "TEXT NOT NULL DEFAULT ''"},
		{"last_transition_at", "TEXT NOT NULL DEFAULT ''"},
		{"submitted_at", "TEXT NOT NULL DEFAULT ''"},
		{"cancelling_at", "TEXT NOT NULL DEFAULT ''"},
		{"cancelled_at", "TEXT NOT NULL DEFAULT ''"},
	}
	for _, c := range cols {
		if err := s.AddColumn("desktop_pet_processing_tasks", c[0], c[1]); err != nil {
			return err
		}
	}
	return nil
}

func addStateMachineFieldsToProcessingAction(s *Step) error {
	cols := [][2]string{
		{"row_version", "INTEGER NOT NULL DEFAULT 0"},
		{"current_stage", "TEXT NOT NULL DEFAULT 'created'"},
		{"status_reason", "TEXT NOT NULL DEFAULT ''"},
		{"failure_stage", "TEXT NOT NULL DEFAULT ''"},
		{"last_transition_at", "TEXT NOT NULL DEFAULT ''"},
		{"execution_id", "TEXT NOT NULL DEFAULT ''"},
		{"worker_id", "TEXT NOT NULL DEFAULT ''"},
		{"attempt_number", "INTEGER NOT NULL DEFAULT 1"},
	}
	for _, c := range cols {
		if err := s.AddColumn("desktop_pet_processing_actions", c[0], c[1]); err != nil {
			return err
		}
	}
	return nil
}

func addStateMachineFieldsToProcessedFrame(s *Step) error {
	cols := [][2]string{
		{"row_version", "INTEGER NOT NULL DEFAULT 0"},
		{"current_stage", "TEXT NOT NULL DEFAULT 'created'"},
		{"status_reason", "TEXT NOT NULL DEFAULT ''"},
		{"failure_stage", "TEXT NOT NULL DEFAULT ''"},
		{"last_transition_at", "TEXT NOT NULL DEFAULT ''"},
		{"started_at", "TEXT NOT NULL DEFAULT ''"},
		{"completed_at", "TEXT NOT NULL DEFAULT ''"},
		{"execution_id", "TEXT NOT NULL DEFAULT ''"},
	}
	for _, c := range cols {
		if err := s.AddColumn("desktop_pet_processed_frames", c[0], c[1]); err != nil {
			return err
		}
	}
	return nil
}

func addStateMachineFieldsToPackage(s *Step) error {
	cols := [][2]string{
		{"row_version", "INTEGER NOT NULL DEFAULT 0"},
		{"current_stage", "TEXT NOT NULL DEFAULT 'created'"},
		{"status_reason", "TEXT NOT NULL DEFAULT ''"},
		{"failure_stage", "TEXT NOT NULL DEFAULT ''"},
		{"last_transition_at", "TEXT NOT NULL DEFAULT ''"},
		{"error_code", "TEXT NOT NULL DEFAULT ''"},
		{"error_message", "TEXT NOT NULL DEFAULT ''"},
	}
	for _, c := range cols {
		if err := s.AddColumn("desktop_pet_packages", c[0], c[1]); err != nil {
			return err
		}
	}
	return nil
}

func createDesktopPetStateTransitionsTable(s *Step) error {
	exists, err := s.TableExists("desktop_pet_state_transitions")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_state_transitions (
    id TEXT PRIMARY KEY,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    parent_task_id TEXT NOT NULL DEFAULT '',
    execution_id TEXT NOT NULL DEFAULT '',
    attempt_id TEXT NOT NULL DEFAULT '',
    from_status TEXT NOT NULL,
    to_status TEXT NOT NULL,
    from_stage TEXT NOT NULL DEFAULT '',
    to_stage TEXT NOT NULL DEFAULT '',
    reason_code TEXT NOT NULL,
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    failure_stage TEXT NOT NULL DEFAULT '',
    actor_type TEXT NOT NULL,
    actor_id TEXT NOT NULL DEFAULT '',
    previous_version INTEGER NOT NULL DEFAULT 0,
    current_version INTEGER NOT NULL DEFAULT 0,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL
)`)
	if err := s.CreateIndex("idx_dp_transition_entity", "desktop_pet_state_transitions", []string{"entity_type", "entity_id", "created_at"}, false); err != nil {
		return err
	}
	if err := s.CreateIndex("idx_dp_transition_parent", "desktop_pet_state_transitions", []string{"parent_task_id", "created_at"}, false); err != nil {
		return err
	}
	return nil
}

func normalizeHistoricalStates(s *Step) error {
	s.Execute(`UPDATE desktop_pet_generation_tasks SET status='cancelled', current_stage='cancelled', status_reason='migration.skipped_to_cancelled' WHERE status='skipped' AND cancel_requested_at!=''`)
	s.Execute(`UPDATE desktop_pet_generation_tasks SET status='failed', current_stage='failed', status_reason='migration.skipped_to_failed' WHERE status='skipped' AND (cancel_requested_at='' OR cancel_requested_at IS NULL)`)
	s.Execute(`UPDATE desktop_pet_generation_task_actions SET status='cancelled' WHERE status='skipped'`)
	s.Execute(`UPDATE desktop_pet_generation_task_actions SET status='failed' WHERE status='skipped'`)
	s.Execute(`UPDATE desktop_pet_generation_frames SET status='cancelled' WHERE status='skipped'`)
	s.Execute(`UPDATE desktop_pet_generation_frames SET status='failed' WHERE status='skipped'`)

	s.Execute(`UPDATE desktop_pet_processing_tasks SET status='cancelled', current_stage='cancelled', status_reason='migration.skipped_to_cancelled' WHERE status='skipped'`)
	s.Execute(`UPDATE desktop_pet_processing_tasks SET status='failed', current_stage='failed', status_reason='migration.skipped_to_failed' WHERE status='skipped'`)
	s.Execute(`UPDATE desktop_pet_processing_actions SET status='cancelled' WHERE status='skipped'`)
	s.Execute(`UPDATE desktop_pet_processing_actions SET status='failed' WHERE status='skipped'`)
	s.Execute(`UPDATE desktop_pet_processed_frames SET status='cancelled' WHERE status='skipped'`)
	s.Execute(`UPDATE desktop_pet_processed_frames SET status='failed' WHERE status='skipped'`)

	s.Execute(`UPDATE desktop_pet_processing_actions SET status='succeeded', status_reason='migration.warning_to_succeeded' WHERE status='warning'`)

	s.Execute(`UPDATE desktop_pet_generation_tasks SET current_stage='created' WHERE status='pending' AND current_stage='queued'`)
	s.Execute(`UPDATE desktop_pet_processing_tasks SET current_stage='created' WHERE status='pending' AND current_stage='queued'`)

	s.Execute(`UPDATE desktop_pet_generation_tasks SET current_stage='queued' WHERE status='queued' AND current_stage!='queued'`)
	s.Execute(`UPDATE desktop_pet_processing_tasks SET current_stage='queued' WHERE status='queued' AND current_stage!='queued'`)

	s.Execute(`UPDATE desktop_pet_generation_tasks SET current_stage='completed' WHERE status='succeeded' AND current_stage!='completed'`)
	s.Execute(`UPDATE desktop_pet_processing_tasks SET current_stage='completed' WHERE status='succeeded' AND current_stage!='completed'`)
	s.Execute(`UPDATE desktop_pet_generation_tasks SET current_stage='completed' WHERE status='partially_succeeded' AND current_stage!='completed'`)
	s.Execute(`UPDATE desktop_pet_processing_tasks SET current_stage='completed' WHERE status='partially_succeeded' AND current_stage!='completed'`)
	s.Execute(`UPDATE desktop_pet_generation_tasks SET current_stage='failed' WHERE status='failed' AND current_stage!='failed'`)
	s.Execute(`UPDATE desktop_pet_processing_tasks SET current_stage='failed' WHERE status='failed' AND current_stage!='failed'`)
	s.Execute(`UPDATE desktop_pet_generation_tasks SET current_stage='cancelled' WHERE status='cancelled' AND current_stage!='cancelled'`)
	s.Execute(`UPDATE desktop_pet_processing_tasks SET current_stage='cancelled' WHERE status='cancelled' AND current_stage!='cancelled'`)

	s.Execute(`UPDATE desktop_pet_generation_tasks SET completed_at=updated_at WHERE status IN ('succeeded','partially_succeeded','failed','cancelled') AND (completed_at='' OR completed_at IS NULL)`)
	s.Execute(`UPDATE desktop_pet_processing_tasks SET completed_at=updated_at WHERE status IN ('succeeded','partially_succeeded','failed','cancelled') AND (completed_at='' OR completed_at IS NULL)`)

	s.Execute(`UPDATE desktop_pet_generation_tasks SET completed_at='' WHERE status IN ('pending','queued','processing','cancelling') AND completed_at!=''`)
	s.Execute(`UPDATE desktop_pet_processing_tasks SET completed_at='' WHERE status IN ('pending','queued','processing','cancelling') AND completed_at!=''`)

	s.Execute(`UPDATE desktop_pet_processing_tasks
SET status='failed', current_stage='failed', failure_stage='packaging',
    error_code='desktop_pet_package_missing', error_message='package missing or invalid (migrated)',
    status_reason='migration.succeeded_without_package'
WHERE status IN ('succeeded','partially_succeeded')
  AND id NOT IN (SELECT processing_task_id FROM desktop_pet_packages WHERE status='ready')`)

	s.Execute(`UPDATE desktop_pet_generation_tasks SET cancelled_at=completed_at WHERE status='cancelled' AND (cancelled_at='' OR cancelled_at IS NULL)`)
	s.Execute(`UPDATE desktop_pet_processing_tasks SET cancelled_at=completed_at WHERE status='cancelled' AND (cancelled_at='' OR cancelled_at IS NULL)`)

	s.Execute(`UPDATE desktop_pet_generation_tasks SET cancelling_at=cancel_requested_at WHERE status='cancelling' AND (cancelling_at='' OR cancelling_at IS NULL)`)
	s.Execute(`UPDATE desktop_pet_processing_tasks SET cancelling_at=cancel_requested_at WHERE status='cancelling' AND (cancelling_at='' OR cancelling_at IS NULL)`)

	return nil
}
