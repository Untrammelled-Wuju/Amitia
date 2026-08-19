// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package migration

import (
	"fmt"

	"github.com/u-ai/backend/internal/desktoppet/contracts"
	"github.com/u-ai/backend/internal/desktoppet/specs"
)

func DesktopPetCatalogPopulateMigration() Migration {
	return Migration{
		Version:           "202607300008",
		Name:              "populate_desktop_pet_action_definitions_from_catalog",
		AcceptedChecksums: []string{"4d608d1bfbedb38bef7b10d47d15fe82d8d7a26183dc59df5f9d1210dc52d0c8"},
		Up: func(s *Step) error {
			populateCatalogProjections(s)
			return nil
		},
	}
}

func populateCatalogProjections(s *Step) {
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
		enabled := 0
		if spec.Identity.Enabled {
			enabled = 1
		}

		insertSQL := fmt.Sprintf(
			`INSERT OR IGNORE INTO desktop_pet_action_definitions
				(action_key, name, description, category_key, category_name,
				 supports_default_idle, recommended, enabled, sort_order,
				 definition_version, default_frame_count, estimated_generation_count,
				 source_type, schema_version, catalog_version, default_fps,
				 playback_mode, return_policy, return_action_key,
				 interruptible, interrupt_after_ms, minimum_play_ms, maximum_play_ms,
				 priority, cooldown_ms, mutex_group, queue_policy, dedup_window_ms,
				 anchor_profile, semantic_tags_json, generation_spec_version,
				 spec_json, spec_hash)
				VALUES ('%s', '%s', '%s', '%s', '%s',
					%d, %d, %d, %d,
					%d, %d, 1,
					'builtin', %d, %d, %d,
					'%s', '%s', '%s',
					%d, %d, %d, %d,
					%d, %d, '%s', '%s', %d,
					'%s', '%s', %d,
					'%s', '%s')`,
			sqlEscape(spec.Identity.Key),
			sqlEscape(spec.Identity.Name),
			sqlEscape(spec.Identity.Description),
			sqlEscape(spec.Identity.CategoryKey),
			sqlEscape(spec.Identity.CategoryName),
			supportsIdle,
			recommended,
			enabled,
			spec.Identity.ActionSortOrder,
			spec.Identity.DefinitionVersion,
			spec.Generation.FrameCount,
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
			sqlEscape(specJSON),
			specHash,
		)
		s.Execute(insertSQL)

		updateSQL := fmt.Sprintf(
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
				enabled=%d,
updated_at=strftime('%Y-%m-%d %H:%M:%S','now')
			WHERE action_key='%s'`,
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
			enabled,
			sqlEscape(spec.Identity.Key),
		)
		s.Execute(updateSQL)
	}
}
