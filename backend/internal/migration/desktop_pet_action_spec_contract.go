// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/desktoppet/contracts"
	"github.com/u-ai/backend/internal/desktoppet/specs"
)

func DesktopPetActionSpecContractMigration() Migration {
	return Migration{
		Version:           "202607300005",
		Name:              "add_desktop_pet_action_spec_contract",
		AcceptedChecksums: []string{"8a2714d2aac7144f000f36b87507cd24f23179b9c6ab2d8a8b54e942fbe93ada"},
		Up: func(s *Step) error {
			if err := addActionDefinitionColumns(s); err != nil {
				return err
			}
			if err := addGenerationTaskActionColumns(s); err != nil {
				return err
			}
			if err := addProcessingActionColumns(s); err != nil {
				return err
			}
			upsertCatalogProjections(s)
			fixDefaultIdleCandidates(s)
			backfillTaskSnapshots(s)
			backfillProcessingActions(s)
			return nil
		},
	}
}

func addActionDefinitionColumns(s *Step) error {
	cols := [][2]string{
		{"source_type", "TEXT NOT NULL DEFAULT 'builtin'"},
		{"schema_version", "INTEGER NOT NULL DEFAULT 1"},
		{"catalog_version", "INTEGER NOT NULL DEFAULT 1"},
		{"default_fps", "INTEGER NOT NULL DEFAULT 10"},
		{"playback_mode", "TEXT NOT NULL DEFAULT 'once'"},
		{"return_policy", "TEXT NOT NULL DEFAULT 'previous'"},
		{"return_action_key", "TEXT NOT NULL DEFAULT ''"},
		{"interruptible", "INTEGER NOT NULL DEFAULT 1"},
		{"interrupt_after_ms", "INTEGER NOT NULL DEFAULT 0"},
		{"minimum_play_ms", "INTEGER NOT NULL DEFAULT 0"},
		{"maximum_play_ms", "INTEGER NOT NULL DEFAULT 0"},
		{"priority", "INTEGER NOT NULL DEFAULT 0"},
		{"cooldown_ms", "INTEGER NOT NULL DEFAULT 0"},
		{"mutex_group", "TEXT NOT NULL DEFAULT ''"},
		{"queue_policy", "TEXT NOT NULL DEFAULT 'replace'"},
		{"dedup_window_ms", "INTEGER NOT NULL DEFAULT 0"},
		{"anchor_profile", "TEXT NOT NULL DEFAULT 'feet_center'"},
		{"semantic_tags_json", "TEXT NOT NULL DEFAULT '[]'"},
		{"generation_spec_version", "INTEGER NOT NULL DEFAULT 1"},
		{"spec_json", "TEXT NOT NULL DEFAULT '{}'"},
		{"spec_hash", "TEXT NOT NULL DEFAULT ''"},
	}
	for _, c := range cols {
		if err := s.AddColumn("desktop_pet_action_definitions", c[0], c[1]); err != nil {
			return err
		}
	}
	return nil
}

func addGenerationTaskActionColumns(s *Step) error {
	cols := [][2]string{
		{"action_spec_schema_version", "INTEGER NOT NULL DEFAULT 1"},
		{"action_spec_version", "INTEGER NOT NULL DEFAULT 1"},
		{"action_spec_json", "TEXT NOT NULL DEFAULT ''"},
		{"action_spec_hash", "TEXT NOT NULL DEFAULT ''"},
		{"playback_mode_snapshot", "TEXT NOT NULL DEFAULT ''"},
		{"default_fps_snapshot", "INTEGER NOT NULL DEFAULT 0"},
		{"return_policy_snapshot", "TEXT NOT NULL DEFAULT ''"},
		{"return_action_key_snapshot", "TEXT NOT NULL DEFAULT ''"},
		{"interruptible_snapshot", "INTEGER NOT NULL DEFAULT 1"},
		{"priority_snapshot", "INTEGER NOT NULL DEFAULT 0"},
		{"cooldown_ms_snapshot", "INTEGER NOT NULL DEFAULT 0"},
		{"mutex_group_snapshot", "TEXT NOT NULL DEFAULT ''"},
		{"anchor_profile_snapshot", "TEXT NOT NULL DEFAULT ''"},
	}
	for _, c := range cols {
		if err := s.AddColumn("desktop_pet_generation_task_actions", c[0], c[1]); err != nil {
			return err
		}
	}
	return nil
}

func addProcessingActionColumns(s *Step) error {
	cols := [][2]string{
		{"action_spec_schema_version", "INTEGER NOT NULL DEFAULT 1"},
		{"action_spec_version", "INTEGER NOT NULL DEFAULT 1"},
		{"action_spec_hash", "TEXT NOT NULL DEFAULT ''"},
		{"return_policy", "TEXT NOT NULL DEFAULT ''"},
		{"return_action_key", "TEXT NOT NULL DEFAULT ''"},
		{"interruptible", "INTEGER NOT NULL DEFAULT 1"},
		{"interrupt_after_ms", "INTEGER NOT NULL DEFAULT 0"},
		{"minimum_play_ms", "INTEGER NOT NULL DEFAULT 0"},
		{"maximum_play_ms", "INTEGER NOT NULL DEFAULT 0"},
		{"priority", "INTEGER NOT NULL DEFAULT 0"},
		{"cooldown_ms", "INTEGER NOT NULL DEFAULT 0"},
		{"mutex_group", "TEXT NOT NULL DEFAULT ''"},
		{"queue_policy", "TEXT NOT NULL DEFAULT 'replace'"},
		{"dedup_window_ms", "INTEGER NOT NULL DEFAULT 0"},
		{"anchor_profile", "TEXT NOT NULL DEFAULT 'feet_center'"},
		{"playback_mode", "TEXT NOT NULL DEFAULT ''"},
	}
	for _, c := range cols {
		if err := s.AddColumn("desktop_pet_processing_actions", c[0], c[1]); err != nil {
			return err
		}
	}
	return nil
}

