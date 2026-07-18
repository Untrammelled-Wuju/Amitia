package migration

func TemporalRelationshipTimeMigration() Migration {
	return Migration{
		Version: "202607180005",
		Name:    "add_temporal_relationship_time",
		Up: func(s *Step) error {
			s.Execute(`INSERT INTO relationship_states (
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
'canonical:' || hex(latest.user_id) || ':' || hex(latest.character_id),
latest.character_id,
'user_character',
latest.relation_data,
latest.created_at,
latest.updated_at,
latest.user_id,
'*'
FROM relationship_states AS latest
WHERE latest.rowid = (
    SELECT candidate.rowid
    FROM relationship_states AS candidate
    WHERE candidate.user_id = latest.user_id
      AND candidate.character_id = latest.character_id
    ORDER BY candidate.updated_at DESC, candidate.rowid DESC
    LIMIT 1
)
AND NOT EXISTS (
    SELECT 1
    FROM relationship_states AS canonical
    WHERE canonical.user_id = latest.user_id
      AND canonical.character_id = latest.character_id
      AND canonical.channel = '*'
      AND canonical.relation_type = 'user_character'
)`)
			s.Execute(`CREATE UNIQUE INDEX IF NOT EXISTS idx_relationship_canonical_scope
ON relationship_states(user_id, character_id, channel, relation_type)
WHERE channel = '*' AND relation_type = 'user_character'`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS temporal_global_presence_states (
user_id TEXT PRIMARY KEY,
first_user_activity_at_utc TEXT NOT NULL DEFAULT '',
last_observed_user_activity_at_utc TEXT NOT NULL DEFAULT '',
last_committed_user_interaction_at_utc TEXT NOT NULL DEFAULT '',
last_channel TEXT NOT NULL DEFAULT '',
last_character_id TEXT NOT NULL DEFAULT '',
interaction_count INTEGER NOT NULL DEFAULT 0,
session_count INTEGER NOT NULL DEFAULT 0,
state_version INTEGER NOT NULL DEFAULT 0,
created_at_utc TEXT NOT NULL DEFAULT '',
updated_at_utc TEXT NOT NULL DEFAULT ''
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS temporal_relationship_presence_states (
id TEXT PRIMARY KEY,
user_id TEXT NOT NULL DEFAULT 'default',
character_id TEXT NOT NULL DEFAULT '',
first_interaction_at_utc TEXT NOT NULL DEFAULT '',
last_observed_user_activity_at_utc TEXT NOT NULL DEFAULT '',
last_committed_user_interaction_at_utc TEXT NOT NULL DEFAULT '',
last_successful_exchange_at_utc TEXT NOT NULL DEFAULT '',
last_assistant_contact_at_utc TEXT NOT NULL DEFAULT '',
interaction_count INTEGER NOT NULL DEFAULT 0,
session_count INTEGER NOT NULL DEFAULT 0,
cadence_sample_count INTEGER NOT NULL DEFAULT 0,
expected_gap_seconds REAL NOT NULL DEFAULT 86400,
gap_mad_seconds REAL NOT NULL DEFAULT 0,
continuity_score REAL NOT NULL DEFAULT 1,
reacclimation_turns_remaining INTEGER NOT NULL DEFAULT 0,
active_reunion_episode_id TEXT NOT NULL DEFAULT '',
state_version INTEGER NOT NULL DEFAULT 0,
created_at_utc TEXT NOT NULL DEFAULT '',
updated_at_utc TEXT NOT NULL DEFAULT ''
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS temporal_cadence_samples (
id TEXT PRIMARY KEY,
user_id TEXT NOT NULL DEFAULT 'default',
character_id TEXT NOT NULL DEFAULT '',
interaction_id TEXT NOT NULL DEFAULT '',
previous_interaction_at_utc TEXT NOT NULL DEFAULT '',
current_interaction_at_utc TEXT NOT NULL DEFAULT '',
gap_seconds REAL NOT NULL DEFAULT 0,
sample_kind TEXT NOT NULL DEFAULT 'relationship',
included INTEGER NOT NULL DEFAULT 1,
created_at_utc TEXT NOT NULL DEFAULT ''
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS temporal_reunion_episodes (
id TEXT PRIMARY KEY,
user_id TEXT NOT NULL DEFAULT 'default',
character_id TEXT NOT NULL DEFAULT '',
reunion_kind TEXT NOT NULL DEFAULT '',
reunion_level TEXT NOT NULL DEFAULT '',
status TEXT NOT NULL DEFAULT 'pending',
previous_relationship_interaction_at_utc TEXT NOT NULL DEFAULT '',
previous_global_interaction_at_utc TEXT NOT NULL DEFAULT '',
detected_at_utc TEXT NOT NULL DEFAULT '',
relationship_gap_seconds REAL NOT NULL DEFAULT 0,
global_gap_seconds REAL NOT NULL DEFAULT 0,
expected_gap_seconds REAL NOT NULL DEFAULT 86400,
normalized_gap REAL NOT NULL DEFAULT 0,
deviation_score REAL NOT NULL DEFAULT 0,
continuity_before REAL NOT NULL DEFAULT 1,
claim_interaction_id TEXT NOT NULL DEFAULT '',
claim_expires_at_utc TEXT NOT NULL DEFAULT '',
handled_interaction_id TEXT NOT NULL DEFAULT '',
handled_at_utc TEXT NOT NULL DEFAULT '',
suppression_reason TEXT NOT NULL DEFAULT '',
policy_json TEXT NOT NULL DEFAULT '{}',
idempotency_key TEXT NOT NULL DEFAULT '',
created_at_utc TEXT NOT NULL DEFAULT '',
updated_at_utc TEXT NOT NULL DEFAULT ''
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS temporal_interaction_receipts (
id TEXT PRIMARY KEY,
request_id TEXT NOT NULL DEFAULT '',
interaction_id TEXT NOT NULL DEFAULT '',
user_id TEXT NOT NULL DEFAULT 'default',
character_id TEXT NOT NULL DEFAULT '',
channel TEXT NOT NULL DEFAULT '',
peer_id TEXT NOT NULL DEFAULT '',
observed_at_utc TEXT NOT NULL DEFAULT '',
previous_global_committed_at_utc TEXT NOT NULL DEFAULT '',
previous_relationship_committed_at_utc TEXT NOT NULL DEFAULT '',
reunion_episode_id TEXT NOT NULL DEFAULT '',
status TEXT NOT NULL DEFAULT 'observed',
created_at_utc TEXT NOT NULL DEFAULT '',
updated_at_utc TEXT NOT NULL DEFAULT ''
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS temporal_effect_ledger (
id TEXT PRIMARY KEY,
effect_key TEXT NOT NULL DEFAULT '',
effect_type TEXT NOT NULL DEFAULT '',
user_id TEXT NOT NULL DEFAULT 'default',
character_id TEXT NOT NULL DEFAULT '',
reunion_episode_id TEXT NOT NULL DEFAULT '',
interaction_id TEXT NOT NULL DEFAULT '',
payload_json TEXT NOT NULL DEFAULT '{}',
applied_at_utc TEXT NOT NULL DEFAULT ''
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS temporal_relationship_time_settings (
character_id TEXT PRIMARY KEY,
enabled INTEGER NOT NULL DEFAULT 1,
reunion_enabled INTEGER NOT NULL DEFAULT 1,
sensitivity TEXT NOT NULL DEFAULT 'balanced',
allow_memory_recall INTEGER NOT NULL DEFAULT 1,
allow_relationship_age INTEGER NOT NULL DEFAULT 1,
allow_reunion_mention INTEGER NOT NULL DEFAULT 1,
allow_proactive_reference INTEGER NOT NULL DEFAULT 1,
max_mention_sentences INTEGER NOT NULL DEFAULT 1,
updated_at_utc TEXT NOT NULL DEFAULT ''
)`)
			if err := s.CreateIndex("idx_temporal_relationship_presence_scope", "temporal_relationship_presence_states", []string{"user_id", "character_id"}, true); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_temporal_cadence_interaction", "temporal_cadence_samples", []string{"interaction_id", "sample_kind"}, true); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_temporal_cadence_scope_time", "temporal_cadence_samples", []string{"user_id", "character_id", "sample_kind", "current_interaction_at_utc"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_temporal_reunion_idempotency", "temporal_reunion_episodes", []string{"idempotency_key"}, true); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_temporal_reunion_scope_time", "temporal_reunion_episodes", []string{"user_id", "character_id", "detected_at_utc"}, false); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_temporal_receipt_request", "temporal_interaction_receipts", []string{"user_id", "request_id"}, true); err != nil {
				return err
			}
			if err := s.CreateIndex("idx_temporal_receipt_interaction", "temporal_interaction_receipts", []string{"interaction_id"}, true); err != nil {
				return err
			}
			return s.CreateIndex("idx_temporal_effect_key", "temporal_effect_ledger", []string{"effect_key"}, true)
		},
	}
}
