package persistence

type BehaviorIndexDef struct {
	Name    string
	Table   string
	Columns []string
	Unique  bool
}

var DesktopPetBehaviorTableSQL = []string{
	`CREATE TABLE IF NOT EXISTS desktop_pet_behavior_contexts (
    user_id TEXT NOT NULL DEFAULT '',
    character_id TEXT NOT NULL DEFAULT '',
    revision INTEGER NOT NULL DEFAULT 1,
    stable_state_json TEXT NOT NULL DEFAULT '{}',
    transient_state_json TEXT NOT NULL DEFAULT '{}',
    active_tools_json TEXT NOT NULL DEFAULT '{}',
    voice_state_json TEXT NOT NULL DEFAULT '{}',
    desktop_gesture_json TEXT NOT NULL DEFAULT '{}',
    foreground_json TEXT NOT NULL DEFAULT '{}',
    cooldowns_json TEXT NOT NULL DEFAULT '{}',
    recent_semantics_json TEXT NOT NULL DEFAULT '[]',
    recent_event_keys_json TEXT NOT NULL DEFAULT '[]',
    desired_state_json TEXT NOT NULL DEFAULT '{}',
    last_source_revisions_json TEXT NOT NULL DEFAULT '{}',
    updated_at TEXT NOT NULL DEFAULT '',
    PRIMARY KEY(user_id, character_id)
)`,
	`CREATE TABLE IF NOT EXISTS desktop_pet_behavior_inbox (
    event_id TEXT PRIMARY KEY,
    dedup_key TEXT NOT NULL DEFAULT '',
    event_type TEXT NOT NULL DEFAULT '',
    schema_version INTEGER NOT NULL DEFAULT 0,
    user_id TEXT NOT NULL DEFAULT '',
    character_id TEXT NOT NULL DEFAULT '',
    conversation_id TEXT NOT NULL DEFAULT '',
    interaction_id TEXT NOT NULL DEFAULT '',
    session_id TEXT NOT NULL DEFAULT '',
    tool_operation_id TEXT NOT NULL DEFAULT '',
    installation_id TEXT NOT NULL DEFAULT '',
    pet_instance_id TEXT NOT NULL DEFAULT '',
    release_id TEXT NOT NULL DEFAULT '',
    occurred_at TEXT NOT NULL DEFAULT '',
    received_at TEXT NOT NULL DEFAULT '',
    expires_at TEXT NOT NULL DEFAULT '',
    origin TEXT NOT NULL DEFAULT '',
    correlation_id TEXT NOT NULL DEFAULT '',
    causation_id TEXT NOT NULL DEFAULT '',
    event_envelope_json TEXT NOT NULL DEFAULT '{}',
    payload_json TEXT NOT NULL DEFAULT '{}',
    payload_hash TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    lease_owner TEXT NOT NULL DEFAULT '',
    lease_expires_at TEXT NOT NULL DEFAULT '',
    heartbeat_at TEXT NOT NULL DEFAULT '',
    available_at TEXT NOT NULL DEFAULT '',
    last_error_code TEXT NOT NULL DEFAULT '',
    last_error_message TEXT NOT NULL DEFAULT '',
    processed_at TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT ''
)`,
	`CREATE TABLE IF NOT EXISTS desktop_pet_behavior_decisions (
    decision_id TEXT PRIMARY KEY,
    event_id TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
    character_id TEXT NOT NULL DEFAULT '',
    installation_id TEXT NOT NULL DEFAULT '',
    context_revision INTEGER NOT NULL DEFAULT 0,
    ruleset_version INTEGER NOT NULL DEFAULT 0,
    interrupt_policy TEXT NOT NULL DEFAULT '',
    minimum_play_ms INTEGER NOT NULL DEFAULT 0,
    maximum_play_ms INTEGER NOT NULL DEFAULT 0,
    fallback_depth INTEGER NOT NULL DEFAULT 0,
    return_policy TEXT NOT NULL DEFAULT '',
    context_hash TEXT NOT NULL DEFAULT '',
    semantic TEXT NOT NULL DEFAULT '',
    action_key TEXT NOT NULL DEFAULT '',
    priority INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'selected',
    reason_code TEXT NOT NULL DEFAULT '',
    rejected_candidates_json TEXT NOT NULL DEFAULT '[]',
    runtime_command_id TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT '',
    started_at TEXT NOT NULL DEFAULT '',
    completed_at TEXT NOT NULL DEFAULT ''
)`,
	`CREATE TABLE IF NOT EXISTS desktop_pet_behavior_cooldowns (
    user_id TEXT NOT NULL DEFAULT '',
    character_id TEXT NOT NULL DEFAULT '',
    cooldown_key TEXT NOT NULL DEFAULT '',
    until_at TEXT NOT NULL DEFAULT '',
    source_decision_id TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT '',
    PRIMARY KEY(user_id, character_id, cooldown_key)
)`,
	`CREATE TABLE IF NOT EXISTS desktop_pet_behavior_bindings (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL DEFAULT '',
    character_id TEXT NOT NULL DEFAULT '',
    installation_id TEXT NOT NULL DEFAULT '',
    event_type TEXT NOT NULL DEFAULT '',
    conditions_json TEXT NOT NULL DEFAULT '{}',
    semantic TEXT NOT NULL DEFAULT '',
    preferred_action TEXT NOT NULL DEFAULT '',
    priority_offset INTEGER NOT NULL DEFAULT 0,
    cooldown_ms INTEGER NOT NULL DEFAULT 0,
    enabled INTEGER NOT NULL DEFAULT 1,
    version INTEGER NOT NULL DEFAULT 1,
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
)`,
}

var DesktopPetBehaviorIndexDefs = []BehaviorIndexDef{
	{Name: "ux_desktop_pet_behavior_inbox_dedup", Table: "desktop_pet_behavior_inbox", Columns: []string{"user_id", "character_id", "dedup_key"}, Unique: true},
	{Name: "idx_behavior_inbox_char", Table: "desktop_pet_behavior_inbox", Columns: []string{"character_id", "status"}, Unique: false},
	{Name: "idx_behavior_inbox_status_available", Table: "desktop_pet_behavior_inbox", Columns: []string{"status", "available_at", "occurred_at"}, Unique: false},
	{Name: "idx_behavior_inbox_status_lease", Table: "desktop_pet_behavior_inbox", Columns: []string{"status", "lease_expires_at"}, Unique: false},
	{Name: "idx_behavior_decisions_char", Table: "desktop_pet_behavior_decisions", Columns: []string{"character_id", "created_at"}, Unique: false},
	{Name: "idx_behavior_decisions_event", Table: "desktop_pet_behavior_decisions", Columns: []string{"event_id", "created_at"}, Unique: false},
	{Name: "idx_behavior_bindings_user_char", Table: "desktop_pet_behavior_bindings", Columns: []string{"user_id", "character_id"}, Unique: false},
}