func upsertCatalogProjections(s *Step) {
	allSpecs := specs.CatalogAll()
	for _, spec := range allSpecs {
		tagsJSON := tagsToJSON(spec.Identity.Tags)
		specJSON := specToJSON(spec)
		specHash := specs.HashSpec(spec)

		interruptible := 0
		if spec.Playback.Interruptible {
			interruptible = 1
		}
		supportsIdle := 0
		if spec.Identity.SupportsDefaultIdle {
			supportsIdle = 1
		}
		recommended := 0
		if spec.Identity.Recommended {
			recommended = 1
		}

		sql := fmt.Sprintf(
			`UPDATE desktop_pet_action_definitions SET
				source_type='builtin',
				schema_version=%d,
				catalog_version=%d,
				default_fps=%d,
				playback_mode='%s',
				return_policy='%s',
				return_action_key='%s',
				interruptible=%d,
				interrupt_after_ms=%d,
				minimum_play_ms=%d,
				maximum_play_ms=%d,
				priority=%d,
				cooldown_ms=%d,
				mutex_group='%s',
				queue_policy='%s',
				dedup_window_ms=%d,
				anchor_profile='%s',
				semantic_tags_json='%s',
				generation_spec_version=%d,
				definition_version=%d,
				spec_json='%s',
				spec_hash='%s',
				name='%s',
				description='%s',
				category_key='%s',
				category_name='%s',
				supports_default_idle=%d,
				recommended=%d,
				sort_order=%d,
				default_frame_count=%d,
			updated_at=strftime('%%Y-%%m-%%d %%H:%%M:%%S','now')
			WHERE action_key='%s' AND (catalog_version < %d OR catalog_version IS NULL OR playback_mode IS NULL OR playback_mode='')`,
			contracts.ActionSpecSchemaVersion,
			contracts.CatalogVersion,
			spec.Playback.DefaultFPS,
			string(spec.Playback.Mode),
			string(spec.Playback.ReturnPolicy),
			sqlEscape(spec.Playback.ReturnActionKey),
			interruptible,
			spec.Playback.InterruptAfterMS,
			spec.Playback.MinimumPlayMS,
			spec.Playback.MaximumPlayMS,
			spec.Playback.Priority,
			spec.Playback.CooldownMS,
			sqlEscape(spec.Playback.MutexGroup),
			string(spec.Playback.QueuePolicy),
			spec.Playback.DedupWindowMS,
			string(spec.Processing.AnchorProfile),
			sqlEscape(tagsJSON),
			spec.Generation.Version,
			spec.Identity.DefinitionVersion,
			sqlEscape(specJSON),
			specHash,
			sqlEscape(spec.Identity.Name),
			sqlEscape(spec.Identity.Description),
			sqlEscape(spec.Identity.CategoryKey),
			sqlEscape(spec.Identity.CategoryName),
			supportsIdle,
			recommended,
			spec.Identity.ActionSortOrder,
			spec.Generation.FrameCount,
			spec.Identity.Key,
			contracts.CatalogVersion,
		)
		s.Execute(sql)
	}
}

func fixDefaultIdleCandidates(s *Step) {
	s.Execute(`UPDATE desktop_pet_action_definitions SET supports_default_idle=0 WHERE action_key IN ('idle_blink', 'idle_look_around')`)
}

func backfillTaskSnapshots(s *Step) {
	s.Execute(`UPDATE desktop_pet_generation_task_actions SET
		action_spec_schema_version=1,
		action_spec_version=1,
		playback_mode_snapshot='once',
		default_fps_snapshot=10,
		return_policy_snapshot='previous',
		interruptible_snapshot=1,
		priority_snapshot=0,
		cooldown_ms_snapshot=0,
		mutex_group_snapshot='',
		anchor_profile_snapshot='feet_center'
	WHERE COALESCE(action_spec_json,'')=''`)
}

func backfillProcessingActions(s *Step) {
	s.Execute(`UPDATE desktop_pet_processing_actions SET
		playback_mode = CASE WHEN loop_type='loop' THEN 'loop' ELSE 'once' END,
		return_policy = CASE WHEN loop_type='loop' THEN 'none' ELSE 'previous' END,
		return_action_key = '',
		interruptible = 1,
		interrupt_after_ms = 0,
		minimum_play_ms = 0,
		maximum_play_ms = 0,
		priority = 0,
		cooldown_ms = 0,
		mutex_group = '',
		queue_policy = 'replace',
		dedup_window_ms = 0,
		anchor_profile = 'feet_center',
		action_spec_schema_version = 1,
		action_spec_version = 1
	WHERE COALESCE(playback_mode,'')=''`)
}

func sqlEscape(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func tagsToJSON(tags []string) string {
	if len(tags) == 0 {
		return "[]"
	}
	b, err := json.Marshal(tags)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func specToJSON(spec contracts.ActionSpec) string {
	normalized := contracts.NormalizeSpec(spec)
	b, err := json.Marshal(normalized)
	if err != nil {
		return "{}"
	}
	return sqlEscape(string(b))
}
