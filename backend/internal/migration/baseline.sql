CREATE TABLE IF NOT EXISTS core_schema_baseline (
    singleton INTEGER PRIMARY KEY CHECK(singleton = 1),
    schema_version INTEGER NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS security_audit_events (
    event_id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    severity TEXT NOT NULL DEFAULT 'info',
    outcome TEXT NOT NULL DEFAULT 'success',
    space_id TEXT NOT NULL DEFAULT '',
    device_id TEXT NOT NULL DEFAULT '',
    runtime_id TEXT NOT NULL DEFAULT '',
    session_id TEXT NOT NULL DEFAULT '',
    principal_type TEXT NOT NULL DEFAULT '',
    auth_method TEXT NOT NULL DEFAULT '',
    ip_address TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    reason_code TEXT NOT NULL DEFAULT '',
    details_json TEXT NOT NULL DEFAULT '',
    occurred_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_audit_space ON security_audit_events(space_id);
CREATE INDEX IF NOT EXISTS idx_audit_device ON security_audit_events(device_id);
CREATE INDEX IF NOT EXISTS idx_audit_type ON security_audit_events(event_type);
CREATE INDEX IF NOT EXISTS idx_audit_time ON security_audit_events(occurred_at);

CREATE TABLE IF NOT EXISTS memory_scope_transfers (owner_id TEXT NOT NULL,request_id TEXT NOT NULL,payload_hash TEXT NOT NULL,ack TEXT NOT NULL,record_kind TEXT NOT NULL,record_id TEXT NOT NULL,original_record TEXT NOT NULL,created_at TEXT NOT NULL,PRIMARY KEY(owner_id,request_id));

CREATE TABLE IF NOT EXISTS characters (
    global_memory_enabled INTEGER NOT NULL DEFAULT 1,
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    avatar TEXT DEFAULT '',
    identity TEXT DEFAULT '',
    personality TEXT DEFAULT '',
    speaking_style TEXT DEFAULT '',
    relationship_style TEXT DEFAULT '',
    system_prompt TEXT DEFAULT '',
    boundary_rules TEXT DEFAULT '',
    personality_sliders TEXT DEFAULT '',
    description TEXT DEFAULT '',
    base_prompt TEXT DEFAULT '',
    generated_prompt TEXT DEFAULT '',
    is_default INTEGER DEFAULT 0,
    status TEXT DEFAULT 'enabled',
    personality_config TEXT DEFAULT '{}',
    chat_style_config TEXT DEFAULT '{}',
    scene_rules TEXT DEFAULT '{}',
    is_active INTEGER DEFAULT 0,
    sort_order INTEGER DEFAULT 0,
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT '',
    gender TEXT DEFAULT 'UNSPECIFIED',
    gender_label TEXT,
    pronoun TEXT DEFAULT 'TA',
    self_reference TEXT DEFAULT '我',
    user_addressing_style TEXT,
    gender_expression INTEGER DEFAULT 30,
    life_identity TEXT DEFAULT 'CUSTOM',
    voice_config_id TEXT DEFAULT '',
    voice_type TEXT DEFAULT '',
    voice_speed REAL DEFAULT 1.0,
    voice_pitch REAL DEFAULT 1.0,
    voice_volume REAL DEFAULT 1.0,
    custom_voice_id TEXT DEFAULT '',
    voice_mode TEXT DEFAULT 'preset',
    emotion TEXT DEFAULT '',
    emotion_scale INTEGER DEFAULT 0,
    silence_duration INTEGER DEFAULT 0,
    character_base TEXT DEFAULT '',
    card_data_json TEXT DEFAULT '{}',
    model_config_id INTEGER NOT NULL DEFAULT 0,
    reasoning_effort TEXT NOT NULL DEFAULT '',
    reasoning_enabled INTEGER NOT NULL DEFAULT -1,
    revision INTEGER NOT NULL DEFAULT 1,
    deleted_at DATETIME
);

CREATE TABLE IF NOT EXISTS character_card_import_operations (
    space_id TEXT NOT NULL,
    id TEXT NOT NULL,
    source_hash TEXT NOT NULL,
    character_id TEXT NOT NULL,
    character_name TEXT NOT NULL DEFAULT '',
    source_format TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    worldbook_expected INTEGER NOT NULL DEFAULT 0,
    worldbook_imported INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT '',
    PRIMARY KEY(space_id, id)
);
CREATE INDEX IF NOT EXISTS idx_character_card_import_source ON character_card_import_operations(space_id, source_hash);
CREATE UNIQUE INDEX IF NOT EXISTS idx_character_card_import_character ON character_card_import_operations(space_id, character_id);

CREATE TABLE IF NOT EXISTS reminders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    space_id TEXT NOT NULL DEFAULT '',
    title TEXT DEFAULT '',
    content TEXT DEFAULT '',
    channel TEXT DEFAULT 'web',
    character_id TEXT DEFAULT '',
    conversation_id TEXT DEFAULT '',
    remind_at TEXT DEFAULT '',
    repeat_rule TEXT DEFAULT 'none',
    enabled INTEGER DEFAULT 1,
    last_triggered_at TEXT DEFAULT '',
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_reminders_user ON reminders(space_id, enabled, remind_at);

CREATE TABLE IF NOT EXISTS trigger_histories (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL DEFAULT '',
    trigger_id TEXT DEFAULT '',
    trigger_type TEXT DEFAULT 'reminder',
    title TEXT DEFAULT '',
    channel TEXT DEFAULT 'web',
    state TEXT DEFAULT 'pending',
    priority TEXT DEFAULT 'normal',
    reason TEXT DEFAULT '',
    attempt_count INTEGER DEFAULT 0,
    last_error TEXT DEFAULT '',
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_trigger_histories_user ON trigger_histories(space_id, created_at);

CREATE TABLE IF NOT EXISTS character_templates (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    category TEXT DEFAULT '',
    description TEXT DEFAULT '',
    builtin INTEGER DEFAULT 0,
    template_json TEXT DEFAULT '',
    created_at TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS conversations (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL DEFAULT '',
    project_id TEXT NOT NULL DEFAULT '',
    workspace_id TEXT NOT NULL DEFAULT '',
    workspace_device_id TEXT NOT NULL DEFAULT '',
    title TEXT DEFAULT '',
    channel TEXT DEFAULT 'web',
    source TEXT DEFAULT 'manual',
    peer_id TEXT DEFAULT '',
    model_config_id INTEGER NOT NULL DEFAULT 0,
    reasoning_effort TEXT NOT NULL DEFAULT '',
    reasoning_enabled INTEGER NOT NULL DEFAULT -1,
    permission_mode TEXT NOT NULL DEFAULT 'request_approval',
    message_count INTEGER DEFAULT 0,
    state_version TEXT DEFAULT '',
    pinned_at TEXT NOT NULL DEFAULT '',
    archived_at TEXT NOT NULL DEFAULT '',
    last_read_turn_sequence INTEGER NOT NULL DEFAULT 0,
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT '',
    revision INTEGER NOT NULL DEFAULT 1,
    deleted_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_conversations_project_updated ON conversations(project_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_conversations_sidebar_pinned ON conversations(space_id, pinned_at, updated_at);
CREATE INDEX IF NOT EXISTS idx_conversations_sidebar_archived ON conversations(space_id, archived_at, updated_at);
CREATE INDEX IF NOT EXISTS idx_conversations_workspace ON conversations(workspace_id);

CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    workspace_id TEXT NOT NULL,
    device_id TEXT NOT NULL DEFAULT '',
    root_uri TEXT NOT NULL DEFAULT '',
    pinned_at TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT '',
    revision INTEGER NOT NULL DEFAULT 1,
    UNIQUE(space_id, workspace_id)
);
CREATE INDEX IF NOT EXISTS idx_projects_space_updated ON projects(space_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_projects_sidebar_pinned ON projects(space_id, pinned_at, updated_at);

CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL,
    character_id TEXT NOT NULL DEFAULT '',
    sequence INTEGER NOT NULL DEFAULT 0,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    reasoning_content TEXT NOT NULL DEFAULT '',
    reasoning_duration_ms INTEGER NOT NULL DEFAULT 0,
    msg_type TEXT DEFAULT 'text',
    tokens INTEGER DEFAULT 0,
    source TEXT DEFAULT 'manual',
    safety_level TEXT DEFAULT 'normal',
    status TEXT DEFAULT 'sent',
    include_in_context INTEGER DEFAULT 1,
    audio_url TEXT DEFAULT '',
    audio_duration REAL DEFAULT 0,
    image_url TEXT DEFAULT '',
    video_url TEXT DEFAULT '',
    request_id TEXT DEFAULT '',
    reply_to_message_id TEXT DEFAULT '',
    reply_to_role TEXT DEFAULT '',
    reply_to_excerpt TEXT DEFAULT '',
    citation_snapshot_json TEXT NOT NULL DEFAULT '[]',
    tool_call_id TEXT DEFAULT '',
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT '',
    alt_text TEXT NOT NULL DEFAULT '',
    is_animated INTEGER NOT NULL DEFAULT 0,
    media_width INTEGER NOT NULL DEFAULT 0,
    media_height INTEGER NOT NULL DEFAULT 0,
    original_asset_reference TEXT NOT NULL DEFAULT '',
    fallback_asset_reference TEXT NOT NULL DEFAULT '',
    delivery_group_id TEXT NOT NULL DEFAULT '',
    delivery_sequence INTEGER NOT NULL DEFAULT 0,
    extension_type TEXT NOT NULL DEFAULT '',
    revision INTEGER NOT NULL DEFAULT 1,
    deleted_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_messages_character_id ON messages(character_id);

CREATE TABLE IF NOT EXISTS chat_import_batches (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    device_id TEXT NOT NULL DEFAULT '',
    role_id TEXT NOT NULL DEFAULT '',
    role_revision INTEGER NOT NULL DEFAULT 0,
    title TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'ready_for_promotion',
    detected_format TEXT NOT NULL DEFAULT '',
    message_count INTEGER NOT NULL DEFAULT 0,
    content_hash TEXT NOT NULL DEFAULT '',
    idempotency_key TEXT NOT NULL DEFAULT '',
    summary_text TEXT NOT NULL DEFAULT '',
    promoted_conversation_id TEXT NOT NULL DEFAULT '',
    promoted_conversation_owner_id TEXT NOT NULL DEFAULT '',
    promoted_core_id TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT '',
    promoted_at TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_chat_import_batches_space_status ON chat_import_batches(space_id, status, created_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_chat_import_batches_idempotency ON chat_import_batches(space_id, device_id, idempotency_key);
CREATE INDEX IF NOT EXISTS idx_chat_import_batches_promoted ON chat_import_batches(space_id, promoted_conversation_id);

CREATE TABLE IF NOT EXISTS chat_import_items (
    id TEXT PRIMARY KEY,
    batch_id TEXT NOT NULL,
    sequence INTEGER NOT NULL DEFAULT 0,
    speaker TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL DEFAULT 'user',
    content TEXT NOT NULL,
    confidence REAL NOT NULL DEFAULT 0,
    timestamp TEXT NOT NULL DEFAULT '',
    line_no INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_chat_import_items_batch_sequence ON chat_import_items(batch_id, sequence);

CREATE TABLE IF NOT EXISTS pipeline_checkpoints (
    conversation_id TEXT NOT NULL,
    pipeline_type TEXT NOT NULL,
    last_message_sequence INTEGER NOT NULL DEFAULT 0,
    checkpoint_version INTEGER NOT NULL DEFAULT 1,
    idempotency_key TEXT DEFAULT '',
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT '',
    PRIMARY KEY (conversation_id, pipeline_type)
);

CREATE TABLE IF NOT EXISTS tool_call_intents (
    id TEXT PRIMARY KEY,
    request_id TEXT DEFAULT '',
    conversation_id TEXT DEFAULT '',
    character_id TEXT DEFAULT '',
    channel TEXT DEFAULT '',
    tool_call_id TEXT DEFAULT '',
    tool_name TEXT NOT NULL,
    args_json TEXT DEFAULT '',
    idempotency_key TEXT DEFAULT '',
    status TEXT DEFAULT 'PENDING',
    attempt_id TEXT NOT NULL DEFAULT '',
    turn_id TEXT NOT NULL DEFAULT '',
    execution_id TEXT NOT NULL DEFAULT '',
    input_hash TEXT NOT NULL DEFAULT '',
    owner_instance_id TEXT NOT NULL DEFAULT '',
    result_ref TEXT NOT NULL DEFAULT '',
    error_class TEXT NOT NULL DEFAULT '',
    started_at TEXT NOT NULL DEFAULT '',
    finished_at TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS tool_call_results (
    id TEXT PRIMARY KEY,
    intent_id TEXT DEFAULT '',
    request_id TEXT DEFAULT '',
    conversation_id TEXT DEFAULT '',
    character_id TEXT DEFAULT '',
    channel TEXT DEFAULT '',
    tool_call_id TEXT DEFAULT '',
    tool_name TEXT NOT NULL,
    status TEXT NOT NULL,
    content TEXT DEFAULT '',
    error_code TEXT DEFAULT '',
    visible_text TEXT DEFAULT '',
    side_effects_json TEXT DEFAULT '[]',
    external_operation_id TEXT DEFAULT '',
    idempotency_key TEXT DEFAULT '',
    audit_json TEXT DEFAULT '{}',
    confidence REAL DEFAULT 0,
    force_voice INTEGER DEFAULT 0,
    attempt_id TEXT NOT NULL DEFAULT '',
    turn_id TEXT NOT NULL DEFAULT '',
    execution_id TEXT NOT NULL DEFAULT '',
    input_hash TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_tool_call_intents_request ON tool_call_intents(request_id, tool_name);
CREATE INDEX IF NOT EXISTS idx_tool_call_intents_idempotency ON tool_call_intents(idempotency_key);
CREATE INDEX IF NOT EXISTS idx_tool_call_results_request ON tool_call_results(request_id, status);

CREATE TABLE IF NOT EXISTS assistant_turns (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL DEFAULT '',
    character_id TEXT NOT NULL DEFAULT '',
    user_message_id TEXT NOT NULL DEFAULT '',
    request_id TEXT NOT NULL DEFAULT '',
    execution_id TEXT NOT NULL DEFAULT '',
    parent_turn_id TEXT NOT NULL DEFAULT '',
    parent_block_id TEXT NOT NULL DEFAULT '',
    agent_id TEXT NOT NULL DEFAULT '',
    sequence INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'running',
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT '',
    completed_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS assistant_turn_items (
    id TEXT PRIMARY KEY,
    turn_id TEXT NOT NULL DEFAULT '',
    conversation_id TEXT NOT NULL DEFAULT '',
    sequence INTEGER NOT NULL DEFAULT 0,
    item_type TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    revision INTEGER NOT NULL DEFAULT 0,
    call_id TEXT NOT NULL DEFAULT '',
    tool_name TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    arguments_json TEXT NOT NULL DEFAULT '',
    result_json TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    duration_ms INTEGER NOT NULL DEFAULT 0,
    is_final INTEGER NOT NULL DEFAULT 0,
    message_id TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_assistant_turns_conversation ON assistant_turns(conversation_id, sequence);
CREATE INDEX IF NOT EXISTS idx_assistant_turns_request ON assistant_turns(request_id);
CREATE INDEX IF NOT EXISTS idx_assistant_turns_execution ON assistant_turns(execution_id);
CREATE INDEX IF NOT EXISTS idx_assistant_turns_parent ON assistant_turns(parent_turn_id);
CREATE INDEX IF NOT EXISTS idx_assistant_turns_agent ON assistant_turns(agent_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_assistant_turns_conv_request_unique
    ON assistant_turns(conversation_id, request_id)
    WHERE request_id <> '';
CREATE INDEX IF NOT EXISTS idx_assistant_turn_items_turn ON assistant_turn_items(turn_id, sequence);
CREATE INDEX IF NOT EXISTS idx_assistant_turn_items_call ON assistant_turn_items(turn_id, call_id);

CREATE TABLE IF NOT EXISTS model_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT DEFAULT '',
    api_type TEXT DEFAULT '',
    base_url TEXT DEFAULT '',
    api_key TEXT DEFAULT '',
    model_name TEXT DEFAULT '',
    temperature REAL DEFAULT 0.7,
    max_tokens INTEGER DEFAULT 4096,
    top_p REAL DEFAULT 1,
    timeout_seconds INTEGER DEFAULT 60,
    retry_count INTEGER DEFAULT 1,
    is_active INTEGER DEFAULT 0,
    last_test_status TEXT DEFAULT '',
    last_test_message TEXT DEFAULT '',
    last_test_at TEXT DEFAULT '',
    protocol TEXT DEFAULT '',
    context_window INTEGER DEFAULT 0,
    max_output_tokens INTEGER DEFAULT 0,
    budget_rate_version TEXT NOT NULL DEFAULT '',
    budget_input_rate INTEGER NOT NULL DEFAULT 0,
    budget_output_rate INTEGER NOT NULL DEFAULT 0,
    capabilities_json TEXT DEFAULT '',
    provider_config_json TEXT DEFAULT '{}',
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS model_artifacts (
    id TEXT PRIMARY KEY,
    owner_space_id TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL DEFAULT '',
    filename TEXT NOT NULL DEFAULT '',
    format TEXT NOT NULL DEFAULT '',
    size_bytes INTEGER NOT NULL DEFAULT 0,
    sha256 TEXT NOT NULL DEFAULT '',
    resource_uri TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'ready',
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_model_artifacts_owner_sha256
    ON model_artifacts(owner_space_id, sha256);

CREATE TABLE IF NOT EXISTS model_artifact_references (
    artifact_id TEXT NOT NULL DEFAULT '',
    model_config_id INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (artifact_id, model_config_id)
);

CREATE INDEX IF NOT EXISTS idx_model_artifact_refs_config
    ON model_artifact_references(model_config_id);

CREATE TABLE IF NOT EXISTS tts_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    api_type TEXT DEFAULT 'volcengine',
    api_key TEXT DEFAULT '',
    base_url TEXT DEFAULT '',
    resource_id TEXT DEFAULT 'seed-tts-2.0',
    voice_type TEXT DEFAULT 'zh_female_vv_uranus_bigtts',
    emotion TEXT DEFAULT '',
    speed REAL DEFAULT 1.0,
    pitch REAL DEFAULT 1.0,
    volume REAL DEFAULT 1.0,
    is_active INTEGER DEFAULT 0,
    is_custom INTEGER DEFAULT 0,
    custom_voice_id TEXT DEFAULT '',
    realtime_app_id TEXT DEFAULT '',
    realtime_access_token TEXT DEFAULT '',
    realtime_secret_key TEXT DEFAULT '',
    clone_resource_id TEXT DEFAULT 'volc.megatts.timbre',
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS tts_cloned_voices (
    space_id TEXT NOT NULL,
    speaker_id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    tts_config_id INTEGER NOT NULL DEFAULT 0,
    language INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'ready',
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_tts_cloned_voices_user_created ON tts_cloned_voices(space_id, created_at);

CREATE TABLE IF NOT EXISTS asr_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    api_type TEXT DEFAULT 'volcengine',
    api_key TEXT DEFAULT '',
    base_url TEXT DEFAULT '',
    resource_id TEXT DEFAULT 'volc.seedasr.auc',
    is_active INTEGER DEFAULT 0,
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
);



CREATE TABLE IF NOT EXISTS vision_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    api_type TEXT DEFAULT 'volcengine',
    api_key TEXT DEFAULT '',
    model_name TEXT DEFAULT 'doubao-seed-2-0-lite-260428',
    base_url TEXT DEFAULT 'https://ark.cn-beijing.volces.com/api/v3',
    is_active INTEGER DEFAULT 0,
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
);


CREATE TABLE IF NOT EXISTS memories (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL DEFAULT '',
    key TEXT DEFAULT '',
    value TEXT DEFAULT '',
    memory_type TEXT DEFAULT 'fact',
    memory_subtype TEXT NOT NULL DEFAULT '',
    importance INTEGER DEFAULT 0,
    confidence INTEGER DEFAULT 50,
    source TEXT DEFAULT 'manual',
    scope TEXT DEFAULT 'character',
    scope_type TEXT DEFAULT 'user_character',
    character_id TEXT DEFAULT '',
    entity_id TEXT DEFAULT '',
    entity_type TEXT DEFAULT '',
    source_msg_id TEXT DEFAULT '',
    source_conv_id TEXT DEFAULT '',
    verified_status TEXT DEFAULT 'unverified',
    last_verified_at TEXT,
    expires_at TEXT,
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT '',
    use_count INTEGER DEFAULT 0,
    last_used_at TEXT,
    retention_level INTEGER NOT NULL DEFAULT 3,
    memory_strength REAL NOT NULL DEFAULT 0.68,
    strength_updated_at TEXT,
    last_reinforced_at TEXT,
    reinforce_count INTEGER NOT NULL DEFAULT 0,
    retrieved_count INTEGER NOT NULL DEFAULT 0,
    injected_count INTEGER NOT NULL DEFAULT 0,
    decay_state TEXT NOT NULL DEFAULT 'active',
    pinned INTEGER NOT NULL DEFAULT 0,
    archived_at TEXT,
    superseded_by TEXT NOT NULL DEFAULT '',
    sensitivity_level TEXT DEFAULT 'internal',
    allow_context_use INTEGER DEFAULT 1,
    allow_proactive_mention INTEGER DEFAULT 1,
    requires_confirmation INTEGER DEFAULT 0,
    version INTEGER NOT NULL DEFAULT 1,
    derivation_key TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS memory_events (
    id TEXT PRIMARY KEY,
    memory_id TEXT DEFAULT '',
    event_type TEXT DEFAULT '',
    key TEXT DEFAULT '',
    value TEXT DEFAULT '',
    memory_type TEXT DEFAULT '',
    importance INTEGER DEFAULT 0,
    confidence INTEGER DEFAULT 50,
    expires_at TEXT DEFAULT NULL,
    entity_id TEXT DEFAULT '',
    entity_type TEXT DEFAULT '',
    source_msg_id TEXT DEFAULT '',
    source_conv_id TEXT DEFAULT '',
    verified_status TEXT DEFAULT 'unverified',
    last_verified_at TEXT DEFAULT NULL,
    source TEXT DEFAULT '',
    character_id TEXT DEFAULT '',
    created_at TEXT DEFAULT '',
    version INTEGER NOT NULL DEFAULT 1,
    operation_id TEXT NOT NULL DEFAULT '',
    snapshot_hash TEXT NOT NULL DEFAULT '',
    event_reason TEXT NOT NULL DEFAULT ''
);




CREATE INDEX IF NOT EXISTS idx_memory_events_operation ON memory_events(operation_id);

CREATE INDEX IF NOT EXISTS idx_memories_user_character ON memories(space_id, character_id);
CREATE INDEX IF NOT EXISTS idx_memories_confidence ON memories(character_id, confidence);
CREATE INDEX IF NOT EXISTS idx_memories_verified ON memories(character_id, verified_status);
CREATE INDEX IF NOT EXISTS idx_memories_entity ON memories(entity_id, entity_type);
CREATE INDEX IF NOT EXISTS idx_memories_importance_conf ON memories(character_id, importance, confidence);
CREATE INDEX IF NOT EXISTS idx_memories_scope_type ON memories(scope_type);
CREATE INDEX IF NOT EXISTS idx_memories_character_type ON memories(character_id, memory_type);
CREATE INDEX IF NOT EXISTS idx_memories_derivation_key ON memories(derivation_key);
CREATE INDEX IF NOT EXISTS idx_memories_retention_state ON memories(character_id, decay_state, retention_level);
CREATE INDEX IF NOT EXISTS idx_memories_subtype ON memories(character_id, memory_subtype);


CREATE TABLE IF NOT EXISTS episodic_memories (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL DEFAULT '',
    character_id TEXT NOT NULL DEFAULT '',
    scene_type TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    context_before TEXT DEFAULT '',
    context_after TEXT DEFAULT '',
    trigger_keywords TEXT DEFAULT '',
    sentiment_score INTEGER DEFAULT 0,
    message_id_start TEXT DEFAULT '',
    message_id_end TEXT DEFAULT '',
    source_conv_id TEXT DEFAULT '',
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT '',
    message_time_start TEXT NOT NULL DEFAULT '',
    message_time_end TEXT NOT NULL DEFAULT '',
    retention_level INTEGER NOT NULL DEFAULT 4,
    memory_strength REAL NOT NULL DEFAULT 0.50,
    strength_updated_at TEXT,
    last_reinforced_at TEXT,
    reinforce_count INTEGER NOT NULL DEFAULT 0,
    decay_state TEXT NOT NULL DEFAULT 'active',
    archived_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_episodic_space_id ON episodic_memories(space_id);
CREATE INDEX IF NOT EXISTS idx_episodic_user_character ON episodic_memories(space_id, character_id);
CREATE INDEX IF NOT EXISTS idx_episodic_scene_type ON episodic_memories(space_id, scene_type);
CREATE INDEX IF NOT EXISTS idx_episodic_created ON episodic_memories(space_id, created_at);
CREATE INDEX IF NOT EXISTS idx_episodic_retention_state ON episodic_memories(space_id, decay_state, retention_level);

CREATE TABLE IF NOT EXISTS user_profiles (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL DEFAULT '',
    character_id TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL,
    attribute_name TEXT NOT NULL,
    attribute_value TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT '',
    confidence INTEGER DEFAULT 50,
    source_conv_id TEXT DEFAULT '',
    verified_at TEXT DEFAULT '',
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT '',
    source_memory_id TEXT NOT NULL DEFAULT '',
    projection_status TEXT NOT NULL DEFAULT 'active'
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_profiles_uid_cat_attr ON user_profiles(space_id, character_id, category, attribute_name);
CREATE INDEX IF NOT EXISTS idx_user_profiles_space_id ON user_profiles(space_id);
CREATE INDEX IF NOT EXISTS idx_user_profiles_confidence ON user_profiles(space_id, confidence);
CREATE INDEX IF NOT EXISTS idx_user_profiles_source_memory ON user_profiles(source_memory_id);
CREATE INDEX IF NOT EXISTS idx_user_profiles_projection_status ON user_profiles(space_id, character_id, projection_status);

CREATE TABLE IF NOT EXISTS world_book (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL DEFAULT '',
    match_type TEXT NOT NULL DEFAULT 'keyword',
    match_pattern TEXT NOT NULL DEFAULT '',
    match_scope TEXT NOT NULL DEFAULT 'full_context',
    inject_content TEXT NOT NULL DEFAULT '',
    priority INTEGER DEFAULT 0,
    hit_count INTEGER DEFAULT 0,
    character_id TEXT DEFAULT '',
    config_json TEXT DEFAULT '{}',
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_world_book_match_type ON world_book(match_type);
CREATE INDEX IF NOT EXISTS idx_world_book_priority ON world_book(priority);
CREATE INDEX IF NOT EXISTS idx_world_book_character_id ON world_book(character_id);
CREATE INDEX IF NOT EXISTS idx_world_book_character_priority ON world_book(character_id, priority);
CREATE INDEX IF NOT EXISTS idx_world_book_user_character ON world_book(space_id, character_id);
CREATE INDEX IF NOT EXISTS idx_world_book_user_priority ON world_book(space_id, priority);

CREATE TABLE IF NOT EXISTS conversation_summaries (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL,
    round_start INTEGER NOT NULL DEFAULT 0,
    round_end INTEGER NOT NULL DEFAULT 0,
    summary_text TEXT NOT NULL DEFAULT '',
    parent_summary_id TEXT DEFAULT '',
    compressed_at TEXT DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_conv_summaries_conv_round ON conversation_summaries(conversation_id, round_start);
CREATE INDEX IF NOT EXISTS idx_conv_summaries_parent ON conversation_summaries(parent_summary_id);

CREATE TABLE IF NOT EXISTS message_feedback (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    space_id TEXT NOT NULL DEFAULT '',
    owner_id TEXT NOT NULL DEFAULT '',
    core_id TEXT NOT NULL DEFAULT '',
    role_id TEXT NOT NULL DEFAULT '',
    conversation_id TEXT NOT NULL DEFAULT '',
    message_id TEXT DEFAULT '',
    request_id TEXT NOT NULL DEFAULT '',
    feedback_type TEXT NOT NULL DEFAULT '',
    reason TEXT DEFAULT '',
    rating INTEGER DEFAULT 0,
    comment TEXT DEFAULT '',
    revision INTEGER NOT NULL DEFAULT 1,
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT '',
    deleted_at DATETIME
);

CREATE TABLE IF NOT EXISTS safety_policies (
    space_id TEXT NOT NULL,
    owner_id TEXT NOT NULL,
    core_id TEXT NOT NULL,
    revision INTEGER NOT NULL DEFAULT 1,
    config_json TEXT NOT NULL DEFAULT '{}',
    bdi_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT '',
    PRIMARY KEY(space_id, owner_id)
);

CREATE INDEX IF NOT EXISTS idx_safety_policies_owner ON safety_policies(space_id, owner_id, core_id);

CREATE TABLE IF NOT EXISTS safety_incidents (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    owner_id TEXT NOT NULL,
    core_id TEXT NOT NULL,
    role_id TEXT NOT NULL DEFAULT '',
    conversation_id TEXT NOT NULL DEFAULT '',
    message_id TEXT NOT NULL DEFAULT '',
    feedback_id INTEGER,
    request_id TEXT NOT NULL DEFAULT '',
    event_type TEXT NOT NULL,
    severity TEXT NOT NULL DEFAULT 'medium',
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'open',
    policy_revision INTEGER NOT NULL DEFAULT 0,
    revision INTEGER NOT NULL DEFAULT 1,
    details_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT '',
    resolved_at TEXT NOT NULL DEFAULT '',
    resolved_by TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_safety_incidents_scope ON safety_incidents(space_id, owner_id, core_id, status, created_at);
CREATE INDEX IF NOT EXISTS idx_safety_incidents_conversation ON safety_incidents(conversation_id);
CREATE INDEX IF NOT EXISTS idx_safety_incidents_message ON safety_incidents(message_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_safety_incidents_feedback ON safety_incidents(feedback_id) WHERE feedback_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS moods (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    character_id TEXT DEFAULT '',
    mood TEXT DEFAULT '',
    mood_value TEXT DEFAULT '',
    level INTEGER DEFAULT 50,
    created_at TEXT DEFAULT ''
);


CREATE TABLE IF NOT EXISTS app_settings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT NOT NULL UNIQUE,
    value TEXT DEFAULT '',
    revision INTEGER NOT NULL DEFAULT 1,
    deleted_at DATETIME,
    updated_at TEXT DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_app_settings_key ON app_settings(key);
CREATE INDEX IF NOT EXISTS idx_app_settings_deleted_at ON app_settings(deleted_at);

CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id);
CREATE INDEX IF NOT EXISTS idx_messages_conversation_sequence ON messages(conversation_id, sequence);
CREATE UNIQUE INDEX IF NOT EXISTS idx_messages_conv_sequence_unique ON messages(conversation_id, sequence);
CREATE INDEX IF NOT EXISTS idx_messages_conv_created ON messages(conversation_id, created_at);
CREATE INDEX IF NOT EXISTS idx_messages_conv_ctx ON messages(conversation_id, include_in_context);
CREATE INDEX IF NOT EXISTS idx_messages_conv_ctx_role_created ON messages(conversation_id, include_in_context, role, created_at);
CREATE INDEX IF NOT EXISTS idx_messages_request ON messages(conversation_id, role, request_id);
CREATE INDEX IF NOT EXISTS idx_pipeline_checkpoints_conversation ON pipeline_checkpoints(conversation_id);
CREATE INDEX IF NOT EXISTS idx_pipeline_checkpoints_updated ON pipeline_checkpoints(updated_at);
CREATE INDEX IF NOT EXISTS idx_conversations_user_updated ON conversations(space_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_conversations_channel_peer ON conversations(channel, peer_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_conversations_channel_peer_unique ON conversations(channel, peer_id) WHERE peer_id <> '';

CREATE TABLE IF NOT EXISTS retrieval_logs (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL DEFAULT '',
    conversation_id TEXT NOT NULL DEFAULT '',
    character_id TEXT NOT NULL DEFAULT '',
    request_id TEXT NOT NULL DEFAULT '',
    channel TEXT NOT NULL DEFAULT '',
    retrieval_version TEXT NOT NULL DEFAULT '',
    query_text TEXT NOT NULL DEFAULT '',
    retrieved_memory_ids TEXT DEFAULT '[]',
    scoring_details TEXT DEFAULT '{}',
    created_at TEXT DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_retrieval_logs_user_created ON retrieval_logs(space_id, created_at);
CREATE INDEX IF NOT EXISTS idx_retrieval_logs_conv_created ON retrieval_logs(conversation_id, created_at);
CREATE INDEX IF NOT EXISTS idx_retrieval_logs_request_created ON retrieval_logs(request_id, created_at);
CREATE INDEX IF NOT EXISTS idx_retrieval_logs_character_created ON retrieval_logs(character_id, created_at);

CREATE TABLE IF NOT EXISTS embedding_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL DEFAULT 'default',
    api_type TEXT DEFAULT 'volcengine',
    api_key TEXT DEFAULT '',
    model_name TEXT DEFAULT 'doubao-embedding-vision-251215',
    base_url TEXT DEFAULT 'https://ark.cn-beijing.volces.com/api/v3',
    is_active INTEGER DEFAULT 0,
    provider_config_json TEXT DEFAULT '',
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS image_gen_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    api_type TEXT DEFAULT 'seedream',
    api_key TEXT DEFAULT '',
    model_name TEXT DEFAULT 'doubao-seedream-5-0',
    base_url TEXT DEFAULT 'https://ark.cn-beijing.volces.com/api/v3',
    is_active INTEGER DEFAULT 0,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS mcp_duplicate_tool_registrations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tool_id TEXT NOT NULL,
    server_id TEXT NOT NULL,
    owner TEXT NOT NULL DEFAULT '',
    generation INTEGER NOT NULL DEFAULT 0,
    detected_at TEXT NOT NULL,
    resolved INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_mcp_dup_unresolved ON mcp_duplicate_tool_registrations(resolved, server_id);


CREATE TABLE IF NOT EXISTS desktop_pet_generation_tasks (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL DEFAULT '',
    character_id TEXT NOT NULL DEFAULT '',
    model_config_id INTEGER NOT NULL DEFAULT 0,
    name TEXT NOT NULL DEFAULT '',
    source_image_path TEXT DEFAULT '',
    source_image_original_name TEXT DEFAULT '',
    source_image_mime_type TEXT DEFAULT '',
    source_image_size INTEGER DEFAULT 0,
    source_image_width INTEGER DEFAULT 0,
    source_image_height INTEGER DEFAULT 0,
    source_image_hash TEXT DEFAULT '',
    prompt TEXT DEFAULT '',
    negative_prompt TEXT DEFAULT '',
    output_width INTEGER DEFAULT 0,
    output_height INTEGER DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',
    current_stage TEXT NOT NULL DEFAULT 'queued',
    progress INTEGER NOT NULL DEFAULT 0,
    selected_action_count INTEGER NOT NULL DEFAULT 0,
    estimated_generation_count INTEGER NOT NULL DEFAULT 0,
    error_code TEXT DEFAULT '',
    error_message TEXT DEFAULT '',
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT '',
    started_at TEXT DEFAULT '',
    completed_at TEXT DEFAULT '',
    execution_id TEXT DEFAULT '',
    worker_id TEXT DEFAULT '',
    lease_expires_at TEXT DEFAULT '',
    last_heartbeat_at TEXT DEFAULT '',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    cancel_requested_at TEXT DEFAULT '',
    row_version INTEGER NOT NULL DEFAULT 0,
    status_reason TEXT NOT NULL DEFAULT '',
    failure_stage TEXT NOT NULL DEFAULT '',
    last_transition_at TEXT NOT NULL DEFAULT '',
    submitted_at TEXT NOT NULL DEFAULT '',
    cancelling_at TEXT NOT NULL DEFAULT '',
    cancelled_at TEXT NOT NULL DEFAULT '',
    reference_asset_id TEXT NOT NULL DEFAULT '',
    generation_plan_version INTEGER NOT NULL DEFAULT 0,
    provider_key_snapshot TEXT NOT NULL DEFAULT '',
    model_name_snapshot TEXT NOT NULL DEFAULT '',
    config_revision_snapshot TEXT NOT NULL DEFAULT '',
    capability_snapshot_json TEXT NOT NULL DEFAULT '{}',
    capability_snapshot_hash TEXT NOT NULL DEFAULT '',
    cost_estimate_json TEXT NOT NULL DEFAULT '{}',
    planned_primary_request_count INTEGER NOT NULL DEFAULT 0,
    planned_max_provider_call_count INTEGER NOT NULL DEFAULT 0,
    actual_provider_call_count INTEGER NOT NULL DEFAULT 0,
    actual_output_image_count INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_dpgt_user ON desktop_pet_generation_tasks(space_id);
CREATE INDEX IF NOT EXISTS idx_dpgt_char ON desktop_pet_generation_tasks(character_id);
CREATE INDEX IF NOT EXISTS idx_dpgt_status ON desktop_pet_generation_tasks(status);
CREATE INDEX IF NOT EXISTS idx_dpgt_created ON desktop_pet_generation_tasks(created_at);







CREATE TABLE IF NOT EXISTS desktop_pet_processing_tasks (
    id TEXT PRIMARY KEY,
    generation_task_id TEXT NOT NULL DEFAULT '',
    space_id TEXT NOT NULL DEFAULT '',
    character_id TEXT NOT NULL DEFAULT '',
    processing_version INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL DEFAULT 'pending',
    current_stage TEXT NOT NULL DEFAULT 'queued',
    progress INTEGER NOT NULL DEFAULT 0,
    output_width INTEGER NOT NULL DEFAULT 512,
    output_height INTEGER NOT NULL DEFAULT 512,
    target_character_height_ratio REAL NOT NULL DEFAULT 0.8,
    anchor_mode TEXT NOT NULL DEFAULT 'feet_center',
    background_mode TEXT NOT NULL DEFAULT 'remove_background',
    output_format TEXT NOT NULL DEFAULT 'png',
    default_fps INTEGER NOT NULL DEFAULT 10,
    execution_id TEXT DEFAULT '',
    worker_id TEXT DEFAULT '',
    lease_expires_at TEXT DEFAULT '',
    last_heartbeat_at TEXT DEFAULT '',
    cancel_requested_at TEXT DEFAULT '',
    error_code TEXT DEFAULT '',
    error_message TEXT DEFAULT '',
    started_at TEXT DEFAULT '',
    completed_at TEXT DEFAULT '',
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT '',
    row_version INTEGER NOT NULL DEFAULT 0,
    status_reason TEXT NOT NULL DEFAULT '',
    failure_stage TEXT NOT NULL DEFAULT '',
    last_transition_at TEXT NOT NULL DEFAULT '',
    submitted_at TEXT NOT NULL DEFAULT '',
    cancelling_at TEXT NOT NULL DEFAULT '',
    cancelled_at TEXT NOT NULL DEFAULT '',
    config_snapshot TEXT NOT NULL DEFAULT '{}',
    config_hash TEXT NOT NULL DEFAULT '',
    pipeline_version TEXT NOT NULL DEFAULT '',
    active_revision_count INTEGER NOT NULL DEFAULT 0,
    publish_state TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_dppt_gen_task ON desktop_pet_processing_tasks(generation_task_id);
CREATE INDEX IF NOT EXISTS idx_dppt_status ON desktop_pet_processing_tasks(status);
CREATE INDEX IF NOT EXISTS idx_dppt_version ON desktop_pet_processing_tasks(processing_version);
CREATE INDEX IF NOT EXISTS idx_dppt_exec ON desktop_pet_processing_tasks(execution_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dppt_gen_version ON desktop_pet_processing_tasks(generation_task_id, processing_version);









CREATE TABLE IF NOT EXISTS desktop_pet_installations (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL DEFAULT '',
    character_id TEXT NOT NULL DEFAULT '',
    package_id TEXT NOT NULL DEFAULT '',
    package_version TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'installed',
    is_active INTEGER NOT NULL DEFAULT 0,
    install_path TEXT DEFAULT '',
    manifest_path TEXT DEFAULT '',
    preview_path TEXT DEFAULT '',
    default_action_key TEXT DEFAULT '',
    canvas_width INTEGER NOT NULL DEFAULT 0,
    canvas_height INTEGER NOT NULL DEFAULT 0,
    package_hash TEXT DEFAULT '',
    installed_at TEXT DEFAULT '',
    last_enabled_at TEXT DEFAULT '',
    last_disabled_at TEXT DEFAULT '',
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT '',
    pet_id TEXT NOT NULL DEFAULT '',
    current_release_id TEXT NOT NULL DEFAULT '',
    lifecycle_state TEXT NOT NULL DEFAULT 'installed',
    desired_state TEXT NOT NULL DEFAULT 'disabled',
    runtime_sync_state TEXT NOT NULL DEFAULT 'pending',
    state_revision INTEGER NOT NULL DEFAULT 0,
    install_storage_key TEXT NOT NULL DEFAULT '',
    integrity_root TEXT NOT NULL DEFAULT '',
    last_error_code TEXT NOT NULL DEFAULT '',
    last_error_message TEXT NOT NULL DEFAULT '',
    device_id TEXT NOT NULL DEFAULT '',
    preview_artifact_path TEXT NOT NULL DEFAULT '',
    default_action_release_id TEXT NOT NULL DEFAULT '',
    installed_content_hash TEXT NOT NULL DEFAULT '',
    integrity_status TEXT NOT NULL DEFAULT 'verified'
);

CREATE INDEX IF NOT EXISTS idx_dpinst_user ON desktop_pet_installations(space_id);
CREATE INDEX IF NOT EXISTS idx_dpinst_character ON desktop_pet_installations(character_id);
CREATE INDEX IF NOT EXISTS idx_dpinst_package ON desktop_pet_installations(package_id);
CREATE INDEX IF NOT EXISTS idx_dpinst_status ON desktop_pet_installations(status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_dpinst_package_version ON desktop_pet_installations(package_id, package_version);
CREATE INDEX IF NOT EXISTS idx_dpinst_pet ON desktop_pet_installations(pet_id);
CREATE INDEX IF NOT EXISTS idx_dpinst_release ON desktop_pet_installations(current_release_id);
CREATE INDEX IF NOT EXISTS idx_dpinst_lifecycle ON desktop_pet_installations(lifecycle_state);
CREATE INDEX IF NOT EXISTS idx_dpinst_desired ON desktop_pet_installations(desired_state);
CREATE INDEX IF NOT EXISTS idx_dpinst_device ON desktop_pet_installations(device_id);
CREATE INDEX IF NOT EXISTS idx_dpinst_user_device_pet ON desktop_pet_installations(space_id, device_id, pet_id);

CREATE TABLE IF NOT EXISTS desktop_pet_runtime_settings (
    id TEXT PRIMARY KEY,
    installation_id TEXT NOT NULL DEFAULT '',
    always_on_top INTEGER NOT NULL DEFAULT 1,
    scale REAL NOT NULL DEFAULT 1.0,
    position_x INTEGER NOT NULL DEFAULT 0,
    position_y INTEGER NOT NULL DEFAULT 0,
    screen_id TEXT NOT NULL DEFAULT '',
    idle_enabled INTEGER NOT NULL DEFAULT 1,
    idle_interval_min_seconds INTEGER NOT NULL DEFAULT 30,
    idle_interval_max_seconds INTEGER NOT NULL DEFAULT 120,
    click_through_mode TEXT NOT NULL DEFAULT 'off',
    sound_enabled INTEGER NOT NULL DEFAULT 0,
    settings_revision INTEGER NOT NULL DEFAULT 0,
    restore_on_app_start INTEGER NOT NULL DEFAULT 1,
    position_mode TEXT NOT NULL DEFAULT 'absolute',
    display_fingerprint TEXT NOT NULL DEFAULT '',
    relative_x REAL NOT NULL DEFAULT 0.5,
    relative_y REAL NOT NULL DEFAULT 0.5,
    last_window_width INTEGER NOT NULL DEFAULT 0,
    last_window_height INTEGER NOT NULL DEFAULT 0,
    position_updated_at TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_dprts_installation ON desktop_pet_runtime_settings(installation_id);

CREATE TABLE IF NOT EXISTS desktop_pet_runtime_clients (
  runtime_id TEXT PRIMARY KEY,
  device_id TEXT NOT NULL DEFAULT '',
  space_id TEXT NOT NULL DEFAULT '',
  display_name TEXT NOT NULL DEFAULT '',
  platform TEXT NOT NULL DEFAULT '',
  arch TEXT NOT NULL DEFAULT '',
  app_version TEXT NOT NULL DEFAULT '',
  protocol_version TEXT NOT NULL DEFAULT '',
  capabilities_json TEXT NOT NULL DEFAULT '[]',
  last_process_instance_id TEXT NOT NULL DEFAULT '',
  last_session_id TEXT NOT NULL DEFAULT '',
  last_seen_at TEXT NOT NULL DEFAULT '',
  last_connected_at TEXT NOT NULL DEFAULT '',
  last_disconnected_at TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
);




CREATE TABLE IF NOT EXISTS desktop_pet_behavior_contexts (
    space_id TEXT NOT NULL DEFAULT '',
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
    PRIMARY KEY(space_id, character_id)
);

CREATE TABLE IF NOT EXISTS desktop_pet_behavior_inbox (
    event_id TEXT PRIMARY KEY,
    dedup_key TEXT NOT NULL DEFAULT '',
    event_type TEXT NOT NULL DEFAULT '',
    schema_version INTEGER NOT NULL DEFAULT 0,
    space_id TEXT NOT NULL DEFAULT '',
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
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_desktop_pet_behavior_inbox_dedup ON desktop_pet_behavior_inbox(dedup_key);
CREATE INDEX IF NOT EXISTS idx_behavior_inbox_char ON desktop_pet_behavior_inbox(character_id, status);
CREATE INDEX IF NOT EXISTS idx_behavior_inbox_status_available ON desktop_pet_behavior_inbox(status, available_at, occurred_at);
CREATE INDEX IF NOT EXISTS idx_behavior_inbox_status_lease ON desktop_pet_behavior_inbox(status, lease_expires_at);

CREATE TABLE IF NOT EXISTS desktop_pet_behavior_decisions (
    decision_id TEXT PRIMARY KEY,
    event_id TEXT NOT NULL DEFAULT '',
    space_id TEXT NOT NULL DEFAULT '',
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
);

CREATE INDEX IF NOT EXISTS idx_behavior_decisions_char ON desktop_pet_behavior_decisions(character_id, created_at);
CREATE INDEX IF NOT EXISTS idx_behavior_decisions_event ON desktop_pet_behavior_decisions(event_id, created_at);

CREATE TABLE IF NOT EXISTS desktop_pet_behavior_cooldowns (
    space_id TEXT NOT NULL DEFAULT '',
    character_id TEXT NOT NULL DEFAULT '',
    cooldown_key TEXT NOT NULL DEFAULT '',
    until_at TEXT NOT NULL DEFAULT '',
    source_decision_id TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT '',
    PRIMARY KEY(space_id, character_id, cooldown_key)
);

CREATE TABLE IF NOT EXISTS desktop_pet_behavior_bindings (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL DEFAULT '',
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
);

CREATE INDEX IF NOT EXISTS idx_behavior_bindings_user_char ON desktop_pet_behavior_bindings(space_id, character_id);

CREATE TABLE IF NOT EXISTS desktop_pet_identities (
    id TEXT PRIMARY KEY,
    owner_space_id TEXT NOT NULL DEFAULT '',
    source_character_id TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL DEFAULT '',
    slug TEXT NOT NULL DEFAULT '',
    binding_policy TEXT NOT NULL DEFAULT 'character_locked',
    upstream_pet_id TEXT NOT NULL DEFAULT '',
    next_release_sequence INTEGER NOT NULL DEFAULT 0,
    default_action_key TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_dpident_owner ON desktop_pet_identities(owner_space_id);
CREATE INDEX IF NOT EXISTS idx_dpident_character ON desktop_pet_identities(source_character_id);
CREATE INDEX IF NOT EXISTS idx_dpident_slug ON desktop_pet_identities(slug);

CREATE TABLE IF NOT EXISTS desktop_pet_package_releases (
    id TEXT PRIMARY KEY,
    pet_id TEXT NOT NULL DEFAULT '',
    owner_space_id TEXT NOT NULL DEFAULT '',
    version TEXT NOT NULL DEFAULT '',
    release_sequence INTEGER NOT NULL DEFAULT 0,
    schema_version INTEGER NOT NULL DEFAULT 2,
    status TEXT NOT NULL DEFAULT 'draft',
    content_root_hash TEXT NOT NULL DEFAULT '',
    manifest_hash TEXT NOT NULL DEFAULT '',
    storage_key TEXT NOT NULL DEFAULT '',
    archive_storage_key TEXT NOT NULL DEFAULT '',
    total_bytes INTEGER NOT NULL DEFAULT 0,
    file_count INTEGER NOT NULL DEFAULT 0,
    action_count INTEGER NOT NULL DEFAULT 0,
    default_action_key TEXT NOT NULL DEFAULT '',
    min_runtime_version TEXT NOT NULL DEFAULT '',
    source_type TEXT NOT NULL DEFAULT 'generated',
    source_processing_task TEXT NOT NULL DEFAULT '',
    source_generation_task TEXT NOT NULL DEFAULT '',
    quality_gate_snapshot_id TEXT NOT NULL DEFAULT '',
    manifest_json TEXT NOT NULL DEFAULT '{}',
    published_at TEXT NOT NULL DEFAULT '',
    active_revision_set_hash TEXT NOT NULL DEFAULT '',
    quality_gate_id TEXT NOT NULL DEFAULT '',
    quality_gate_hash TEXT NOT NULL DEFAULT '',
    build_snapshot_id TEXT NOT NULL DEFAULT '',
    integrity_status TEXT NOT NULL DEFAULT 'unknown',
    compatibility_status TEXT NOT NULL DEFAULT 'unknown',
    lifecycle TEXT NOT NULL DEFAULT 'building',
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT '',
    UNIQUE(pet_id, version)
);
CREATE INDEX IF NOT EXISTS idx_dprel_pet ON desktop_pet_package_releases(pet_id);
CREATE INDEX IF NOT EXISTS idx_dprel_owner ON desktop_pet_package_releases(owner_space_id);
CREATE INDEX IF NOT EXISTS idx_dprel_status ON desktop_pet_package_releases(status);
CREATE INDEX IF NOT EXISTS idx_dprel_content_hash ON desktop_pet_package_releases(content_root_hash);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dprel_pet_seq ON desktop_pet_package_releases(pet_id, release_sequence);

CREATE TABLE IF NOT EXISTS desktop_pet_release_files (
    id TEXT PRIMARY KEY,
    release_id TEXT NOT NULL DEFAULT '',
    path TEXT NOT NULL DEFAULT '',
    sha256 TEXT NOT NULL DEFAULT '',
    bytes INTEGER NOT NULL DEFAULT 0,
    media_type TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL DEFAULT '',
    action_key TEXT NOT NULL DEFAULT '',
    frame_id TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT '',
    UNIQUE(release_id, path)
);
CREATE INDEX IF NOT EXISTS idx_dprf_release ON desktop_pet_release_files(release_id);


CREATE TABLE IF NOT EXISTS desktop_pet_release_build_snapshots (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL DEFAULT '',
    pet_id TEXT NOT NULL DEFAULT '',
    character_id TEXT NOT NULL DEFAULT '',
    processing_task_id TEXT NOT NULL DEFAULT '',
    active_revision_set_hash TEXT NOT NULL DEFAULT '',
    active_revision_set_json TEXT NOT NULL DEFAULT '',
    active_revision_set_id TEXT NOT NULL DEFAULT '',
    quality_gate_id TEXT NOT NULL DEFAULT '',
    quality_gate_hash TEXT NOT NULL DEFAULT '',
    quality_gate_json TEXT NOT NULL DEFAULT '',
    default_action_key TEXT NOT NULL DEFAULT '',
    included_actions_json TEXT NOT NULL DEFAULT '[]',
    required_actions_json TEXT NOT NULL DEFAULT '[]',
    excluded_actions_json TEXT NOT NULL DEFAULT '[]',
    action_snapshots_json TEXT NOT NULL DEFAULT '[]',
    preview_snapshot_json TEXT NOT NULL DEFAULT '',
    package_schema_version INTEGER NOT NULL DEFAULT 2,
    package_contract_hash TEXT NOT NULL DEFAULT '',
    runtime_contract_version TEXT NOT NULL DEFAULT '',
    build_config_hash TEXT NOT NULL DEFAULT '',
    build_profile_id TEXT NOT NULL DEFAULT '',
    build_profile_version TEXT NOT NULL DEFAULT '',
    build_config_json TEXT NOT NULL DEFAULT '',
    input_hash TEXT NOT NULL DEFAULT '',
    snapshot_hash TEXT NOT NULL DEFAULT '',
    evaluation_set_hash TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_drbs_pet ON desktop_pet_release_build_snapshots(pet_id);
CREATE INDEX IF NOT EXISTS idx_drbs_task ON desktop_pet_release_build_snapshots(processing_task_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_drbs_input_hash ON desktop_pet_release_build_snapshots(input_hash);
CREATE UNIQUE INDEX IF NOT EXISTS uq_drbs_snapshot_hash ON desktop_pet_release_build_snapshots(snapshot_hash);

CREATE TABLE IF NOT EXISTS desktop_pet_release_build_operations (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL DEFAULT '',
    pet_id TEXT NOT NULL DEFAULT '',
    snapshot_id TEXT NOT NULL DEFAULT '',
    release_id TEXT NOT NULL DEFAULT '',
    idempotency_key TEXT NOT NULL DEFAULT '',
    execution_id TEXT NOT NULL DEFAULT '',
    input_hash TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL DEFAULT 'created',
    stage TEXT NOT NULL DEFAULT '',
    lease_owner TEXT NOT NULL DEFAULT '',
    lease_expires_at TEXT NOT NULL DEFAULT '',
    heartbeat_at TEXT NOT NULL DEFAULT '',
    staging_path_key TEXT NOT NULL DEFAULT '',
    published_path_key TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    retry_count INTEGER NOT NULL DEFAULT 0,
    result_json TEXT NOT NULL DEFAULT '{}',
    started_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT '',
    completed_at TEXT NOT NULL DEFAULT '',
    UNIQUE(space_id, idempotency_key)
);
CREATE INDEX IF NOT EXISTS idx_drbo_user ON desktop_pet_release_build_operations(space_id);
CREATE INDEX IF NOT EXISTS idx_drbo_state ON desktop_pet_release_build_operations(state);
CREATE INDEX IF NOT EXISTS idx_drbo_release ON desktop_pet_release_build_operations(release_id);
CREATE INDEX IF NOT EXISTS idx_drbo_lease ON desktop_pet_release_build_operations(lease_expires_at);

CREATE TABLE IF NOT EXISTS desktop_pet_release_publish_journals (
    id TEXT PRIMARY KEY,
    operation_id TEXT NOT NULL DEFAULT '',
    release_id TEXT NOT NULL DEFAULT '',
    pet_id TEXT NOT NULL DEFAULT '',
    operation_kind TEXT NOT NULL DEFAULT 'build',
    stage TEXT NOT NULL DEFAULT 'snapshot_created',
    content_root_hash TEXT NOT NULL DEFAULT '',
    staging_path TEXT NOT NULL DEFAULT '',
    published_path TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    snapshot_id TEXT NOT NULL DEFAULT '',
    snapshot_hash TEXT NOT NULL DEFAULT '',
    workspace_storage_key TEXT NOT NULL DEFAULT '',
    staging_storage_key TEXT NOT NULL DEFAULT '',
    published_storage_key TEXT NOT NULL DEFAULT '',
    archive_storage_key TEXT NOT NULL DEFAULT '',
    archive_hash TEXT NOT NULL DEFAULT '',
    file_count INTEGER NOT NULL DEFAULT 0,
    total_bytes INTEGER NOT NULL DEFAULT 0,
    archive_bytes INTEGER NOT NULL DEFAULT 0,
    completed_at TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_drpj_operation ON desktop_pet_release_publish_journals(operation_id);
CREATE INDEX IF NOT EXISTS idx_drpj_release ON desktop_pet_release_publish_journals(release_id);
CREATE INDEX IF NOT EXISTS idx_drpj_stage ON desktop_pet_release_publish_journals(stage);
CREATE INDEX IF NOT EXISTS idx_drpj_kind_stage ON desktop_pet_release_publish_journals(operation_kind, stage);

CREATE TABLE IF NOT EXISTS desktop_pet_installation_operations (
id TEXT PRIMARY KEY,
operation_type TEXT NOT NULL DEFAULT '',
space_id TEXT NOT NULL DEFAULT '',
device_id TEXT NOT NULL DEFAULT '',
installation_id TEXT NOT NULL DEFAULT '',
pet_id TEXT NOT NULL DEFAULT '',
release_id TEXT NOT NULL DEFAULT '',
target_release_id TEXT NOT NULL DEFAULT '',
idempotency_key TEXT NOT NULL DEFAULT '',
request_hash TEXT NOT NULL DEFAULT '',
stage TEXT NOT NULL DEFAULT 'prepare',
status TEXT NOT NULL DEFAULT 'pending',
attempt_number INTEGER NOT NULL DEFAULT 0,
lease_owner TEXT NOT NULL DEFAULT '',
lease_expires_at TEXT NOT NULL DEFAULT '',
staging_path_key TEXT NOT NULL DEFAULT '',
published_path_key TEXT NOT NULL DEFAULT '',
trash_path_key TEXT NOT NULL DEFAULT '',
error_code TEXT NOT NULL DEFAULT '',
error_message TEXT NOT NULL DEFAULT '',
retry_count INTEGER NOT NULL DEFAULT 0,
created_at TEXT DEFAULT '',
started_at TEXT DEFAULT '',
updated_at TEXT DEFAULT '',
completed_at TEXT NOT NULL DEFAULT '',
UNIQUE(space_id, idempotency_key, operation_type)
);
CREATE INDEX IF NOT EXISTS idx_dpinstop_user ON desktop_pet_installation_operations(space_id);
CREATE INDEX IF NOT EXISTS idx_dpinstop_installation ON desktop_pet_installation_operations(installation_id);
CREATE INDEX IF NOT EXISTS idx_dpinstop_status ON desktop_pet_installation_operations(status);
CREATE INDEX IF NOT EXISTS idx_dpinstop_device ON desktop_pet_installation_operations(device_id);
CREATE INDEX IF NOT EXISTS idx_dpinstop_lease ON desktop_pet_installation_operations(lease_owner, lease_expires_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_dpinstop_idempotency_unique ON desktop_pet_installation_operations(space_id, device_id, operation_type, idempotency_key) WHERE idempotency_key <> '';




CREATE TABLE IF NOT EXISTS desktop_pet_action_revisions (
  id TEXT PRIMARY KEY,
  character_id TEXT NOT NULL DEFAULT '',
  processing_task_id TEXT NOT NULL DEFAULT '',
  processing_action_id TEXT NOT NULL DEFAULT '',
  generation_task_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  parent_revision_id TEXT NOT NULL DEFAULT '',
  root_revision_id TEXT NOT NULL DEFAULT '',
  revision_number INTEGER NOT NULL DEFAULT 1,
  revision_type TEXT NOT NULL DEFAULT 'processed',
  status TEXT NOT NULL DEFAULT 'building',
  manifest_path TEXT NOT NULL DEFAULT '',
  manifest_hash TEXT NOT NULL DEFAULT '',
  frame_count INTEGER NOT NULL DEFAULT 0,
  duration_ms INTEGER NOT NULL DEFAULT 0,
  default_fps INTEGER NOT NULL DEFAULT 0,
  loop_type TEXT NOT NULL DEFAULT '',
  return_action TEXT NOT NULL DEFAULT '',
  interruptible INTEGER NOT NULL DEFAULT 1,
  priority_override INTEGER,
  cooldown_ms_override INTEGER,
  quality_evaluation_id TEXT NOT NULL DEFAULT '',
  quality_verdict TEXT NOT NULL DEFAULT '',
  source_processing_revision_id TEXT NOT NULL DEFAULT '',
  quality_overall_score REAL,
  quality_ruleset_version TEXT NOT NULL DEFAULT '',
  quality_source_content_hash TEXT NOT NULL DEFAULT '',
  quality_evaluated_at TEXT NOT NULL DEFAULT '',
  quality_profile_id TEXT NOT NULL DEFAULT '',
  created_by_space_id TEXT NOT NULL DEFAULT '',
  created_from_session_id TEXT NOT NULL DEFAULT '',
  change_summary TEXT NOT NULL DEFAULT '',
  source_summary_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT '',
  ready_at TEXT NOT NULL DEFAULT '',
  space_id TEXT NOT NULL DEFAULT '',
  source_type TEXT NOT NULL DEFAULT '',
  content_hash TEXT NOT NULL DEFAULT '',
  content_hash_version TEXT NOT NULL DEFAULT '',
  action_config_hash TEXT NOT NULL DEFAULT '',
  frame_set_hash TEXT NOT NULL DEFAULT '',
  origin TEXT NOT NULL DEFAULT '',
  playback_mode TEXT NOT NULL DEFAULT '',
  anchor_json TEXT NOT NULL DEFAULT '',
  archived_at TEXT NOT NULL DEFAULT '',
  archived_reason TEXT NOT NULL DEFAULT '',
  action_stream_id TEXT NOT NULL DEFAULT '',
  source_processing_task_id TEXT NOT NULL DEFAULT '',
  source_processing_action_id TEXT NOT NULL DEFAULT '',
  source_processing_attempt_id TEXT NOT NULL DEFAULT '',
  parent_action_revision_id TEXT NOT NULL DEFAULT '',
  root_action_revision_id TEXT NOT NULL DEFAULT '',
  action_config_snapshot_json TEXT NOT NULL DEFAULT '',
  action_spec_hash TEXT NOT NULL DEFAULT '',
  revision_snapshot_json TEXT NOT NULL DEFAULT '',
  revision_snapshot_hash TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_dpar_task ON desktop_pet_action_revisions(processing_task_id);
CREATE INDEX IF NOT EXISTS idx_dpar_action ON desktop_pet_action_revisions(processing_action_id);
CREATE INDEX IF NOT EXISTS idx_dpar_parent ON desktop_pet_action_revisions(parent_revision_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dpar_task_action_rev ON desktop_pet_action_revisions(processing_task_id, action_key, revision_number);
CREATE INDEX IF NOT EXISTS idx_dpar_source_type ON desktop_pet_action_revisions(source_processing_revision_id, source_type);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dpar_processing_baseline_source ON desktop_pet_action_revisions(source_processing_revision_id) WHERE source_type = 'processing_baseline' AND source_processing_revision_id <> '';
CREATE INDEX IF NOT EXISTS idx_dpar_stream_rev ON desktop_pet_action_revisions(action_stream_id, revision_number);






CREATE TABLE IF NOT EXISTS desktop_pet_edit_sessions (
  id TEXT PRIMARY KEY,
  space_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
  action_stream_id TEXT NOT NULL DEFAULT '',
  processing_task_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  base_revision_id TEXT NOT NULL DEFAULT '',
  session_version INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'open',
  cursor INTEGER NOT NULL DEFAULT 0,
  last_operation_seq INTEGER NOT NULL DEFAULT 0,
  checkpoint_id TEXT NOT NULL DEFAULT '',
  draft_snapshot_id TEXT NOT NULL DEFAULT '',
  draft_snapshot_hash TEXT NOT NULL DEFAULT '',
  client_instance_id TEXT NOT NULL DEFAULT '',
  expires_at TEXT NOT NULL DEFAULT '',
  closed_at TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT '',
  committed_revision_id TEXT NOT NULL DEFAULT '',
  base_action_content_hash TEXT NOT NULL DEFAULT '',
  base_binding_revision INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_des_user ON desktop_pet_edit_sessions(space_id);
CREATE INDEX IF NOT EXISTS idx_des_character ON desktop_pet_edit_sessions(character_id);
CREATE INDEX IF NOT EXISTS idx_des_task_action ON desktop_pet_edit_sessions(processing_task_id, action_key);
CREATE INDEX IF NOT EXISTS idx_des_status ON desktop_pet_edit_sessions(status);





CREATE TABLE IF NOT EXISTS desktop_pet_regeneration_jobs (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
  processing_task_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  target_frame_id TEXT NOT NULL DEFAULT '',
  job_type TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'created',
  idempotency_key TEXT NOT NULL DEFAULT '',
  provider_attempt_id TEXT NOT NULL DEFAULT '',
  request_snapshot_json TEXT NOT NULL DEFAULT '{}',
  cost_estimate_json TEXT NOT NULL DEFAULT '{}',
  cost_actual_json TEXT NOT NULL DEFAULT '{}',
  error_code TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT '',
  mode TEXT NOT NULL DEFAULT '',
  active_attempt_id TEXT NOT NULL DEFAULT '',
  provider_receipt_id TEXT NOT NULL DEFAULT '',
  request_hash TEXT NOT NULL DEFAULT '',
  artifact_id TEXT NOT NULL DEFAULT '',
  processing_revision_id TEXT NOT NULL DEFAULT '',
  candidate_revision_id TEXT NOT NULL DEFAULT '',
  quality_evaluation_id TEXT NOT NULL DEFAULT '',
  lease_owner TEXT NOT NULL DEFAULT '',
  lease_expires_at TEXT NOT NULL DEFAULT '',
  heartbeat_at TEXT NOT NULL DEFAULT '',
  base_action_revision_id TEXT NOT NULL DEFAULT '',
  base_content_hash TEXT NOT NULL DEFAULT '',
  base_binding_revision INTEGER NOT NULL DEFAULT 0,
  reject_reason TEXT NOT NULL DEFAULT '',
  rejected_by TEXT NOT NULL DEFAULT '',
  rejected_at TEXT NOT NULL DEFAULT '',
  space_id TEXT NOT NULL DEFAULT '',
  action_stream_id TEXT NOT NULL DEFAULT '',
  draft_snapshot_id TEXT NOT NULL DEFAULT '',
  draft_snapshot_hash TEXT NOT NULL DEFAULT '',
  stage TEXT NOT NULL DEFAULT '',
  generation_attempt_id TEXT NOT NULL DEFAULT '',
  generation_artifact_id TEXT NOT NULL DEFAULT '',
  processing_attempt_id TEXT NOT NULL DEFAULT '',
  execution_id TEXT NOT NULL DEFAULT '',
  attempt_count INTEGER NOT NULL DEFAULT 0,
  instance_id TEXT NOT NULL DEFAULT '',
  cancel_requested_at TEXT NOT NULL DEFAULT '',
  completed_at TEXT NOT NULL DEFAULT '',
  supersedes_job_id TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_drj_session ON desktop_pet_regeneration_jobs(session_id);
CREATE INDEX IF NOT EXISTS idx_drj_status ON desktop_pet_regeneration_jobs(status);
CREATE UNIQUE INDEX IF NOT EXISTS uq_drj_idempotency ON desktop_pet_regeneration_jobs(session_id, idempotency_key);
CREATE INDEX IF NOT EXISTS idx_drj_lease ON desktop_pet_regeneration_jobs(lease_owner, lease_expires_at);
CREATE INDEX IF NOT EXISTS idx_drj_candidate ON desktop_pet_regeneration_jobs(candidate_revision_id);
CREATE INDEX IF NOT EXISTS idx_drj_user ON desktop_pet_regeneration_jobs(space_id);
CREATE INDEX IF NOT EXISTS idx_drj_execution ON desktop_pet_regeneration_jobs(execution_id);
CREATE INDEX IF NOT EXISTS idx_drj_stage ON desktop_pet_regeneration_jobs(stage);

CREATE TABLE IF NOT EXISTS desktop_pet_edit_candidates (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL DEFAULT '',
  job_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
  target_frame_id TEXT NOT NULL DEFAULT '',
  candidate_type TEXT NOT NULL DEFAULT '',
  asset_id TEXT NOT NULL DEFAULT '',
  candidate_revision_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  metadata_json TEXT NOT NULL DEFAULT '{}',
  decided_by TEXT NOT NULL DEFAULT '',
  decided_at TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT '',
  source_type TEXT NOT NULL DEFAULT '',
  parent_action_revision_id TEXT NOT NULL DEFAULT '',
  base_binding_revision INTEGER NOT NULL DEFAULT 0,
  quality_status TEXT NOT NULL DEFAULT '',
  quality_evaluation_id TEXT NOT NULL DEFAULT '',
  content_hash TEXT NOT NULL DEFAULT '',
  frame_set_hash TEXT NOT NULL DEFAULT '',
  action_config_hash TEXT NOT NULL DEFAULT '',
  accepted_at TEXT NOT NULL DEFAULT '',
  rejected_at TEXT NOT NULL DEFAULT '',
  reject_reason TEXT NOT NULL DEFAULT '',
  space_id TEXT NOT NULL DEFAULT '',
  action_stream_id TEXT NOT NULL DEFAULT '',
  candidate_version INTEGER NOT NULL DEFAULT 0,
  draft_snapshot_id TEXT NOT NULL DEFAULT '',
  draft_snapshot_hash TEXT NOT NULL DEFAULT '',
  parent_content_hash TEXT NOT NULL DEFAULT '',
  effective_verdict TEXT NOT NULL DEFAULT '',
  activation_policy TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_dec_session_job ON desktop_pet_edit_candidates(session_id, job_id);
CREATE INDEX IF NOT EXISTS idx_dec_status ON desktop_pet_edit_candidates(status);
CREATE INDEX IF NOT EXISTS idx_dec_revision ON desktop_pet_edit_candidates(candidate_revision_id);
CREATE INDEX IF NOT EXISTS idx_dec_status_quality ON desktop_pet_edit_candidates(status, quality_status);
CREATE INDEX IF NOT EXISTS idx_dec_user ON desktop_pet_edit_candidates(space_id);
CREATE INDEX IF NOT EXISTS idx_dec_candidate_rev ON desktop_pet_edit_candidates(candidate_revision_id);













































CREATE TABLE IF NOT EXISTS desktop_pet_quality_evaluations (
  id TEXT PRIMARY KEY,
  character_id TEXT NOT NULL DEFAULT '',
  processing_task_id TEXT NOT NULL,
  processing_action_id TEXT NOT NULL,
  action_revision_id TEXT NOT NULL,
  measurement_set_id TEXT NOT NULL,
  action_key TEXT NOT NULL,
  execution_status TEXT NOT NULL DEFAULT 'pending',
  verdict TEXT DEFAULT '',
  overall_score REAL,
  overall_confidence REAL NOT NULL DEFAULT 0,
  profile_snapshot_json TEXT NOT NULL,
  profile_hash TEXT NOT NULL,
  engine_version TEXT NOT NULL,
  quality_mode TEXT DEFAULT 'balanced',
  report_path TEXT DEFAULT '',
  report_hash TEXT DEFAULT '',
  supersedes_evaluation_id TEXT DEFAULT '',
  execution_id TEXT DEFAULT '',
  worker_id TEXT DEFAULT '',
  lease_expires_at TEXT DEFAULT '',
  error_code TEXT DEFAULT '',
  error_message TEXT DEFAULT '',
  is_active INTEGER NOT NULL DEFAULT 0,
  started_at TEXT DEFAULT '',
  completed_at TEXT DEFAULT '',
  space_id TEXT NOT NULL DEFAULT '',
  action_content_hash TEXT NOT NULL DEFAULT '',
  processing_revision_id TEXT NOT NULL DEFAULT '',
  profile_id TEXT NOT NULL DEFAULT '',
  profile_version TEXT NOT NULL DEFAULT '',
  rule_set_version TEXT NOT NULL DEFAULT '',
  ruleset_content_hash TEXT NOT NULL DEFAULT '',
  measurement_version TEXT NOT NULL DEFAULT '',
  lease_owner TEXT NOT NULL DEFAULT '',
  heartbeat_at TEXT NOT NULL DEFAULT '',
  attempt_count INTEGER NOT NULL DEFAULT 0,
  idempotency_key TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dpqe_input ON desktop_pet_quality_evaluations(action_revision_id, profile_hash, engine_version);
CREATE INDEX IF NOT EXISTS idx_dpqe_task ON desktop_pet_quality_evaluations(processing_task_id);
CREATE INDEX IF NOT EXISTS idx_dpqe_action ON desktop_pet_quality_evaluations(processing_action_id);
CREATE INDEX IF NOT EXISTS idx_dpqe_status ON desktop_pet_quality_evaluations(execution_status);




















-- ====== 自动同步：迁移中缺失的表定义 ======

-- 来源: consolidation_auto_migrate.go
CREATE TABLE IF NOT EXISTS outbox_records (
				id TEXT PRIMARY KEY,
				aggregate_id TEXT DEFAULT '',
				event_type TEXT DEFAULT '',
				payload TEXT DEFAULT '',
				payload_version TEXT DEFAULT '',
				status TEXT DEFAULT '',
				lease_owner TEXT DEFAULT '',
				lease_token TEXT DEFAULT '',
				leased_until TEXT DEFAULT '',
				available_at TEXT DEFAULT '',
				published_at TEXT DEFAULT '',
				updated_at TEXT DEFAULT '',
				retry_count INTEGER DEFAULT 0,
				max_retries INTEGER DEFAULT 0,
				next_retry_at TEXT DEFAULT '',
				last_error TEXT DEFAULT '',
				idempotency_key TEXT DEFAULT '',
				created_at TEXT DEFAULT ''
			);

-- 来源: consolidation_auto_migrate.go
CREATE TABLE IF NOT EXISTS dead_letter_records (
				id TEXT PRIMARY KEY,
				outbox_id TEXT UNIQUE,
				event_type TEXT DEFAULT '',
				payload TEXT DEFAULT '',
				status TEXT DEFAULT '',
				retry_count INTEGER DEFAULT 0,
				max_retries INTEGER DEFAULT 0,
				next_retry_at TEXT DEFAULT '',
				last_error TEXT DEFAULT '',
				created_at TEXT DEFAULT '',
				updated_at TEXT DEFAULT ''
			);

-- 来源: consolidation_auto_migrate.go
CREATE TABLE IF NOT EXISTS output_leases (
				id TEXT PRIMARY KEY,
				interaction_id TEXT DEFAULT '',
				character_id TEXT DEFAULT '',
				space_id TEXT DEFAULT '',
				channel TEXT DEFAULT '',
				owner_token TEXT DEFAULT '',
				generation INTEGER DEFAULT 0,
				status TEXT DEFAULT '',
				acquired_at TEXT DEFAULT '',
				expires_at TEXT DEFAULT '',
				released_at TEXT DEFAULT '',
				preempted_by TEXT DEFAULT ''
			);

-- 来源: extensions.go
CREATE TABLE IF NOT EXISTS extensions (
id TEXT PRIMARY KEY,
extension_id TEXT NOT NULL UNIQUE,
kind TEXT NOT NULL DEFAULT 'Skill',
name TEXT NOT NULL DEFAULT '',
current_version TEXT NOT NULL DEFAULT '',
source TEXT NOT NULL DEFAULT '',
enabled INTEGER NOT NULL DEFAULT 0,
manifest_json TEXT NOT NULL DEFAULT '{}',
normalized_manifest_json TEXT NOT NULL DEFAULT '{}',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
archived_at TEXT NOT NULL DEFAULT ''
,
    owner_space_id TEXT NOT NULL DEFAULT '',
    scope_type TEXT NOT NULL DEFAULT 'global',
    scope_id TEXT NOT NULL DEFAULT '',
    lifecycle_status TEXT NOT NULL DEFAULT 'registered',
    health_status TEXT NOT NULL DEFAULT 'unknown',
    last_error_code TEXT NOT NULL DEFAULT '',
    last_error_at TEXT NOT NULL DEFAULT '',
    enabled_at TEXT NOT NULL DEFAULT '',
    disabled_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS extension_versions (
id TEXT PRIMARY KEY,
extension_id TEXT NOT NULL,
version TEXT NOT NULL,
manifest_json TEXT NOT NULL DEFAULT '{}',
checksum TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
    artifact_id TEXT NOT NULL DEFAULT '',
    artifact_hash TEXT NOT NULL DEFAULT '',
    package_hash TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT '',
    signature_status TEXT NOT NULL DEFAULT 'unsigned',
    signer_fingerprint TEXT NOT NULL DEFAULT '',
    compatibility_status TEXT NOT NULL DEFAULT '',
    capabilities_json TEXT NOT NULL DEFAULT '[]',
    installed_by TEXT NOT NULL DEFAULT '',
    validation_status TEXT NOT NULL DEFAULT '',
    test_status TEXT NOT NULL DEFAULT '',
    archived_at TEXT NOT NULL DEFAULT '',
    package_blob BLOB,
    artifact_status TEXT NOT NULL DEFAULT 'ready',
    activation_status TEXT NOT NULL DEFAULT 'inactive',
    operation_id TEXT NOT NULL DEFAULT '',
    failure_code TEXT NOT NULL DEFAULT '',
UNIQUE(extension_id, version)
);

-- 来源: extensions.go

-- 来源: extensions.go

-- 来源: extensions.go

-- 来源: extension_agent_skills.go
CREATE TABLE IF NOT EXISTS extension_owned_resources (
id TEXT PRIMARY KEY,
extension_id TEXT NOT NULL,
extension_version TEXT NOT NULL DEFAULT '',
resource_type TEXT NOT NULL,
resource_id TEXT NOT NULL,
owner_scope_type TEXT NOT NULL DEFAULT 'global',
owner_scope_id TEXT NOT NULL DEFAULT '',
source_run_id TEXT NOT NULL DEFAULT '',
status TEXT NOT NULL DEFAULT 'active',
cleanup_attempts INTEGER NOT NULL DEFAULT 0,
last_error TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL,
updated_at TEXT NOT NULL,
cleaned_at TEXT NOT NULL DEFAULT '',
UNIQUE(extension_id, resource_type, resource_id)
);

-- 来源: extension_packages.go
CREATE TABLE IF NOT EXISTS extension_package_import_sessions (
id TEXT PRIMARY KEY,
space_id TEXT NOT NULL,
scope_type TEXT NOT NULL,
scope_id TEXT NOT NULL DEFAULT '',
format TEXT NOT NULL,
package_hash TEXT NOT NULL,
status TEXT NOT NULL,
preview_json TEXT NOT NULL DEFAULT '{}',
package_blob BLOB NOT NULL,
file_name TEXT NOT NULL DEFAULT '',
expires_at TEXT NOT NULL,
consumed_at TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL,
updated_at TEXT NOT NULL
);

-- 来源: extension_packages.go

-- 来源: extension_packages.go

-- 来源: extension_packages.go

-- 来源: extension_packages.go
CREATE TABLE IF NOT EXISTS extension_package_exports (
id TEXT PRIMARY KEY,
space_id TEXT NOT NULL,
extension_id TEXT NOT NULL,
file_name TEXT NOT NULL,
mime TEXT NOT NULL,
package_hash TEXT NOT NULL,
content_blob BLOB NOT NULL,
expires_at TEXT NOT NULL,
created_at TEXT NOT NULL,
downloaded_at TEXT NOT NULL DEFAULT ''
);

-- 来源: extension_schedule_ownership_repair.go
CREATE TABLE IF NOT EXISTS schedules (
id TEXT PRIMARY KEY,
title TEXT NOT NULL DEFAULT '',
description TEXT NOT NULL DEFAULT '',
due_time TEXT NOT NULL DEFAULT '',
repeat_mode TEXT NOT NULL DEFAULT 'none',
channel TEXT NOT NULL DEFAULT 'all',
status TEXT NOT NULL DEFAULT 'pending',
source_type TEXT NOT NULL DEFAULT 'user',
source_extension_id TEXT NOT NULL DEFAULT '',
source_extension_version TEXT NOT NULL DEFAULT '',
source_run_id TEXT NOT NULL DEFAULT '',
owner_scope_type TEXT NOT NULL DEFAULT 'global',
owner_scope_id TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT ''
);

-- 来源: extension_scope_bindings.go
CREATE TABLE IF NOT EXISTS extension_scope_bindings (
id TEXT PRIMARY KEY,
extension_id TEXT NOT NULL,
scope_type TEXT NOT NULL,
scope_id TEXT NOT NULL DEFAULT '',
enabled INTEGER NOT NULL DEFAULT 0,
created_at TEXT NOT NULL,
updated_at TEXT NOT NULL,
UNIQUE(extension_id, scope_type, scope_id)
);

-- 来源: extension_workshop.go

-- 来源: extension_workshop.go

-- 来源: extension_workshop.go

-- 来源: extension_workshop.go

-- 来源: interaction_records_create.go
CREATE TABLE IF NOT EXISTS interaction_records (
				id TEXT PRIMARY KEY,
				space_id TEXT,
				character_id TEXT,
				conversation_id TEXT,
				thread_id TEXT NOT NULL DEFAULT '',
				channel TEXT,
				peer_id TEXT,
				session_id TEXT,
				source TEXT,
				request_id TEXT,
				priority INTEGER,
				path_type TEXT,
				status TEXT,
				status_version INTEGER NOT NULL DEFAULT 0,
				supersedes_id TEXT,
				superseded_by_id TEXT,
				cancel_reason TEXT,
				error_code TEXT,
				error_message TEXT,
				result_ref TEXT,
				commit_id TEXT,
				executor_id TEXT,
				deadline_at DATETIME,
				cancel_requested_at DATETIME,
				created_at DATETIME,
				started_at DATETIME,
				committed_at DATETIME,
				completed_at DATETIME,
				updated_at DATETIME,
				owner_instance_id TEXT DEFAULT '',
				heartbeat_at DATETIME,
				commit_token TEXT DEFAULT '',
				commit_owner TEXT DEFAULT '',
				commit_acquired_at DATETIME,
				result_message_ids TEXT DEFAULT '',
				delivery_intent_ids TEXT DEFAULT '',
				correlation_id TEXT DEFAULT '',
				causation_id TEXT DEFAULT '',
				recovery_descriptor_json TEXT DEFAULT ''
			);


CREATE INDEX IF NOT EXISTS idx_interaction_records_thread_id ON interaction_records(thread_id);

-- 来源: message_plan.go
CREATE TABLE IF NOT EXISTS delivery_intents (
id TEXT PRIMARY KEY,
interaction_id TEXT NOT NULL DEFAULT '',
channel TEXT NOT NULL DEFAULT '',
peer_id TEXT NOT NULL DEFAULT '',
content_type TEXT NOT NULL DEFAULT '',
payload BLOB,
status TEXT NOT NULL DEFAULT 'pending',
created_at TEXT NOT NULL DEFAULT '',
sent_at TEXT NOT NULL DEFAULT '',
delivered_at TEXT NOT NULL DEFAULT '',
retry_count INTEGER NOT NULL DEFAULT 0,
max_retries INTEGER NOT NULL DEFAULT 5,
last_error TEXT NOT NULL DEFAULT '',
lease_owner TEXT NOT NULL DEFAULT '',
lease_token TEXT NOT NULL DEFAULT '',
lease_until TEXT NOT NULL DEFAULT '',
next_retry TEXT NOT NULL DEFAULT '',
delivery_group_id TEXT NOT NULL DEFAULT '',
delivery_sequence INTEGER NOT NULL DEFAULT 0
);

-- 来源: plugin_runtime.go

-- 来源: plugin_runtime.go

-- 来源: plugin_runtime.go

-- 来源: plugin_runtime.go
CREATE TABLE IF NOT EXISTS extension_event_deliveries (
id TEXT PRIMARY KEY,
event_id TEXT NOT NULL,
plugin_id TEXT NOT NULL,
status TEXT NOT NULL DEFAULT 'pending',
attempts INTEGER NOT NULL DEFAULT 0,
next_attempt_at TEXT NOT NULL DEFAULT '',
last_error_code TEXT NOT NULL DEFAULT '',
last_error_detail TEXT NOT NULL DEFAULT '',
processed_at TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
UNIQUE(event_id, plugin_id)
);

-- 来源: plugin_runtime.go

-- 来源: plugin_runtime.go

-- 来源: plugin_runtime.go

-- 来源: qdrant_collections.go
CREATE TABLE IF NOT EXISTS qdrant_collection_versions (
				collection_name TEXT PRIMARY KEY,
				schema_version TEXT NOT NULL DEFAULT '',
				vector_dim INTEGER NOT NULL,
				distance TEXT NOT NULL DEFAULT 'Cosine',
    created_at TEXT DEFAULT ''
			);

-- 来源: runtime_queue.go
CREATE TABLE IF NOT EXISTS runtime_queue (
				task_id TEXT PRIMARY KEY,
				scope TEXT NOT NULL DEFAULT '',
				priority INTEGER NOT NULL DEFAULT 5,
				status TEXT NOT NULL DEFAULT 'pending',
				available_at DATETIME,
				deadline DATETIME,
				lease TEXT DEFAULT '',
				attempt INTEGER DEFAULT 0,
				payload_version INTEGER DEFAULT 1
			);

-- 来源: surreal_schema.go
CREATE TABLE IF NOT EXISTS surreal_schema_versions (
	projection_contract TEXT NOT NULL DEFAULT '',
				schema_version TEXT PRIMARY KEY,
				entity_types TEXT NOT NULL DEFAULT '',
				edge_types TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT ''
			);

-- 来源: temporal_core.go
CREATE TABLE IF NOT EXISTS temporal_profiles (
id TEXT PRIMARY KEY,
owner_type TEXT NOT NULL,
owner_id TEXT NOT NULL,
timezone_mode TEXT NOT NULL DEFAULT 'follow_device',
timezone TEXT NOT NULL DEFAULT 'Asia/Shanghai',
locale TEXT NOT NULL DEFAULT 'zh-CN',
calendar_system TEXT NOT NULL DEFAULT 'gregorian',
week_start INTEGER NOT NULL DEFAULT 1,
holiday_region TEXT NOT NULL DEFAULT '',
hemisphere TEXT NOT NULL DEFAULT 'unknown',
daypart_config_json TEXT NOT NULL DEFAULT '{}',
quiet_hours_json TEXT NOT NULL DEFAULT '{}',
auto_detect_timezone INTEGER NOT NULL DEFAULT 1,
travel_mode INTEGER NOT NULL DEFAULT 0,
awareness_level INTEGER NOT NULL DEFAULT 70,
source TEXT NOT NULL DEFAULT 'fallback',
confidence INTEGER NOT NULL DEFAULT 30,
pending_timezone TEXT NOT NULL DEFAULT '',
enabled INTEGER NOT NULL DEFAULT 1,
holiday_awareness INTEGER NOT NULL DEFAULT 1,
daypart_awareness INTEGER NOT NULL DEFAULT 1,
anniversary_awareness INTEGER NOT NULL DEFAULT 1,
memory_resonance INTEGER NOT NULL DEFAULT 1,
allow_shared_date_mention INTEGER NOT NULL DEFAULT 1,
version INTEGER NOT NULL DEFAULT 1,
created_at_utc DATETIME NOT NULL,
updated_at_utc DATETIME NOT NULL,
UNIQUE(owner_type, owner_id)
);

-- 来源: temporal_core.go
CREATE TABLE IF NOT EXISTS temporal_anchors (
id TEXT PRIMARY KEY,
scope_type TEXT NOT NULL DEFAULT 'space',
space_id TEXT NOT NULL DEFAULT '',
character_id TEXT NOT NULL DEFAULT '',
anchor_type TEXT NOT NULL DEFAULT 'custom',
title TEXT NOT NULL DEFAULT '',
description TEXT NOT NULL DEFAULT '',
time_kind TEXT NOT NULL,
instant_at_utc DATETIME,
end_at_utc DATETIME,
local_date TEXT NOT NULL DEFAULT '',
local_time TEXT NOT NULL DEFAULT '',
timezone TEXT NOT NULL DEFAULT 'Asia/Shanghai',
rrule TEXT NOT NULL DEFAULT '',
duration_seconds INTEGER NOT NULL DEFAULT 0,
pre_window_seconds INTEGER NOT NULL DEFAULT 0,
post_window_seconds INTEGER NOT NULL DEFAULT 0,
importance INTEGER NOT NULL DEFAULT 0,
confidence INTEGER NOT NULL DEFAULT 0,
sensitivity_level TEXT NOT NULL DEFAULT 'internal',
allow_prompt_mention INTEGER NOT NULL DEFAULT 0,
allow_proactive_mention INTEGER NOT NULL DEFAULT 0,
requires_confirmation INTEGER NOT NULL DEFAULT 0,
source TEXT NOT NULL DEFAULT 'manual',
source_ref TEXT NOT NULL DEFAULT '',
payload_json TEXT NOT NULL DEFAULT '{}',
status TEXT NOT NULL DEFAULT 'active',
next_occurrence_at_utc DATETIME,
last_occurrence_at_utc DATETIME,
created_at_utc DATETIME NOT NULL,
updated_at_utc DATETIME NOT NULL
);

-- 来源: temporal_core.go
CREATE TABLE IF NOT EXISTS temporal_events (
id TEXT PRIMARY KEY,
event_type TEXT NOT NULL,
space_id TEXT NOT NULL DEFAULT '',
character_id TEXT NOT NULL DEFAULT '',
anchor_id TEXT NOT NULL DEFAULT '',
occurred_at_utc DATETIME NOT NULL,
effective_local_date TEXT NOT NULL DEFAULT '',
timezone TEXT NOT NULL DEFAULT 'UTC',
salience REAL NOT NULL DEFAULT 0,
source TEXT NOT NULL DEFAULT 'temporal-runtime',
source_event_id TEXT NOT NULL DEFAULT '',
idempotency_key TEXT NOT NULL,
payload_json TEXT NOT NULL DEFAULT '{}',
created_at_utc DATETIME NOT NULL,
UNIQUE(idempotency_key)
);

-- 来源: temporal_core.go
CREATE TABLE IF NOT EXISTS memory_temporal_metadata (
memory_id TEXT PRIMARY KEY,
occurred_at_utc DATETIME,
ended_at_utc DATETIME,
timezone TEXT NOT NULL DEFAULT '',
local_date TEXT NOT NULL DEFAULT '',
daypart TEXT NOT NULL DEFAULT '',
temporal_precision TEXT NOT NULL DEFAULT 'unknown',
valid_from_utc DATETIME,
valid_to_utc DATETIME,
anchor_ids_json TEXT NOT NULL DEFAULT '[]',
source_time_text TEXT NOT NULL DEFAULT '',
created_at_utc DATETIME NOT NULL,
updated_at_utc DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_memory_temporal_validity ON memory_temporal_metadata(valid_from_utc, valid_to_utc);
CREATE INDEX IF NOT EXISTS idx_memory_temporal_local_date ON memory_temporal_metadata(local_date);

-- 来源: temporal_relationship_time.go
CREATE TABLE IF NOT EXISTS temporal_global_presence_states (
space_id TEXT PRIMARY KEY,
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
);

-- 来源: temporal_relationship_time.go
CREATE TABLE IF NOT EXISTS temporal_relationship_presence_states (
id TEXT PRIMARY KEY,
space_id TEXT NOT NULL DEFAULT '',
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
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_temporal_relationship_presence_scope
ON temporal_relationship_presence_states(space_id, character_id);

-- 来源: temporal_relationship_time.go
CREATE TABLE IF NOT EXISTS temporal_cadence_samples (
id TEXT PRIMARY KEY,
space_id TEXT NOT NULL DEFAULT '',
character_id TEXT NOT NULL DEFAULT '',
interaction_id TEXT NOT NULL DEFAULT '',
previous_interaction_at_utc TEXT NOT NULL DEFAULT '',
current_interaction_at_utc TEXT NOT NULL DEFAULT '',
gap_seconds REAL NOT NULL DEFAULT 0,
sample_kind TEXT NOT NULL DEFAULT 'relationship',
included INTEGER NOT NULL DEFAULT 1,
created_at_utc TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_temporal_cadence_interaction
ON temporal_cadence_samples(interaction_id, sample_kind);

-- 来源: temporal_relationship_time.go
CREATE TABLE IF NOT EXISTS temporal_reunion_episodes (
id TEXT PRIMARY KEY,
space_id TEXT NOT NULL DEFAULT '',
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
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_temporal_reunion_idempotency
ON temporal_reunion_episodes(idempotency_key);

-- 来源: temporal_relationship_time.go
CREATE TABLE IF NOT EXISTS temporal_interaction_receipts (
id TEXT PRIMARY KEY,
request_id TEXT NOT NULL DEFAULT '',
interaction_id TEXT NOT NULL DEFAULT '',
space_id TEXT NOT NULL DEFAULT '',
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
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_temporal_receipt_request
ON temporal_interaction_receipts(space_id, request_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_temporal_receipt_interaction
ON temporal_interaction_receipts(interaction_id);

-- 来源: temporal_relationship_time.go
CREATE TABLE IF NOT EXISTS temporal_effect_ledger (
id TEXT PRIMARY KEY,
effect_key TEXT NOT NULL DEFAULT '',
effect_type TEXT NOT NULL DEFAULT '',
space_id TEXT NOT NULL DEFAULT '',
character_id TEXT NOT NULL DEFAULT '',
reunion_episode_id TEXT NOT NULL DEFAULT '',
interaction_id TEXT NOT NULL DEFAULT '',
payload_json TEXT NOT NULL DEFAULT '{}',
applied_at_utc TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_temporal_effect_key
ON temporal_effect_ledger(effect_key);

-- 来源: temporal_relationship_time.go
CREATE TABLE IF NOT EXISTS temporal_relationship_time_settings (
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
);




CREATE TABLE IF NOT EXISTS psyche_states (
character_id TEXT PRIMARY KEY,
version TEXT DEFAULT '',
state_version INTEGER DEFAULT 0,
emotion TEXT DEFAULT '{}',
mood TEXT DEFAULT '{}',
stress REAL DEFAULT 0,
energy REAL DEFAULT 0.7,
created_at TEXT DEFAULT '',
updated_at TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS psyche_events (
id TEXT PRIMARY KEY,
character_id TEXT NOT NULL DEFAULT '',
event_type TEXT NOT NULL DEFAULT '',
event_data TEXT DEFAULT '{}',
created_at TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS psyche_snapshots (
id TEXT PRIMARY KEY,
character_id TEXT NOT NULL DEFAULT '',
snapshot_data TEXT DEFAULT '{}',
created_at TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS relationship_states (
id TEXT PRIMARY KEY,
space_id TEXT NOT NULL DEFAULT '',
character_id TEXT NOT NULL DEFAULT '',
relation_type TEXT NOT NULL DEFAULT '',
relation_data TEXT DEFAULT '{}',
channel TEXT NOT NULL DEFAULT '',
created_at TEXT DEFAULT '',
updated_at TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS relationship_events (
id TEXT PRIMARY KEY,
character_id TEXT NOT NULL DEFAULT '',
event_type TEXT NOT NULL DEFAULT '',
event_data TEXT DEFAULT '{}',
created_at TEXT DEFAULT ''
);


CREATE TABLE IF NOT EXISTS mcp_servers (
id TEXT PRIMARY KEY,
name TEXT NOT NULL,
display_name TEXT NOT NULL DEFAULT '',
description TEXT NOT NULL DEFAULT '',
transport TEXT NOT NULL,
endpoint TEXT NOT NULL DEFAULT '',
command TEXT NOT NULL DEFAULT '',
args_json TEXT NOT NULL DEFAULT '[]',
work_dir TEXT NOT NULL DEFAULT '',
protocol_version TEXT NOT NULL DEFAULT '',
server_info_json TEXT NOT NULL DEFAULT '{}',
capabilities_json TEXT NOT NULL DEFAULT '{}',
instructions TEXT NOT NULL DEFAULT '',
auth_type TEXT NOT NULL DEFAULT 'none',
enabled INTEGER NOT NULL DEFAULT 0,
status TEXT NOT NULL DEFAULT 'draft',
source TEXT NOT NULL DEFAULT 'manual',
owner_kind TEXT NOT NULL DEFAULT 'manual',
owner_extension_id TEXT NOT NULL DEFAULT '',
owner_module_id TEXT NOT NULL DEFAULT '',
owner_contribution_id TEXT NOT NULL DEFAULT '',
runtime_placement TEXT NOT NULL DEFAULT '',
desired_enabled INTEGER NOT NULL DEFAULT 0,
config_revision INTEGER NOT NULL DEFAULT 1,
package_generation INTEGER NOT NULL DEFAULT 0,
normalized_identity TEXT NOT NULL UNIQUE,
configuration_hash TEXT NOT NULL DEFAULT '',
last_connected_at TEXT NOT NULL DEFAULT '',
last_error_code TEXT NOT NULL DEFAULT '',
last_error_message TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS mcp_server_scope_bindings (
id TEXT PRIMARY KEY,
server_id TEXT NOT NULL,
scope_type TEXT NOT NULL,
scope_id TEXT NOT NULL DEFAULT '',
enabled INTEGER NOT NULL DEFAULT 0,
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
UNIQUE(server_id, scope_type, scope_id)
);

CREATE TABLE IF NOT EXISTS mcp_server_credentials (
id TEXT PRIMARY KEY,
server_id TEXT NOT NULL,
credential_type TEXT NOT NULL,
secret_reference TEXT NOT NULL,
expires_at TEXT NOT NULL DEFAULT '',
scopes_json TEXT NOT NULL DEFAULT '[]',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS mcp_server_capabilities (
id TEXT PRIMARY KEY,
server_id TEXT NOT NULL,
capability TEXT NOT NULL,
configuration_json TEXT NOT NULL DEFAULT '{}',
enabled INTEGER NOT NULL DEFAULT 0,
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
UNIQUE(server_id, capability)
);

CREATE TABLE IF NOT EXISTS mcp_tools (
id TEXT PRIMARY KEY,
server_id TEXT NOT NULL,
remote_name TEXT NOT NULL,
skill_id TEXT NOT NULL UNIQUE,
title TEXT NOT NULL DEFAULT '',
description TEXT NOT NULL DEFAULT '',
input_schema_json TEXT NOT NULL DEFAULT '{}',
output_schema_json TEXT NOT NULL DEFAULT '{}',
annotations_json TEXT NOT NULL DEFAULT '{}',
execution_json TEXT NOT NULL DEFAULT '{}',
capability_hints_json TEXT NOT NULL DEFAULT '[]',
risk_level TEXT NOT NULL DEFAULT 'high',
enabled INTEGER NOT NULL DEFAULT 0,
hash TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
UNIQUE(server_id, remote_name)
);

CREATE TABLE IF NOT EXISTS mcp_resources (
id TEXT PRIMARY KEY,
server_id TEXT NOT NULL,
uri TEXT NOT NULL,
name TEXT NOT NULL DEFAULT '',
title TEXT NOT NULL DEFAULT '',
description TEXT NOT NULL DEFAULT '',
mime_type TEXT NOT NULL DEFAULT '',
enabled INTEGER NOT NULL DEFAULT 0,
hash TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
UNIQUE(server_id, uri)
);

CREATE TABLE IF NOT EXISTS mcp_resource_templates (
id TEXT PRIMARY KEY,
server_id TEXT NOT NULL,
uri_template TEXT NOT NULL,
name TEXT NOT NULL DEFAULT '',
title TEXT NOT NULL DEFAULT '',
description TEXT NOT NULL DEFAULT '',
mime_type TEXT NOT NULL DEFAULT '',
enabled INTEGER NOT NULL DEFAULT 0,
hash TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
UNIQUE(server_id, uri_template)
);

CREATE TABLE IF NOT EXISTS mcp_prompts (
id TEXT PRIMARY KEY,
server_id TEXT NOT NULL,
remote_name TEXT NOT NULL,
title TEXT NOT NULL DEFAULT '',
description TEXT NOT NULL DEFAULT '',
arguments_json TEXT NOT NULL DEFAULT '[]',
enabled INTEGER NOT NULL DEFAULT 0,
hash TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
UNIQUE(server_id, remote_name)
);

CREATE TABLE IF NOT EXISTS mcp_dependency_links (
id TEXT PRIMARY KEY,
agent_skill_extension_id TEXT NOT NULL,
server_id TEXT NOT NULL,
dependency_name TEXT NOT NULL,
required INTEGER NOT NULL DEFAULT 1,
install_status TEXT NOT NULL DEFAULT 'missing',
binding_status TEXT NOT NULL DEFAULT 'missing',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
UNIQUE(agent_skill_extension_id, server_id, dependency_name)
);

CREATE TABLE IF NOT EXISTS mcp_operations (
id TEXT PRIMARY KEY,
type TEXT NOT NULL,
status TEXT NOT NULL DEFAULT 'pending',
server_id TEXT NOT NULL DEFAULT '',
agent_skill_id TEXT NOT NULL DEFAULT '',
scope_type TEXT NOT NULL DEFAULT '',
scope_id TEXT NOT NULL DEFAULT '',
plan_json TEXT NOT NULL DEFAULT '{}',
result_json TEXT NOT NULL DEFAULT '{}',
error_code TEXT NOT NULL DEFAULT '',
error_message TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS mcp_oauth_sessions (
id TEXT PRIMARY KEY,
server_id TEXT NOT NULL,
state_hash TEXT NOT NULL UNIQUE,
code_verifier_reference TEXT NOT NULL,
redirect_uri TEXT NOT NULL,
requested_scopes_json TEXT NOT NULL DEFAULT '[]',
status TEXT NOT NULL DEFAULT 'pending',
expires_at TEXT NOT NULL,
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS mcp_tasks (
id TEXT PRIMARY KEY,
server_id TEXT NOT NULL,
remote_task_id TEXT NOT NULL,
character_id TEXT NOT NULL DEFAULT '',
run_id TEXT NOT NULL DEFAULT '',
status TEXT NOT NULL DEFAULT 'working',
status_message TEXT NOT NULL DEFAULT '',
result_json TEXT NOT NULL DEFAULT '{}',
expires_at TEXT NOT NULL DEFAULT '',
last_updated_at TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
UNIQUE(server_id, remote_task_id)
);

CREATE TABLE IF NOT EXISTS mcp_audit_logs (
id TEXT PRIMARY KEY,
server_id TEXT NOT NULL,
operation TEXT NOT NULL,
tool_name TEXT NOT NULL DEFAULT '',
character_id TEXT NOT NULL DEFAULT '',
conversation_id TEXT NOT NULL DEFAULT '',
channel TEXT NOT NULL DEFAULT '',
trace_id TEXT NOT NULL DEFAULT '',
operation_id TEXT NOT NULL DEFAULT '',
status TEXT NOT NULL DEFAULT '',
duration_ms INTEGER NOT NULL DEFAULT 0,
error_code TEXT NOT NULL DEFAULT '',
summary_json TEXT NOT NULL DEFAULT '{}',
created_at TEXT NOT NULL DEFAULT ''
);






CREATE TABLE IF NOT EXISTS desktop_pet_runtime_sessions (
  id TEXT DEFAULT '',
  runtime_instance_id TEXT PRIMARY KEY,
  space_id TEXT NOT NULL DEFAULT '',
  device_id TEXT NOT NULL DEFAULT '',
  runtime_id TEXT NOT NULL DEFAULT '',
  protocol_version TEXT NOT NULL DEFAULT '',
  client_version TEXT NOT NULL DEFAULT '',
  connection_generation INTEGER NOT NULL DEFAULT 0,
  runtime_version TEXT NOT NULL DEFAULT '',
  runtime_contract_version TEXT NOT NULL DEFAULT '',
  capabilities_json TEXT NOT NULL DEFAULT '{}',
  capabilities_hash TEXT NOT NULL DEFAULT '',
  last_applied_desired_revision INTEGER NOT NULL DEFAULT 0,
  last_processed_command_sequence INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT '',
  connected_at TEXT NOT NULL DEFAULT '',
  last_heartbeat_at TEXT NOT NULL DEFAULT '',
  disconnected_at TEXT NOT NULL DEFAULT '',
  superseded_at TEXT NOT NULL DEFAULT '',
  superseded_by TEXT NOT NULL DEFAULT '',
  last_command_sequence INTEGER NOT NULL DEFAULT 0,
  last_event_sequence INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT ''
);




CREATE TABLE IF NOT EXISTS desktop_pet_runtime_desired_states (
    id TEXT PRIMARY KEY,
    installation_id TEXT NOT NULL DEFAULT '',
    space_id TEXT NOT NULL DEFAULT '',
    device_id TEXT NOT NULL DEFAULT '',
    runtime_id TEXT NOT NULL DEFAULT '',
    pet_id TEXT NOT NULL DEFAULT '',
    release_id TEXT NOT NULL DEFAULT '',
    desired_enabled INTEGER NOT NULL DEFAULT 0,
    desired_visible INTEGER NOT NULL DEFAULT 0,
    desired_release_id TEXT NOT NULL DEFAULT '',
    desired_action_key TEXT NOT NULL DEFAULT '',
    settings_snapshot_json TEXT NOT NULL DEFAULT '',
    settings_revision INTEGER NOT NULL DEFAULT 0,
    desired_revision INTEGER NOT NULL DEFAULT 0,
    desired_hash TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    operation_id TEXT NOT NULL DEFAULT '',
    position_x REAL,
    position_y REAL,
    scale REAL NOT NULL DEFAULT 1.0,
    opacity REAL NOT NULL DEFAULT 1.0,
    always_on_top INTEGER NOT NULL DEFAULT 1,
    click_through_mode TEXT NOT NULL DEFAULT 'off',
    position_policy TEXT NOT NULL DEFAULT '',
    revision INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT DEFAULT '',
    created_at TEXT DEFAULT '',
    UNIQUE(installation_id)
);
CREATE INDEX IF NOT EXISTS idx_dprds_installation ON desktop_pet_runtime_desired_states(installation_id);
CREATE INDEX IF NOT EXISTS idx_dprds_user ON desktop_pet_runtime_desired_states(space_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dprds_user_device ON desktop_pet_runtime_desired_states(space_id, device_id) WHERE space_id <> '' AND device_id <> '';
CREATE INDEX IF NOT EXISTS idx_dprds_runtime ON desktop_pet_runtime_desired_states(runtime_id);
CREATE INDEX IF NOT EXISTS idx_dprds_desired_revision ON desktop_pet_runtime_desired_states(space_id, device_id, desired_revision);

CREATE TABLE IF NOT EXISTS desktop_pet_installation_commit_journals (
    id TEXT PRIMARY KEY,
    operation_id TEXT NOT NULL DEFAULT '',
    installation_id TEXT NOT NULL DEFAULT '',
    space_id TEXT NOT NULL DEFAULT '',
    device_id TEXT NOT NULL DEFAULT '',
    operation_type TEXT NOT NULL DEFAULT '',
    release_id TEXT NOT NULL DEFAULT '',
    target_release_id TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL DEFAULT 'operation_created',
    staging_path_key TEXT NOT NULL DEFAULT '',
    published_path_key TEXT NOT NULL DEFAULT '',
    trash_path_key TEXT NOT NULL DEFAULT '',
    previous_release_id TEXT NOT NULL DEFAULT '',
    rollback_reason TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_dpicj_operation ON desktop_pet_installation_commit_journals(operation_id);
CREATE INDEX IF NOT EXISTS idx_dpicj_installation ON desktop_pet_installation_commit_journals(installation_id);
CREATE INDEX IF NOT EXISTS idx_dpicj_state ON desktop_pet_installation_commit_journals(state);

CREATE TABLE IF NOT EXISTS desktop_pet_installation_switch_journals (
    id TEXT PRIMARY KEY,
    operation_id TEXT NOT NULL DEFAULT '',
    space_id TEXT NOT NULL DEFAULT '',
    device_id TEXT NOT NULL DEFAULT '',
    old_installation_id TEXT NOT NULL DEFAULT '',
    new_installation_id TEXT NOT NULL DEFAULT '',
    old_desired_revision INTEGER NOT NULL DEFAULT 0,
    new_desired_revision INTEGER NOT NULL DEFAULT 0,
    binding_revision INTEGER NOT NULL DEFAULT 0,
    state TEXT NOT NULL DEFAULT 'pending',
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_dpisj_operation ON desktop_pet_installation_switch_journals(operation_id);





CREATE TABLE IF NOT EXISTS desktop_pet_action_streams (
  id TEXT PRIMARY KEY,
  space_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  root_processing_task_id TEXT NOT NULL DEFAULT '',
  stream_key TEXT NOT NULL DEFAULT '',
  next_revision_number INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_das_stream_key ON desktop_pet_action_streams(stream_key);






CREATE UNIQUE INDEX IF NOT EXISTS uq_dpqe_idempotency_nonempty ON desktop_pet_quality_evaluations(idempotency_key) WHERE idempotency_key <> '';












CREATE TABLE IF NOT EXISTS desktop_pet_release_validation_reports (
  id TEXT PRIMARY KEY,
  release_id TEXT NOT NULL DEFAULT '',
  operation_id TEXT NOT NULL DEFAULT '',
  snapshot_id TEXT NOT NULL DEFAULT '',
  source TEXT NOT NULL DEFAULT 'build',
  validator_version TEXT NOT NULL DEFAULT '',
  verdict TEXT NOT NULL DEFAULT 'pending',
  findings_json TEXT NOT NULL DEFAULT '[]',
  file_count INTEGER NOT NULL DEFAULT 0,
  error_count INTEGER NOT NULL DEFAULT 0,
  warning_count INTEGER NOT NULL DEFAULT 0,
  manifest_hash TEXT NOT NULL DEFAULT '',
  content_root_hash TEXT NOT NULL DEFAULT '',
  archive_hash TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_dprvr_release ON desktop_pet_release_validation_reports(release_id);
CREATE INDEX IF NOT EXISTS idx_dprvr_operation ON desktop_pet_release_validation_reports(operation_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dprvr_release ON desktop_pet_release_validation_reports(release_id);

CREATE TABLE IF NOT EXISTS desktop_pet_release_event_outbox (
  id TEXT PRIMARY KEY,
  event_id TEXT NOT NULL DEFAULT '',
  event_type TEXT NOT NULL DEFAULT '',
  aggregate_type TEXT NOT NULL DEFAULT 'release',
  aggregate_id TEXT NOT NULL DEFAULT '',
  aggregate_sequence INTEGER NOT NULL DEFAULT 0,
  payload_json TEXT NOT NULL DEFAULT '',
  payload_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  attempt_count INTEGER NOT NULL DEFAULT 0,
    available_at TEXT NOT NULL DEFAULT '',
  last_error TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT '',
  published_at TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_drevo_event_id ON desktop_pet_release_event_outbox(event_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_drevo_agg_seq_type ON desktop_pet_release_event_outbox(aggregate_id, aggregate_sequence, event_type);
CREATE INDEX IF NOT EXISTS idx_drevo_status ON desktop_pet_release_event_outbox(status);
CREATE INDEX IF NOT EXISTS idx_drevo_available ON desktop_pet_release_event_outbox(available_at) WHERE status='pending';

CREATE TABLE IF NOT EXISTS desktop_pet_release_build_request_inbox (
  id TEXT PRIMARY KEY,
  request_id TEXT NOT NULL DEFAULT '',
  space_id TEXT NOT NULL DEFAULT '',
  idempotency_key TEXT NOT NULL DEFAULT '',
  input_hash TEXT NOT NULL DEFAULT '',
  payload_json TEXT NOT NULL DEFAULT '',
  payload_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  operation_id TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT '',
  processed_at TEXT NOT NULL DEFAULT '',
  last_error TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_drbrbi_request_id ON desktop_pet_release_build_request_inbox(request_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_drbrbi_idempotent ON desktop_pet_release_build_request_inbox(space_id, idempotency_key);
CREATE INDEX IF NOT EXISTS idx_drbrbi_status ON desktop_pet_release_build_request_inbox(status);

CREATE TABLE IF NOT EXISTS desktop_pet_import_package_snapshots (
  id TEXT PRIMARY KEY,
  import_staging_id TEXT NOT NULL DEFAULT '',
  source_package_hash TEXT NOT NULL DEFAULT '',
  source_manifest_hash TEXT NOT NULL DEFAULT '',
  source_schema_version INTEGER NOT NULL DEFAULT 0,
  normalization_warnings TEXT NOT NULL DEFAULT '',
  selected_actions_json TEXT NOT NULL DEFAULT '[]',
  binding_decision TEXT NOT NULL DEFAULT '',
  license_decision TEXT NOT NULL DEFAULT '',
  runtime_compatibility TEXT NOT NULL DEFAULT '',
  space_id TEXT NOT NULL DEFAULT '',
  pet_id TEXT NOT NULL DEFAULT '',
  release_id TEXT NOT NULL DEFAULT '',
  operation_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'preparing',
  last_error TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dips_staging ON desktop_pet_import_package_snapshots(import_staging_id);
CREATE INDEX IF NOT EXISTS idx_dips_release ON desktop_pet_import_package_snapshots(release_id);
CREATE INDEX IF NOT EXISTS idx_dips_operation ON desktop_pet_import_package_snapshots(operation_id);
CREATE INDEX IF NOT EXISTS idx_dips_status ON desktop_pet_import_package_snapshots(status);

CREATE TABLE IF NOT EXISTS desktop_pet_device_active_installation_bindings (
    space_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    installation_id TEXT NOT NULL DEFAULT '',
    pet_id TEXT NOT NULL DEFAULT '',
    release_id TEXT NOT NULL DEFAULT '',
    binding_revision INTEGER NOT NULL DEFAULT 0,
    bound_reason TEXT NOT NULL DEFAULT 'install_bound',
    bound_at TEXT NOT NULL DEFAULT '',
    bound_by TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT '',
    PRIMARY KEY(space_id, device_id)
);
CREATE INDEX IF NOT EXISTS idx_dpdainst_installation ON desktop_pet_device_active_installation_bindings(installation_id);
CREATE INDEX IF NOT EXISTS idx_dpdainst_pet ON desktop_pet_device_active_installation_bindings(pet_id);

CREATE TABLE IF NOT EXISTS desktop_pet_device_installation_binding_history (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    previous_installation_id TEXT NOT NULL DEFAULT '',
    new_installation_id TEXT NOT NULL DEFAULT '',
    binding_revision INTEGER NOT NULL DEFAULT 0,
    reason TEXT NOT NULL DEFAULT '',
    actor TEXT NOT NULL DEFAULT '',
    operation_id TEXT NOT NULL DEFAULT '',
    occurred_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_dpbih_user_device ON desktop_pet_device_installation_binding_history(space_id, device_id);
CREATE INDEX IF NOT EXISTS idx_dpbih_operation ON desktop_pet_device_installation_binding_history(operation_id);

CREATE TABLE IF NOT EXISTS desktop_pet_device_desired_revision_counters (
    space_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    current_revision INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT DEFAULT '',
    PRIMARY KEY(space_id, device_id)
);

CREATE TABLE IF NOT EXISTS desktop_pet_installation_runtime_projections (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    runtime_id TEXT NOT NULL DEFAULT '',
    installation_id TEXT NOT NULL DEFAULT '',
    pet_id TEXT NOT NULL DEFAULT '',
    applied_desired_revision INTEGER NOT NULL DEFAULT 0,
    applied_settings_revision INTEGER NOT NULL DEFAULT 0,
    actual_release_id TEXT NOT NULL DEFAULT '',
    actual_visible INTEGER NOT NULL DEFAULT 0,
    actual_action_key TEXT NOT NULL DEFAULT '',
    actual_health TEXT NOT NULL DEFAULT 'unknown',
    runtime_sync_state TEXT NOT NULL DEFAULT 'pending',
    last_applied_at TEXT NOT NULL DEFAULT '',
    last_heartbeat_at TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_drirp_user_device ON desktop_pet_installation_runtime_projections(space_id, device_id);
CREATE INDEX IF NOT EXISTS idx_drirp_installation ON desktop_pet_installation_runtime_projections(installation_id);
CREATE INDEX IF NOT EXISTS idx_drirp_runtime ON desktop_pet_installation_runtime_projections(runtime_id);

CREATE TABLE IF NOT EXISTS desktop_pet_runtime_desired_state_outbox (
    event_id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL DEFAULT 'desired_state_changed',
    space_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    runtime_id TEXT NOT NULL DEFAULT '',
    installation_id TEXT NOT NULL DEFAULT '',
    desired_revision INTEGER NOT NULL DEFAULT 0,
    desired_hash TEXT NOT NULL DEFAULT '',
    operation_id TEXT NOT NULL DEFAULT '',
    payload_json TEXT NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'pending',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    available_at TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT '',
    published_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_drdso_user_device_status ON desktop_pet_runtime_desired_state_outbox(space_id, device_id, status);
CREATE INDEX IF NOT EXISTS idx_drdso_available ON desktop_pet_runtime_desired_state_outbox(status, available_at);

CREATE TABLE IF NOT EXISTS desktop_pet_runtime_sessions (
  id TEXT PRIMARY KEY,
  space_id TEXT NOT NULL DEFAULT '',
  device_id TEXT NOT NULL DEFAULT '',
  runtime_id TEXT NOT NULL DEFAULT '',
  connection_generation INTEGER NOT NULL DEFAULT 0,
  runtime_version TEXT NOT NULL DEFAULT '',
  runtime_contract_version TEXT NOT NULL DEFAULT '',
  capabilities_json TEXT NOT NULL DEFAULT '{}',
  capabilities_hash TEXT NOT NULL DEFAULT '',
  last_applied_desired_revision INTEGER NOT NULL DEFAULT 0,
  last_processed_command_sequence INTEGER NOT NULL DEFAULT 0,
  last_event_sequence INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT '',
  connected_at TEXT NOT NULL DEFAULT '',
  last_heartbeat_at TEXT NOT NULL DEFAULT '',
  disconnected_at TEXT NOT NULL DEFAULT '',
  superseded_at TEXT NOT NULL DEFAULT '',
  superseded_by TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_rtsessv2_user_device ON desktop_pet_runtime_sessions(space_id, device_id);
CREATE INDEX IF NOT EXISTS idx_rtsessv2_user_device_runtime_status ON desktop_pet_runtime_sessions(space_id, device_id, runtime_id, status);

CREATE TABLE IF NOT EXISTS desktop_pet_runtime_commands_v2 (
  id TEXT PRIMARY KEY,
  space_id TEXT NOT NULL DEFAULT '',
  device_id TEXT NOT NULL DEFAULT '',
  runtime_id TEXT NOT NULL DEFAULT '',
  runtime_session_id TEXT NOT NULL DEFAULT '',
  command_type TEXT NOT NULL DEFAULT '',
  durability TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT '',
  revision INTEGER NOT NULL DEFAULT 0,
  desired_revision INTEGER NOT NULL DEFAULT 0,
  settings_revision INTEGER NOT NULL DEFAULT 0,
  device_sequence INTEGER NOT NULL DEFAULT 0,
  idempotency_key TEXT NOT NULL DEFAULT '',
  coalesce_key TEXT NOT NULL DEFAULT '',
  runtime_correlation_id TEXT NOT NULL DEFAULT '',
  runtime_playback_id TEXT NOT NULL DEFAULT '',
  payload TEXT NOT NULL DEFAULT '{}',
  payload_json TEXT NOT NULL DEFAULT '{}',
  payload_hash TEXT NOT NULL DEFAULT '',
  payload_schema_version INTEGER NOT NULL DEFAULT 0,
  hash_code INTEGER NOT NULL DEFAULT 0,
  attempt INTEGER NOT NULL DEFAULT 0,
  installation_id TEXT NOT NULL DEFAULT '',
  pet_id TEXT NOT NULL DEFAULT '',
  release_id TEXT NOT NULL DEFAULT '',
  expires_at TEXT NOT NULL DEFAULT '',
  last_attempt_id TEXT NOT NULL DEFAULT '',
  error_code TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
    inserted_at TEXT DEFAULT '',
  created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT DEFAULT '',
  dispatch_at TEXT NOT NULL DEFAULT '',
  transport_dispatched_at TEXT NOT NULL DEFAULT '',
  runtime_received_at TEXT NOT NULL DEFAULT '',
  runtime_accepted_at TEXT NOT NULL DEFAULT '',
  renderer_accepted_at TEXT NOT NULL DEFAULT '',
  playback_started_at TEXT NOT NULL DEFAULT '',
  completed_at TEXT NOT NULL DEFAULT '',
  superseded_at TEXT NOT NULL DEFAULT '',
  superseded_by TEXT NOT NULL DEFAULT '',
  superseded_by_command_id TEXT NOT NULL DEFAULT '',
  UNIQUE(space_id, device_id, idempotency_key)
);
CREATE INDEX IF NOT EXISTS idx_rtcv2_user_device_type ON desktop_pet_runtime_commands_v2(space_id, device_id, command_type);
CREATE INDEX IF NOT EXISTS idx_rtcv2_status ON desktop_pet_runtime_commands_v2(status, inserted_at);
CREATE INDEX IF NOT EXISTS idx_rtcv2_device_seq ON desktop_pet_runtime_commands_v2(space_id, device_id, device_sequence);
CREATE INDEX IF NOT EXISTS idx_rtcv2_user_device_runtime_status ON desktop_pet_runtime_commands_v2(space_id, device_id, runtime_id, status);

CREATE TABLE IF NOT EXISTS desktop_pet_import_stagings (
  id TEXT PRIMARY KEY,
  owner_space_id TEXT NOT NULL,
  source_filename TEXT NOT NULL DEFAULT '',
  source_type TEXT NOT NULL DEFAULT '',
  source_content_hash TEXT NOT NULL DEFAULT '',
  source_bytes INTEGER NOT NULL DEFAULT 0,
  root_kind TEXT NOT NULL DEFAULT '',
  storage_key TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  quarantine_path TEXT NOT NULL DEFAULT '',
  inventory_hash TEXT NOT NULL DEFAULT '',
  inventory_json TEXT NOT NULL DEFAULT '[]',
  state_revision INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  consumption_started_at TEXT NOT NULL DEFAULT '',
  consumed_at TEXT NOT NULL DEFAULT '',
  failed_at TEXT NOT NULL DEFAULT '',
  failure_reason TEXT NOT NULL DEFAULT '',
  rejected_reason TEXT NOT NULL DEFAULT '',
  correlation_id TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_dpis_owner_status ON desktop_pet_import_stagings(owner_space_id, status);

CREATE TABLE IF NOT EXISTS desktop_pet_runtime_command_attempts (
  attempt_id TEXT PRIMARY KEY,
  command_id TEXT NOT NULL DEFAULT '',
  runtime_session_id TEXT NOT NULL DEFAULT '',
  connection_generation INTEGER NOT NULL DEFAULT 0,
  dispatched_at TEXT NOT NULL DEFAULT '',
  runtime_received_at TEXT NOT NULL DEFAULT '',
  runtime_accepted_at TEXT NOT NULL DEFAULT '',
  renderer_accepted_at TEXT NOT NULL DEFAULT '',
  playback_started_at TEXT NOT NULL DEFAULT '',
  finished_at TEXT NOT NULL DEFAULT '',
  error_code TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
    inserted_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_rtca_command ON desktop_pet_runtime_command_attempts(command_id);

CREATE TABLE IF NOT EXISTS desktop_pet_runtime_command_results (
  id TEXT PRIMARY KEY,
  command_id TEXT NOT NULL DEFAULT '',
  runtime_id TEXT NOT NULL DEFAULT '',
  attempt_id TEXT NOT NULL DEFAULT '',
  result_type TEXT NOT NULL DEFAULT '',
  result_json TEXT NOT NULL DEFAULT '{}',
  result_hash TEXT NOT NULL DEFAULT '',
  runtime_session_id TEXT NOT NULL DEFAULT '',
  connection_generation INTEGER NOT NULL DEFAULT 0,
  event_sequence INTEGER NOT NULL DEFAULT 0,
  error_code TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
    inserted_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT '',
  UNIQUE(command_id, runtime_id)
);
CREATE INDEX IF NOT EXISTS idx_rtcr_command ON desktop_pet_runtime_command_results(command_id);
CREATE INDEX IF NOT EXISTS idx_rtcr_runtime ON desktop_pet_runtime_command_results(runtime_id);

CREATE TABLE IF NOT EXISTS desktop_pet_runtime_device_command_sequences (
  space_id TEXT NOT NULL DEFAULT '',
  device_id TEXT NOT NULL DEFAULT '',
  sequence INTEGER NOT NULL DEFAULT 0,
  last_reserved_at TEXT NOT NULL DEFAULT '',
    inserted_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT '',
  PRIMARY KEY(space_id, device_id)
);

CREATE TABLE IF NOT EXISTS desktop_pet_runtime_event_records (
  id TEXT PRIMARY KEY,
  event_type TEXT NOT NULL DEFAULT '',
  payload TEXT NOT NULL DEFAULT '{}',
  payload_hash TEXT NOT NULL DEFAULT '',
  source TEXT NOT NULL DEFAULT '',
  runtime_session_id TEXT NOT NULL DEFAULT '',
  command_id TEXT NOT NULL DEFAULT '',
  sequence INTEGER NOT NULL DEFAULT 0,
  occurred_at TEXT NOT NULL DEFAULT '',
  delivered INTEGER NOT NULL DEFAULT 0,
  delivered_at TEXT NOT NULL DEFAULT '',
    inserted_at TEXT DEFAULT '',
  UNIQUE(runtime_session_id, sequence)
);
CREATE INDEX IF NOT EXISTS idx_rter_session_seq ON desktop_pet_runtime_event_records(runtime_session_id, sequence);
CREATE INDEX IF NOT EXISTS idx_rter_session_delivered ON desktop_pet_runtime_event_records(runtime_session_id, delivered, sequence);

CREATE TABLE IF NOT EXISTS desktop_pet_runtime_actual_states_v2 (
  space_id TEXT NOT NULL DEFAULT '',
  device_id TEXT NOT NULL DEFAULT '',
  runtime_id TEXT NOT NULL DEFAULT '',
  runtime_session_id TEXT NOT NULL DEFAULT '',
  connection_generation INTEGER NOT NULL DEFAULT 0,
  last_event_sequence INTEGER NOT NULL DEFAULT 0,
  applied_desired_revision INTEGER NOT NULL DEFAULT 0,
  applied_desired_hash TEXT NOT NULL DEFAULT '',
  applied_settings_revision INTEGER NOT NULL DEFAULT 0,
  installation_id TEXT NOT NULL DEFAULT '',
  pet_id TEXT NOT NULL DEFAULT '',
  release_id TEXT NOT NULL DEFAULT '',
  instance_status TEXT NOT NULL DEFAULT '',
  window_status TEXT NOT NULL DEFAULT '',
  renderer_status TEXT NOT NULL DEFAULT '',
  playback_status TEXT NOT NULL DEFAULT '',
  visible INTEGER NOT NULL DEFAULT 0,
  position_x INTEGER NOT NULL DEFAULT 0,
  position_y INTEGER NOT NULL DEFAULT 0,
  screen_id TEXT NOT NULL DEFAULT '',
  window_width INTEGER NOT NULL DEFAULT 0,
  window_height INTEGER NOT NULL DEFAULT 0,
  scale REAL NOT NULL DEFAULT 1,
  stable_action_key TEXT NOT NULL DEFAULT '',
  current_action_key TEXT NOT NULL DEFAULT '',
  playback_instance_id TEXT NOT NULL DEFAULT '',
  current_command_id TEXT NOT NULL DEFAULT '',
  actual_state_hash TEXT NOT NULL DEFAULT '',
  health_status TEXT NOT NULL DEFAULT '',
  last_error_code TEXT NOT NULL DEFAULT '',
    updated_at TEXT DEFAULT '',
  PRIMARY KEY(space_id, device_id, runtime_id)
);
CREATE INDEX IF NOT EXISTS idx_rtasv2_user_device ON desktop_pet_runtime_actual_states_v2(space_id, device_id);

CREATE TABLE IF NOT EXISTS desktop_pet_runtime_domain_event_outbox (
  id TEXT PRIMARY KEY,
  event_type TEXT NOT NULL DEFAULT '',
  aggregate_id TEXT NOT NULL DEFAULT '',
  payload TEXT NOT NULL DEFAULT '{}',
  status TEXT NOT NULL DEFAULT 'pending',
  attempt INTEGER NOT NULL DEFAULT 0,
  idempotency_key TEXT NOT NULL DEFAULT '',
  claim_expires_at TEXT NOT NULL DEFAULT '',
  last_error TEXT NOT NULL DEFAULT '',
    inserted_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT '',
  published_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_dteo_status ON desktop_pet_runtime_domain_event_outbox(status, inserted_at);

CREATE TABLE IF NOT EXISTS desktop_pet_runtime_command_dedup (
  id TEXT PRIMARY KEY,
  space_id TEXT NOT NULL DEFAULT '',
  device_id TEXT NOT NULL DEFAULT '',
  idempotency_key TEXT NOT NULL DEFAULT '',
  nak_count INTEGER NOT NULL DEFAULT 0,
  last_nak_at TEXT NOT NULL DEFAULT '',
    inserted_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_rtcdd_user_device_idem ON desktop_pet_runtime_command_dedup(space_id, device_id, idempotency_key);

CREATE TABLE IF NOT EXISTS desktop_pet_runtime_reconcile_leases (
  reconciler_id TEXT PRIMARY KEY,
  last_heartbeat_at TEXT NOT NULL DEFAULT '',
    inserted_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS desktop_pet_installation_trash_entries (
    id TEXT PRIMARY KEY,
    operation_id TEXT NOT NULL,
    installation_id TEXT NOT NULL,
    storage_key TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    content_hash TEXT NOT NULL DEFAULT '',
    retain_until TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    created_at TEXT DEFAULT '',
    purged_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_dptein_installation ON desktop_pet_installation_trash_entries(installation_id);
CREATE INDEX IF NOT EXISTS idx_dptein_retain ON desktop_pet_installation_trash_entries(retain_until);

CREATE TABLE IF NOT EXISTS extension_package_resource_quarantine (
    quarantine_id TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    extension_id TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    logical_path TEXT NOT NULL DEFAULT '',
    original_path TEXT NOT NULL DEFAULT '',
    content_hash TEXT NOT NULL DEFAULT '',
    storage_reference TEXT NOT NULL DEFAULT '',
    content_storage_reference TEXT NOT NULL DEFAULT '',
    namespace_hash TEXT NOT NULL DEFAULT '',
    size INTEGER NOT NULL DEFAULT 0,
    quarantine_path TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL DEFAULT 'preparing',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (quarantine_id)
);
CREATE INDEX IF NOT EXISTS idx_ext_pkg_resource_quarantine_ns_hash ON extension_package_resource_quarantine(extension_id, namespace_hash, state);

CREATE TABLE IF NOT EXISTS extension_package_user_data_restore_journal (
    journal_id TEXT NOT NULL,
    operation_id TEXT NOT NULL,
    extension_id TEXT NOT NULL,
    table_name TEXT NOT NULL,
    total_rows INTEGER NOT NULL DEFAULT 0,
    imported_rows INTEGER NOT NULL DEFAULT 0,
    applied_count INTEGER NOT NULL DEFAULT 0,
    cursor TEXT NOT NULL DEFAULT '',
    batch_hash TEXT NOT NULL DEFAULT '',
    batch_index INTEGER NOT NULL DEFAULT 0,
    prev_batch_hash TEXT NOT NULL DEFAULT '',
    batch_algorithm_version TEXT NOT NULL DEFAULT '',
    batch_size INTEGER NOT NULL DEFAULT 0,
    namespace_hash TEXT NOT NULL DEFAULT '',
    expected_aggregate_hash TEXT NOT NULL DEFAULT '',
    aggregate_hash TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL DEFAULT 'pending',
    started_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    error_detail TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (operation_id, table_name)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_dpinst_user_device_pet ON desktop_pet_installations(space_id, device_id, pet_id);

CREATE TABLE IF NOT EXISTS desktop_pet_local_sessions (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL DEFAULT '',
    desktop_instance_id TEXT NOT NULL DEFAULT '',
    token_hash TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    created_at DATETIME NOT NULL,
    expires_at DATETIME NOT NULL,
    last_used_at DATETIME NOT NULL,
    revoked_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_dpls_token ON desktop_pet_local_sessions(token_hash, status);
CREATE INDEX IF NOT EXISTS idx_dpls_user ON desktop_pet_local_sessions(space_id, status);

CREATE TABLE IF NOT EXISTS desktop_pet_runtime_bootstrap_tickets (
    id TEXT PRIMARY KEY,
    ticket_hash TEXT UNIQUE NOT NULL,
    space_id TEXT NOT NULL DEFAULT '',
    device_id TEXT NOT NULL DEFAULT '',
    runtime_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    expires_at TEXT NOT NULL DEFAULT '',
    consumed_at TEXT NOT NULL DEFAULT '',
    consumed_by_runtime TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_bt_user ON desktop_pet_runtime_bootstrap_tickets(space_id, status);
CREATE INDEX IF NOT EXISTS idx_bt_device ON desktop_pet_runtime_bootstrap_tickets(device_id);
CREATE INDEX IF NOT EXISTS idx_bt_status ON desktop_pet_runtime_bootstrap_tickets(status);
CREATE INDEX IF NOT EXISTS idx_dprbt_runtime ON desktop_pet_runtime_bootstrap_tickets(runtime_id);

---- 来源: desktop_session.go ---
CREATE TABLE IF NOT EXISTS desktop_pet_token_rotation_journal (
    id TEXT PRIMARY KEY,
    old_version TEXT NOT NULL DEFAULT '',
    new_version TEXT NOT NULL DEFAULT '',
    stage TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT '',
    completed_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_dptrj_stage ON desktop_pet_token_rotation_journal(stage);




CREATE TABLE IF NOT EXISTS workspace_mounts (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    kind TEXT NOT NULL,
    local_root TEXT,
    native_grant_id TEXT,
    backend_config_json TEXT,
    credential_ref TEXT,
    read_only INTEGER NOT NULL DEFAULT 0,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    last_used_at DATETIME
);

CREATE TABLE IF NOT EXISTS message_attachments (
    id TEXT PRIMARY KEY,
    message_id TEXT NOT NULL DEFAULT '',
    sequence INTEGER NOT NULL DEFAULT 0,
    type TEXT NOT NULL DEFAULT '',
    resource_uri TEXT NOT NULL DEFAULT '',
    mime_type TEXT DEFAULT '',
    filename TEXT DEFAULT '',
    size_bytes INTEGER DEFAULT 0,
    content_hash TEXT DEFAULT '',
    width INTEGER DEFAULT 0,
    height INTEGER DEFAULT 0,
    duration_ms INTEGER DEFAULT 0,
    created_at TEXT DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_message_attachments_message_id ON message_attachments(message_id);


CREATE TABLE IF NOT EXISTS wake_configs (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    enabled INTEGER DEFAULT 0,
    backend TEXT DEFAULT 'software',
    model_resource_uri TEXT DEFAULT '',
    phrases TEXT DEFAULT '',
    threshold REAL DEFAULT 0.05,
    cooldown_ms INTEGER DEFAULT 2000,
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS kernel_device_runtime_sessions (
    runtime_session_id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    runtime_id TEXT NOT NULL,
    platform TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    connection_generation INTEGER NOT NULL DEFAULT 1,
    runtime_version TEXT NOT NULL DEFAULT '',
    runtime_contract_version TEXT NOT NULL DEFAULT '',
    capabilities_json TEXT NOT NULL DEFAULT '[]',
    capabilities_hash TEXT NOT NULL DEFAULT '',
    last_applied_state_revision INTEGER NOT NULL DEFAULT 0,
    last_processed_command_sequence INTEGER NOT NULL DEFAULT 0,
    last_event_sequence INTEGER NOT NULL DEFAULT 0,
    actual_state_hash TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    last_heartbeat_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL DEFAULT 0,
    closed_at INTEGER NOT NULL DEFAULT 0,
    close_reason TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_kernel_device_runtime_sessions_identity ON kernel_device_runtime_sessions(space_id, device_id, runtime_id);
CREATE INDEX IF NOT EXISTS idx_kernel_device_runtime_sessions_status ON kernel_device_runtime_sessions(status);
CREATE INDEX IF NOT EXISTS idx_kernel_device_runtime_sessions_heartbeat ON kernel_device_runtime_sessions(last_heartbeat_at);

CREATE TABLE IF NOT EXISTS artifacts (
    artifact_id TEXT PRIMARY KEY,
    owner_space_id TEXT NOT NULL,
    workspace_id TEXT NOT NULL DEFAULT '',
    kind TEXT NOT NULL,
    blob_digest TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    mime_type TEXT NOT NULL,
    filename TEXT NOT NULL DEFAULT '',
    file_extension TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    source TEXT NOT NULL,
    width INTEGER NOT NULL DEFAULT 0,
    height INTEGER NOT NULL DEFAULT 0,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    revision INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME
);
CREATE TABLE IF NOT EXISTS artifact_references (
    artifact_id TEXT NOT NULL,
    reference_type TEXT NOT NULL,
    reference_id TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    PRIMARY KEY(artifact_id, reference_type, reference_id)
);
CREATE INDEX IF NOT EXISTS idx_artifacts_owner ON artifacts(owner_space_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_blob_digest ON artifacts(blob_digest);
CREATE INDEX IF NOT EXISTS idx_artifacts_status ON artifacts(status);
CREATE INDEX IF NOT EXISTS idx_artifacts_created_at ON artifacts(created_at);


CREATE TABLE IF NOT EXISTS sync_changes (
	change_id TEXT PRIMARY KEY,
	seq INTEGER NOT NULL,
	space_id TEXT NOT NULL,
	scope TEXT NOT NULL DEFAULT 'device',
	entity_type TEXT NOT NULL,
	entity_id TEXT NOT NULL,
	operation TEXT NOT NULL,
	revision INTEGER NOT NULL,
	mutation_id TEXT,
	origin_device TEXT,
	payload BLOB,
	checksum TEXT NOT NULL,
	created_at DATETIME NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sync_changes_seq ON sync_changes(seq);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sync_changes_user_scope_mutation ON sync_changes(space_id, scope, mutation_id);
CREATE INDEX IF NOT EXISTS idx_sync_changes_user ON sync_changes(space_id);
CREATE INDEX IF NOT EXISTS idx_sync_changes_scope ON sync_changes(scope);
CREATE INDEX IF NOT EXISTS idx_sync_changes_entity ON sync_changes(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_sync_changes_origin ON sync_changes(origin_device);
CREATE TABLE IF NOT EXISTS sync_cursors (
    device_id TEXT NOT NULL,
    space_id TEXT NOT NULL,
    scope TEXT NOT NULL DEFAULT 'device',
    last_applied INTEGER NOT NULL DEFAULT 0,
    last_pushed INTEGER NOT NULL DEFAULT 0,
    updated_at DATETIME NOT NULL,
    PRIMARY KEY (space_id, scope, device_id)
);
CREATE INDEX IF NOT EXISTS idx_sync_cursors_user ON sync_cursors(space_id);

CREATE TABLE IF NOT EXISTS sync_mutation_claims (
    space_id TEXT NOT NULL,
    scope TEXT NOT NULL DEFAULT 'device',
    mutation_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at DATETIME NOT NULL DEFAULT '',
    PRIMARY KEY (space_id, scope, mutation_id)
);
CREATE INDEX IF NOT EXISTS idx_sync_mutation_claims_status ON sync_mutation_claims(status);

CREATE TABLE IF NOT EXISTS sync_sequence (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    seq INTEGER NOT NULL DEFAULT 0
);
INSERT OR IGNORE INTO sync_sequence (id, seq) VALUES (1, 0);

CREATE TABLE IF NOT EXISTS execution_resumes (
    resume_id TEXT PRIMARY KEY,
    root_execution_id TEXT,
    parent_execution_id TEXT,
    resume_type TEXT NOT NULL,
    resume_state TEXT NOT NULL,
    checkpoint_ref TEXT,
    required_capability_id TEXT,
    acquisition_transaction_id TEXT,
    task_id TEXT,
    payload_ref TEXT,
    reason TEXT,
    metadata TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_execution_resumes_state ON execution_resumes(resume_state);
CREATE INDEX IF NOT EXISTS idx_execution_resumes_root ON execution_resumes(root_execution_id);
CREATE INDEX IF NOT EXISTS idx_execution_resumes_acq_txn ON execution_resumes(acquisition_transaction_id);






CREATE TABLE IF NOT EXISTS desktop_pet_owner_mappings (
  cloud_space_id TEXT NOT NULL DEFAULT '',
  device_id TEXT NOT NULL DEFAULT '',
  local_owner_id TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  PRIMARY KEY(cloud_space_id, device_id)
);
CREATE INDEX IF NOT EXISTS idx_dpom_local_owner ON desktop_pet_owner_mappings(local_owner_id);

CREATE TABLE IF NOT EXISTS desktop_pet_behavior_mesh_affinities (
  cloud_space_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
  device_id TEXT NOT NULL DEFAULT '',
  installation_id TEXT NOT NULL DEFAULT '',
  binding_revision INTEGER NOT NULL DEFAULT 0,
  verified_at DATETIME NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  PRIMARY KEY(cloud_space_id, character_id)
);
CREATE INDEX IF NOT EXISTS idx_dpbma_device ON desktop_pet_behavior_mesh_affinities(cloud_space_id, device_id);

CREATE TABLE IF NOT EXISTS desktop_pet_behavior_mesh_outbox (
  event_id TEXT NOT NULL DEFAULT '',
  cloud_space_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
  target_device_id TEXT NOT NULL DEFAULT '',
  target_installation_id TEXT NOT NULL DEFAULT '',
  reliability TEXT NOT NULL DEFAULT '',
  payload_json TEXT NOT NULL DEFAULT '{}',
  payload_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  attempt_count INTEGER NOT NULL DEFAULT 0,
  available_at DATETIME NOT NULL,
  claim_expires_at DATETIME,
  expires_at DATETIME,
  last_error TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  delivered_at DATETIME,
  PRIMARY KEY(cloud_space_id, event_id)
);
CREATE INDEX IF NOT EXISTS idx_dpbmo_due ON desktop_pet_behavior_mesh_outbox(status, available_at);
CREATE INDEX IF NOT EXISTS idx_dpbmo_claim ON desktop_pet_behavior_mesh_outbox(status, claim_expires_at);
CREATE INDEX IF NOT EXISTS idx_dpbmo_expiry ON desktop_pet_behavior_mesh_outbox(status, expires_at);
CREATE INDEX IF NOT EXISTS idx_dpbmo_character ON desktop_pet_behavior_mesh_outbox(cloud_space_id, character_id, status);
CREATE INDEX IF NOT EXISTS idx_dpbmo_device ON desktop_pet_behavior_mesh_outbox(cloud_space_id, target_device_id, status);

-- Tenant ownership indexes kept in the baseline so fresh installs and upgraded databases converge.
CREATE INDEX IF NOT EXISTS idx_characters_user_updated ON characters(space_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_characters_user_active ON characters(space_id, is_active, is_default);

CREATE TABLE IF NOT EXISTS sandbox_environment_variables (
    scope_id TEXT NOT NULL,
    name TEXT NOT NULL,
    value TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT '',
    updated_at DATETIME NOT NULL DEFAULT '',
    PRIMARY KEY (scope_id, name)
);
CREATE INDEX IF NOT EXISTS idx_sandbox_environment_scope ON sandbox_environment_variables(scope_id);

CREATE TABLE IF NOT EXISTS model_scenario_routes (
    scenario TEXT PRIMARY KEY,
    model_config_id INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS continuity_threads (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    character_id TEXT NOT NULL DEFAULT '',
    parent_thread_id TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL,
    goal TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    summary TEXT NOT NULL DEFAULT '',
    current_state TEXT NOT NULL DEFAULT '',
    next_action TEXT NOT NULL DEFAULT '',
    priority INTEGER NOT NULL DEFAULT 0,
    confidence REAL NOT NULL DEFAULT 1,
    revision INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    last_active_at DATETIME NOT NULL,
    completed_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_continuity_threads_scope_status ON continuity_threads(space_id, status, last_active_at);
CREATE INDEX IF NOT EXISTS idx_continuity_threads_character ON continuity_threads(character_id, last_active_at);

CREATE TABLE IF NOT EXISTS continuity_thread_bindings (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL,
    binding_type TEXT NOT NULL,
    binding_id TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'context',
    confidence REAL NOT NULL DEFAULT 1,
    source TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    last_active_at DATETIME NOT NULL,
    UNIQUE(thread_id, binding_type, binding_id)
);
CREATE INDEX IF NOT EXISTS idx_continuity_bindings_lookup ON continuity_thread_bindings(binding_type, binding_id, last_active_at);

CREATE TABLE IF NOT EXISTS continuity_thread_events (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    source_type TEXT NOT NULL DEFAULT '',
    source_id TEXT NOT NULL DEFAULT '',
    conversation_id TEXT NOT NULL DEFAULT '',
    request_id TEXT NOT NULL DEFAULT '',
    execution_id TEXT NOT NULL DEFAULT '',
    payload_json TEXT NOT NULL DEFAULT '{}',
    idempotency_key TEXT NOT NULL UNIQUE,
    occurred_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_continuity_events_thread_time ON continuity_thread_events(thread_id, occurred_at);

CREATE TABLE IF NOT EXISTS continuity_waits (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL,
    wait_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'waiting',
    description TEXT NOT NULL DEFAULT '',
    condition_json TEXT NOT NULL DEFAULT '{}',
    resume_hint TEXT NOT NULL DEFAULT '',
    due_at DATETIME,
    resolved_at DATETIME,
    resolved_by TEXT NOT NULL DEFAULT '',
    resolution_json TEXT NOT NULL DEFAULT '{}',
    source_execution_id TEXT NOT NULL DEFAULT '',
    auto_resume INTEGER NOT NULL DEFAULT 1,
    wake_state TEXT NOT NULL DEFAULT '',
    wake_request_id TEXT NOT NULL DEFAULT '',
    wake_attempts INTEGER NOT NULL DEFAULT 0,
    next_wake_at DATETIME,
    last_wake_error TEXT NOT NULL DEFAULT '',
    wake_delivered_at DATETIME,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_continuity_waits_thread_status ON continuity_waits(thread_id, status, wait_type);


-- Additive Knowledge/RAG domain; modern WorldBook/Memory tables unchanged.
CREATE TABLE IF NOT EXISTS knowledge_sources (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    logical_name TEXT NOT NULL DEFAULT '',
    filename TEXT NOT NULL DEFAULT '',
    mime_type TEXT NOT NULL DEFAULT '',
    source_type TEXT NOT NULL DEFAULT 'file',
    storage_key TEXT NOT NULL DEFAULT '',
    size_bytes INTEGER NOT NULL DEFAULT 0,
    sha256 TEXT NOT NULL DEFAULT '',
    revision INTEGER NOT NULL DEFAULT 1,
    parser_version TEXT NOT NULL DEFAULT 'knowledge-parser-v1',
    chunk_schema_version TEXT NOT NULL DEFAULT 'knowledge-chunk-v1',
    config_hash TEXT NOT NULL DEFAULT '',
    embedding_version TEXT NOT NULL DEFAULT '',
    index_version TEXT NOT NULL DEFAULT 'v1',
    status TEXT NOT NULL DEFAULT 'pending',
    expected_units INTEGER NOT NULL DEFAULT 0,
    processed_units INTEGER NOT NULL DEFAULT 0,
    failed_units INTEGER NOT NULL DEFAULT 0,
    indexed_chunks INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_knowledge_sources_tenant_status ON knowledge_sources(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_knowledge_sources_tenant_sha ON knowledge_sources(tenant_id, sha256);

CREATE TABLE IF NOT EXISTS knowledge_source_revisions (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    source_id TEXT NOT NULL,
    revision INTEGER NOT NULL,
    filename TEXT NOT NULL DEFAULT '',
    mime_type TEXT NOT NULL DEFAULT '',
    source_type TEXT NOT NULL DEFAULT 'file',
    storage_key TEXT NOT NULL DEFAULT '',
    size_bytes INTEGER NOT NULL DEFAULT 0,
    sha256 TEXT NOT NULL DEFAULT '',
    parser_version TEXT NOT NULL DEFAULT 'knowledge-parser-v1',
    chunk_schema_version TEXT NOT NULL DEFAULT 'knowledge-chunk-v1',
    config_hash TEXT NOT NULL DEFAULT '',
    embedding_version TEXT NOT NULL DEFAULT '',
    index_version TEXT NOT NULL DEFAULT 'v1',
    status TEXT NOT NULL DEFAULT 'pending',
    expected_units INTEGER NOT NULL DEFAULT 0,
    processed_units INTEGER NOT NULL DEFAULT 0,
    failed_units INTEGER NOT NULL DEFAULT 0,
    indexed_chunks INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    activated_at DATETIME,
    retain_until DATETIME,
    purged_at DATETIME,
    UNIQUE(tenant_id, source_id, revision)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_knowledge_revisions_source ON knowledge_source_revisions(tenant_id, source_id, revision);
CREATE INDEX IF NOT EXISTS idx_knowledge_revisions_sha ON knowledge_source_revisions(tenant_id, sha256);
CREATE INDEX IF NOT EXISTS idx_knowledge_revisions_status ON knowledge_source_revisions(tenant_id, status, updated_at);
CREATE INDEX IF NOT EXISTS idx_knowledge_revisions_retention ON knowledge_source_revisions(tenant_id, retain_until, purged_at);

CREATE TABLE IF NOT EXISTS knowledge_units (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    source_id TEXT NOT NULL,
    source_revision INTEGER NOT NULL,
    ordinal INTEGER NOT NULL,
    unit_type TEXT NOT NULL DEFAULT 'text',
    title TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    locator_json TEXT NOT NULL DEFAULT '{}',
    metadata_json TEXT NOT NULL DEFAULT '{}',
    checksum TEXT NOT NULL DEFAULT '',
    valid_from DATETIME,
    valid_to DATETIME,
    supersedes_json TEXT NOT NULL DEFAULT '[]',
    conflict_group TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    UNIQUE(source_id, source_revision, ordinal)
);
CREATE INDEX IF NOT EXISTS idx_knowledge_units_tenant_source ON knowledge_units(tenant_id, source_id, source_revision);
CREATE INDEX IF NOT EXISTS idx_knowledge_units_temporal ON knowledge_units(tenant_id, valid_from, valid_to);
CREATE INDEX IF NOT EXISTS idx_knowledge_units_conflict ON knowledge_units(tenant_id, conflict_group);

CREATE TABLE IF NOT EXISTS knowledge_chunks (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    source_id TEXT NOT NULL,
    source_revision INTEGER NOT NULL,
    unit_id TEXT NOT NULL,
    chunk_index INTEGER NOT NULL,
    parent_chunk_id TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    embedding_text TEXT NOT NULL DEFAULT '',
    token_count INTEGER NOT NULL DEFAULT 0,
    locator_json TEXT NOT NULL DEFAULT '{}',
    metadata_json TEXT NOT NULL DEFAULT '{}',
    checksum TEXT NOT NULL DEFAULT '',
    embedding_version TEXT NOT NULL DEFAULT '',
    index_version TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    valid_from DATETIME,
    valid_to DATETIME,
    supersedes_json TEXT NOT NULL DEFAULT '[]',
    conflict_group TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    UNIQUE(unit_id, chunk_index)
);
CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_tenant_source ON knowledge_chunks(tenant_id, source_id, source_revision);
CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_index_status ON knowledge_chunks(tenant_id, index_version, status);
CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_temporal ON knowledge_chunks(tenant_id, valid_from, valid_to);
CREATE INDEX IF NOT EXISTS idx_knowledge_chunks_conflict ON knowledge_chunks(tenant_id, conflict_group);

CREATE TABLE IF NOT EXISTS knowledge_ingest_jobs (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    source_id TEXT NOT NULL,
    source_revision INTEGER NOT NULL,
    kind TEXT NOT NULL DEFAULT 'ingest',
    stage TEXT NOT NULL DEFAULT 'queued',
    status TEXT NOT NULL DEFAULT 'queued',
    cursor_json TEXT NOT NULL DEFAULT '{}',
    last_committed_ordinal INTEGER NOT NULL DEFAULT -1,
    expected_units INTEGER NOT NULL DEFAULT 0,
    processed_units INTEGER NOT NULL DEFAULT 0,
    failed_units INTEGER NOT NULL DEFAULT 0,
    indexed_chunks INTEGER NOT NULL DEFAULT 0,
    retry_count INTEGER NOT NULL DEFAULT 0,
    heartbeat_at DATETIME,
    last_error TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    finished_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_knowledge_jobs_tenant_status ON knowledge_ingest_jobs(tenant_id, status, updated_at);
CREATE INDEX IF NOT EXISTS idx_knowledge_jobs_source_revision ON knowledge_ingest_jobs(source_id, source_revision);

CREATE TABLE IF NOT EXISTS knowledge_index_outbox (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    aggregate_type TEXT NOT NULL,
    aggregate_id TEXT NOT NULL,
    source_revision INTEGER NOT NULL,
    operation TEXT NOT NULL,
    payload_json TEXT NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'pending',
    retry_count INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    next_attempt_at DATETIME,
    lease_token TEXT NOT NULL DEFAULT '',
    lease_until DATETIME,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_knowledge_outbox_status ON knowledge_index_outbox(status, next_attempt_at);
CREATE INDEX IF NOT EXISTS idx_knowledge_outbox_aggregate ON knowledge_index_outbox(tenant_id, aggregate_type, aggregate_id, source_revision);
CREATE INDEX IF NOT EXISTS idx_knowledge_outbox_lease ON knowledge_index_outbox(tenant_id, status, next_attempt_at, lease_until);

CREATE TABLE IF NOT EXISTS knowledge_index_aliases (
    tenant_id TEXT PRIMARY KEY,
    active_version TEXT NOT NULL DEFAULT 'v1',
    previous_version TEXT NOT NULL DEFAULT '',
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS knowledge_benchmark_runs (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    config_hash TEXT NOT NULL DEFAULT '',
    k INTEGER NOT NULL DEFAULT 0,
    cases INTEGER NOT NULL DEFAULT 0,
    successful_cases INTEGER NOT NULL DEFAULT 0,
    error_cases INTEGER NOT NULL DEFAULT 0,
    failed_cases INTEGER NOT NULL DEFAULT 0,
    positive_cases INTEGER NOT NULL DEFAULT 0,
    no_answer_cases INTEGER NOT NULL DEFAULT 0,
    citation_observed_cases INTEGER NOT NULL DEFAULT 0,
    scenario_counts_json TEXT NOT NULL DEFAULT '{}',
    recall_at_k REAL NOT NULL DEFAULT 0,
    mrr REAL NOT NULL DEFAULT 0,
    ndcg_at_k REAL NOT NULL DEFAULT 0,
    context_precision REAL NOT NULL DEFAULT 0,
    context_recall REAL NOT NULL DEFAULT 0,
    no_answer_accuracy REAL NOT NULL DEFAULT 0,
    citation_precision REAL NOT NULL DEFAULT 0,
    citation_recall REAL NOT NULL DEFAULT 0,
    groundedness REAL NOT NULL DEFAULT 0,
    error_rate REAL NOT NULL DEFAULT 0,
    p50_latency_ms INTEGER NOT NULL DEFAULT 0,
    p95_latency_ms INTEGER NOT NULL DEFAULT 0,
    passed INTEGER NOT NULL DEFAULT 0,
    violations_json TEXT NOT NULL DEFAULT '[]',
    created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_knowledge_benchmark_runs_tenant_created ON knowledge_benchmark_runs(tenant_id, created_at);
CREATE INDEX IF NOT EXISTS idx_knowledge_benchmark_runs_tenant_config_created ON knowledge_benchmark_runs(tenant_id, config_hash, created_at);

CREATE TABLE IF NOT EXISTS retrieval_traces (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    request_id TEXT NOT NULL DEFAULT '',
    query_hash TEXT NOT NULL DEFAULT '',
    sources_json TEXT NOT NULL DEFAULT '[]',
    candidate_count INTEGER NOT NULL DEFAULT 0,
    selected_count INTEGER NOT NULL DEFAULT 0,
    sufficient INTEGER NOT NULL DEFAULT 0,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    error_code TEXT NOT NULL DEFAULT '',
    temporal_mode TEXT NOT NULL DEFAULT 'current',
    as_of DATETIME,
    degraded INTEGER NOT NULL DEFAULT 0,
    ambiguous INTEGER NOT NULL DEFAULT 0,
    warnings_json TEXT NOT NULL DEFAULT '[]',
    candidate_trace_json TEXT NOT NULL DEFAULT '[]',
    evidence_trace_json TEXT NOT NULL DEFAULT '[]',
    query_plan_json TEXT NOT NULL DEFAULT '{}',
    config_hash TEXT NOT NULL DEFAULT '',
    slo_target_ms INTEGER NOT NULL DEFAULT 0,
    slo_exceeded INTEGER NOT NULL DEFAULT 0,
    policy_mode TEXT NOT NULL DEFAULT '',
    route_reason TEXT NOT NULL DEFAULT '',
    shadow_level TEXT NOT NULL DEFAULT '',
    shadow_reason TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_retrieval_traces_tenant_created ON retrieval_traces(tenant_id, created_at);
CREATE INDEX IF NOT EXISTS idx_retrieval_traces_tenant_mode_created ON retrieval_traces(tenant_id, temporal_mode, created_at);
CREATE INDEX IF NOT EXISTS idx_retrieval_traces_tenant_config_created ON retrieval_traces(tenant_id, config_hash, created_at);
CREATE INDEX IF NOT EXISTS idx_retrieval_traces_tenant_slo_created ON retrieval_traces(tenant_id, slo_exceeded, created_at);
CREATE INDEX IF NOT EXISTS idx_retrieval_traces_tenant_policy_created ON retrieval_traces(tenant_id, policy_mode, created_at);

CREATE TABLE IF NOT EXISTS knowledge_vector_audit_sessions (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    collections_hash TEXT NOT NULL,
    page_limit INTEGER NOT NULL,
    page INTEGER NOT NULL DEFAULT 0,
    next_cursor TEXT NOT NULL DEFAULT '',
    last_cursor TEXT NOT NULL DEFAULT '',
    last_result_json TEXT NOT NULL DEFAULT '{}',
    sampled INTEGER NOT NULL DEFAULT 0,
    suspected_orphans INTEGER NOT NULL DEFAULT 0,
    metadata_mismatches INTEGER NOT NULL DEFAULT 0,
    in_flight INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'running',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    expires_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_knowledge_vector_audit_tenant_updated ON knowledge_vector_audit_sessions(tenant_id, updated_at);

CREATE TABLE IF NOT EXISTS knowledge_vector_audit_pages (
    audit_id TEXT NOT NULL,
    tenant_id TEXT NOT NULL,
    page INTEGER NOT NULL,
    request_hash TEXT NOT NULL,
    result_json TEXT NOT NULL,
    result_hash TEXT NOT NULL,
    previous_hash TEXT NOT NULL,
    receipt_hash TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    PRIMARY KEY (audit_id, page)
);
CREATE INDEX IF NOT EXISTS idx_knowledge_vector_audit_pages_tenant ON knowledge_vector_audit_pages(tenant_id, audit_id, page);

CREATE TABLE IF NOT EXISTS knowledge_vector_audit_recheck_leases (
    audit_id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    lease_token TEXT NOT NULL,
    lease_expires_at DATETIME NOT NULL,
    next_allowed_at DATETIME NOT NULL
   );
CREATE INDEX IF NOT EXISTS idx_knowledge_vector_audit_recheck_lease_tenant ON knowledge_vector_audit_recheck_leases(tenant_id, next_allowed_at);

CREATE TABLE IF NOT EXISTS knowledge_vector_audit_recheck_results (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    audit_id TEXT NOT NULL,
    page INTEGER NOT NULL,
    status TEXT NOT NULL,
    result_hash TEXT NOT NULL,
    receipt_hash TEXT NOT NULL,
    observed_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL
   );
CREATE INDEX IF NOT EXISTS idx_knowledge_vector_audit_recheck_result_history ON knowledge_vector_audit_recheck_results(tenant_id, audit_id, created_at);

CREATE TABLE IF NOT EXISTS knowledge_vector_recovery_reviews (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    audit_id TEXT NOT NULL,
    recheck_id TEXT NOT NULL,
    receipt_hash TEXT NOT NULL,
    request_key TEXT NOT NULL,
    requested_by_hash TEXT NOT NULL,
    reviewed_by_hash TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'revoked')),
    action TEXT NOT NULL CHECK (action = 'manual_investigation'),
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    expires_at DATETIME NOT NULL,
    UNIQUE (tenant_id, request_key),
    UNIQUE (tenant_id, audit_id, recheck_id)
);
CREATE INDEX IF NOT EXISTS idx_knowledge_vector_recovery_reviews_scope ON knowledge_vector_recovery_reviews(tenant_id, audit_id, created_at);

CREATE TABLE IF NOT EXISTS knowledge_vector_recovery_review_events (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    review_id TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('requested', 'approved', 'revoked')),
    actor_hash TEXT NOT NULL,
    receipt_hash TEXT NOT NULL,
    created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_knowledge_vector_recovery_review_events_scope ON knowledge_vector_recovery_review_events(tenant_id, review_id, created_at);

CREATE TABLE IF NOT EXISTS user_feedback_tickets (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    contact TEXT NOT NULL DEFAULT '',
    platform TEXT NOT NULL DEFAULT '',
    client_type TEXT NOT NULL DEFAULT '',
    app_version TEXT NOT NULL DEFAULT '',
    runtime_mode TEXT NOT NULL DEFAULT '',
    os_info TEXT NOT NULL DEFAULT '',
    device_hash TEXT NOT NULL DEFAULT '',
    locale TEXT NOT NULL DEFAULT '',
    timezone TEXT NOT NULL DEFAULT '',
    include_logs INTEGER NOT NULL DEFAULT 1,
    log_line_count INTEGER NOT NULL DEFAULT 0,
    diagnostics TEXT NOT NULL DEFAULT '',
    upload_url TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    remote_id TEXT NOT NULL DEFAULT '',
    remote_url TEXT NOT NULL DEFAULT '',
    screenshot_name TEXT NOT NULL DEFAULT '',
    screenshot_path TEXT NOT NULL DEFAULT '',
    screenshot_size INTEGER NOT NULL DEFAULT 0,
    has_screenshot INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT '',
    uploaded_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_user_feedback_tickets_space ON user_feedback_tickets(space_id, created_at);
CREATE INDEX IF NOT EXISTS idx_user_feedback_tickets_status ON user_feedback_tickets(status, attempts);

CREATE TABLE IF NOT EXISTS kernel_security_audit_outbox (
    event_id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    payload_json TEXT NOT NULL,
    occurred_at TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS runtime_model_usage (
    sequence INTEGER PRIMARY KEY AUTOINCREMENT,
    id TEXT NOT NULL,
    space_id TEXT NOT NULL,
    device_id TEXT,
    owner_id TEXT,
    role_id TEXT,
    request_id TEXT,
    model TEXT,
    provider TEXT,
    source TEXT,
    status TEXT,
    started_at TEXT,
    completed_at TEXT,
    input_tokens INTEGER,
    output_tokens INTEGER,
    cached_tokens INTEGER,
    total_tokens INTEGER,
    usage_reported NUMERIC,
    coordination_id TEXT DEFAULT '',
    parent_interaction_id TEXT DEFAULT '',
    assignment_id TEXT DEFAULT '',
    reserved_tokens INTEGER NOT NULL DEFAULT 0,
    reserved_cost_micro_usd INTEGER NOT NULL DEFAULT 0,
    budget_cost_limit_micro_usd INTEGER NOT NULL DEFAULT 0,
    budget_input_rate INTEGER NOT NULL DEFAULT 0,
    budget_output_rate INTEGER NOT NULL DEFAULT 0,
    budget_rate_version TEXT DEFAULT '',
    budget_overrun NUMERIC NOT NULL DEFAULT 0,
    review_evidence_ref TEXT NOT NULL DEFAULT '',
    review_actor_id TEXT NOT NULL DEFAULT '',
    review_recorded_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS runtime_usage_baselines (
    space_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    sequence INTEGER,
    updated_at TEXT,
    PRIMARY KEY (space_id, device_id)
);

CREATE TABLE IF NOT EXISTS runtime_owned_tool_calls (
    effect_id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    coordination_id TEXT NOT NULL,
    parent_interaction_id TEXT NOT NULL,
    owner_id TEXT NOT NULL,
    role_id TEXT NOT NULL,
    assignment_id TEXT NOT NULL,
    tool_name TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    review_evidence_ref TEXT NOT NULL DEFAULT '',
    review_actor_id TEXT NOT NULL DEFAULT '',
    review_recorded_at TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_runtime_model_usage_id ON runtime_model_usage(id);
CREATE INDEX IF NOT EXISTS idx_model_usage_scope ON runtime_model_usage(space_id, device_id);
CREATE INDEX IF NOT EXISTS idx_runtime_model_usage_started_at ON runtime_model_usage(started_at);
CREATE INDEX IF NOT EXISTS idx_owned_tool_coord ON runtime_owned_tool_calls(space_id, coordination_id);

CREATE TABLE IF NOT EXISTS channel_sandbox_policies (
    space_id TEXT NOT NULL DEFAULT '',
    channel_id TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 0,
    allow_network INTEGER NOT NULL DEFAULT 1,
    workspace_id TEXT NOT NULL DEFAULT '',
    sandbox_root TEXT NOT NULL DEFAULT '',
    revision INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (space_id, channel_id)
);

CREATE TABLE IF NOT EXISTS agent_profiles (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    character_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    archetype TEXT NOT NULL DEFAULT 'companion',
    status TEXT NOT NULL DEFAULT 'draft',
    owner_id TEXT NOT NULL DEFAULT '',
    team_id TEXT NOT NULL DEFAULT '',
    current_revision_id TEXT NOT NULL DEFAULT '',
    current_release_id TEXT NOT NULL DEFAULT '',
    revision INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_agent_profiles_deleted_at ON agent_profiles(deleted_at);
CREATE INDEX IF NOT EXISTS idx_agent_profiles_team_id ON agent_profiles(team_id);
CREATE INDEX IF NOT EXISTS idx_agent_profiles_owner_id ON agent_profiles(owner_id);
CREATE INDEX IF NOT EXISTS idx_agent_profiles_character_id ON agent_profiles(character_id);
CREATE INDEX IF NOT EXISTS idx_agent_profiles_space_id ON agent_profiles(space_id);

CREATE TABLE IF NOT EXISTS agent_runs (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    conversation_id TEXT NOT NULL,
    parent_run_id TEXT NOT NULL DEFAULT '',
    coordination_id TEXT NOT NULL DEFAULT '',
    assignment_id TEXT NOT NULL DEFAULT '',
    parent_interaction_id TEXT NOT NULL DEFAULT '',
    child_interaction_id TEXT NOT NULL DEFAULT '',
    character_id TEXT NOT NULL DEFAULT '',
    worker_role TEXT NOT NULL DEFAULT '',
    objective TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'planning',
    depth INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_agent_runs_updated_at ON agent_runs(updated_at);
CREATE INDEX IF NOT EXISTS idx_agent_runs_status ON agent_runs(status);
CREATE INDEX IF NOT EXISTS idx_agent_runs_character_id ON agent_runs(character_id);
CREATE INDEX IF NOT EXISTS idx_agent_runs_child_interaction ON agent_runs(child_interaction_id);
CREATE INDEX IF NOT EXISTS idx_agent_runs_parent_interaction ON agent_runs(parent_interaction_id);
CREATE INDEX IF NOT EXISTS idx_agent_runs_assignment_id ON agent_runs(assignment_id);
CREATE INDEX IF NOT EXISTS idx_agent_runs_coordination_id ON agent_runs(coordination_id);
CREATE INDEX IF NOT EXISTS idx_agent_runs_parent_run_id ON agent_runs(parent_run_id);
CREATE INDEX IF NOT EXISTS idx_agent_run_conversation ON agent_runs(space_id, conversation_id);

CREATE TABLE IF NOT EXISTS agent_run_messages (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    conversation_id TEXT NOT NULL,
    sender_run_id TEXT NOT NULL,
    recipient_run_id TEXT NOT NULL,
    kind TEXT NOT NULL DEFAULT 'instruction',
    content TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    correlation_id TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    read_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_agent_run_messages_created_at ON agent_run_messages(created_at);
CREATE INDEX IF NOT EXISTS idx_agent_run_messages_correlation_id ON agent_run_messages(correlation_id);
CREATE INDEX IF NOT EXISTS idx_agent_run_messages_status ON agent_run_messages(status);
CREATE INDEX IF NOT EXISTS idx_agent_run_messages_kind ON agent_run_messages(kind);
CREATE INDEX IF NOT EXISTS idx_agent_run_messages_sender_run_id ON agent_run_messages(sender_run_id);
CREATE INDEX IF NOT EXISTS idx_agent_run_messages_conversation ON agent_run_messages(conversation_id);
CREATE INDEX IF NOT EXISTS idx_agent_run_message_run ON agent_run_messages(space_id, recipient_run_id);

CREATE TABLE IF NOT EXISTS agent_revisions (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    revision INTEGER NOT NULL,
    config_json TEXT NOT NULL,
    config_hash TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    change_note TEXT NOT NULL DEFAULT '',
    created_by TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    published_at DATETIME,
    test_run_id TEXT NOT NULL DEFAULT '',
    test_status TEXT NOT NULL DEFAULT 'not_run',
    test_evidence TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_agent_revisions_agent_id ON agent_revisions(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_revisions_space_id ON agent_revisions(space_id);

CREATE TABLE IF NOT EXISTS agent_releases (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    revision_id TEXT NOT NULL,
    version TEXT NOT NULL,
    release_notes TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'published',
    evaluation_run_id TEXT NOT NULL DEFAULT '',
    created_by TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    superseded_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_agent_releases_revision_id ON agent_releases(revision_id);
CREATE INDEX IF NOT EXISTS idx_agent_releases_agent_id ON agent_releases(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_releases_space_id ON agent_releases(space_id);

CREATE TABLE IF NOT EXISTS agent_templates (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT 'general',
    archetype TEXT NOT NULL DEFAULT 'companion',
    description TEXT NOT NULL DEFAULT '',
    config_json TEXT NOT NULL,
    publisher_id TEXT NOT NULL DEFAULT 'com.amitia',
    trust_level TEXT NOT NULL DEFAULT 'official',
    builtin INTEGER NOT NULL DEFAULT 0,
    schema_version INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_agent_templates_category ON agent_templates(category);

CREATE TABLE IF NOT EXISTS platform_users (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    display_name TEXT NOT NULL,
    email TEXT NOT NULL DEFAULT '',
    auth_hash TEXT NOT NULL DEFAULT '',
    avatar TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    is_local_owner INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    revoked_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_platform_users_email ON platform_users(email);
CREATE INDEX IF NOT EXISTS idx_platform_users_space_id ON platform_users(space_id);

CREATE TABLE IF NOT EXISTS platform_sessions (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL,
    revoked_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_platform_sessions_expires_at ON platform_sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_platform_sessions_status ON platform_sessions(status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_platform_sessions_token_hash ON platform_sessions(token_hash);
CREATE INDEX IF NOT EXISTS idx_platform_sessions_user_id ON platform_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_platform_sessions_space_id ON platform_sessions(space_id);

CREATE TABLE IF NOT EXISTS platform_memberships (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    team_id TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL DEFAULT 'member',
    status TEXT NOT NULL DEFAULT 'active',
    permissions TEXT NOT NULL DEFAULT '[]',
    revision INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_platform_memberships_team_id ON platform_memberships(team_id);
CREATE INDEX IF NOT EXISTS idx_platform_memberships_user_id ON platform_memberships(user_id);
CREATE INDEX IF NOT EXISTS idx_platform_memberships_space_id ON platform_memberships(space_id);

CREATE TABLE IF NOT EXISTS platform_invitations (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    team_id TEXT NOT NULL DEFAULT '',
    inviter_id TEXT NOT NULL,
    email TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'member',
    token_hash TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    expires_at DATETIME NOT NULL,
    accepted_at DATETIME,
    created_at DATETIME NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_platform_invitations_token_hash ON platform_invitations(token_hash);
CREATE INDEX IF NOT EXISTS idx_platform_invitations_email ON platform_invitations(email);
CREATE INDEX IF NOT EXISTS idx_platform_invitations_team_id ON platform_invitations(team_id);
CREATE INDEX IF NOT EXISTS idx_platform_invitations_space_id ON platform_invitations(space_id);

CREATE TABLE IF NOT EXISTS teams (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    owner_id TEXT NOT NULL DEFAULT '',
    lead_agent_id TEXT NOT NULL DEFAULT '',
    workspace_id TEXT NOT NULL DEFAULT '',
    workspace_root_uri TEXT NOT NULL DEFAULT '',
    shared_memory INTEGER NOT NULL DEFAULT 0,
    shared_workspace INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'active',
    revision INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_teams_deleted_at ON teams(deleted_at);
CREATE INDEX IF NOT EXISTS idx_teams_owner_id ON teams(owner_id);
CREATE INDEX IF NOT EXISTS idx_teams_space_id ON teams(space_id);

CREATE TABLE IF NOT EXISTS team_templates (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    shared_memory INTEGER NOT NULL DEFAULT 1,
    shared_workspace INTEGER NOT NULL DEFAULT 1,
    routing_policy_json TEXT NOT NULL DEFAULT '{}',
    builtin INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS team_members (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    team_id TEXT NOT NULL,
    member_type TEXT NOT NULL,
    member_id TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'member',
    responder_mode TEXT NOT NULL DEFAULT 'mention_or_lead',
    capabilities TEXT NOT NULL DEFAULT '[]',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_team_members_member_id ON team_members(member_id);
CREATE INDEX IF NOT EXISTS idx_team_members_team_id ON team_members(team_id);
CREATE INDEX IF NOT EXISTS idx_team_members_space_id ON team_members(space_id);

CREATE TABLE IF NOT EXISTS team_contexts (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    team_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    key TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    visibility TEXT NOT NULL DEFAULT 'team',
    revision INTEGER NOT NULL DEFAULT 1,
    created_by TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_team_contexts_kind ON team_contexts(kind);
CREATE INDEX IF NOT EXISTS idx_team_contexts_team_id ON team_contexts(team_id);
CREATE INDEX IF NOT EXISTS idx_team_contexts_space_id ON team_contexts(space_id);

CREATE TABLE IF NOT EXISTS team_handoffs (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    team_id TEXT NOT NULL,
    task_id TEXT NOT NULL,
    from_agent_id TEXT NOT NULL,
    to_agent_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    summary TEXT NOT NULL DEFAULT '',
    context_json TEXT NOT NULL DEFAULT '{}',
    evidence_ref TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    accepted_at DATETIME,
    completed_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_team_handoffs_to_agent_id ON team_handoffs(to_agent_id);
CREATE INDEX IF NOT EXISTS idx_team_handoffs_from_agent_id ON team_handoffs(from_agent_id);
CREATE INDEX IF NOT EXISTS idx_team_handoffs_task_id ON team_handoffs(task_id);
CREATE INDEX IF NOT EXISTS idx_team_handoffs_team_id ON team_handoffs(team_id);
CREATE INDEX IF NOT EXISTS idx_team_handoffs_space_id ON team_handoffs(space_id);

CREATE TABLE IF NOT EXISTS approval_records (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    agent_id TEXT NOT NULL DEFAULT '',
    team_id TEXT NOT NULL DEFAULT '',
    conversation_id TEXT NOT NULL DEFAULT '',
    turn_id TEXT NOT NULL DEFAULT '',
    tool_call_id TEXT NOT NULL DEFAULT '',
    tool_name TEXT NOT NULL,
    action_summary TEXT NOT NULL DEFAULT '',
    resource_summary TEXT NOT NULL DEFAULT '',
    arguments_json TEXT NOT NULL DEFAULT '{}',
    evidence_json TEXT NOT NULL DEFAULT '{}',
    scope_json TEXT NOT NULL DEFAULT '{}',
    risk_level TEXT NOT NULL DEFAULT 'medium',
    reversible INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',
    allow_once INTEGER NOT NULL DEFAULT 1,
    session_grant INTEGER NOT NULL DEFAULT 0,
    timed_grant_until DATETIME,
    request_hash TEXT NOT NULL,
    requested_by TEXT NOT NULL DEFAULT '',
    decided_by TEXT NOT NULL DEFAULT '',
    decision_reason TEXT NOT NULL DEFAULT '',
    expires_at DATETIME NOT NULL,
    decided_at DATETIME,
    resumed_at DATETIME,
    revision INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_approval_records_expires_at ON approval_records(expires_at);
CREATE INDEX IF NOT EXISTS idx_approval_records_status ON approval_records(status);
CREATE INDEX IF NOT EXISTS idx_approval_records_tool_call_id ON approval_records(tool_call_id);
CREATE INDEX IF NOT EXISTS idx_approval_records_conversation_id ON approval_records(conversation_id);
CREATE INDEX IF NOT EXISTS idx_approval_records_team_id ON approval_records(team_id);
CREATE INDEX IF NOT EXISTS idx_approval_records_agent_id ON approval_records(agent_id);
CREATE INDEX IF NOT EXISTS idx_approval_records_space_id ON approval_records(space_id);

CREATE TABLE IF NOT EXISTS approval_decisions (
    id TEXT PRIMARY KEY,
    approval_id TEXT NOT NULL,
    decision TEXT NOT NULL,
    grant_mode TEXT NOT NULL DEFAULT 'once',
    reason TEXT NOT NULL DEFAULT '',
    actor_id TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_approval_decisions_approval_id ON approval_decisions(approval_id);

CREATE TABLE IF NOT EXISTS resource_leases (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    holder_agent_id TEXT NOT NULL,
    task_id TEXT NOT NULL DEFAULT '',
    mode TEXT NOT NULL DEFAULT 'exclusive',
    fingerprint TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL,
    released_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_resource_leases_status ON resource_leases(status);
CREATE INDEX IF NOT EXISTS idx_resource_leases_holder_agent_id ON resource_leases(holder_agent_id);
CREATE INDEX IF NOT EXISTS idx_resource_leases_resource_id ON resource_leases(resource_id);
CREATE INDEX IF NOT EXISTS idx_resource_leases_resource_type ON resource_leases(resource_type);
CREATE INDEX IF NOT EXISTS idx_resource_leases_space_id ON resource_leases(space_id);

CREATE TABLE IF NOT EXISTS idempotent_operations (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    scope_key TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'running',
    result_json TEXT NOT NULL DEFAULT '',
    original_error TEXT NOT NULL DEFAULT '',
    revision INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    completed_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_idempotent_operations_status ON idempotent_operations(status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_idempotent_scope ON idempotent_operations(idempotency_key, scope_key);
CREATE INDEX IF NOT EXISTS idx_idempotent_operations_agent_id ON idempotent_operations(agent_id);
CREATE INDEX IF NOT EXISTS idx_idempotent_operations_space_id ON idempotent_operations(space_id);

CREATE TABLE IF NOT EXISTS runtime_instances (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    provider TEXT NOT NULL,
    provider_ref TEXT NOT NULL DEFAULT '',
    image TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL DEFAULT 'created',
    root_path TEXT NOT NULL DEFAULT '',
    workspace_id TEXT NOT NULL DEFAULT '',
    workspace_root_uri TEXT NOT NULL DEFAULT '',
    endpoint TEXT NOT NULL DEFAULT '',
    cpu_quota REAL NOT NULL DEFAULT 1,
    memory_bytes INTEGER NOT NULL DEFAULT 1073741824,
    disk_bytes INTEGER NOT NULL DEFAULT 10737418240,
    process_limit INTEGER NOT NULL DEFAULT 64,
    idle_timeout_seconds INTEGER NOT NULL DEFAULT 1800,
    snapshot_id TEXT NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    revision INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    started_at DATETIME,
    stopped_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_runtime_instances_workspace_id ON runtime_instances(workspace_id);
CREATE INDEX IF NOT EXISTS idx_runtime_instances_state ON runtime_instances(state);
CREATE INDEX IF NOT EXISTS idx_runtime_instances_agent_id ON runtime_instances(agent_id);
CREATE INDEX IF NOT EXISTS idx_runtime_instances_space_id ON runtime_instances(space_id);

CREATE TABLE IF NOT EXISTS runtime_snapshots (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    runtime_id TEXT NOT NULL,
    name TEXT NOT NULL,
    path TEXT NOT NULL DEFAULT '',
    manifest_json TEXT NOT NULL DEFAULT '{}',
    content_hash TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_runtime_snapshots_runtime_id ON runtime_snapshots(runtime_id);
CREATE INDEX IF NOT EXISTS idx_runtime_snapshots_agent_id ON runtime_snapshots(agent_id);
CREATE INDEX IF NOT EXISTS idx_runtime_snapshots_space_id ON runtime_snapshots(space_id);

CREATE TABLE IF NOT EXISTS host_control_sessions (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    platform TEXT NOT NULL,
    capabilities TEXT NOT NULL DEFAULT '[]',
    permission_json TEXT NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'pending',
    approved_by TEXT NOT NULL DEFAULT '',
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL,
    ended_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_host_control_sessions_status ON host_control_sessions(status);
CREATE INDEX IF NOT EXISTS idx_host_control_sessions_agent_id ON host_control_sessions(agent_id);
CREATE INDEX IF NOT EXISTS idx_host_control_sessions_space_id ON host_control_sessions(space_id);

CREATE TABLE IF NOT EXISTS evaluation_suites (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    agent_id TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    config_json TEXT NOT NULL DEFAULT '{}',
    gate_policy_json TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_evaluation_suites_agent_id ON evaluation_suites(agent_id);
CREATE INDEX IF NOT EXISTS idx_evaluation_suites_space_id ON evaluation_suites(space_id);

CREATE TABLE IF NOT EXISTS evaluation_cases (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    suite_id TEXT NOT NULL,
    name TEXT NOT NULL,
    input_json TEXT NOT NULL DEFAULT '{}',
    expected_json TEXT NOT NULL DEFAULT '{}',
    category TEXT NOT NULL DEFAULT 'general',
    safety_critical INTEGER NOT NULL DEFAULT 0,
    weight REAL NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_evaluation_cases_suite_id ON evaluation_cases(suite_id);
CREATE INDEX IF NOT EXISTS idx_evaluation_cases_space_id ON evaluation_cases(space_id);

CREATE TABLE IF NOT EXISTS evaluation_runs (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    revision_id TEXT NOT NULL,
    suite_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'running',
    task_success_rate REAL NOT NULL DEFAULT 0,
    tool_accuracy REAL NOT NULL DEFAULT 0,
    safety_violations INTEGER NOT NULL DEFAULT 0,
    false_success_rate REAL NOT NULL DEFAULT 0,
    recovery_rate REAL NOT NULL DEFAULT 0,
    memory_leak_count INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    total_cost_micro_usd INTEGER NOT NULL DEFAULT 0,
    average_latency_ms INTEGER NOT NULL DEFAULT 0,
    gate_status TEXT NOT NULL DEFAULT 'pending',
    gate_reason TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    completed_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_evaluation_runs_status ON evaluation_runs(status);
CREATE INDEX IF NOT EXISTS idx_evaluation_runs_suite_id ON evaluation_runs(suite_id);
CREATE INDEX IF NOT EXISTS idx_evaluation_runs_revision_id ON evaluation_runs(revision_id);
CREATE INDEX IF NOT EXISTS idx_evaluation_runs_agent_id ON evaluation_runs(agent_id);
CREATE INDEX IF NOT EXISTS idx_evaluation_runs_space_id ON evaluation_runs(space_id);

CREATE TABLE IF NOT EXISTS evaluation_results (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    run_id TEXT NOT NULL,
    case_id TEXT NOT NULL,
    passed INTEGER NOT NULL DEFAULT 0,
    score REAL NOT NULL DEFAULT 0,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    tokens INTEGER NOT NULL DEFAULT 0,
    cost_micro_usd INTEGER NOT NULL DEFAULT 0,
    tool_trace_json TEXT NOT NULL DEFAULT '[]',
    failure_json TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_evaluation_results_case_id ON evaluation_results(case_id);
CREATE INDEX IF NOT EXISTS idx_evaluation_results_run_id ON evaluation_results(run_id);
CREATE INDEX IF NOT EXISTS idx_evaluation_results_space_id ON evaluation_results(space_id);

CREATE TABLE IF NOT EXISTS activity_events (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    actor_id TEXT NOT NULL DEFAULT '',
    actor_type TEXT NOT NULL DEFAULT '',
    agent_id TEXT NOT NULL DEFAULT '',
    team_id TEXT NOT NULL DEFAULT '',
    runtime_id TEXT NOT NULL DEFAULT '',
    conversation_id TEXT NOT NULL DEFAULT '',
    kind TEXT NOT NULL,
    state TEXT NOT NULL,
    title TEXT NOT NULL,
    detail TEXT NOT NULL DEFAULT '',
    waiting_reason TEXT NOT NULL DEFAULT '',
    cost_micro_usd INTEGER NOT NULL DEFAULT 0,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    payload_json TEXT NOT NULL DEFAULT '{}',
    occurred_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_activity_events_occurred_at ON activity_events(occurred_at);
CREATE INDEX IF NOT EXISTS idx_activity_events_state ON activity_events(state);
CREATE INDEX IF NOT EXISTS idx_activity_events_kind ON activity_events(kind);
CREATE INDEX IF NOT EXISTS idx_activity_events_conversation_id ON activity_events(conversation_id);
CREATE INDEX IF NOT EXISTS idx_activity_events_runtime_id ON activity_events(runtime_id);
CREATE INDEX IF NOT EXISTS idx_activity_events_team_id ON activity_events(team_id);
CREATE INDEX IF NOT EXISTS idx_activity_events_agent_id ON activity_events(agent_id);
CREATE INDEX IF NOT EXISTS idx_activity_events_actor_id ON activity_events(actor_id);
CREATE INDEX IF NOT EXISTS idx_activity_events_space_id ON activity_events(space_id);

CREATE TABLE IF NOT EXISTS agent_autonomy_policies (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    level INTEGER NOT NULL DEFAULT 1,
    watch_only INTEGER NOT NULL DEFAULT 1,
    auto_low_risk INTEGER NOT NULL DEFAULT 0,
    external_approval INTEGER NOT NULL DEFAULT 1,
    max_actions_per_day INTEGER NOT NULL DEFAULT 20,
    max_cost_micro_usd INTEGER NOT NULL DEFAULT 0,
    quiet_hours TEXT NOT NULL DEFAULT '',
    escalation_target TEXT NOT NULL DEFAULT '',
    triggers_json TEXT NOT NULL DEFAULT '[]',
    kill_switch INTEGER NOT NULL DEFAULT 0,
    revision INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_autonomy_policies_agent_id ON agent_autonomy_policies(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_autonomy_policies_space_id ON agent_autonomy_policies(space_id);

CREATE TABLE IF NOT EXISTS a2a_agent_cards (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    url TEXT NOT NULL,
    version TEXT NOT NULL DEFAULT '1.0.0',
    skills_json TEXT NOT NULL DEFAULT '[]',
    security_json TEXT NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'active',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_a2a_agent_cards_status ON a2a_agent_cards(status);
CREATE INDEX IF NOT EXISTS idx_a2a_agent_cards_agent_id ON a2a_agent_cards(agent_id);
CREATE INDEX IF NOT EXISTS idx_a2a_agent_cards_space_id ON a2a_agent_cards(space_id);

CREATE TABLE IF NOT EXISTS a2a_tasks (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    remote_agent_url TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'submitted',
    input_json TEXT NOT NULL DEFAULT '{}',
    output_json TEXT NOT NULL DEFAULT '',
    original_error TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    completed_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_a2a_tasks_status ON a2a_tasks(status);
CREATE INDEX IF NOT EXISTS idx_a2a_tasks_agent_id ON a2a_tasks(agent_id);
CREATE INDEX IF NOT EXISTS idx_a2a_tasks_space_id ON a2a_tasks(space_id);

CREATE TABLE IF NOT EXISTS a2_a_replay_records (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    sender_agent_id TEXT NOT NULL,
    nonce TEXT NOT NULL,
    signature_hash TEXT NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_a2_a_replay_records_expires_at ON a2_a_replay_records(expires_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_a2_a_replay_records_nonce ON a2_a_replay_records(nonce);
CREATE INDEX IF NOT EXISTS idx_a2_a_replay_records_sender_agent_id ON a2_a_replay_records(sender_agent_id);
CREATE INDEX IF NOT EXISTS idx_a2_a_replay_records_space_id ON a2_a_replay_records(space_id);

CREATE TABLE IF NOT EXISTS agent_access_policies (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    scope_type TEXT NOT NULL,
    channel_id TEXT NOT NULL DEFAULT '',
    account_id TEXT NOT NULL DEFAULT '',
    conversation_id TEXT NOT NULL DEFAULT '',
    sender_id TEXT NOT NULL DEFAULT '',
    explicit_mode TEXT NOT NULL DEFAULT 'allow',
    inferred_mode TEXT NOT NULL DEFAULT 'observe',
    trigger_mode TEXT NOT NULL DEFAULT 'direct_or_mention',
    permission_ceiling TEXT NOT NULL DEFAULT 'request_approval',
    enabled INTEGER NOT NULL DEFAULT 1,
    expires_at DATETIME,
    revision INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_agent_access_scope ON agent_access_policies(space_id, scope_type, channel_id, account_id, conversation_id, sender_id);

CREATE TABLE IF NOT EXISTS agent_access_decisions (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    request_id TEXT NOT NULL DEFAULT '',
    channel_id TEXT NOT NULL DEFAULT '',
    account_id TEXT NOT NULL DEFAULT '',
    conversation_id TEXT NOT NULL DEFAULT '',
    sender_id TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT '',
    trigger_mode TEXT NOT NULL DEFAULT '',
    strength TEXT NOT NULL DEFAULT '',
    decision TEXT NOT NULL DEFAULT '',
    allowed INTEGER NOT NULL DEFAULT 0,
    observed INTEGER NOT NULL DEFAULT 0,
    policy_id TEXT NOT NULL DEFAULT '',
    policy_revision INTEGER NOT NULL DEFAULT 0,
    permission_mode TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_agent_access_decision_request ON agent_access_decisions(space_id, request_id);
CREATE INDEX IF NOT EXISTS idx_agent_access_decision_created ON agent_access_decisions(created_at);

CREATE TABLE IF NOT EXISTS agent_attention_policies (
    space_id TEXT PRIMARY KEY,
    enabled INTEGER NOT NULL DEFAULT 1,
    shadow_mode INTEGER NOT NULL DEFAULT 1,
    max_interruptions_per_day INTEGER NOT NULL DEFAULT 24,
    max_repeated_confirmations INTEGER NOT NULL DEFAULT 3,
    max_consecutive_followups INTEGER NOT NULL DEFAULT 2,
    max_unchanged_notifications INTEGER NOT NULL DEFAULT 1,
    dedupe_window_minutes INTEGER NOT NULL DEFAULT 30,
    revision INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS agent_attention_events (
    id TEXT PRIMARY KEY,
    space_id TEXT NOT NULL,
    role_id TEXT NOT NULL DEFAULT '',
    conversation_id TEXT NOT NULL DEFAULT '',
    source_kind TEXT NOT NULL DEFAULT '',
    source_id TEXT NOT NULL DEFAULT '',
    event_kind TEXT NOT NULL DEFAULT '',
    delivery_state TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    payload_json TEXT NOT NULL DEFAULT '{}',
    idempotency_key TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_agent_attention_event_scope ON agent_attention_events(space_id, created_at);
CREATE INDEX IF NOT EXISTS idx_agent_attention_event_key ON agent_attention_events(space_id, idempotency_key, created_at);
