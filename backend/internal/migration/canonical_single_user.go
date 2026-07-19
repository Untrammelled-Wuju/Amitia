package migration

func CanonicalSingleUserMigration() Migration {
	return Migration{
		Version: "202607180006",
		Name:    "canonicalize_single_user_temporal_relationship_data",
		Up: func(s *Step) error {
			s.Execute(`INSERT OR IGNORE INTO relationship_states (
id,
character_id,
relation_type,
relation_data,
created_at,
updated_at,
user_id,
channel
)
SELECT
'canonical:default:' || hex(latest.character_id),
latest.character_id,
'user_character',
latest.relation_data,
latest.created_at,
latest.updated_at,
'default',
'*'
FROM relationship_states AS latest
WHERE latest.rowid = (
    SELECT candidate.rowid
    FROM relationship_states AS candidate
    WHERE candidate.character_id = latest.character_id
    ORDER BY candidate.updated_at DESC, candidate.rowid DESC
    LIMIT 1
)
AND NOT EXISTS (
    SELECT 1
    FROM relationship_states AS canonical
    WHERE canonical.user_id = 'default'
      AND canonical.character_id = latest.character_id
      AND canonical.channel = '*'
      AND canonical.relation_type = 'user_character'
)`)

			s.Execute(`UPDATE relationship_states
SET relation_data = (
    SELECT latest.relation_data
    FROM relationship_states AS latest
    WHERE latest.character_id = relationship_states.character_id
    ORDER BY latest.updated_at DESC, latest.rowid DESC
    LIMIT 1
),
updated_at = (
    SELECT latest.updated_at
    FROM relationship_states AS latest
    WHERE latest.character_id = relationship_states.character_id
    ORDER BY latest.updated_at DESC, latest.rowid DESC
    LIMIT 1
)
WHERE user_id = 'default'
  AND channel = '*'
  AND relation_type = 'user_character'
  AND EXISTS (
    SELECT 1
    FROM relationship_states AS latest
    WHERE latest.character_id = relationship_states.character_id
      AND latest.updated_at > relationship_states.updated_at
    LIMIT 1
)`)

			s.Execute(`INSERT OR IGNORE INTO temporal_profiles (
id, owner_type, owner_id, timezone_mode, timezone, locale,
calendar_system, week_start, holiday_region, hemisphere,
daypart_config_json, quiet_hours_json, auto_detect_timezone,
travel_mode, awareness_level, source, confidence,
pending_timezone, enabled, holiday_awareness, daypart_awareness,
anniversary_awareness, memory_resonance, allow_shared_date_mention,
version, created_at_utc, updated_at_utc
)
SELECT
'canonical-profile:' || hex(owner_type) || ':' || hex('default'),
owner_type,
'default',
timezone_mode, timezone, locale,
calendar_system, week_start, holiday_region, hemisphere,
daypart_config_json, quiet_hours_json, auto_detect_timezone,
travel_mode, awareness_level, source, confidence,
pending_timezone, enabled, holiday_awareness, daypart_awareness,
anniversary_awareness, memory_resonance, allow_shared_date_mention,
version, created_at_utc, updated_at_utc
FROM temporal_profiles AS latest
WHERE latest.owner_type = 'user'
  AND latest.rowid = (
    SELECT candidate.rowid
    FROM temporal_profiles AS candidate
    WHERE candidate.owner_type = 'user'
    ORDER BY candidate.updated_at_utc DESC, candidate.rowid DESC
    LIMIT 1
  )
  AND NOT EXISTS (
    SELECT 1
    FROM temporal_profiles AS canonical
    WHERE canonical.owner_type = 'user'
      AND canonical.owner_id = 'default'
  )`)

			s.Execute(`UPDATE temporal_profiles
SET timezone_mode = latest.timezone_mode,
    timezone = latest.timezone,
    locale = latest.locale,
    holiday_region = latest.holiday_region,
    daypart_config_json = latest.daypart_config_json,
    quiet_hours_json = latest.quiet_hours_json,
    auto_detect_timezone = latest.auto_detect_timezone,
    travel_mode = latest.travel_mode,
    awareness_level = latest.awareness_level,
    source = latest.source,
    confidence = latest.confidence,
    pending_timezone = latest.pending_timezone,
    enabled = latest.enabled,
    holiday_awareness = latest.holiday_awareness,
    daypart_awareness = latest.daypart_awareness,
    anniversary_awareness = latest.anniversary_awareness,
    memory_resonance = latest.memory_resonance,
    allow_shared_date_mention = latest.allow_shared_date_mention,
    updated_at_utc = latest.updated_at_utc
FROM temporal_profiles AS latest
WHERE temporal_profiles.owner_type = 'user'
  AND temporal_profiles.owner_id = 'default'
  AND latest.owner_type = 'user'
  AND latest.owner_id != 'default'
  AND latest.updated_at_utc > temporal_profiles.updated_at_utc
  AND latest.rowid = (
    SELECT candidate.rowid
    FROM temporal_profiles AS candidate
    WHERE candidate.owner_type = 'user'
    ORDER BY candidate.updated_at_utc DESC, candidate.rowid DESC
    LIMIT 1
  )`)

			s.Execute(`UPDATE temporal_anchors
SET user_id = 'default'
WHERE user_id IS NOT NULL AND user_id != '' AND user_id != 'default'`)

			s.Execute(`UPDATE temporal_events
SET user_id = 'default'
WHERE user_id IS NOT NULL AND user_id != '' AND user_id != 'default'`)

			s.Execute(`INSERT OR IGNORE INTO temporal_global_presence_states (
user_id, first_user_activity_at_utc, last_observed_user_activity_at_utc,
last_committed_user_interaction_at_utc, last_channel, last_character_id,
interaction_count, session_count, state_version, created_at_utc, updated_at_utc
)
SELECT
'default',
MIN(all_rows.first_user_activity_at_utc),
MAX(all_rows.last_observed_user_activity_at_utc),
last_committed.last_committed_user_interaction_at_utc,
last_committed.last_channel,
last_committed.last_character_id,
last_committed.interaction_count,
last_committed.session_count,
MAX(all_rows.state_version),
MIN(all_rows.created_at_utc),
MAX(all_rows.updated_at_utc)
FROM temporal_global_presence_states AS all_rows
JOIN (
    SELECT user_id,
           last_committed_user_interaction_at_utc,
           last_channel,
           last_character_id,
           interaction_count,
           session_count
    FROM temporal_global_presence_states
    WHERE user_id IS NOT NULL AND user_id != ''
    ORDER BY last_committed_user_interaction_at_utc DESC
    LIMIT 1
) AS last_committed
WHERE all_rows.user_id IS NOT NULL AND all_rows.user_id != ''
  AND NOT EXISTS (
    SELECT 1
    FROM temporal_global_presence_states AS canonical
    WHERE canonical.user_id = 'default'
  )`)

			s.Execute(`UPDATE temporal_global_presence_states AS target
SET first_user_activity_at_utc = (
        SELECT MIN(first_user_activity_at_utc)
        FROM temporal_global_presence_states
        WHERE user_id IS NOT NULL AND user_id != ''
    ),
    last_observed_user_activity_at_utc = (
        SELECT MAX(last_observed_user_activity_at_utc)
        FROM temporal_global_presence_states
        WHERE user_id IS NOT NULL AND user_id != ''
    ),
    last_committed_user_interaction_at_utc = (
        SELECT last_committed_user_interaction_at_utc
        FROM temporal_global_presence_states
        WHERE user_id IS NOT NULL AND user_id != ''
        ORDER BY last_committed_user_interaction_at_utc DESC
        LIMIT 1
    ),
    last_channel = (
        SELECT last_channel
        FROM temporal_global_presence_states
        WHERE user_id IS NOT NULL AND user_id != ''
        ORDER BY last_committed_user_interaction_at_utc DESC
        LIMIT 1
    ),
    last_character_id = (
        SELECT last_character_id
        FROM temporal_global_presence_states
        WHERE user_id IS NOT NULL AND user_id != ''
        ORDER BY last_committed_user_interaction_at_utc DESC
        LIMIT 1
    ),
    interaction_count = (
        SELECT interaction_count
        FROM temporal_global_presence_states
        WHERE user_id IS NOT NULL AND user_id != ''
        ORDER BY last_committed_user_interaction_at_utc DESC
        LIMIT 1
    ),
    session_count = (
        SELECT session_count
        FROM temporal_global_presence_states
        WHERE user_id IS NOT NULL AND user_id != ''
        ORDER BY last_committed_user_interaction_at_utc DESC
        LIMIT 1
    ),
    updated_at_utc = (
        SELECT MAX(updated_at_utc)
        FROM temporal_global_presence_states
        WHERE user_id IS NOT NULL AND user_id != ''
    )
WHERE target.user_id = 'default'
  AND EXISTS (
    SELECT 1 FROM temporal_global_presence_states
    WHERE user_id IS NOT NULL AND user_id != '' AND user_id != 'default'
  )`)

			s.Execute(`INSERT OR IGNORE INTO temporal_relationship_presence_states (
id, user_id, character_id, first_interaction_at_utc,
last_observed_user_activity_at_utc, last_committed_user_interaction_at_utc,
last_successful_exchange_at_utc, last_assistant_contact_at_utc,
interaction_count, session_count, cadence_sample_count,
expected_gap_seconds, gap_mad_seconds, continuity_score,
reacclimation_turns_remaining, active_reunion_episode_id,
state_version, created_at_utc, updated_at_utc
)
SELECT
'canonical-rel:' || hex(latest.character_id),
'default',
latest.character_id,
latest.first_interaction_at_utc,
latest.last_observed_user_activity_at_utc,
latest.last_committed_user_interaction_at_utc,
latest.last_successful_exchange_at_utc,
COALESCE((
    SELECT MAX(last_assistant_contact_at_utc)
    FROM temporal_relationship_presence_states
    WHERE character_id = latest.character_id
      AND last_assistant_contact_at_utc != ''
), latest.last_assistant_contact_at_utc),
latest.interaction_count,
latest.session_count,
latest.cadence_sample_count,
latest.expected_gap_seconds,
latest.gap_mad_seconds,
latest.continuity_score,
latest.reacclimation_turns_remaining,
latest.active_reunion_episode_id,
latest.state_version,
latest.created_at_utc,
latest.updated_at_utc
FROM temporal_relationship_presence_states AS latest
WHERE latest.rowid = (
    SELECT candidate.rowid
    FROM temporal_relationship_presence_states AS candidate
    WHERE candidate.character_id = latest.character_id
    ORDER BY candidate.last_committed_user_interaction_at_utc DESC, candidate.rowid DESC
    LIMIT 1
)
AND NOT EXISTS (
    SELECT 1
    FROM temporal_relationship_presence_states AS canonical
    WHERE canonical.user_id = 'default'
      AND canonical.character_id = latest.character_id
)`)

			s.Execute(`UPDATE temporal_relationship_presence_states AS target
SET first_interaction_at_utc = (
        SELECT MIN(first_interaction_at_utc)
        FROM temporal_relationship_presence_states
        WHERE character_id = target.character_id
          AND first_interaction_at_utc != ''
    ),
    last_assistant_contact_at_utc = COALESCE((
        SELECT MAX(last_assistant_contact_at_utc)
        FROM temporal_relationship_presence_states
        WHERE character_id = target.character_id
          AND last_assistant_contact_at_utc != ''
    ), target.last_assistant_contact_at_utc),
    updated_at_utc = (
        SELECT MAX(updated_at_utc)
        FROM temporal_relationship_presence_states
        WHERE character_id = target.character_id
    )
WHERE target.user_id = 'default'
  AND EXISTS (
    SELECT 1
    FROM temporal_relationship_presence_states AS other
    WHERE other.character_id = target.character_id
      AND other.user_id != 'default'
      AND (other.updated_at_utc > target.updated_at_utc
           OR other.first_interaction_at_utc < target.first_interaction_at_utc
           OR (other.last_assistant_contact_at_utc != '' AND other.last_assistant_contact_at_utc > COALESCE(target.last_assistant_contact_at_utc, '')))
  )`)

			s.Execute(`UPDATE temporal_cadence_samples
SET user_id = 'default'
WHERE user_id IS NOT NULL AND user_id != '' AND user_id != 'default'`)

			s.Execute(`UPDATE temporal_reunion_episodes
SET user_id = 'default'
WHERE user_id IS NOT NULL AND user_id != '' AND user_id != 'default'`)

			s.Execute(`UPDATE temporal_effect_ledger
SET user_id = 'default'
WHERE user_id IS NOT NULL AND user_id != '' AND user_id != 'default'`)

			s.Execute(`UPDATE OR IGNORE temporal_interaction_receipts
SET user_id = 'default'
WHERE user_id IS NOT NULL AND user_id != '' AND user_id != 'default'`)

			return nil
		},
	}
}
