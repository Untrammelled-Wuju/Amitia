package migration

func DesktopPetDataConsistencyMigration() Migration {
	return Migration{
		Version: "202607300004",
		Name:    "desktop_pet_data_consistency_rebuild",
		Up: func(s *Step) error {
			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_data_repair_audit (
id INTEGER PRIMARY KEY AUTOINCREMENT,
migration_version TEXT NOT NULL DEFAULT '',
entity_type TEXT NOT NULL DEFAULT '',
entity_id TEXT NOT NULL DEFAULT '',
group_key TEXT NOT NULL DEFAULT '',
decision TEXT NOT NULL DEFAULT '',
kept_id TEXT NOT NULL DEFAULT '',
removed_ids TEXT NOT NULL DEFAULT '[]',
details TEXT NOT NULL DEFAULT '{}',
created_at TEXT NOT NULL DEFAULT (datetime('now'))
)`)

			if err := s.AddColumn("desktop_pet_generation_task_actions", "row_version", "INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}

			if err := s.AddColumn("desktop_pet_generation_frames", "generation_attempt", "INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_generation_frames", "provider_attempt", "INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}

			s.Execute(`UPDATE desktop_pet_generation_frames SET
generation_attempt = COALESCE(
	NULLIF((SELECT MAX(l.attempt_number) FROM desktop_pet_generation_call_logs l WHERE l.frame_id = desktop_pet_generation_frames.id AND l.attempt_number > 0), 0),
	NULLIF((SELECT a.current_attempt FROM desktop_pet_generation_task_actions a WHERE a.id = desktop_pet_generation_frames.task_action_id), 0),
	1
),
provider_attempt = attempt_number
WHERE generation_attempt IS NULL OR generation_attempt = 0`)

			s.Execute(`CREATE TEMP TABLE IF NOT EXISTS tmp_keep_gta AS
SELECT b.id AS keep_id, b.task_id, b.action_key
FROM desktop_pet_generation_task_actions b
WHERE EXISTS (
	SELECT 1 FROM desktop_pet_generation_task_actions c
	WHERE c.task_id = b.task_id AND c.action_key = b.action_key AND c.id <> b.id
)
AND b.id = (
	SELECT a.id FROM desktop_pet_generation_task_actions a
	WHERE a.task_id = b.task_id AND a.action_key = b.action_key
	ORDER BY
		CASE a.status WHEN 'succeeded' THEN 0 WHEN 'processing' THEN 1 WHEN 'pending' THEN 2 WHEN 'failed' THEN 3 WHEN 'cancelled' THEN 4 ELSE 5 END,
		a.current_attempt DESC,
		a.attempt_number DESC,
		a.updated_at DESC,
		a.id DESC
	LIMIT 1
)`)

			s.Execute(`CREATE TEMP TABLE IF NOT EXISTS tmp_gta_remap AS
SELECT a.id AS old_id, k.keep_id AS new_id
FROM desktop_pet_generation_task_actions a
JOIN tmp_keep_gta k ON k.task_id = a.task_id AND k.action_key = a.action_key
WHERE a.id <> k.keep_id`)

			s.Execute(`UPDATE desktop_pet_generation_frames
SET task_action_id = (SELECT r.new_id FROM tmp_gta_remap r WHERE r.old_id = desktop_pet_generation_frames.task_action_id)
WHERE task_action_id IN (SELECT old_id FROM tmp_gta_remap)`)

			s.Execute(`INSERT INTO desktop_pet_data_repair_audit (migration_version, entity_type, entity_id, group_key, decision, kept_id, removed_ids, details)
SELECT '202607300004', 'generation_task_action', r.old_id, a.task_id || ':' || a.action_key, 'merged_duplicate', r.new_id, '["' || r.old_id || '"]', '{}'
FROM tmp_gta_remap r
JOIN desktop_pet_generation_task_actions a ON a.id = r.old_id`)

			s.Execute(`DELETE FROM desktop_pet_generation_task_actions
WHERE id IN (SELECT old_id FROM tmp_gta_remap)`)

			s.Execute(`DELETE FROM desktop_pet_generation_frames
WHERE id IN (
	SELECT f.id FROM desktop_pet_generation_frames f
	WHERE EXISTS (
		SELECT 1 FROM desktop_pet_generation_frames g
		WHERE g.task_action_id = f.task_action_id
		AND g.generation_attempt = f.generation_attempt
		AND g.frame_index = f.frame_index
		AND (
			CASE f.status WHEN 'succeeded' THEN 0 WHEN 'processing' THEN 1 WHEN 'pending' THEN 2 WHEN 'failed' THEN 3 WHEN 'cancelled' THEN 4 ELSE 5 END
			>
			CASE g.status WHEN 'succeeded' THEN 0 WHEN 'processing' THEN 1 WHEN 'pending' THEN 2 WHEN 'failed' THEN 3 WHEN 'cancelled' THEN 4 ELSE 5 END
			OR (
				CASE f.status WHEN 'succeeded' THEN 0 WHEN 'processing' THEN 1 WHEN 'pending' THEN 2 WHEN 'failed' THEN 3 WHEN 'cancelled' THEN 4 ELSE 5 END
				=
				CASE g.status WHEN 'succeeded' THEN 0 WHEN 'processing' THEN 1 WHEN 'pending' THEN 2 WHEN 'failed' THEN 3 WHEN 'cancelled' THEN 4 ELSE 5 END
				AND g.updated_at > f.updated_at
			)
			OR (
				CASE f.status WHEN 'succeeded' THEN 0 WHEN 'processing' THEN 1 WHEN 'pending' THEN 2 WHEN 'failed' THEN 3 WHEN 'cancelled' THEN 4 ELSE 5 END
				=
				CASE g.status WHEN 'succeeded' THEN 0 WHEN 'processing' THEN 1 WHEN 'pending' THEN 2 WHEN 'failed' THEN 3 WHEN 'cancelled' THEN 4 ELSE 5 END
				and g.updated_at = f.updated_at
				and g.id > f.id
			)
		)
	)
)`)

			if err := s.AddColumn("desktop_pet_processing_tasks", "row_version", "INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}

			if err := s.AddColumn("desktop_pet_processing_actions", "processing_attempt", "INTEGER NOT NULL DEFAULT 1"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_processing_actions", "last_successful_attempt", "INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_processing_actions", "active_execution_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_processing_actions", "row_version", "INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}

			s.CreateTable(`CREATE TABLE IF NOT EXISTS desktop_pet_processing_action_attempts (
id TEXT PRIMARY KEY,
processing_action_id TEXT NOT NULL DEFAULT '',
processing_task_id TEXT NOT NULL DEFAULT '',
action_key TEXT NOT NULL DEFAULT '',
attempt_number INTEGER NOT NULL DEFAULT 1,
source_generation_attempt INTEGER NOT NULL DEFAULT 1,
execution_id TEXT NOT NULL DEFAULT '',
status TEXT NOT NULL DEFAULT 'pending',
progress INTEGER NOT NULL DEFAULT 0,
error_code TEXT NOT NULL DEFAULT '',
error_message TEXT NOT NULL DEFAULT '',
started_at TEXT NOT NULL DEFAULT '',
completed_at TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT (datetime('now')),
updated_at TEXT NOT NULL DEFAULT (datetime('now'))
)`)

			s.Execute(`CREATE TEMP TABLE IF NOT EXISTS tmp_keep_dppa AS
SELECT b.id AS keep_id, b.processing_task_id, b.action_key
FROM desktop_pet_processing_actions b
WHERE EXISTS (
	SELECT 1 FROM desktop_pet_processing_actions c
	WHERE c.processing_task_id = b.processing_task_id AND c.action_key = b.action_key AND c.id <> b.id
)
AND b.id = (
	SELECT a.id FROM desktop_pet_processing_actions a
	WHERE a.processing_task_id = b.processing_task_id AND a.action_key = b.action_key
	ORDER BY
		CASE a.status WHEN 'succeeded' THEN 0 WHEN 'processing' THEN 1 WHEN 'pending' THEN 2 WHEN 'failed' THEN 3 WHEN 'cancelled' THEN 4 ELSE 5 END,
		a.updated_at DESC,
		a.created_at DESC,
		a.id DESC
	LIMIT 1
)`)

			s.Execute(`CREATE TEMP TABLE IF NOT EXISTS tmp_dppa_remap AS
SELECT a.id AS old_id, k.keep_id AS new_id
FROM desktop_pet_processing_actions a
JOIN tmp_keep_dppa k ON k.processing_task_id = a.processing_task_id AND k.action_key = a.action_key
WHERE a.id <> k.keep_id`)

			s.Execute(`INSERT INTO desktop_pet_processing_action_attempts
(id, processing_action_id, processing_task_id, action_key, attempt_number,
 source_generation_attempt, execution_id, status, progress,
 error_code, error_message, started_at, completed_at)
SELECT
	'migr_' || a.id,
	r.new_id,
	a.processing_task_id,
	a.action_key,
	(SELECT COUNT(*) FROM desktop_pet_processing_actions c
	 WHERE c.processing_task_id = a.processing_task_id AND c.action_key = a.action_key
	 AND (c.created_at < a.created_at OR (c.created_at = a.created_at AND c.id < a.id))) + 1,
	a.source_attempt_number,
	'',
	a.status,
	a.progress,
	a.error_code,
	a.error_message,
	a.started_at,
	a.completed_at
FROM desktop_pet_processing_actions a
JOIN tmp_dppa_remap r ON r.old_id = a.id`)

			s.Execute(`INSERT INTO desktop_pet_data_repair_audit (migration_version, entity_type, entity_id, group_key, decision, kept_id, removed_ids, details)
SELECT '202607300004', 'processing_action', r.old_id, a.processing_task_id || ':' || a.action_key, 'converted_to_attempt', r.new_id, '["' || r.old_id || '"]', '{}'
FROM tmp_dppa_remap r
JOIN desktop_pet_processing_actions a ON a.id = r.old_id`)

			s.Execute(`UPDATE desktop_pet_processed_frames
SET processing_action_id = (SELECT r.new_id FROM tmp_dppa_remap r WHERE r.old_id = desktop_pet_processed_frames.processing_action_id)
WHERE processing_action_id IN (SELECT old_id FROM tmp_dppa_remap)`)

			s.Execute(`DELETE FROM desktop_pet_processing_actions
WHERE id IN (SELECT old_id FROM tmp_dppa_remap)`)

			s.Execute(`UPDATE desktop_pet_processing_actions
SET processing_attempt = COALESCE(
	(SELECT MAX(x.attempt_number) FROM desktop_pet_processing_action_attempts x WHERE x.processing_action_id = desktop_pet_processing_actions.id),
	1
),
last_successful_attempt = COALESCE(
	(SELECT MAX(x.attempt_number) FROM desktop_pet_processing_action_attempts x WHERE x.processing_action_id = desktop_pet_processing_actions.id AND x.status = 'succeeded'),
	0
)`)

			s.Execute(`INSERT INTO desktop_pet_processing_action_attempts
(id, processing_action_id, processing_task_id, action_key, attempt_number,
 source_generation_attempt, execution_id, status, progress,
 error_code, error_message, started_at, completed_at)
SELECT
	'migr_' || a.id,
	a.id,
	a.processing_task_id,
	a.action_key,
	1,
	a.source_attempt_number,
	'',
	a.status,
	a.progress,
	a.error_code,
	a.error_message,
	a.started_at,
	a.completed_at
FROM desktop_pet_processing_actions a
WHERE NOT EXISTS (
	SELECT 1 FROM desktop_pet_processing_action_attempts x WHERE x.processing_action_id = a.id
)`)

			if err := s.AddColumn("desktop_pet_processed_frames", "processing_attempt_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_processed_frames", "processing_attempt_number", "INTEGER NOT NULL DEFAULT 1"); err != nil {
				return err
			}
			if err := s.AddColumn("desktop_pet_processed_frames", "execution_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
				return err
			}

			s.Execute(`UPDATE desktop_pet_processed_frames
SET processing_attempt_id = COALESCE(
	(SELECT x.id FROM desktop_pet_processing_action_attempts x WHERE x.processing_action_id = desktop_pet_processed_frames.processing_action_id ORDER BY x.attempt_number DESC LIMIT 1),
	''
),
processing_attempt_number = COALESCE(
	(SELECT MAX(x.attempt_number) FROM desktop_pet_processing_action_attempts x WHERE x.processing_action_id = desktop_pet_processed_frames.processing_action_id),
	1
)
WHERE processing_attempt_id = '' OR processing_attempt_id IS NULL`)

			s.Execute(`DELETE FROM desktop_pet_processed_frames
WHERE id IN (
	SELECT f.id FROM desktop_pet_processed_frames f
	WHERE f.processing_attempt_id <> ''
	AND EXISTS (
		SELECT 1 FROM desktop_pet_processed_frames g
		WHERE g.processing_attempt_id = f.processing_attempt_id
		AND g.frame_index = f.frame_index
		AND (
			CASE f.status WHEN 'succeeded' THEN 0 WHEN 'processing' THEN 1 WHEN 'pending' THEN 2 WHEN 'failed' THEN 3 WHEN 'cancelled' THEN 4 ELSE 5 END
			>
			CASE g.status WHEN 'succeeded' THEN 0 WHEN 'processing' THEN 1 WHEN 'pending' THEN 2 WHEN 'failed' THEN 3 WHEN 'cancelled' THEN 4 ELSE 5 END
			OR (
				CASE f.status WHEN 'succeeded' THEN 0 WHEN 'processing' THEN 1 WHEN 'pending' THEN 2 WHEN 'failed' THEN 3 WHEN 'cancelled' THEN 4 ELSE 5 END
				=
				CASE g.status WHEN 'succeeded' THEN 0 WHEN 'processing' THEN 1 WHEN 'pending' THEN 2 WHEN 'failed' THEN 3 WHEN 'cancelled' THEN 4 ELSE 5 END
				and g.updated_at > f.updated_at
			)
			OR (
				CASE f.status WHEN 'succeeded' THEN 0 WHEN 'processing' THEN 1 WHEN 'pending' THEN 2 WHEN 'failed' THEN 3 WHEN 'cancelled' THEN 4 ELSE 5 END
				=
				CASE g.status WHEN 'succeeded' THEN 0 WHEN 'processing' THEN 1 WHEN 'pending' THEN 2 WHEN 'failed' THEN 3 WHEN 'cancelled' THEN 4 ELSE 5 END
				and g.updated_at = f.updated_at
				and g.id > f.id
			)
		)
	)
)`)

			s.Execute(`UPDATE desktop_pet_generation_tasks
SET selected_action_count = (
	SELECT COUNT(*) FROM desktop_pet_generation_task_actions a WHERE a.task_id = desktop_pet_generation_tasks.id
)
WHERE selected_action_count <> (
	SELECT COUNT(*) FROM desktop_pet_generation_task_actions a WHERE a.task_id = desktop_pet_generation_tasks.id
)`)

			s.Execute(`UPDATE desktop_pet_processing_tasks
SET execution_id = '', worker_id = '', lease_expires_at = '', last_heartbeat_at = ''
WHERE status = 'queued' AND execution_id <> ''`)

			s.Execute(`UPDATE desktop_pet_generation_tasks
SET execution_id = '', worker_id = '', lease_expires_at = '', last_heartbeat_at = ''
WHERE status = 'queued' AND execution_id <> ''`)

			if err := s.CreateIndex("uq_dpgta_task_action", "desktop_pet_generation_task_actions", []string{"task_id", "action_key"}, true); err != nil {
				return err
			}
			if err := s.CreateIndex("uq_dpgf_action_gen_frame", "desktop_pet_generation_frames", []string{"task_action_id", "generation_attempt", "frame_index"}, true); err != nil {
				return err
			}
			if err := s.CreateIndex("uq_dppt_gen_version", "desktop_pet_processing_tasks", []string{"generation_task_id", "processing_version"}, true); err != nil {
				return err
			}
			if err := s.CreateIndex("uq_dppa_task_action", "desktop_pet_processing_actions", []string{"processing_task_id", "action_key"}, true); err != nil {
				return err
			}
			if err := s.CreateIndex("uq_dppaa_action_attempt", "desktop_pet_processing_action_attempts", []string{"processing_action_id", "attempt_number"}, true); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_dppaa_task", "desktop_pet_processing_action_attempts", []string{"processing_task_id"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("uq_dppf_attempt_frame", "desktop_pet_processed_frames", []string{"processing_attempt_id", "frame_index"}, true); err != nil {
				return err
			}

			return nil
		},
	}
}
