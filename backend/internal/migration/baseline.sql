CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    checksum TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    error_message TEXT NOT NULL DEFAULT '',
    started_at TEXT NOT NULL DEFAULT '',
    finished_at TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS auth_users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT DEFAULT 'admin',
    is_active INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now')),
    last_login_at TEXT
);

CREATE TABLE IF NOT EXISTS auth_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    token_hash TEXT NOT NULL,
    device_name TEXT DEFAULT '',
    ip_address TEXT DEFAULT '',
    user_agent TEXT DEFAULT '',
    last_active_at TEXT DEFAULT (datetime('now')),
    expires_at TEXT,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS characters (
    id TEXT PRIMARY KEY,
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
    conversation_id TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
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
    card_data_json TEXT DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS character_templates (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    category TEXT DEFAULT '',
    description TEXT DEFAULT '',
    builtin INTEGER DEFAULT 0,
    template_json TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS conversations (
    id TEXT PRIMARY KEY,
    character_id TEXT DEFAULT '',
    title TEXT DEFAULT '',
    channel TEXT DEFAULT 'web',
    source TEXT DEFAULT 'manual',
    peer_id TEXT DEFAULT '',
    message_count INTEGER DEFAULT 0,
    state_version TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL,
    sequence INTEGER NOT NULL DEFAULT 0,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
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
    tool_call_id TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    emote_id TEXT NOT NULL DEFAULT '',
    alt_text TEXT NOT NULL DEFAULT '',
    is_animated INTEGER NOT NULL DEFAULT 0,
    media_width INTEGER NOT NULL DEFAULT 0,
    media_height INTEGER NOT NULL DEFAULT 0,
    original_asset_reference TEXT NOT NULL DEFAULT '',
    fallback_asset_reference TEXT NOT NULL DEFAULT '',
    response_group_id TEXT NOT NULL DEFAULT '',
    delivery_sequence INTEGER NOT NULL DEFAULT 0,
    emote_decision_status TEXT NOT NULL DEFAULT 'none'
);

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
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
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
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_tool_call_intents_request ON tool_call_intents(request_id, tool_name);
CREATE INDEX IF NOT EXISTS idx_tool_call_intents_idempotency ON tool_call_intents(idempotency_key);
CREATE INDEX IF NOT EXISTS idx_tool_call_results_request ON tool_call_results(request_id, status);

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
    capabilities_json TEXT DEFAULT '',
    provider_config_json TEXT DEFAULT '{}',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

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
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS asr_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    api_type TEXT DEFAULT 'volcengine',
    api_key TEXT DEFAULT '',
    base_url TEXT DEFAULT '',
    resource_id TEXT DEFAULT 'volc.seedasr.auc',
    is_active INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS vision_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    api_type TEXT DEFAULT 'volcengine',
    api_key TEXT DEFAULT '',
    model_name TEXT DEFAULT 'doubao-seed-2-0-lite-260428',
    base_url TEXT DEFAULT 'https://ark.cn-beijing.volces.com/api/v3',
    is_active INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS sleep_settings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    character_id TEXT DEFAULT '',
    bed_time TEXT DEFAULT '23:00',
    wake_time TEXT DEFAULT '07:00',
    enabled INTEGER DEFAULT 1,
    sleep_reply_enabled INTEGER DEFAULT 0,
    sleep_reply_mode TEXT DEFAULT 'NO_REPLY',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS fixed_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    character_id TEXT DEFAULT '',
    title TEXT NOT NULL,
    description TEXT DEFAULT '',
    week_day INTEGER DEFAULT -1,
    start_time TEXT DEFAULT '',
    end_time TEXT DEFAULT '',
    event_type TEXT DEFAULT 'CUSTOM_BUSY',
    repeat_type TEXT DEFAULT 'weekly',
    repeat_days TEXT DEFAULT '',
    prepare_min_minutes INTEGER DEFAULT 10,
    prepare_max_minutes INTEGER DEFAULT 40,
    reply_mode TEXT DEFAULT 'SHORT_REPLY',
    enabled INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS special_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    character_id TEXT DEFAULT '',
    title TEXT NOT NULL,
    description TEXT DEFAULT '',
    start_date TEXT DEFAULT '',
    end_date TEXT DEFAULT '',
    start_time TEXT DEFAULT '',
    end_time TEXT DEFAULT '',
    event_type TEXT DEFAULT 'CUSTOM',
    repeat_type TEXT DEFAULT 'none',
    repeat_days TEXT DEFAULT '',
    enabled INTEGER DEFAULT 1,
    priority INTEGER DEFAULT 0,
    active_message_allowed INTEGER DEFAULT 1,
    reply_mode TEXT DEFAULT 'SHORT_REPLY',
    affect_schedule INTEGER DEFAULT 0,
    affect_sleep INTEGER DEFAULT 0,
    affect_meal INTEGER DEFAULT 0,
    affect_energy INTEGER DEFAULT 0,
    payload TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS class_adjustments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    character_id TEXT DEFAULT '',
    date TEXT DEFAULT '',
    slot_index INTEGER DEFAULT 0,
    class_name TEXT DEFAULT '',
    adjust_type TEXT DEFAULT 'swap',
    description TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS lifestyle_tendencies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    character_id TEXT DEFAULT '',
    punctuality_tendency INTEGER DEFAULT 50,
    early_prepare_tendency INTEGER DEFAULT 50,
    self_discipline_tendency INTEGER DEFAULT 50,
    sleepiness_tendency INTEGER DEFAULT 50,
    randomness_tendency INTEGER DEFAULT 50,
    activity_energy INTEGER DEFAULT 50,
    social_energy INTEGER DEFAULT 50,
    care_tendency INTEGER DEFAULT 50,
    daily_share_tendency INTEGER DEFAULT 50,
    manually_configured INTEGER DEFAULT 0,
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS work_profiles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    character_id TEXT DEFAULT '',
    enabled INTEGER DEFAULT 0,
    work_days TEXT DEFAULT 'MON,TUE,WED,THU,FRI',
    work_start_time TEXT DEFAULT '09:00',
    work_end_time TEXT DEFAULT '18:00',
    lunch_break_start_time TEXT DEFAULT '12:00',
    lunch_break_end_time TEXT DEFAULT '13:30',
    commute_min_minutes INTEGER DEFAULT 15,
    commute_max_minutes INTEGER DEFAULT 45,
    prepare_min_minutes INTEGER DEFAULT 20,
    prepare_max_minutes INTEGER DEFAULT 60,
    reply_mode TEXT DEFAULT 'SHORT_REPLY',
    allow_overtime INTEGER DEFAULT 0,
    overtime_probability INTEGER DEFAULT 10,
    overtime_min_minutes INTEGER DEFAULT 30,
    overtime_max_minutes INTEGER DEFAULT 180,
    overtime_reply_mode TEXT DEFAULT 'SHORT_REPLY',
    delayed_reply_enabled INTEGER DEFAULT 0,
    commute_home_share_enabled INTEGER DEFAULT 1,
    commute_home_share_probability INTEGER DEFAULT 60,
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS role_profiles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    character_id TEXT DEFAULT '',
    role_name TEXT DEFAULT '',
    gender TEXT DEFAULT 'UNSPECIFIED',
    gender_label TEXT DEFAULT '',
    pronoun TEXT DEFAULT 'TA',
    self_reference TEXT DEFAULT '我',
    user_addressing_style TEXT DEFAULT '',
    gender_expression INTEGER DEFAULT 30,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS active_message_settings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    character_id TEXT DEFAULT '',
    enabled INTEGER DEFAULT 1,
    active_level INTEGER DEFAULT 50,
    min_interval INTEGER DEFAULT 60,
    quiet_start TEXT DEFAULT '23:00',
    quiet_end TEXT DEFAULT '07:00',
    quiet_minutes TEXT DEFAULT '',
    max_per_day INTEGER DEFAULT 6,
    max_daily_calls INTEGER DEFAULT 10,
    channel TEXT DEFAULT 'all',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS active_message_task (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    character_id TEXT DEFAULT '',
    task_type TEXT DEFAULT '',
    due_time TEXT,
    prompt TEXT DEFAULT '',
    status TEXT DEFAULT 'PENDING',
    reason TEXT DEFAULT '',
    retry_count INTEGER DEFAULT 0,
    max_retry INTEGER DEFAULT 3,
    last_error TEXT DEFAULT '',
    sent_at TEXT,
    interaction_id TEXT DEFAULT '',
    delivery_intent_id TEXT DEFAULT '',
    delivery_id TEXT DEFAULT '',
    request_id TEXT DEFAULT '',
    delivery_status TEXT DEFAULT 'PENDING',
    delivered_at TEXT DEFAULT '',
    error_message TEXT DEFAULT '',
    canceled_at TEXT,
    cancel_reason TEXT DEFAULT '',
    source TEXT DEFAULT 'schedule_based',
    lock_until TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS proactive_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT DEFAULT '',
    enabled INTEGER DEFAULT 1,
    channel TEXT DEFAULT 'web',
    character_id TEXT DEFAULT '',
    rule_type TEXT DEFAULT 'cron',
    schedule_cron TEXT DEFAULT '',
    quiet_start TEXT DEFAULT '',
    quiet_end TEXT DEFAULT '',
    max_per_day INTEGER DEFAULT 10,
    sent_count_today INTEGER DEFAULT 0,
    prompt_template TEXT DEFAULT '',
    random_minutes INTEGER DEFAULT 0,
    last_sent_at TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS proactive_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id INTEGER,
    conversation_id TEXT DEFAULT '',
    message_content TEXT DEFAULT '',
    channel TEXT DEFAULT '',
    status TEXT DEFAULT '',
    task_type TEXT DEFAULT '',
    prompt TEXT DEFAULT '',
    error TEXT DEFAULT '',
    sent_at TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    interaction_id TEXT DEFAULT '',
    delivery_intent_id TEXT DEFAULT '',
    delivery_id TEXT DEFAULT '',
    request_id TEXT DEFAULT '',
    delivery_status TEXT DEFAULT 'PENDING',
    delivered_at TEXT DEFAULT '',
    error_message TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS reminders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT DEFAULT '',
    content TEXT DEFAULT '',
    channel TEXT DEFAULT 'web',
    character_id TEXT DEFAULT '',
    conversation_id TEXT DEFAULT '',
    remind_at TEXT DEFAULT '',
    repeat_rule TEXT DEFAULT '',
    enabled INTEGER DEFAULT 1,
    last_triggered_at TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS memories (
    id TEXT PRIMARY KEY,
    key TEXT DEFAULT '',
    value TEXT DEFAULT '',
    memory_type TEXT DEFAULT 'fact',
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
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    use_count INTEGER DEFAULT 0,
    last_used_at TEXT,
    sensitivity_level TEXT DEFAULT 'internal',
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
    created_at TEXT DEFAULT (datetime('now')),
    version INTEGER NOT NULL DEFAULT 1,
    operation_id TEXT NOT NULL DEFAULT '',
    snapshot_hash TEXT NOT NULL DEFAULT '',
    event_reason TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS memory_candidates (
    id TEXT PRIMARY KEY,
    key TEXT NOT NULL DEFAULT '',
    value TEXT NOT NULL DEFAULT '',
    memory_type TEXT DEFAULT 'custom',
    importance INTEGER DEFAULT 5,
    source_text TEXT DEFAULT '',
    conversation_id TEXT DEFAULT '',
    character_id TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    sensitivity_level TEXT DEFAULT 'internal',
    allow_proactive_mention INTEGER DEFAULT 1,
    requires_confirmation INTEGER DEFAULT 0,
    candidate_kind TEXT NOT NULL DEFAULT 'extracted',
    confidence REAL NOT NULL DEFAULT 0,
    target_memory_id TEXT NOT NULL DEFAULT '',
    proposed_action TEXT NOT NULL DEFAULT '',
    source_memory_ids_json TEXT NOT NULL DEFAULT '',
    source_versions_json TEXT NOT NULL DEFAULT '',
    derivation_key TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS memory_derivations (
    id TEXT PRIMARY KEY,
    output_memory_id TEXT NOT NULL,
    input_memory_id TEXT NOT NULL,
    input_version INTEGER NOT NULL,
    input_snapshot_hash TEXT NOT NULL DEFAULT '',
    derivation_kind TEXT NOT NULL,
    ordinal INTEGER NOT NULL DEFAULT 0,
    operation_id TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    UNIQUE(output_memory_id, input_memory_id, input_version, derivation_kind)
);

CREATE INDEX IF NOT EXISTS idx_memory_derivations_output ON memory_derivations(output_memory_id);
CREATE INDEX IF NOT EXISTS idx_memory_derivations_input ON memory_derivations(input_memory_id);
CREATE INDEX IF NOT EXISTS idx_memory_events_operation ON memory_events(operation_id);

CREATE INDEX IF NOT EXISTS idx_memories_confidence ON memories(character_id, confidence);
CREATE INDEX IF NOT EXISTS idx_memories_verified ON memories(character_id, verified_status);
CREATE INDEX IF NOT EXISTS idx_memories_entity ON memories(entity_id, entity_type);
CREATE INDEX IF NOT EXISTS idx_memories_importance_conf ON memories(character_id, importance, confidence);
CREATE INDEX IF NOT EXISTS idx_memories_scope_type ON memories(scope_type);
CREATE INDEX IF NOT EXISTS idx_memories_character_type ON memories(character_id, memory_type);
CREATE INDEX IF NOT EXISTS idx_memories_derivation_key ON memories(derivation_key);

CREATE TABLE IF NOT EXISTS memory_embeddings (
    memory_id TEXT PRIMARY KEY,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS episodic_memories (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL DEFAULT 'default',
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
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    message_time_start TEXT NOT NULL DEFAULT '',
    message_time_end TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_episodic_user_id ON episodic_memories(user_id);
CREATE INDEX IF NOT EXISTS idx_episodic_scene_type ON episodic_memories(user_id, scene_type);
CREATE INDEX IF NOT EXISTS idx_episodic_created ON episodic_memories(user_id, created_at);

CREATE TABLE IF NOT EXISTS user_profiles (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL DEFAULT 'default',
    character_id TEXT NOT NULL DEFAULT '',
    category TEXT NOT NULL,
    attribute_name TEXT NOT NULL,
    attribute_value TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT '',
    confidence INTEGER DEFAULT 50,
    source_conv_id TEXT DEFAULT '',
    verified_at TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_profiles_uid_cat_attr ON user_profiles(user_id, character_id, category, attribute_name);
CREATE INDEX IF NOT EXISTS idx_user_profiles_user_id ON user_profiles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_profiles_confidence ON user_profiles(user_id, confidence);

CREATE TABLE IF NOT EXISTS world_book (
    id TEXT PRIMARY KEY,
    match_type TEXT NOT NULL DEFAULT 'keyword',
    match_pattern TEXT NOT NULL DEFAULT '',
    match_scope TEXT NOT NULL DEFAULT 'full_context',
    inject_content TEXT NOT NULL DEFAULT '',
    priority INTEGER DEFAULT 0,
    hit_count INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_world_book_match_type ON world_book(match_type);
CREATE INDEX IF NOT EXISTS idx_world_book_priority ON world_book(priority);

CREATE TABLE IF NOT EXISTS conversation_summaries (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL,
    round_start INTEGER NOT NULL DEFAULT 0,
    round_end INTEGER NOT NULL DEFAULT 0,
    summary_text TEXT NOT NULL DEFAULT '',
    parent_summary_id TEXT DEFAULT '',
    compressed_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_conv_summaries_conv_round ON conversation_summaries(conversation_id, round_start);
CREATE INDEX IF NOT EXISTS idx_conv_summaries_parent ON conversation_summaries(parent_summary_id);

CREATE TABLE IF NOT EXISTS message_feedback (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id TEXT DEFAULT '',
    rating INTEGER DEFAULT 0,
    comment TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS safety_events (
    id TEXT PRIMARY KEY,
    conversation_id TEXT DEFAULT '',
    event_type TEXT DEFAULT '',
    description TEXT DEFAULT '',
    direction TEXT DEFAULT '',
    handled INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS moods (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    character_id TEXT DEFAULT '',
    mood TEXT DEFAULT '',
    mood_value TEXT DEFAULT '',
    level INTEGER DEFAULT 50,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS need_states (
    id TEXT PRIMARY KEY,
    character_id TEXT NOT NULL DEFAULT '',
    need_key TEXT NOT NULL DEFAULT '',
    current_value REAL DEFAULT 0,
    baseline REAL DEFAULT 0,
    trend REAL DEFAULT 0,
    saturated INTEGER DEFAULT 0,
    updated_at TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS app_settings (
    key TEXT PRIMARY KEY,
    value TEXT DEFAULT '',
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_active_task_due ON active_message_task(due_time);
CREATE INDEX IF NOT EXISTS idx_active_task_status_due ON active_message_task(status, due_time);
CREATE INDEX IF NOT EXISTS idx_active_task_char ON active_message_task(character_id);
CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id);
CREATE INDEX IF NOT EXISTS idx_messages_conversation_sequence ON messages(conversation_id, sequence);
CREATE UNIQUE INDEX IF NOT EXISTS idx_messages_conv_sequence_unique ON messages(conversation_id, sequence);
CREATE INDEX IF NOT EXISTS idx_messages_conv_created ON messages(conversation_id, created_at);
CREATE INDEX IF NOT EXISTS idx_messages_conv_ctx ON messages(conversation_id, include_in_context);
CREATE INDEX IF NOT EXISTS idx_messages_conv_ctx_role_created ON messages(conversation_id, include_in_context, role, created_at);
CREATE INDEX IF NOT EXISTS idx_messages_request ON messages(conversation_id, role, request_id);
CREATE INDEX IF NOT EXISTS idx_pipeline_checkpoints_conversation ON pipeline_checkpoints(conversation_id);
CREATE INDEX IF NOT EXISTS idx_pipeline_checkpoints_updated ON pipeline_checkpoints(updated_at);
CREATE INDEX IF NOT EXISTS idx_conversations_character ON conversations(character_id);
CREATE INDEX IF NOT EXISTS idx_conversations_channel_peer ON conversations(channel, peer_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_conversations_channel_peer_unique ON conversations(channel, peer_id) WHERE peer_id <> '';
CREATE INDEX IF NOT EXISTS idx_conversations_character_channel_updated ON conversations(character_id, channel, updated_at);
CREATE INDEX IF NOT EXISTS idx_conversations_character_updated ON conversations(character_id, updated_at);

CREATE TABLE IF NOT EXISTS retrieval_logs (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL DEFAULT '',
    character_id TEXT NOT NULL DEFAULT '',
    request_id TEXT NOT NULL DEFAULT '',
    channel TEXT NOT NULL DEFAULT '',
    retrieval_version TEXT NOT NULL DEFAULT '',
    legacy INTEGER NOT NULL DEFAULT 0,
    query_text TEXT NOT NULL DEFAULT '',
    retrieved_memory_ids TEXT DEFAULT '[]',
    scoring_details TEXT DEFAULT '{}',
    created_at TEXT DEFAULT (datetime('now'))
);

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
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
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
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
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

CREATE TABLE IF NOT EXISTS desktop_pet_action_definitions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    action_key TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL DEFAULT '',
    description TEXT DEFAULT '',
    category_key TEXT NOT NULL DEFAULT '',
    category_name TEXT NOT NULL DEFAULT '',
    supports_default_idle INTEGER NOT NULL DEFAULT 0,
    recommended INTEGER NOT NULL DEFAULT 0,
    enabled INTEGER NOT NULL DEFAULT 1,
    sort_order INTEGER NOT NULL DEFAULT 0,
    definition_version INTEGER NOT NULL DEFAULT 1,
    default_frame_count INTEGER NOT NULL DEFAULT 8,
    estimated_generation_count INTEGER NOT NULL DEFAULT 1,
    source_type TEXT NOT NULL DEFAULT 'builtin',
    schema_version INTEGER NOT NULL DEFAULT 1,
    catalog_version INTEGER NOT NULL DEFAULT 1,
    default_fps INTEGER NOT NULL DEFAULT 10,
    playback_mode TEXT NOT NULL DEFAULT 'once',
    return_policy TEXT NOT NULL DEFAULT 'previous',
    return_action_key TEXT NOT NULL DEFAULT '',
    interruptible INTEGER NOT NULL DEFAULT 1,
    interrupt_after_ms INTEGER NOT NULL DEFAULT 0,
    minimum_play_ms INTEGER NOT NULL DEFAULT 0,
    maximum_play_ms INTEGER NOT NULL DEFAULT 0,
    priority INTEGER NOT NULL DEFAULT 0,
    cooldown_ms INTEGER NOT NULL DEFAULT 0,
    mutex_group TEXT NOT NULL DEFAULT '',
    queue_policy TEXT NOT NULL DEFAULT 'replace',
    dedup_window_ms INTEGER NOT NULL DEFAULT 0,
    anchor_profile TEXT NOT NULL DEFAULT 'feet_center',
    semantic_tags_json TEXT NOT NULL DEFAULT '[]',
    generation_spec_version INTEGER NOT NULL DEFAULT 1,
    spec_json TEXT NOT NULL DEFAULT '{}',
    spec_hash TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS desktop_pet_generation_tasks (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL DEFAULT '',
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
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
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

CREATE INDEX IF NOT EXISTS idx_dpgt_user ON desktop_pet_generation_tasks(user_id);
CREATE INDEX IF NOT EXISTS idx_dpgt_char ON desktop_pet_generation_tasks(character_id);
CREATE INDEX IF NOT EXISTS idx_dpgt_status ON desktop_pet_generation_tasks(status);
CREATE INDEX IF NOT EXISTS idx_dpgt_created ON desktop_pet_generation_tasks(created_at);

CREATE TABLE IF NOT EXISTS desktop_pet_generation_task_actions (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL DEFAULT '',
    action_definition_id INTEGER NOT NULL DEFAULT 0,
    action_key TEXT NOT NULL DEFAULT '',
    action_name_snapshot TEXT DEFAULT '',
    action_description_snapshot TEXT DEFAULT '',
    category_key_snapshot TEXT DEFAULT '',
    category_name_snapshot TEXT DEFAULT '',
    definition_version INTEGER NOT NULL DEFAULT 1,
    supports_default_idle INTEGER NOT NULL DEFAULT 0,
    sort_order INTEGER NOT NULL DEFAULT 0,
    frame_count INTEGER NOT NULL DEFAULT 8,
    estimated_generation_count INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL DEFAULT 'pending',
    progress INTEGER NOT NULL DEFAULT 0,
    error_code TEXT DEFAULT '',
    error_message TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    started_at TEXT DEFAULT '',
    completed_at TEXT DEFAULT '',
    attempt_number INTEGER NOT NULL DEFAULT 1,
    generation_spec_version TEXT DEFAULT '',
    current_attempt INTEGER NOT NULL DEFAULT 1,
    row_version INTEGER NOT NULL DEFAULT 0,
    current_stage TEXT NOT NULL DEFAULT 'created',
    status_reason TEXT NOT NULL DEFAULT '',
    failure_stage TEXT NOT NULL DEFAULT '',
    last_transition_at TEXT NOT NULL DEFAULT '',
    execution_id TEXT NOT NULL DEFAULT '',
    worker_id TEXT NOT NULL DEFAULT '',
    action_spec_schema_version INTEGER NOT NULL DEFAULT 1,
    action_spec_version INTEGER NOT NULL DEFAULT 1,
    action_spec_json TEXT NOT NULL DEFAULT '',
    action_spec_hash TEXT NOT NULL DEFAULT '',
    playback_mode_snapshot TEXT NOT NULL DEFAULT '',
    default_fps_snapshot INTEGER NOT NULL DEFAULT 0,
    return_policy_snapshot TEXT NOT NULL DEFAULT '',
    return_action_key_snapshot TEXT NOT NULL DEFAULT '',
    interruptible_snapshot INTEGER NOT NULL DEFAULT 1,
    priority_snapshot INTEGER NOT NULL DEFAULT 0,
    cooldown_ms_snapshot INTEGER NOT NULL DEFAULT 0,
    mutex_group_snapshot TEXT NOT NULL DEFAULT '',
    anchor_profile_snapshot TEXT NOT NULL DEFAULT '',
    generation_mode TEXT NOT NULL DEFAULT 'legacy_frame',
    generation_plan_json TEXT NOT NULL DEFAULT '{}',
    generation_plan_hash TEXT NOT NULL DEFAULT '',
    prompt_template_version TEXT NOT NULL DEFAULT '',
    active_attempt_id TEXT NOT NULL DEFAULT '',
    active_attempt_number INTEGER NOT NULL DEFAULT 0,
    planned_segment_count INTEGER NOT NULL DEFAULT 0,
    planned_primary_request_count INTEGER NOT NULL DEFAULT 0,
    planned_max_provider_call_count INTEGER NOT NULL DEFAULT 0,
    next_attempt_number INTEGER NOT NULL DEFAULT 1,
    actual_submission_count INTEGER NOT NULL DEFAULT 0,
    actual_provider_job_count INTEGER NOT NULL DEFAULT 0,
    actual_success_count INTEGER NOT NULL DEFAULT 0,
    actual_failed_count INTEGER NOT NULL DEFAULT 0,
    actual_input_units INTEGER NOT NULL DEFAULT 0,
    actual_output_units INTEGER NOT NULL DEFAULT 0,
    estimated_cost REAL NOT NULL DEFAULT 0,
    actual_cost REAL NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'CNY',
    pricing_version TEXT NOT NULL DEFAULT '',
    planned_call_count INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_dpgta_task ON desktop_pet_generation_task_actions(task_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dpgta_task_action ON desktop_pet_generation_task_actions(task_id, action_key);

CREATE TABLE IF NOT EXISTS desktop_pet_generation_frames (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL DEFAULT '',
    task_action_id TEXT NOT NULL DEFAULT '',
    execution_id TEXT DEFAULT '',
    frame_index INTEGER NOT NULL DEFAULT 0,
    frame_phase TEXT DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    attempt_number INTEGER NOT NULL DEFAULT 1,
    generation_attempt INTEGER NOT NULL DEFAULT 0,
    provider_attempt INTEGER NOT NULL DEFAULT 0,
    prompt_snapshot TEXT DEFAULT '',
    negative_prompt_snapshot TEXT DEFAULT '',
    provider TEXT DEFAULT '',
    model TEXT DEFAULT '',
    provider_request_id TEXT DEFAULT '',
    provider_operation_id TEXT DEFAULT '',
    source_image_path TEXT DEFAULT '',
    previous_frame_path TEXT DEFAULT '',
    result_image_path TEXT DEFAULT '',
    result_mime_type TEXT DEFAULT '',
    result_width INTEGER DEFAULT 0,
    result_height INTEGER DEFAULT 0,
    result_size INTEGER DEFAULT 0,
    result_hash TEXT DEFAULT '',
    provider_seed TEXT DEFAULT '',
    error_code TEXT DEFAULT '',
    error_message TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    started_at TEXT DEFAULT '',
    completed_at TEXT DEFAULT '',
    row_version INTEGER NOT NULL DEFAULT 0,
    current_stage TEXT NOT NULL DEFAULT 'created',
    status_reason TEXT NOT NULL DEFAULT '',
    failure_stage TEXT NOT NULL DEFAULT '',
    last_transition_at TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_dpgf_task ON desktop_pet_generation_frames(task_id);
CREATE INDEX IF NOT EXISTS idx_dpgf_action ON desktop_pet_generation_frames(task_action_id);
CREATE INDEX IF NOT EXISTS idx_dpgf_exec ON desktop_pet_generation_frames(execution_id);
CREATE INDEX IF NOT EXISTS idx_dpgf_status ON desktop_pet_generation_frames(status);
ALTER TABLE desktop_pet_generation_frames ADD COLUMN generation_attempt INTEGER NOT NULL DEFAULT 0;
ALTER TABLE desktop_pet_generation_frames ADD COLUMN provider_attempt INTEGER NOT NULL DEFAULT 0;
CREATE UNIQUE INDEX IF NOT EXISTS uq_dpgf_action_gen_frame ON desktop_pet_generation_frames(task_action_id, generation_attempt, frame_index);

CREATE TABLE IF NOT EXISTS desktop_pet_generation_call_logs (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL DEFAULT '',
    task_action_id TEXT DEFAULT '',
    frame_id TEXT DEFAULT '',
    execution_id TEXT DEFAULT '',
    provider TEXT DEFAULT '',
    model TEXT DEFAULT '',
    provider_request_id TEXT DEFAULT '',
    request_started_at TEXT DEFAULT '',
    request_completed_at TEXT DEFAULT '',
    request_status TEXT DEFAULT '',
    attempt_number INTEGER NOT NULL DEFAULT 0,
    usage TEXT DEFAULT '',
    error_code TEXT DEFAULT '',
    error_message TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    attempt_id TEXT NOT NULL DEFAULT '',
    artifact_id TEXT NOT NULL DEFAULT '',
    call_type TEXT NOT NULL DEFAULT 'primary',
    call_attempt_index INTEGER NOT NULL DEFAULT 0,
    idempotency_key_hash TEXT NOT NULL DEFAULT '',
    request_hash TEXT NOT NULL DEFAULT '',
    submission_state TEXT NOT NULL DEFAULT '',
    retry_class TEXT NOT NULL DEFAULT '',
    http_status INTEGER NOT NULL DEFAULT 0,
    usage_json TEXT NOT NULL DEFAULT '{}',
    estimated_cost_json TEXT NOT NULL DEFAULT '{}',
    actual_cost_json TEXT NOT NULL DEFAULT '{}',
    response_receipt_json TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_dpgcl_task ON desktop_pet_generation_call_logs(task_id);
CREATE INDEX IF NOT EXISTS idx_dpgcl_exec ON desktop_pet_generation_call_logs(execution_id);

CREATE TABLE IF NOT EXISTS desktop_pet_processing_tasks (
    id TEXT PRIMARY KEY,
    generation_task_id TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
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
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
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

CREATE TABLE IF NOT EXISTS desktop_pet_processing_actions (
    id TEXT PRIMARY KEY,
    processing_task_id TEXT NOT NULL DEFAULT '',
    generation_task_action_id TEXT NOT NULL DEFAULT '',
    action_key TEXT NOT NULL DEFAULT '',
    action_name_snapshot TEXT DEFAULT '',
    source_attempt_number INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL DEFAULT 'pending',
    progress INTEGER NOT NULL DEFAULT 0,
    source_frame_count INTEGER NOT NULL DEFAULT 0,
    processed_frame_count INTEGER NOT NULL DEFAULT 0,
    loop_type TEXT DEFAULT 'once',
    fps INTEGER NOT NULL DEFAULT 10,
    frame_duration_ms INTEGER NOT NULL DEFAULT 100,
    anchor_type TEXT DEFAULT 'feet_center',
    anchor_x REAL NOT NULL DEFAULT 0.5,
    anchor_y REAL NOT NULL DEFAULT 0.92,
    bounding_box TEXT DEFAULT '',
    excluded INTEGER NOT NULL DEFAULT 0,
    processing_attempt INTEGER NOT NULL DEFAULT 1,
    last_successful_attempt INTEGER NOT NULL DEFAULT 0,
    active_execution_id TEXT NOT NULL DEFAULT '',
    row_version INTEGER NOT NULL DEFAULT 0,
    error_code TEXT DEFAULT '',
    error_message TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    started_at TEXT DEFAULT '',
    completed_at TEXT DEFAULT '',
    current_stage TEXT NOT NULL DEFAULT 'created',
    status_reason TEXT NOT NULL DEFAULT '',
    failure_stage TEXT NOT NULL DEFAULT '',
    last_transition_at TEXT NOT NULL DEFAULT '',
    execution_id TEXT NOT NULL DEFAULT '',
    worker_id TEXT NOT NULL DEFAULT '',
    attempt_number INTEGER NOT NULL DEFAULT 1,
    action_spec_schema_version INTEGER NOT NULL DEFAULT 1,
    action_spec_version INTEGER NOT NULL DEFAULT 1,
    action_spec_hash TEXT NOT NULL DEFAULT '',
    return_policy TEXT NOT NULL DEFAULT '',
    return_action_key TEXT NOT NULL DEFAULT '',
    interruptible INTEGER NOT NULL DEFAULT 1,
    interrupt_after_ms INTEGER NOT NULL DEFAULT 0,
    minimum_play_ms INTEGER NOT NULL DEFAULT 0,
    maximum_play_ms INTEGER NOT NULL DEFAULT 0,
    priority INTEGER NOT NULL DEFAULT 0,
    cooldown_ms INTEGER NOT NULL DEFAULT 0,
    mutex_group TEXT NOT NULL DEFAULT '',
    queue_policy TEXT NOT NULL DEFAULT 'replace',
    dedup_window_ms INTEGER NOT NULL DEFAULT 0,
    anchor_profile TEXT NOT NULL DEFAULT 'feet_center',
    playback_mode TEXT NOT NULL DEFAULT '',
    active_revision_id TEXT NOT NULL DEFAULT '',
    next_revision_number INTEGER NOT NULL DEFAULT 1,
    source_attempt_id TEXT NOT NULL DEFAULT '',
    source_candidate_index INTEGER NOT NULL DEFAULT 0,
    processing_profile_snapshot TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_dppa_task ON desktop_pet_processing_actions(processing_task_id);
CREATE INDEX IF NOT EXISTS idx_dppa_action ON desktop_pet_processing_actions(action_key);
CREATE INDEX IF NOT EXISTS idx_dppa_status ON desktop_pet_processing_actions(status);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dppa_task_action ON desktop_pet_processing_actions(processing_task_id, action_key);

CREATE TABLE IF NOT EXISTS desktop_pet_processing_action_attempts (
    id TEXT PRIMARY KEY,
    processing_action_id TEXT NOT NULL DEFAULT '',
    processing_task_id TEXT NOT NULL DEFAULT '',
    action_key TEXT NOT NULL DEFAULT '',
    attempt_number INTEGER NOT NULL DEFAULT 1,
    source_generation_attempt INTEGER NOT NULL DEFAULT 1,
    source_generation_attempt_id TEXT NOT NULL DEFAULT '',
    source_generation_artifact_id TEXT NOT NULL DEFAULT '',
    source_manifest_id TEXT NOT NULL DEFAULT '',
    source_artifact_content_hash TEXT NOT NULL DEFAULT '',
    execution_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    progress INTEGER NOT NULL DEFAULT 0,
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    started_at TEXT NOT NULL DEFAULT '',
    completed_at TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_dppaa_task ON desktop_pet_processing_action_attempts(processing_task_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dppaa_action_attempt ON desktop_pet_processing_action_attempts(processing_action_id, attempt_number);

CREATE TABLE IF NOT EXISTS desktop_pet_processed_frames (
    id TEXT PRIMARY KEY,
    processing_action_id TEXT NOT NULL DEFAULT '',
    source_frame_id TEXT NOT NULL DEFAULT '',
    frame_index INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',
    source_path TEXT DEFAULT '',
    processed_path TEXT DEFAULT '',
    width INTEGER DEFAULT 0,
    height INTEGER DEFAULT 0,
    content_hash TEXT DEFAULT '',
    subject_box TEXT DEFAULT '',
    anchor_x REAL DEFAULT 0,
    anchor_y REAL DEFAULT 0,
    alpha_coverage REAL DEFAULT 0,
    quality_flags TEXT DEFAULT '',
    processing_attempt_id TEXT NOT NULL DEFAULT '',
    processing_attempt_number INTEGER NOT NULL DEFAULT 1,
    execution_id TEXT NOT NULL DEFAULT '',
    error_code TEXT DEFAULT '',
    error_message TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    row_version INTEGER NOT NULL DEFAULT 0,
    current_stage TEXT NOT NULL DEFAULT 'created',
    status_reason TEXT NOT NULL DEFAULT '',
    failure_stage TEXT NOT NULL DEFAULT '',
    last_transition_at TEXT NOT NULL DEFAULT '',
    started_at TEXT NOT NULL DEFAULT '',
    completed_at TEXT NOT NULL DEFAULT '',
    revision_id TEXT NOT NULL DEFAULT '',
    mask_path TEXT NOT NULL DEFAULT '',
    transform_chain_id TEXT NOT NULL DEFAULT '',
    measurement_id TEXT NOT NULL DEFAULT '',
    source_artifact_id TEXT NOT NULL DEFAULT '',
    source_cell_index INTEGER
);

CREATE INDEX IF NOT EXISTS idx_dppf_action ON desktop_pet_processed_frames(processing_action_id);
CREATE INDEX IF NOT EXISTS idx_dppf_index ON desktop_pet_processed_frames(frame_index);
CREATE INDEX IF NOT EXISTS idx_dppf_status ON desktop_pet_processed_frames(status);
ALTER TABLE desktop_pet_processed_frames ADD COLUMN processing_attempt_id TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processed_frames ADD COLUMN processing_attempt_number INTEGER NOT NULL DEFAULT 1;
CREATE UNIQUE INDEX IF NOT EXISTS uq_dppf_attempt_frame ON desktop_pet_processed_frames(processing_attempt_id, frame_index);

CREATE TABLE IF NOT EXISTS desktop_pet_packages (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL DEFAULT '',
    character_id TEXT NOT NULL DEFAULT '',
    generation_task_id TEXT NOT NULL DEFAULT '',
    processing_task_id TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL DEFAULT '',
    version INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL DEFAULT 'draft',
    default_action_key TEXT NOT NULL DEFAULT '',
    canvas_width INTEGER NOT NULL DEFAULT 512,
    canvas_height INTEGER NOT NULL DEFAULT 512,
    package_path TEXT DEFAULT '',
    manifest_path TEXT DEFAULT '',
    preview_path TEXT DEFAULT '',
    action_count INTEGER NOT NULL DEFAULT 0,
    package_hash TEXT DEFAULT '',
    included_actions TEXT DEFAULT '[]',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    row_version INTEGER NOT NULL DEFAULT 0,
    current_stage TEXT NOT NULL DEFAULT 'created',
    status_reason TEXT NOT NULL DEFAULT '',
    failure_stage TEXT NOT NULL DEFAULT '',
    last_transition_at TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    migrated_release_id TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_dppkg_user ON desktop_pet_packages(user_id);
CREATE INDEX IF NOT EXISTS idx_dppkg_gen ON desktop_pet_packages(generation_task_id);
CREATE INDEX IF NOT EXISTS idx_dppkg_proc ON desktop_pet_packages(processing_task_id);
CREATE INDEX IF NOT EXISTS idx_dppkg_status ON desktop_pet_packages(status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_dppkg_proc_version ON desktop_pet_packages(processing_task_id, version);

CREATE TABLE IF NOT EXISTS desktop_pet_installations (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL DEFAULT '',
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
    installed_at TEXT DEFAULT (datetime('now')),
    last_enabled_at TEXT DEFAULT '',
    last_disabled_at TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
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
    legacy_package_id TEXT NOT NULL DEFAULT '',
    device_id TEXT NOT NULL DEFAULT '',
    preview_artifact_path TEXT NOT NULL DEFAULT '',
    default_action_release_id TEXT NOT NULL DEFAULT '',
    installed_content_hash TEXT NOT NULL DEFAULT '',
    integrity_status TEXT NOT NULL DEFAULT 'verified'
);

CREATE INDEX IF NOT EXISTS idx_dpinst_user ON desktop_pet_installations(user_id);
CREATE INDEX IF NOT EXISTS idx_dpinst_character ON desktop_pet_installations(character_id);
CREATE INDEX IF NOT EXISTS idx_dpinst_package ON desktop_pet_installations(package_id);
CREATE INDEX IF NOT EXISTS idx_dpinst_status ON desktop_pet_installations(status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_dpinst_package_version ON desktop_pet_installations(package_id, package_version);
CREATE INDEX IF NOT EXISTS idx_dpinst_pet ON desktop_pet_installations(pet_id);
CREATE INDEX IF NOT EXISTS idx_dpinst_release ON desktop_pet_installations(current_release_id);
CREATE INDEX IF NOT EXISTS idx_dpinst_lifecycle ON desktop_pet_installations(lifecycle_state);
CREATE INDEX IF NOT EXISTS idx_dpinst_desired ON desktop_pet_installations(desired_state);
CREATE INDEX IF NOT EXISTS idx_dpinst_device ON desktop_pet_installations(device_id);
CREATE INDEX IF NOT EXISTS idx_dpinst_user_device_pet ON desktop_pet_installations(user_id, device_id, pet_id);

CREATE TABLE IF NOT EXISTS desktop_pet_runtime_settings (
    id TEXT PRIMARY KEY,
    installation_id TEXT NOT NULL DEFAULT '',
    always_on_top INTEGER NOT NULL DEFAULT 1,
    launch_on_startup INTEGER NOT NULL DEFAULT 0,
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
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_dprts_installation ON desktop_pet_runtime_settings(installation_id);

CREATE TABLE IF NOT EXISTS desktop_pet_runtime_clients (
  runtime_id TEXT PRIMARY KEY,
  device_id TEXT NOT NULL DEFAULT '',
  user_id TEXT NOT NULL DEFAULT '',
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
  created_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS desktop_pet_runtime_commands (
  id TEXT PRIMARY KEY,
  runtime_id TEXT NOT NULL DEFAULT '',
  user_id TEXT NOT NULL DEFAULT '',
  installation_id TEXT NOT NULL DEFAULT '',
  pet_instance_id TEXT NOT NULL DEFAULT '',
  name TEXT NOT NULL DEFAULT '',
  payload_json TEXT NOT NULL DEFAULT '{}',
  durability TEXT NOT NULL DEFAULT 'durable',
  coalesce_key TEXT NOT NULL DEFAULT '',
  idempotency_key TEXT NOT NULL DEFAULT '',
  desired_revision INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'pending',
  attempt_count INTEGER NOT NULL DEFAULT 0,
  max_attempts INTEGER NOT NULL DEFAULT 5,
  next_attempt_at TEXT NOT NULL DEFAULT '',
  deadline_at TEXT NOT NULL DEFAULT '',
  last_session_id TEXT NOT NULL DEFAULT '',
  last_error_code TEXT NOT NULL DEFAULT '',
  last_error_message TEXT NOT NULL DEFAULT '',
  result_json TEXT NOT NULL DEFAULT '',
  created_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now')),
  completed_at TEXT NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_dprcmd_idempotency ON desktop_pet_runtime_commands(runtime_id, idempotency_key);
CREATE INDEX IF NOT EXISTS idx_dprcmd_dispatch ON desktop_pet_runtime_commands(runtime_id, status, next_attempt_at);
CREATE INDEX IF NOT EXISTS idx_dprcmd_coalesce ON desktop_pet_runtime_commands(runtime_id, coalesce_key, desired_revision);

CREATE TABLE IF NOT EXISTS desktop_pet_runtime_actual_states (
  runtime_id TEXT NOT NULL,
  installation_id TEXT NOT NULL DEFAULT '',
  pet_instance_id TEXT NOT NULL DEFAULT '',
  session_id TEXT NOT NULL DEFAULT '',
  desired_revision INTEGER NOT NULL DEFAULT 0,
  applied_settings_revision INTEGER NOT NULL DEFAULT 0,
  visible INTEGER NOT NULL DEFAULT 0,
  current_action_key TEXT NOT NULL DEFAULT '',
  position_x INTEGER NOT NULL DEFAULT 0,
  position_y INTEGER NOT NULL DEFAULT 0,
  screen_id TEXT NOT NULL DEFAULT '',
  scale REAL NOT NULL DEFAULT 1.0,
  health TEXT NOT NULL DEFAULT 'unknown',
  state_json TEXT NOT NULL DEFAULT '{}',
  observed_at TEXT NOT NULL DEFAULT '',
  updated_at TEXT DEFAULT (datetime('now')),
  PRIMARY KEY(runtime_id, installation_id)
);

CREATE TABLE IF NOT EXISTS desktop_pet_behavior_contexts (
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
    desired_state_json TEXT NOT NULL DEFAULT '{}',
    last_source_revisions_json TEXT NOT NULL DEFAULT '{}',
    updated_at TEXT NOT NULL DEFAULT '',
    PRIMARY KEY(user_id, character_id)
);

CREATE TABLE IF NOT EXISTS desktop_pet_behavior_inbox (
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
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_desktop_pet_behavior_inbox_dedup ON desktop_pet_behavior_inbox(dedup_key);
CREATE INDEX IF NOT EXISTS idx_behavior_inbox_char ON desktop_pet_behavior_inbox(character_id, status);

CREATE TABLE IF NOT EXISTS desktop_pet_behavior_decisions (
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
    semantic TEXT NOT NULL DEFAULT '',
    action_key TEXT NOT NULL DEFAULT '',
    priority INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'selected',
    reason_code TEXT NOT NULL DEFAULT '',
    rejected_candidates_json TEXT NOT NULL DEFAULT '[]',
    runtime_command_id TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    started_at TEXT NOT NULL DEFAULT '',
    completed_at TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_behavior_decisions_char ON desktop_pet_behavior_decisions(character_id, created_at);

CREATE TABLE IF NOT EXISTS desktop_pet_behavior_cooldowns (
    user_id TEXT NOT NULL DEFAULT '',
    character_id TEXT NOT NULL DEFAULT '',
    cooldown_key TEXT NOT NULL DEFAULT '',
    until_at TEXT NOT NULL DEFAULT '',
    source_decision_id TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT '',
    PRIMARY KEY(user_id, character_id, cooldown_key)
);

CREATE TABLE IF NOT EXISTS desktop_pet_behavior_bindings (
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
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_behavior_bindings_user_char ON desktop_pet_behavior_bindings(user_id, character_id);

CREATE TABLE IF NOT EXISTS desktop_pet_identities (
    id TEXT PRIMARY KEY,
    owner_user_id TEXT NOT NULL DEFAULT '',
    source_character_id TEXT NOT NULL DEFAULT '',
    name TEXT NOT NULL DEFAULT '',
    slug TEXT NOT NULL DEFAULT '',
    binding_policy TEXT NOT NULL DEFAULT 'character_locked',
    upstream_pet_id TEXT NOT NULL DEFAULT '',
    next_release_sequence INTEGER NOT NULL DEFAULT 0,
    default_action_key TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_dpident_owner ON desktop_pet_identities(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_dpident_character ON desktop_pet_identities(source_character_id);
CREATE INDEX IF NOT EXISTS idx_dpident_slug ON desktop_pet_identities(slug);

CREATE TABLE IF NOT EXISTS desktop_pet_package_releases (
    id TEXT PRIMARY KEY,
    pet_id TEXT NOT NULL DEFAULT '',
    owner_user_id TEXT NOT NULL DEFAULT '',
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
    legacy_package_id TEXT NOT NULL DEFAULT '',
    legacy_version INTEGER NOT NULL DEFAULT 0,
    active_revision_set_hash TEXT NOT NULL DEFAULT '',
    quality_gate_id TEXT NOT NULL DEFAULT '',
    quality_gate_hash TEXT NOT NULL DEFAULT '',
    build_snapshot_id TEXT NOT NULL DEFAULT '',
    integrity_status TEXT NOT NULL DEFAULT 'unknown',
    compatibility_status TEXT NOT NULL DEFAULT 'unknown',
    lifecycle TEXT NOT NULL DEFAULT 'building',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    UNIQUE(pet_id, version)
);
CREATE INDEX IF NOT EXISTS idx_dprel_pet ON desktop_pet_package_releases(pet_id);
CREATE INDEX IF NOT EXISTS idx_dprel_owner ON desktop_pet_package_releases(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_dprel_status ON desktop_pet_package_releases(status);
CREATE INDEX IF NOT EXISTS idx_dprel_content_hash ON desktop_pet_package_releases(content_root_hash);
CREATE INDEX IF NOT EXISTS idx_dprel_legacy ON desktop_pet_package_releases(legacy_package_id);
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
    created_at TEXT DEFAULT (datetime('now')),
    UNIQUE(release_id, path)
);
CREATE INDEX IF NOT EXISTS idx_dprf_release ON desktop_pet_release_files(release_id);

CREATE TABLE IF NOT EXISTS desktop_pet_package_operations (
    id TEXT PRIMARY KEY,
    operation_type TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
    pet_id TEXT NOT NULL DEFAULT '',
    release_id TEXT NOT NULL DEFAULT '',
    idempotency_key TEXT NOT NULL DEFAULT '',
    stage TEXT NOT NULL DEFAULT 'prepare',
    status TEXT NOT NULL DEFAULT 'pending',
    input_hash TEXT NOT NULL DEFAULT '',
    staging_path_key TEXT NOT NULL DEFAULT '',
    published_path_key TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    retry_count INTEGER NOT NULL DEFAULT 0,
    result_json TEXT NOT NULL DEFAULT '{}',
    lease_owner TEXT NOT NULL DEFAULT '',
    lease_expires_at TEXT NOT NULL DEFAULT '',
    heartbeat_at TEXT NOT NULL DEFAULT '',
    snapshot_id TEXT NOT NULL DEFAULT '',
    started_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    completed_at TEXT NOT NULL DEFAULT '',
    UNIQUE(user_id, idempotency_key, operation_type)
);
CREATE INDEX IF NOT EXISTS idx_dppkgop_user ON desktop_pet_package_operations(user_id);
CREATE INDEX IF NOT EXISTS idx_dppkgop_status ON desktop_pet_package_operations(status);
CREATE INDEX IF NOT EXISTS idx_dppkgop_release ON desktop_pet_package_operations(release_id);

CREATE TABLE IF NOT EXISTS desktop_pet_release_build_snapshots (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL DEFAULT '',
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
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_drbs_pet ON desktop_pet_release_build_snapshots(pet_id);
CREATE INDEX IF NOT EXISTS idx_drbs_task ON desktop_pet_release_build_snapshots(processing_task_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_drbs_input_hash ON desktop_pet_release_build_snapshots(input_hash);
CREATE UNIQUE INDEX IF NOT EXISTS uq_drbs_snapshot_hash ON desktop_pet_release_build_snapshots(snapshot_hash);

CREATE TABLE IF NOT EXISTS desktop_pet_release_build_operations (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL DEFAULT '',
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
    started_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    completed_at TEXT NOT NULL DEFAULT '',
    UNIQUE(user_id, idempotency_key)
);
CREATE INDEX IF NOT EXISTS idx_drbo_user ON desktop_pet_release_build_operations(user_id);
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
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_drpj_operation ON desktop_pet_release_publish_journals(operation_id);
CREATE INDEX IF NOT EXISTS idx_drpj_release ON desktop_pet_release_publish_journals(release_id);
CREATE INDEX IF NOT EXISTS idx_drpj_stage ON desktop_pet_release_publish_journals(stage);
CREATE INDEX IF NOT EXISTS idx_drpj_kind_stage ON desktop_pet_release_publish_journals(operation_kind, stage);

CREATE TABLE IF NOT EXISTS desktop_pet_legacy_package_mappings (
    id TEXT PRIMARY KEY,
    legacy_package_id TEXT NOT NULL DEFAULT '',
    migrated_pet_id TEXT NOT NULL DEFAULT '',
    migrated_release_id TEXT NOT NULL DEFAULT '',
    migration_status TEXT NOT NULL DEFAULT 'pending',
    source_content_hash TEXT NOT NULL DEFAULT '',
    owner_user_id TEXT NOT NULL DEFAULT '',
    source_manifest_hash TEXT NOT NULL DEFAULT '',
    migration_operation_id TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(legacy_package_id)
);
CREATE INDEX IF NOT EXISTS idx_dlpm_status ON desktop_pet_legacy_package_mappings(migration_status);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dlpm_owner_legacy ON desktop_pet_legacy_package_mappings(owner_user_id, legacy_package_id);

CREATE TABLE IF NOT EXISTS desktop_pet_legacy_package_migration_operations (
    id TEXT PRIMARY KEY,
    legacy_package_id TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL DEFAULT 'pending',
    staging_path TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    started_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    completed_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_dlpmo_legacy ON desktop_pet_legacy_package_migration_operations(legacy_package_id);
CREATE INDEX IF NOT EXISTS idx_dlpmo_state ON desktop_pet_legacy_package_migration_operations(state);

CREATE TABLE IF NOT EXISTS desktop_pet_installation_operations (
    id TEXT PRIMARY KEY,
    operation_type TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
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
    started_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    completed_at TEXT NOT NULL DEFAULT '',
    UNIQUE(user_id, idempotency_key, operation_type)
);
CREATE INDEX IF NOT EXISTS idx_dpinstop_user ON desktop_pet_installation_operations(user_id);
CREATE INDEX IF NOT EXISTS idx_dpinstop_installation ON desktop_pet_installation_operations(installation_id);
CREATE INDEX IF NOT EXISTS idx_dpinstop_status ON desktop_pet_installation_operations(status);
CREATE INDEX IF NOT EXISTS idx_dpinstop_device ON desktop_pet_installation_operations(device_id);
CREATE INDEX IF NOT EXISTS idx_dpinstop_lease ON desktop_pet_installation_operations(lease_owner, lease_expires_at);

CREATE TABLE IF NOT EXISTS desktop_pet_active_bindings (
    user_id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL DEFAULT '',
    installation_id TEXT NOT NULL DEFAULT '',
    pet_id TEXT NOT NULL DEFAULT '',
    release_id TEXT NOT NULL DEFAULT '',
    binding_revision INTEGER NOT NULL DEFAULT 0,
    bound_reason TEXT NOT NULL DEFAULT 'install',
    bound_at TEXT NOT NULL DEFAULT '',
    desired_state TEXT NOT NULL DEFAULT 'disabled',
    runtime_sync_state TEXT NOT NULL DEFAULT 'pending',
    desired_updated_at TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_dpab_installation ON desktop_pet_active_bindings(installation_id);
CREATE INDEX IF NOT EXISTS idx_dpab_user_device ON desktop_pet_active_bindings(user_id, device_id);

CREATE TABLE IF NOT EXISTS desktop_pet_installation_release_history (
    id TEXT PRIMARY KEY,
    installation_id TEXT NOT NULL DEFAULT '',
    release_id TEXT NOT NULL DEFAULT '',
    pet_id TEXT NOT NULL DEFAULT '',
    version TEXT NOT NULL DEFAULT '',
    activated_at TEXT NOT NULL DEFAULT '',
    deactivated_at TEXT NOT NULL DEFAULT '',
    deactivation_reason TEXT NOT NULL DEFAULT '',
    is_current INTEGER NOT NULL DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_dpirh_installation ON desktop_pet_installation_release_history(installation_id);
CREATE INDEX IF NOT EXISTS idx_dpirh_current ON desktop_pet_installation_release_history(installation_id, is_current);

CREATE TABLE IF NOT EXISTS desktop_pet_package_validation_reports (
    id TEXT PRIMARY KEY,
    release_id TEXT NOT NULL DEFAULT '',
    operation_id TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'build',
    verdict TEXT NOT NULL DEFAULT 'pending',
    findings_json TEXT NOT NULL DEFAULT '[]',
    file_count INTEGER NOT NULL DEFAULT 0,
    error_count INTEGER NOT NULL DEFAULT 0,
    warning_count INTEGER NOT NULL DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_dpvr_release ON desktop_pet_package_validation_reports(release_id);
CREATE INDEX IF NOT EXISTS idx_dpvr_operation ON desktop_pet_package_validation_reports(operation_id);

CREATE TABLE IF NOT EXISTS desktop_pet_action_revisions (
  id TEXT PRIMARY KEY,
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
  created_by_user_id TEXT NOT NULL DEFAULT '',
  created_from_session_id TEXT NOT NULL DEFAULT '',
  change_summary TEXT NOT NULL DEFAULT '',
  source_summary_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
  ready_at TEXT NOT NULL DEFAULT '',
  user_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
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
CREATE UNIQUE INDEX IF NOT EXISTS uq_dpar_source_type ON desktop_pet_action_revisions(source_processing_revision_id, source_type);
CREATE INDEX IF NOT EXISTS idx_dpar_stream_rev ON desktop_pet_action_revisions(action_stream_id, revision_number);

CREATE TABLE IF NOT EXISTS desktop_pet_action_active_revisions (
  processing_task_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  revision_id TEXT NOT NULL DEFAULT '',
  binding_version INTEGER NOT NULL DEFAULT 0,
  activated_by TEXT NOT NULL DEFAULT '',
  reason TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
  user_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
  active_action_revision_id TEXT NOT NULL DEFAULT '',
  binding_revision INTEGER NOT NULL DEFAULT 0,
  bound_reason TEXT NOT NULL DEFAULT '',
  bound_by TEXT NOT NULL DEFAULT '',
  bound_at TEXT NOT NULL DEFAULT '',
  PRIMARY KEY(processing_task_id, action_key)
);

CREATE TABLE IF NOT EXISTS desktop_pet_frame_assets (
  id TEXT PRIMARY KEY,
  content_hash TEXT NOT NULL DEFAULT '',
  storage_path TEXT NOT NULL DEFAULT '',
  mime_type TEXT NOT NULL DEFAULT '',
  width INTEGER NOT NULL DEFAULT 0,
  height INTEGER NOT NULL DEFAULT 0,
  byte_size INTEGER NOT NULL DEFAULT 0,
  alpha_mode TEXT NOT NULL DEFAULT '',
  color_space TEXT NOT NULL DEFAULT '',
  source_type TEXT NOT NULL DEFAULT '',
  source_ref_id TEXT NOT NULL DEFAULT '',
  original_hash TEXT NOT NULL DEFAULT '',
  created_by TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'staging',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  user_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
  storage_key TEXT NOT NULL DEFAULT '',
  source_processing_revision_id TEXT NOT NULL DEFAULT '',
  source_processing_artifact_id TEXT NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_dfa_hash ON desktop_pet_frame_assets(content_hash, mime_type);
CREATE INDEX IF NOT EXISTS idx_dfa_source ON desktop_pet_frame_assets(source_type, source_ref_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dfa_source_artifact ON desktop_pet_frame_assets(source_type, source_processing_artifact_id);

CREATE TABLE IF NOT EXISTS desktop_pet_action_revision_frames (
  id TEXT PRIMARY KEY,
  revision_id TEXT NOT NULL DEFAULT '',
  frame_id TEXT NOT NULL DEFAULT '',
  asset_id TEXT NOT NULL DEFAULT '',
  logical_index INTEGER NOT NULL DEFAULT 0,
  duration_ms INTEGER NOT NULL DEFAULT 100,
  source_frame_id TEXT NOT NULL DEFAULT '',
  source_revision_id TEXT NOT NULL DEFAULT '',
  source_attempt_id TEXT NOT NULL DEFAULT '',
  anchor_x REAL NOT NULL DEFAULT 0.5,
  anchor_y REAL NOT NULL DEFAULT 0.9,
  anchor_space TEXT NOT NULL DEFAULT 'normalized_canvas',
  offset_x REAL NOT NULL DEFAULT 0,
  offset_y REAL NOT NULL DEFAULT 0,
  mask_asset_id TEXT NOT NULL DEFAULT '',
  transform_json TEXT NOT NULL DEFAULT '{}',
  metadata_json TEXT NOT NULL DEFAULT '{}',
  copied_from_frame_id TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  source_processing_frame_artifact_id TEXT NOT NULL DEFAULT '',
  source_processing_revision_id TEXT NOT NULL DEFAULT '',
  source_processing_attempt_id TEXT NOT NULL DEFAULT '',
  transform_hash TEXT NOT NULL DEFAULT '',
  measurement_hash TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_dparf_revision ON desktop_pet_action_revision_frames(revision_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dparf_rev_index ON desktop_pet_action_revision_frames(revision_id, logical_index);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dparf_rev_frame_id ON desktop_pet_action_revision_frames(revision_id, frame_id);

CREATE TABLE IF NOT EXISTS desktop_pet_edit_sessions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL DEFAULT '',
  processing_task_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  base_revision_id TEXT NOT NULL DEFAULT '',
  session_version INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'open',
  cursor INTEGER NOT NULL DEFAULT 0,
  last_operation_seq INTEGER NOT NULL DEFAULT 0,
  checkpoint_id TEXT NOT NULL DEFAULT '',
  client_instance_id TEXT NOT NULL DEFAULT '',
  expires_at TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
  committed_revision_id TEXT NOT NULL DEFAULT '',
  base_action_content_hash TEXT NOT NULL DEFAULT '',
  base_binding_revision INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_des_user ON desktop_pet_edit_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_des_task_action ON desktop_pet_edit_sessions(processing_task_id, action_key);
CREATE INDEX IF NOT EXISTS idx_des_status ON desktop_pet_edit_sessions(status);

CREATE TABLE IF NOT EXISTS desktop_pet_edit_operations (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL DEFAULT '',
  sequence INTEGER NOT NULL DEFAULT 0,
  operation_type TEXT NOT NULL DEFAULT '',
  payload_json TEXT NOT NULL DEFAULT '{}',
  inverse_json TEXT NOT NULL DEFAULT '{}',
  idempotency_key TEXT NOT NULL DEFAULT '',
  base_version INTEGER NOT NULL DEFAULT 0,
  result_version INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'applied',
  created_by TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_deo_session ON desktop_pet_edit_operations(session_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_deo_session_seq ON desktop_pet_edit_operations(session_id, sequence);
CREATE UNIQUE INDEX IF NOT EXISTS uq_deo_idempotency ON desktop_pet_edit_operations(session_id, idempotency_key);

CREATE TABLE IF NOT EXISTS desktop_pet_edit_checkpoints (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL DEFAULT '',
  sequence INTEGER NOT NULL DEFAULT 0,
  manifest_json TEXT NOT NULL DEFAULT '{}',
  manifest_hash TEXT NOT NULL DEFAULT '',
  frame_count INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_dec_session ON desktop_pet_edit_checkpoints(session_id);

CREATE TABLE IF NOT EXISTS desktop_pet_regeneration_jobs (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL DEFAULT '',
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
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
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
  user_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
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
CREATE INDEX IF NOT EXISTS idx_drj_user ON desktop_pet_regeneration_jobs(user_id);
CREATE INDEX IF NOT EXISTS idx_drj_execution ON desktop_pet_regeneration_jobs(execution_id);
CREATE INDEX IF NOT EXISTS idx_drj_stage ON desktop_pet_regeneration_jobs(stage);

CREATE TABLE IF NOT EXISTS desktop_pet_edit_candidates (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL DEFAULT '',
  job_id TEXT NOT NULL DEFAULT '',
  target_frame_id TEXT NOT NULL DEFAULT '',
  candidate_type TEXT NOT NULL DEFAULT '',
  asset_id TEXT NOT NULL DEFAULT '',
  candidate_revision_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  metadata_json TEXT NOT NULL DEFAULT '{}',
  decided_by TEXT NOT NULL DEFAULT '',
  decided_at TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
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
  user_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
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

CREATE TABLE IF NOT EXISTS desktop_pet_regeneration_journals (
  id TEXT PRIMARY KEY,
  job_id TEXT NOT NULL DEFAULT '',
  state TEXT NOT NULL DEFAULT '',
  attempt_id TEXT NOT NULL DEFAULT '',
  provider_receipt_id TEXT NOT NULL DEFAULT '',
  artifact_id TEXT NOT NULL DEFAULT '',
  processing_revision_id TEXT NOT NULL DEFAULT '',
  candidate_revision_id TEXT NOT NULL DEFAULT '',
  quality_evaluation_id TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_drj_journal_job ON desktop_pet_regeneration_journals(job_id);
CREATE INDEX IF NOT EXISTS idx_drj_journal_state ON desktop_pet_regeneration_journals(state);

CREATE TABLE IF NOT EXISTS desktop_pet_candidate_revision_metadata (
  id TEXT PRIMARY KEY,
  candidate_revision_id TEXT NOT NULL DEFAULT '',
  source_type TEXT NOT NULL DEFAULT '',
  parent_action_revision_id TEXT NOT NULL DEFAULT '',
  base_binding_revision INTEGER NOT NULL DEFAULT 0,
  regeneration_job_id TEXT NOT NULL DEFAULT '',
  content_hash TEXT NOT NULL DEFAULT '',
  frame_set_hash TEXT NOT NULL DEFAULT '',
  action_config_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'candidate_committing',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_dcrm_candidate ON desktop_pet_candidate_revision_metadata(candidate_revision_id);
CREATE INDEX IF NOT EXISTS idx_dcrm_job ON desktop_pet_candidate_revision_metadata(regeneration_job_id);

CREATE TABLE IF NOT EXISTS desktop_pet_edit_draft_snapshots (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL DEFAULT '',
  session_version INTEGER NOT NULL DEFAULT 0,
  user_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
  action_stream_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  base_revision_id TEXT NOT NULL DEFAULT '',
  base_content_hash TEXT NOT NULL DEFAULT '',
  base_binding_revision INTEGER NOT NULL DEFAULT 0,
  action_config_snapshot_json TEXT NOT NULL DEFAULT '{}',
  action_config_hash TEXT NOT NULL DEFAULT '',
  frames_json TEXT NOT NULL DEFAULT '[]',
  frame_set_hash TEXT NOT NULL DEFAULT '',
  snapshot_hash TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_deds_session ON desktop_pet_edit_draft_snapshots(session_id);
CREATE INDEX IF NOT EXISTS idx_deds_hash ON desktop_pet_edit_draft_snapshots(snapshot_hash);
CREATE UNIQUE INDEX IF NOT EXISTS uq_deds_session_version ON desktop_pet_edit_draft_snapshots(session_id, session_version);

CREATE TABLE IF NOT EXISTS desktop_pet_regeneration_job_input_snapshots (
  job_id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL DEFAULT '',
  draft_snapshot_id TEXT NOT NULL DEFAULT '',
  draft_snapshot_hash TEXT NOT NULL DEFAULT '',
  request_json TEXT NOT NULL DEFAULT '{}',
  request_hash TEXT NOT NULL DEFAULT '',
  base_revision_id TEXT NOT NULL DEFAULT '',
  base_content_hash TEXT NOT NULL DEFAULT '',
  base_binding_revision INTEGER NOT NULL DEFAULT 0,
  target_frame_id TEXT NOT NULL DEFAULT '',
  target_frame_content_hash TEXT NOT NULL DEFAULT '',
  generation_profile_id TEXT NOT NULL DEFAULT '',
  processing_profile_id TEXT NOT NULL DEFAULT '',
  quality_profile_id TEXT NOT NULL DEFAULT '',
  cost_estimate_json TEXT NOT NULL DEFAULT '{}',
  cost_estimate_hash TEXT NOT NULL DEFAULT '',
  cost_confirmation_id TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_dris_session ON desktop_pet_regeneration_job_input_snapshots(session_id);
CREATE INDEX IF NOT EXISTS idx_dris_draft ON desktop_pet_regeneration_job_input_snapshots(draft_snapshot_id);

CREATE TABLE IF NOT EXISTS desktop_pet_candidate_acceptance_operations (
  id TEXT PRIMARY KEY,
  candidate_id TEXT NOT NULL DEFAULT '',
  session_id TEXT NOT NULL DEFAULT '',
  user_id TEXT NOT NULL DEFAULT '',
  action TEXT NOT NULL DEFAULT '',
  idempotency_key TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  error_message TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT '',
  completed_at TEXT NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_dcao_candidate_idem ON desktop_pet_candidate_acceptance_operations(candidate_id, idempotency_key);
CREATE INDEX IF NOT EXISTS idx_dcao_session ON desktop_pet_candidate_acceptance_operations(session_id);

CREATE TABLE IF NOT EXISTS desktop_pet_editing_event_outbox (
  id TEXT PRIMARY KEY,
  event_id TEXT NOT NULL DEFAULT '',
  event_type TEXT NOT NULL DEFAULT '',
  aggregate_type TEXT NOT NULL DEFAULT 'editing_job',
  aggregate_id TEXT NOT NULL DEFAULT '',
  user_id TEXT NOT NULL DEFAULT '',
  payload_json TEXT NOT NULL DEFAULT '{}',
  payload_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  attempt_count INTEGER NOT NULL DEFAULT 0,
  available_at TEXT NOT NULL DEFAULT (datetime('now')),
  last_error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  published_at TEXT NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_deeo_event ON desktop_pet_editing_event_outbox(event_id);
CREATE INDEX IF NOT EXISTS idx_deeo_status ON desktop_pet_editing_event_outbox(status, available_at);
CREATE INDEX IF NOT EXISTS idx_deeo_user ON desktop_pet_editing_event_outbox(user_id);

CREATE TABLE IF NOT EXISTS desktop_pet_edit_audit_logs (
  id TEXT PRIMARY KEY,
  event_type TEXT NOT NULL DEFAULT '',
  user_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  edit_session_id TEXT NOT NULL DEFAULT '',
  job_id TEXT NOT NULL DEFAULT '',
  base_revision_id TEXT NOT NULL DEFAULT '',
  candidate_revision_id TEXT NOT NULL DEFAULT '',
  previous_active_revision_id TEXT NOT NULL DEFAULT '',
  new_active_revision_id TEXT NOT NULL DEFAULT '',
  reason TEXT NOT NULL DEFAULT '',
  occurred_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_deal_session ON desktop_pet_edit_audit_logs(edit_session_id);
CREATE INDEX IF NOT EXISTS idx_deal_event ON desktop_pet_edit_audit_logs(event_type);

CREATE TABLE IF NOT EXISTS desktop_pet_mask_patches (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL DEFAULT '',
  frame_id TEXT NOT NULL DEFAULT '',
  source_asset_hash TEXT NOT NULL DEFAULT '',
  result_asset_id TEXT NOT NULL DEFAULT '',
  patch_type TEXT NOT NULL DEFAULT '',
  brush_data_path TEXT NOT NULL DEFAULT '',
  brush_size INTEGER NOT NULL DEFAULT 0,
  brush_hardness REAL NOT NULL DEFAULT 0,
  brush_opacity REAL NOT NULL DEFAULT 1,
  coordinate_space TEXT NOT NULL DEFAULT 'normalized_canvas',
  canvas_width INTEGER NOT NULL DEFAULT 0,
  canvas_height INTEGER NOT NULL DEFAULT 0,
  algorithm_version TEXT NOT NULL DEFAULT '',
  operation_seq INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_dmp_session_frame ON desktop_pet_mask_patches(session_id, frame_id);

CREATE TABLE IF NOT EXISTS desktop_pet_publish_journal (
  id TEXT PRIMARY KEY,
  revision_id TEXT NOT NULL DEFAULT '',
  session_id TEXT NOT NULL DEFAULT '',
  action TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  tmp_dir_path TEXT NOT NULL DEFAULT '',
  final_dir_path TEXT NOT NULL DEFAULT '',
  manifest_path TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  completed_at TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_dpj_revision ON desktop_pet_publish_journal(revision_id);
CREATE INDEX IF NOT EXISTS idx_dpj_status ON desktop_pet_publish_journal(status);

CREATE TABLE IF NOT EXISTS desktop_pet_edit_idempotency (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL DEFAULT '',
  session_id TEXT NOT NULL DEFAULT '',
  idempotency_key TEXT NOT NULL DEFAULT '',
  endpoint TEXT NOT NULL DEFAULT '',
  result_json TEXT NOT NULL DEFAULT '{}',
  status TEXT NOT NULL DEFAULT 'pending',
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_dei_user_session_key ON desktop_pet_edit_idempotency(user_id, session_id, idempotency_key);

CREATE TABLE IF NOT EXISTS desktop_pet_processing_revisions (
  id TEXT PRIMARY KEY,
  processing_task_id TEXT NOT NULL DEFAULT '',
  processing_action_id TEXT NOT NULL DEFAULT '',
  processing_attempt_id TEXT NOT NULL DEFAULT '',
  revision_number INTEGER NOT NULL DEFAULT 1,
  source_attempt_id TEXT NOT NULL DEFAULT '',
  source_candidate_index INTEGER NOT NULL DEFAULT 0,
  source_manifest_id TEXT NOT NULL DEFAULT '',
  source_generation_attempt_id TEXT NOT NULL DEFAULT '',
  source_generation_artifact_id TEXT NOT NULL DEFAULT '',
  source_artifact_content_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'preparing',
  config_snapshot TEXT NOT NULL DEFAULT '{}',
  config_hash TEXT NOT NULL DEFAULT '',
  pipeline_version TEXT NOT NULL DEFAULT '',
  frame_count INTEGER NOT NULL DEFAULT 0,
  root_relative_path TEXT NOT NULL DEFAULT '',
  root_storage_key TEXT NOT NULL DEFAULT '',
  revision_hash TEXT NOT NULL DEFAULT '',
  content_root_hash TEXT NOT NULL DEFAULT '',
  commit_id TEXT NOT NULL DEFAULT '',
  active INTEGER NOT NULL DEFAULT 0,
  error_code TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  published_at TEXT NOT NULL DEFAULT '',
  committed_at TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
  activated_at TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_dppr_task ON desktop_pet_processing_revisions(processing_task_id);
CREATE INDEX IF NOT EXISTS idx_dppr_action ON desktop_pet_processing_revisions(processing_action_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dppr_action_revision ON desktop_pet_processing_revisions(processing_action_id, revision_number);

CREATE TABLE IF NOT EXISTS desktop_pet_processing_artifacts (
  id TEXT PRIMARY KEY,
  revision_id TEXT NOT NULL DEFAULT '',
  frame_index INTEGER,
  artifact_kind TEXT NOT NULL DEFAULT '',
  stage TEXT NOT NULL DEFAULT '',
  relative_path TEXT NOT NULL DEFAULT '',
  mime_type TEXT NOT NULL DEFAULT '',
  width INTEGER NOT NULL DEFAULT 0,
  height INTEGER NOT NULL DEFAULT 0,
  byte_size INTEGER NOT NULL DEFAULT 0,
  content_hash TEXT NOT NULL DEFAULT '',
  source_artifact_id TEXT NOT NULL DEFAULT '',
  source_cell_index INTEGER,
  metadata_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_dppa_revision ON desktop_pet_processing_artifacts(revision_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dppa_rev_kind_frame ON desktop_pet_processing_artifacts(revision_id, artifact_kind, frame_index);

CREATE TABLE IF NOT EXISTS desktop_pet_processing_transforms (
  id TEXT PRIMARY KEY,
  revision_id TEXT NOT NULL DEFAULT '',
  frame_index INTEGER NOT NULL DEFAULT 0,
  sequence_number INTEGER NOT NULL DEFAULT 0,
  from_space TEXT NOT NULL DEFAULT '',
  to_space TEXT NOT NULL DEFAULT '',
  transform_type TEXT NOT NULL DEFAULT '',
  matrix_json TEXT NOT NULL DEFAULT '[]',
  parameters_json TEXT NOT NULL DEFAULT '{}',
  algorithm_version TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_dppt_revision ON desktop_pet_processing_transforms(revision_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dppt_rev_frame_seq ON desktop_pet_processing_transforms(revision_id, frame_index, sequence_number);

CREATE TABLE IF NOT EXISTS desktop_pet_frame_measurements (
  id TEXT PRIMARY KEY,
  revision_id TEXT NOT NULL DEFAULT '',
  frame_index INTEGER NOT NULL DEFAULT 0,
  measurement_schema_version INTEGER NOT NULL DEFAULT 1,
  subject_box_json TEXT NOT NULL DEFAULT '{}',
  source_anchor_json TEXT NOT NULL DEFAULT '{}',
  target_anchor_json TEXT NOT NULL DEFAULT '{}',
  alpha_coverage REAL NOT NULL DEFAULT 0,
  component_count INTEGER NOT NULL DEFAULT 0,
  edge_contact_json TEXT NOT NULL DEFAULT '{}',
  clipping_json TEXT NOT NULL DEFAULT '{}',
  trajectory_json TEXT NOT NULL DEFAULT '{}',
  measurement_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_dpfm_revision ON desktop_pet_frame_measurements(revision_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dpfm_rev_frame ON desktop_pet_frame_measurements(revision_id, frame_index);

ALTER TABLE desktop_pet_processing_tasks ADD COLUMN config_snapshot TEXT NOT NULL DEFAULT '{}';
ALTER TABLE desktop_pet_processing_tasks ADD COLUMN config_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processing_tasks ADD COLUMN pipeline_version TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processing_tasks ADD COLUMN active_revision_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE desktop_pet_processing_tasks ADD COLUMN publish_state TEXT NOT NULL DEFAULT '';

ALTER TABLE desktop_pet_processing_actions ADD COLUMN active_revision_id TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processing_actions ADD COLUMN next_revision_number INTEGER NOT NULL DEFAULT 1;
ALTER TABLE desktop_pet_processing_actions ADD COLUMN source_attempt_id TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processing_actions ADD COLUMN source_candidate_index INTEGER NOT NULL DEFAULT 0;
ALTER TABLE desktop_pet_processing_actions ADD COLUMN processing_profile_snapshot TEXT NOT NULL DEFAULT '{}';

ALTER TABLE desktop_pet_processed_frames ADD COLUMN revision_id TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processed_frames ADD COLUMN mask_path TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processed_frames ADD COLUMN transform_chain_id TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processed_frames ADD COLUMN measurement_id TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processed_frames ADD COLUMN source_artifact_id TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processed_frames ADD COLUMN source_cell_index INTEGER;

ALTER TABLE desktop_pet_processing_revisions ADD COLUMN content_root_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processing_revisions ADD COLUMN activated_at TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processing_revisions ADD COLUMN source_manifest_id TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processing_revisions ADD COLUMN commit_id TEXT NOT NULL DEFAULT '';

ALTER TABLE desktop_pet_processing_action_attempts ADD COLUMN lease_owner TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processing_action_attempts ADD COLUMN lease_expires_at TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processing_action_attempts ADD COLUMN heartbeat_at TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processing_action_attempts ADD COLUMN commit_id TEXT NOT NULL DEFAULT '';

ALTER TABLE desktop_pet_processing_actions ADD COLUMN pending_retry_request_id TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processing_actions ADD COLUMN processing_warnings TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processing_actions ADD COLUMN warning_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE desktop_pet_processing_actions ADD COLUMN action_spec_snapshot TEXT NOT NULL DEFAULT '{}';

ALTER TABLE desktop_pet_processing_tasks ADD COLUMN user_id TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processing_tasks ADD COLUMN character_id TEXT NOT NULL DEFAULT '';

ALTER TABLE desktop_pet_processing_action_attempts ADD COLUMN source_generation_attempt_id TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processing_action_attempts ADD COLUMN source_generation_artifact_id TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processing_action_attempts ADD COLUMN source_manifest_id TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processing_action_attempts ADD COLUMN source_artifact_content_hash TEXT NOT NULL DEFAULT '';

ALTER TABLE desktop_pet_processing_revisions ADD COLUMN processing_attempt_id TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processing_revisions ADD COLUMN source_generation_attempt_id TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processing_revisions ADD COLUMN source_generation_artifact_id TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processing_revisions ADD COLUMN source_artifact_content_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processing_revisions ADD COLUMN root_storage_key TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processing_revisions ADD COLUMN committed_at TEXT NOT NULL DEFAULT '';

ALTER TABLE desktop_pet_processing_source_manifests ADD COLUMN user_id TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processing_source_manifests ADD COLUMN character_id TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processing_source_manifests ADD COLUMN active_artifact_binding_revision INTEGER NOT NULL DEFAULT 0;
ALTER TABLE desktop_pet_processing_source_manifests ADD COLUMN artifact_bytes INTEGER NOT NULL DEFAULT 0;
ALTER TABLE desktop_pet_processing_source_manifests ADD COLUMN reference_asset_content_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processing_source_manifests ADD COLUMN generation_plan_id TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processing_source_manifests ADD COLUMN generation_plan_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE desktop_pet_processing_source_manifests ADD COLUMN action_spec_hash TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS desktop_pet_processing_commit_journals (
  id TEXT PRIMARY KEY,
  commit_id TEXT NOT NULL DEFAULT '',
  processing_attempt_id TEXT NOT NULL DEFAULT '',
  processing_revision_id TEXT NOT NULL DEFAULT '',
  source_manifest_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'created',
  staging_path TEXT NOT NULL DEFAULT '',
  final_path TEXT NOT NULL DEFAULT '',
  content_root_hash TEXT NOT NULL DEFAULT '',
  pipeline_result_hash TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
  last_error TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_dppcj_commit ON desktop_pet_processing_commit_journals(commit_id);
CREATE INDEX IF NOT EXISTS idx_dppcj_attempt ON desktop_pet_processing_commit_journals(processing_attempt_id);
CREATE INDEX IF NOT EXISTS idx_dppcj_status ON desktop_pet_processing_commit_journals(status);

CREATE TABLE IF NOT EXISTS desktop_pet_processing_event_outbox (
  id TEXT PRIMARY KEY,
  event_type TEXT NOT NULL DEFAULT '',
  aggregate_id TEXT NOT NULL DEFAULT '',
  payload TEXT NOT NULL DEFAULT '{}',
  status TEXT NOT NULL DEFAULT 'pending',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  published_at TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  retry_count INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_dppeo_status ON desktop_pet_processing_event_outbox(status);
CREATE INDEX IF NOT EXISTS idx_dppeo_aggregate ON desktop_pet_processing_event_outbox(aggregate_id);
CREATE INDEX IF NOT EXISTS idx_dppeo_created ON desktop_pet_processing_event_outbox(created_at);

CREATE TABLE IF NOT EXISTS desktop_pet_processing_source_manifests (
  id TEXT PRIMARY KEY,
  schema_version INTEGER NOT NULL DEFAULT 1,
  user_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
  processing_task_id TEXT NOT NULL DEFAULT '',
  processing_action_id TEXT NOT NULL DEFAULT '',
  generation_task_id TEXT NOT NULL DEFAULT '',
  generation_action_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  generation_mode TEXT NOT NULL DEFAULT '',
  generation_attempt_id TEXT NOT NULL DEFAULT '',
  active_artifact_binding_revision INTEGER NOT NULL DEFAULT 0,
  source_artifact_id TEXT NOT NULL DEFAULT '',
  artifact_role TEXT NOT NULL DEFAULT 'primary',
  artifact_kind TEXT NOT NULL DEFAULT '',
  artifact_content_hash TEXT NOT NULL DEFAULT '',
  artifact_storage_key TEXT NOT NULL DEFAULT '',
  artifact_relative_path TEXT NOT NULL DEFAULT '',
  artifact_width INTEGER NOT NULL DEFAULT 0,
  artifact_height INTEGER NOT NULL DEFAULT 0,
  artifact_mime_type TEXT NOT NULL DEFAULT '',
  artifact_bytes INTEGER NOT NULL DEFAULT 0,
  candidate_index INTEGER NOT NULL DEFAULT 0,
  reference_asset_id TEXT NOT NULL DEFAULT '',
  reference_asset_content_hash TEXT NOT NULL DEFAULT '',
  generation_plan_id TEXT NOT NULL DEFAULT '',
  generation_plan_hash TEXT NOT NULL DEFAULT '',
  prompt_document_id TEXT NOT NULL DEFAULT '',
  prompt_content_hash TEXT NOT NULL DEFAULT '',
  expected_frame_count INTEGER NOT NULL DEFAULT 0,
  sprite_sheet_layout_json TEXT NOT NULL DEFAULT '{}',
  keyframes_json TEXT NOT NULL DEFAULT '[]',
  legacy_frames_json TEXT NOT NULL DEFAULT '[]',
  frames_json TEXT NOT NULL DEFAULT '[]',
  action_spec_snapshot_json TEXT NOT NULL DEFAULT '{}',
  action_spec_hash TEXT NOT NULL DEFAULT '',
  source_config_hash TEXT NOT NULL DEFAULT '',
  manifest_hash TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_dppsm_task ON desktop_pet_processing_source_manifests(processing_task_id);
CREATE INDEX IF NOT EXISTS idx_dppsm_gen_action ON desktop_pet_processing_source_manifests(generation_action_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dppsm_processing_action ON desktop_pet_processing_source_manifests(processing_action_id);

CREATE TABLE IF NOT EXISTS desktop_pet_processing_publish_journals (
  id TEXT PRIMARY KEY,
  revision_id TEXT NOT NULL DEFAULT '',
  processing_task_id TEXT NOT NULL DEFAULT '',
  processing_action_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  commit_id TEXT NOT NULL DEFAULT '',
  stage TEXT NOT NULL DEFAULT 'preparing',
  journal_status TEXT NOT NULL DEFAULT 'prepared',
  staging_path TEXT NOT NULL DEFAULT '',
  final_path TEXT NOT NULL DEFAULT '',
  content_root_hash TEXT NOT NULL DEFAULT '',
  detail TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
  completed_at TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_dppj_revision ON desktop_pet_processing_publish_journals(revision_id);
CREATE INDEX IF NOT EXISTS idx_dppj_action ON desktop_pet_processing_publish_journals(processing_action_id);
CREATE INDEX IF NOT EXISTS idx_dppj_status ON desktop_pet_processing_publish_journals(journal_status);

CREATE TABLE IF NOT EXISTS desktop_pet_processing_revision_active_bindings (
  processing_action_id TEXT PRIMARY KEY,
  active_revision_id TEXT NOT NULL DEFAULT '',
  binding_revision INTEGER NOT NULL DEFAULT 0,
  bound_at TEXT NOT NULL DEFAULT '',
  bound_reason TEXT NOT NULL DEFAULT '',
  superseded_revision_id TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_dprab_revision ON desktop_pet_processing_revision_active_bindings(active_revision_id);

CREATE TABLE IF NOT EXISTS desktop_pet_processing_workspace_leases (
  id TEXT PRIMARY KEY,
  processing_task_id TEXT NOT NULL DEFAULT '',
  processing_action_id TEXT NOT NULL DEFAULT '',
  processing_attempt_id TEXT NOT NULL DEFAULT '',
  commit_id TEXT NOT NULL DEFAULT '',
  workspace_root TEXT NOT NULL DEFAULT '',
  lease_owner TEXT NOT NULL DEFAULT '',
  lease_expires_at TEXT NOT NULL DEFAULT '',
  heartbeat_at TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active',
  cleanup_reason TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_dpwl_task ON desktop_pet_processing_workspace_leases(processing_task_id);
CREATE INDEX IF NOT EXISTS idx_dpwl_attempt ON desktop_pet_processing_workspace_leases(processing_attempt_id);
CREATE INDEX IF NOT EXISTS idx_dpwl_status ON desktop_pet_processing_workspace_leases(status);
CREATE INDEX IF NOT EXISTS idx_dpwl_lease_expires ON desktop_pet_processing_workspace_leases(lease_expires_at);

CREATE TABLE IF NOT EXISTS desktop_pet_processing_retry_requests (
  id TEXT PRIMARY KEY,
  processing_task_id TEXT NOT NULL DEFAULT '',
  processing_action_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  requested_by TEXT NOT NULL DEFAULT '',
  request_reason TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'queued',
  allocated_attempt_id TEXT NOT NULL DEFAULT '',
  error_code TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
  completed_at TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_dprr_task ON desktop_pet_processing_retry_requests(processing_task_id);
CREATE INDEX IF NOT EXISTS idx_dprr_action ON desktop_pet_processing_retry_requests(processing_action_id);
CREATE INDEX IF NOT EXISTS idx_dprr_status ON desktop_pet_processing_retry_requests(status);

CREATE TABLE IF NOT EXISTS desktop_pet_quality_evaluations (
  id TEXT PRIMARY KEY,
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
  user_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
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
  created_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dpqe_input ON desktop_pet_quality_evaluations(action_revision_id, profile_hash, engine_version);
CREATE INDEX IF NOT EXISTS idx_dpqe_task ON desktop_pet_quality_evaluations(processing_task_id);
CREATE INDEX IF NOT EXISTS idx_dpqe_action ON desktop_pet_quality_evaluations(processing_action_id);
CREATE INDEX IF NOT EXISTS idx_dpqe_status ON desktop_pet_quality_evaluations(execution_status);

CREATE TABLE IF NOT EXISTS desktop_pet_quality_findings (
  id TEXT PRIMARY KEY,
  evaluation_id TEXT NOT NULL,
  rule_code TEXT NOT NULL,
  rule_version INTEGER NOT NULL,
  dimension_key TEXT NOT NULL,
  severity TEXT NOT NULL,
  hard_gate INTEGER NOT NULL DEFAULT 0,
  frame_indexes_json TEXT NOT NULL DEFAULT '[]',
  frame_pairs_json TEXT NOT NULL DEFAULT '[]',
  regions_json TEXT NOT NULL DEFAULT '[]',
  metric_name TEXT DEFAULT '',
  observed_value REAL,
  threshold_value REAL,
  comparison TEXT DEFAULT '',
  confidence REAL NOT NULL DEFAULT 0,
  message_key TEXT NOT NULL,
  message TEXT NOT NULL,
  suggested_action TEXT DEFAULT '',
  evidence_ref TEXT DEFAULT '',
  sort_key TEXT NOT NULL,
  created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_dpqf_eval ON desktop_pet_quality_findings(evaluation_id);
CREATE INDEX IF NOT EXISTS idx_dpqf_rule ON desktop_pet_quality_findings(rule_code);
CREATE INDEX IF NOT EXISTS idx_dpqf_severity ON desktop_pet_quality_findings(severity);

CREATE TABLE IF NOT EXISTS desktop_pet_quality_dimension_scores (
  id TEXT PRIMARY KEY,
  evaluation_id TEXT NOT NULL,
  dimension_key TEXT NOT NULL,
  applicability TEXT NOT NULL,
  score REAL,
  confidence REAL NOT NULL DEFAULT 0,
  weight REAL NOT NULL DEFAULT 0,
  details_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT DEFAULT (datetime('now')),
  UNIQUE(evaluation_id, dimension_key)
);

CREATE TABLE IF NOT EXISTS desktop_pet_quality_gate_results (
  id TEXT PRIMARY KEY,
  processing_task_id TEXT NOT NULL,
  gate_status TEXT NOT NULL,
  required_action_count INTEGER NOT NULL DEFAULT 0,
  accepted_action_count INTEGER NOT NULL DEFAULT 0,
  warning_action_count INTEGER NOT NULL DEFAULT 0,
  review_action_count INTEGER NOT NULL DEFAULT 0,
  rejected_action_count INTEGER NOT NULL DEFAULT 0,
  failed_evaluation_count INTEGER NOT NULL DEFAULT 0,
  snapshot_json TEXT NOT NULL,
  snapshot_hash TEXT NOT NULL,
  active_revision_set_hash TEXT NOT NULL DEFAULT '',
  evaluation_set_hash TEXT NOT NULL DEFAULT '',
  ruleset_version TEXT NOT NULL DEFAULT '',
  invalidated_at TEXT NOT NULL DEFAULT '',
  profile_id TEXT NOT NULL DEFAULT '',
  created_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dpqg_task_snapshot ON desktop_pet_quality_gate_results(processing_task_id, snapshot_hash);

CREATE TABLE IF NOT EXISTS desktop_pet_action_generation_attempts (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL DEFAULT '',
    task_action_id TEXT NOT NULL DEFAULT '',
    attempt_number INTEGER NOT NULL DEFAULT 1,
    parent_attempt_id TEXT NOT NULL DEFAULT '',
    mode TEXT NOT NULL DEFAULT 'sprite_sheet',
    reason TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    provider TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    config_id INTEGER NOT NULL DEFAULT 0,
    config_revision TEXT NOT NULL DEFAULT '',
    capability_hash TEXT NOT NULL DEFAULT '',
    reference_asset_id TEXT NOT NULL DEFAULT '',
    plan_json TEXT NOT NULL DEFAULT '{}',
    prompt_document_json TEXT NOT NULL DEFAULT '{}',
    prompt_snapshot TEXT NOT NULL DEFAULT '',
    prompt_hash TEXT NOT NULL DEFAULT '',
    negative_prompt_snapshot TEXT NOT NULL DEFAULT '',
    seed_policy TEXT NOT NULL DEFAULT 'auto',
    seed_value INTEGER,
    output_count INTEGER NOT NULL DEFAULT 1,
    execution_id TEXT NOT NULL DEFAULT '',
    worker_id TEXT NOT NULL DEFAULT '',
    lease TEXT NOT NULL DEFAULT '',
    provider_request_id TEXT NOT NULL DEFAULT '',
    provider_operation_id TEXT NOT NULL DEFAULT '',
    submitted_at TEXT NOT NULL DEFAULT '',
    completed_at TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT '',
    active_succeeded_attempt_id TEXT NOT NULL DEFAULT '',
    active_primary_artifact_id TEXT NOT NULL DEFAULT '',
    artifact_role TEXT NOT NULL DEFAULT 'primary',
    cancel_reason TEXT NOT NULL DEFAULT '',
    cancel_requested_at TEXT NOT NULL DEFAULT '',
    cancelled_at TEXT NOT NULL DEFAULT '',
    request_hash TEXT NOT NULL DEFAULT '',
    action_spec_hash TEXT NOT NULL DEFAULT '',
    provider_config_hash TEXT NOT NULL DEFAULT '',
    prompt_document_id TEXT NOT NULL DEFAULT '',
    prompt_content_hash TEXT NOT NULL DEFAULT '',
    negative_prompt_hash TEXT NOT NULL DEFAULT '',
    actual_cost REAL NOT NULL DEFAULT 0,
    actual_input_units INTEGER NOT NULL DEFAULT 0,
    actual_output_units INTEGER NOT NULL DEFAULT 0,
    lease_owner TEXT NOT NULL DEFAULT '',
    lease_expires_at TEXT NOT NULL DEFAULT '',
    heartbeat_at TEXT NOT NULL DEFAULT '',
    retry_after_hint INTEGER NOT NULL DEFAULT 0,
    poll_count INTEGER NOT NULL DEFAULT 0,
    UNIQUE(task_action_id, attempt_number)
);

CREATE INDEX IF NOT EXISTS idx_gen_attempts_task_id ON desktop_pet_action_generation_attempts(task_id);
CREATE INDEX IF NOT EXISTS idx_gen_attempts_action_id ON desktop_pet_action_generation_attempts(task_action_id);
CREATE INDEX IF NOT EXISTS idx_gen_attempts_status ON desktop_pet_action_generation_attempts(status);

CREATE TABLE IF NOT EXISTS desktop_pet_generation_artifacts (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL DEFAULT '',
    task_action_id TEXT NOT NULL DEFAULT '',
    attempt_id TEXT NOT NULL DEFAULT '',
    artifact_type TEXT NOT NULL DEFAULT 'sprite_sheet_raw',
    segment_index INTEGER NOT NULL DEFAULT 0,
    candidate_index INTEGER NOT NULL DEFAULT 0,
    is_primary INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',
    relative_path TEXT NOT NULL DEFAULT '',
    mime TEXT NOT NULL DEFAULT '',
    width INTEGER NOT NULL DEFAULT 0,
    height INTEGER NOT NULL DEFAULT 0,
    size INTEGER NOT NULL DEFAULT 0,
    hash TEXT NOT NULL DEFAULT '',
    provider_request_id TEXT NOT NULL DEFAULT '',
    provider_operation_id TEXT NOT NULL DEFAULT '',
    layout_json TEXT NOT NULL DEFAULT '{}',
    metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT '',
    artifact_role TEXT NOT NULL DEFAULT 'primary',
    storage_key TEXT NOT NULL DEFAULT '',
    content_hash TEXT NOT NULL DEFAULT '',
    source_reference_asset_id TEXT NOT NULL DEFAULT '',
    source_prompt_hash TEXT NOT NULL DEFAULT '',
    storage_backend TEXT NOT NULL DEFAULT 'local',
    UNIQUE(attempt_id, artifact_type, segment_index, candidate_index)
);

CREATE INDEX IF NOT EXISTS idx_gen_artifacts_task_id ON desktop_pet_generation_artifacts(task_id);
CREATE INDEX IF NOT EXISTS idx_gen_artifacts_attempt_id ON desktop_pet_generation_artifacts(attempt_id);
CREATE INDEX IF NOT EXISTS idx_gen_artifacts_action_id ON desktop_pet_generation_artifacts(task_action_id);

CREATE TABLE IF NOT EXISTS desktop_pet_reference_assets (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL DEFAULT '',
    source_path TEXT NOT NULL DEFAULT '',
    source_hash TEXT NOT NULL DEFAULT '',
    source_mime TEXT NOT NULL DEFAULT '',
    source_width INTEGER NOT NULL DEFAULT 0,
    source_height INTEGER NOT NULL DEFAULT 0,
    normalized_path TEXT NOT NULL DEFAULT '',
    normalized_hash TEXT NOT NULL DEFAULT '',
    normalized_mime TEXT NOT NULL DEFAULT '',
    normalized_width INTEGER NOT NULL DEFAULT 0,
    normalized_height INTEGER NOT NULL DEFAULT 0,
    config_hash TEXT NOT NULL DEFAULT '',
    content_hash TEXT NOT NULL DEFAULT '',
    normalizer_version TEXT NOT NULL DEFAULT '',
    subject_box TEXT NOT NULL DEFAULT '{}',
    anchor TEXT NOT NULL DEFAULT '{}',
    coordinate_space TEXT NOT NULL DEFAULT '',
    character_id TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
    source_artifact_id TEXT NOT NULL DEFAULT '',
    storage_path TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_ref_assets_task_id ON desktop_pet_reference_assets(task_id);
CREATE INDEX IF NOT EXISTS idx_ref_assets_source_hash ON desktop_pet_reference_assets(source_hash);
CREATE INDEX IF NOT EXISTS idx_ref_assets_content_hash ON desktop_pet_reference_assets(content_hash);

CREATE TABLE IF NOT EXISTS desktop_pet_generation_action_active_bindings (
    generation_action_id TEXT PRIMARY KEY,
    active_attempt_id TEXT NOT NULL DEFAULT '',
    active_primary_artifact_id TEXT NOT NULL DEFAULT '',
    artifact_content_hash TEXT NOT NULL DEFAULT '',
    binding_revision INTEGER NOT NULL DEFAULT 0,
    bound_at TEXT NOT NULL DEFAULT '',
    bound_reason TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_active_bindings_attempt ON desktop_pet_generation_action_active_bindings(active_attempt_id);
CREATE INDEX IF NOT EXISTS idx_active_bindings_artifact ON desktop_pet_generation_action_active_bindings(active_primary_artifact_id);

CREATE TABLE IF NOT EXISTS desktop_pet_generation_provider_receipts (
    id TEXT PRIMARY KEY,
    attempt_id TEXT NOT NULL DEFAULT '',
    provider_id TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    idempotency_key TEXT NOT NULL DEFAULT '',
    provider_request_id TEXT NOT NULL DEFAULT '',
    provider_task_id TEXT NOT NULL DEFAULT '',
    submitted_at TEXT NOT NULL DEFAULT '',
    first_polled_at TEXT NOT NULL DEFAULT '',
    completed_at TEXT NOT NULL DEFAULT '',
    request_hash TEXT NOT NULL DEFAULT '',
    response_hash TEXT NOT NULL DEFAULT '',
    provider_status TEXT NOT NULL DEFAULT '',
    raw_metadata_json TEXT NOT NULL DEFAULT '{}',
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_provider_receipts_attempt ON desktop_pet_generation_provider_receipts(attempt_id);
CREATE INDEX IF NOT EXISTS idx_provider_receipts_request ON desktop_pet_generation_provider_receipts(provider_request_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_provider_receipts_idempotency ON desktop_pet_generation_provider_receipts(idempotency_key) WHERE idempotency_key != '';

CREATE TABLE IF NOT EXISTS desktop_pet_generation_artifact_publish_journal (
    id TEXT PRIMARY KEY,
    artifact_id TEXT NOT NULL DEFAULT '',
    attempt_id TEXT NOT NULL DEFAULT '',
    task_id TEXT NOT NULL DEFAULT '',
    task_action_id TEXT NOT NULL DEFAULT '',
    staging_path TEXT NOT NULL DEFAULT '',
    final_path TEXT NOT NULL DEFAULT '',
    content_hash TEXT NOT NULL DEFAULT '',
    storage_key TEXT NOT NULL DEFAULT '',
    journal_status TEXT NOT NULL DEFAULT 'staging',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT '',
    completed_at TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_publish_journal_artifact ON desktop_pet_generation_artifact_publish_journal(artifact_id);
CREATE INDEX IF NOT EXISTS idx_publish_journal_status ON desktop_pet_generation_artifact_publish_journal(journal_status);
CREATE INDEX IF NOT EXISTS idx_publish_journal_attempt ON desktop_pet_generation_artifact_publish_journal(attempt_id);

CREATE TABLE IF NOT EXISTS desktop_pet_data_repair_audit (
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
);

CREATE TABLE IF NOT EXISTS desktop_pet_state_transitions (
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
);

CREATE INDEX IF NOT EXISTS idx_dp_transition_entity ON desktop_pet_state_transitions(entity_type, entity_id, created_at);
CREATE INDEX IF NOT EXISTS idx_dp_transition_parent ON desktop_pet_state_transitions(parent_task_id, created_at);


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
CREATE TABLE IF NOT EXISTS deletion_tombstones (
				id TEXT PRIMARY KEY,
				target_id TEXT DEFAULT '',
				target_type TEXT DEFAULT '',
				scope TEXT DEFAULT '',
				status TEXT DEFAULT '',
				items_count INTEGER DEFAULT 0,
				cleaned_count INTEGER DEFAULT 0,
				failed_count INTEGER DEFAULT 0,
				requested_at DATETIME,
				blocked_until DATETIME,
				completed_at DATETIME,
				retrieval_blocked INTEGER DEFAULT 0
			);

-- 来源: consolidation_auto_migrate.go
CREATE TABLE IF NOT EXISTS data_lifecycle_outbox_cleanup_items (
				id TEXT PRIMARY KEY,
				storage TEXT DEFAULT '',
				target_id TEXT DEFAULT '',
				target_kind TEXT DEFAULT '',
				status TEXT DEFAULT '',
				attempts INTEGER DEFAULT 0,
				max_attempts INTEGER DEFAULT 5,
				next_retry_at DATETIME,
				lease_owner TEXT DEFAULT '',
				lease_token TEXT DEFAULT '',
				leased_until DATETIME,
				last_error TEXT DEFAULT '',
				cleaned_at DATETIME
			);

-- 来源: consolidation_auto_migrate.go
CREATE TABLE IF NOT EXISTS data_lifecycle_recalculation_tasks (
				id TEXT PRIMARY KEY,
				trigger_type TEXT DEFAULT '',
				target_id TEXT DEFAULT '',
				affected_zone TEXT DEFAULT '',
				priority INTEGER DEFAULT 0,
				created_at DATETIME,
				status TEXT DEFAULT '',
				description TEXT DEFAULT '',
				attempts INTEGER DEFAULT 0,
				max_attempts INTEGER DEFAULT 3,
				next_retry_at DATETIME,
				lease_owner TEXT DEFAULT '',
				lease_token TEXT DEFAULT '',
				leased_until DATETIME,
				last_error TEXT DEFAULT '',
				completed_at DATETIME
			);

-- 来源: consolidation_auto_migrate.go
CREATE TABLE IF NOT EXISTS output_leases (
				id TEXT PRIMARY KEY,
				interaction_id TEXT DEFAULT '',
				character_id TEXT DEFAULT '',
				user_id TEXT DEFAULT '',
				channel TEXT DEFAULT '',
				owner_token TEXT DEFAULT '',
				generation INTEGER DEFAULT 0,
				status TEXT DEFAULT '',
				acquired_at TEXT DEFAULT '',
				expires_at TEXT DEFAULT '',
				released_at TEXT DEFAULT '',
				preempted_by TEXT DEFAULT ''
			);

-- 来源: emotes.go
CREATE TABLE IF NOT EXISTS emotes (
id TEXT PRIMARY KEY,
name TEXT NOT NULL DEFAULT '',
meaning TEXT NOT NULL DEFAULT '',
keywords TEXT NOT NULL DEFAULT '[]',
original_filename TEXT NOT NULL DEFAULT '',
file_path TEXT NOT NULL DEFAULT '',
thumbnail_path TEXT NOT NULL DEFAULT '',
fallback_path TEXT NOT NULL DEFAULT '',
mime_type TEXT NOT NULL DEFAULT '',
file_extension TEXT NOT NULL DEFAULT '',
file_size INTEGER NOT NULL DEFAULT 0,
width INTEGER NOT NULL DEFAULT 0,
height INTEGER NOT NULL DEFAULT 0,
is_animated INTEGER NOT NULL DEFAULT 0,
duration_ms INTEGER NOT NULL DEFAULT 0,
frame_count INTEGER NOT NULL DEFAULT 1,
file_hash TEXT NOT NULL,
enabled INTEGER NOT NULL DEFAULT 1,
ai_enabled INTEGER NOT NULL DEFAULT 0,
role_scope TEXT NOT NULL DEFAULT 'all_characters',
vector_status TEXT NOT NULL DEFAULT 'disabled',
vector_error TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
deleted_at TEXT
);

-- 来源: emotes.go
CREATE TABLE IF NOT EXISTS emote_groups (
id TEXT PRIMARY KEY,
name TEXT NOT NULL,
cover_emote_id TEXT,
sort_order INTEGER NOT NULL DEFAULT 0,
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
FOREIGN KEY (cover_emote_id) REFERENCES emotes(id) ON DELETE SET NULL
);

-- 来源: emotes.go
CREATE TABLE IF NOT EXISTS emote_group_items (
group_id TEXT NOT NULL,
emote_id TEXT NOT NULL,
sort_order INTEGER NOT NULL DEFAULT 0,
PRIMARY KEY (group_id, emote_id),
FOREIGN KEY (group_id) REFERENCES emote_groups(id) ON DELETE CASCADE,
FOREIGN KEY (emote_id) REFERENCES emotes(id) ON DELETE CASCADE
);

-- 来源: emotes.go
CREATE TABLE IF NOT EXISTS emote_character_bindings (
emote_id TEXT NOT NULL,
character_id TEXT NOT NULL,
PRIMARY KEY (emote_id, character_id),
FOREIGN KEY (emote_id) REFERENCES emotes(id) ON DELETE CASCADE,
FOREIGN KEY (character_id) REFERENCES characters(id) ON DELETE CASCADE
);

-- 来源: emotes.go
CREATE TABLE IF NOT EXISTS character_emote_settings (
character_id TEXT PRIMARY KEY,
enabled INTEGER NOT NULL DEFAULT 1,
base_probability REAL NOT NULL DEFAULT 0.10,
max_probability REAL NOT NULL DEFAULT 0.30,
max_per_hour INTEGER NOT NULL DEFAULT 5,
min_reply_gap INTEGER NOT NULL DEFAULT 3,
same_emote_cooldown_minutes INTEGER NOT NULL DEFAULT 30,
allow_emote_only INTEGER NOT NULL DEFAULT 0,
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
FOREIGN KEY (character_id) REFERENCES characters(id) ON DELETE CASCADE
);

-- 来源: emotes.go
CREATE TABLE IF NOT EXISTS emote_send_records (
id TEXT PRIMARY KEY,
emote_id TEXT,
character_id TEXT NOT NULL DEFAULT '',
conversation_id TEXT NOT NULL DEFAULT '',
message_id TEXT NOT NULL DEFAULT '',
response_id TEXT NOT NULL DEFAULT '',
platform TEXT NOT NULL DEFAULT '',
trigger_type TEXT NOT NULL DEFAULT '',
trigger_probability REAL NOT NULL DEFAULT 0,
random_sample REAL NOT NULL DEFAULT 0,
trigger_hit INTEGER NOT NULL DEFAULT 0,
decision_reason TEXT NOT NULL DEFAULT '',
send_mode TEXT NOT NULL DEFAULT '',
delivery_key TEXT NOT NULL DEFAULT '',
status TEXT NOT NULL DEFAULT '',
failure_reason TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
sent_at TEXT,
FOREIGN KEY (emote_id) REFERENCES emotes(id) ON DELETE SET NULL,
FOREIGN KEY (character_id) REFERENCES characters(id) ON DELETE CASCADE,
FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
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
    owner_user_id TEXT NOT NULL DEFAULT '',
    scope_type TEXT NOT NULL DEFAULT 'global',
    scope_id TEXT NOT NULL DEFAULT '',
    lifecycle_status TEXT NOT NULL DEFAULT 'registered',
    health_status TEXT NOT NULL DEFAULT 'unknown',
    last_error_code TEXT NOT NULL DEFAULT '',
    last_error_at TEXT NOT NULL DEFAULT '',
    enabled_at TEXT NOT NULL DEFAULT '',
    disabled_at TEXT NOT NULL DEFAULT ''
);

--- 来源: extensions.go
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
CREATE TABLE IF NOT EXISTS extension_capability_grants (
id TEXT PRIMARY KEY,
extension_id TEXT NOT NULL,
capability TEXT NOT NULL,
decision TEXT NOT NULL DEFAULT 'deny',
scope_type TEXT NOT NULL DEFAULT 'global',
scope_id TEXT NOT NULL DEFAULT '',
expires_at TEXT NOT NULL DEFAULT '',
consumed_at TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
UNIQUE(extension_id, capability, scope_type, scope_id)
);

-- 来源: extensions.go
CREATE TABLE IF NOT EXISTS extension_configs (
id TEXT PRIMARY KEY,
extension_id TEXT NOT NULL,
scope_type TEXT NOT NULL DEFAULT 'global',
scope_id TEXT NOT NULL DEFAULT '',
config_json TEXT NOT NULL DEFAULT '{}',
config_version INTEGER NOT NULL DEFAULT 1,
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
    archived_at TEXT NOT NULL DEFAULT '',
UNIQUE(extension_id, scope_type, scope_id)
);

-- 来源: extensions.go
CREATE TABLE IF NOT EXISTS extension_runs (
run_id TEXT PRIMARY KEY,
extension_id TEXT NOT NULL,
extension_version TEXT NOT NULL DEFAULT '',
skill_id TEXT NOT NULL,
user_id TEXT NOT NULL DEFAULT '',
character_id TEXT NOT NULL DEFAULT '',
conversation_id TEXT NOT NULL DEFAULT '',
channel TEXT NOT NULL DEFAULT '',
trigger TEXT NOT NULL DEFAULT '',
status TEXT NOT NULL DEFAULT 'pending',
input_summary TEXT NOT NULL DEFAULT '{}',
output_summary TEXT NOT NULL DEFAULT '{}',
side_effects_json TEXT NOT NULL DEFAULT '[]',
idempotency_key TEXT NOT NULL DEFAULT '',
started_at TEXT NOT NULL DEFAULT '',
finished_at TEXT NOT NULL DEFAULT '',
duration_ms INTEGER NOT NULL DEFAULT 0,
error_code TEXT NOT NULL DEFAULT '',
error_detail TEXT NOT NULL DEFAULT '',
trace_id TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
UNIQUE(skill_id, character_id, conversation_id, idempotency_key)
);

-- 来源: extension_agent_skills.go
CREATE TABLE IF NOT EXISTS extension_agent_skill_metadata (
id TEXT PRIMARY KEY,
extension_id TEXT NOT NULL UNIQUE,
user_id TEXT NOT NULL,
name TEXT NOT NULL,
description TEXT NOT NULL,
license TEXT NOT NULL DEFAULT '',
compatibility TEXT NOT NULL DEFAULT '',
metadata_json TEXT NOT NULL DEFAULT '{}',
allowed_tools TEXT NOT NULL DEFAULT '',
display_name TEXT NOT NULL DEFAULT '',
short_description TEXT NOT NULL DEFAULT '',
default_prompt TEXT NOT NULL DEFAULT '',
openai_metadata_json TEXT NOT NULL DEFAULT '{}',
scope_type TEXT NOT NULL,
scope_id TEXT NOT NULL DEFAULT '',
source TEXT NOT NULL,
compatibility_status TEXT NOT NULL,
compatibility_report_json TEXT NOT NULL DEFAULT '{}',
content_hash TEXT NOT NULL,
artifact_id TEXT NOT NULL,
raw_frontmatter_json TEXT NOT NULL DEFAULT '{}',
extra_frontmatter_json TEXT NOT NULL DEFAULT '{}',
resource_index_json TEXT NOT NULL DEFAULT '[]',
tool_mappings_json TEXT NOT NULL DEFAULT '[]',
scripts_present INTEGER NOT NULL DEFAULT 0,
scripts_required INTEGER NOT NULL DEFAULT 0,
enabled INTEGER NOT NULL DEFAULT 0,
created_at TEXT NOT NULL,
updated_at TEXT NOT NULL,
removed_at TEXT NOT NULL DEFAULT '',
UNIQUE(user_id, name, scope_type, scope_id)
);

-- 来源: extension_agent_skills.go
CREATE TABLE IF NOT EXISTS extension_agent_skill_activations (
id TEXT PRIMARY KEY,
activation_id TEXT NOT NULL UNIQUE,
extension_id TEXT NOT NULL,
user_id TEXT NOT NULL,
character_id TEXT NOT NULL DEFAULT '',
conversation_id TEXT NOT NULL DEFAULT '',
channel TEXT NOT NULL DEFAULT '',
trigger_type TEXT NOT NULL,
explicit INTEGER NOT NULL DEFAULT 0,
status TEXT NOT NULL,
loaded_tokens INTEGER NOT NULL DEFAULT 0,
resource_reads INTEGER NOT NULL DEFAULT 0,
resource_paths_json TEXT NOT NULL DEFAULT '[]',
trace_id TEXT NOT NULL DEFAULT '',
error_code TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL,
agent_skill_name TEXT NOT NULL DEFAULT '',
source TEXT NOT NULL DEFAULT '',
scope_type TEXT NOT NULL DEFAULT '',
compatibility_status TEXT NOT NULL DEFAULT '',
scripts_used INTEGER NOT NULL DEFAULT 0,
tool_mappings_json TEXT NOT NULL DEFAULT '[]',
instruction_position TEXT NOT NULL DEFAULT '',
token_limit_hit INTEGER NOT NULL DEFAULT 0
);

--- 来源: extension_owned_resources.go
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
user_id TEXT NOT NULL,
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
CREATE TABLE IF NOT EXISTS extension_package_installations (
id TEXT PRIMARY KEY,
extension_id TEXT NOT NULL DEFAULT '',
extension_version TEXT NOT NULL DEFAULT '',
operation TEXT NOT NULL,
source TEXT NOT NULL DEFAULT '',
package_hash TEXT NOT NULL DEFAULT '',
signature_status TEXT NOT NULL DEFAULT '',
signer_fingerprint TEXT NOT NULL DEFAULT '',
previous_version TEXT NOT NULL DEFAULT '',
target_version TEXT NOT NULL DEFAULT '',
user_id TEXT NOT NULL,
scope_type TEXT NOT NULL,
scope_id TEXT NOT NULL DEFAULT '',
status TEXT NOT NULL,
error_code TEXT NOT NULL DEFAULT '',
trace_id TEXT NOT NULL,
created_at TEXT NOT NULL,
completed_at TEXT NOT NULL DEFAULT ''
);

-- 来源: extension_packages.go
CREATE TABLE IF NOT EXISTS extension_package_signers (
id TEXT PRIMARY KEY,
fingerprint TEXT NOT NULL UNIQUE,
public_key TEXT NOT NULL,
algorithm TEXT NOT NULL,
display_name TEXT NOT NULL DEFAULT '',
trusted INTEGER NOT NULL DEFAULT 0,
trusted_at TEXT NOT NULL DEFAULT '',
revoked_at TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL,
updated_at TEXT NOT NULL
);

-- 来源: extension_packages.go
CREATE TABLE IF NOT EXISTS extension_version_dependencies (
extension_id TEXT NOT NULL,
extension_version TEXT NOT NULL,
dependency_id TEXT NOT NULL,
version_constraint TEXT NOT NULL DEFAULT '',
required INTEGER NOT NULL DEFAULT 1,
created_at TEXT NOT NULL,
UNIQUE(extension_id, extension_version, dependency_id)
);

-- 来源: extension_packages.go
CREATE TABLE IF NOT EXISTS extension_package_exports (
id TEXT PRIMARY KEY,
user_id TEXT NOT NULL,
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
CREATE TABLE IF NOT EXISTS extension_workshop_sessions (
id TEXT PRIMARY KEY,
user_id TEXT NOT NULL,
character_id TEXT NOT NULL DEFAULT '',
status TEXT NOT NULL DEFAULT 'draft',
requirement TEXT NOT NULL,
current_revision INTEGER NOT NULL DEFAULT 0,
current_draft_id TEXT NOT NULL DEFAULT '',
validation_summary TEXT NOT NULL DEFAULT '{}',
risk_summary TEXT NOT NULL DEFAULT '{}',
test_summary TEXT NOT NULL DEFAULT '{}',
installed_skill_id TEXT NOT NULL DEFAULT '',
installed_version TEXT NOT NULL DEFAULT '',
permission_confirmation_json TEXT NOT NULL DEFAULT '{}',
permission_revision INTEGER NOT NULL DEFAULT 0,
permission_checksum TEXT NOT NULL DEFAULT '',
test_permission_confirmation_json TEXT NOT NULL DEFAULT '{}',
test_permission_revision INTEGER NOT NULL DEFAULT 0,
test_permission_checksum TEXT NOT NULL DEFAULT '',
lock_version INTEGER NOT NULL DEFAULT 1,
created_at TEXT NOT NULL,
updated_at TEXT NOT NULL,
archived_at TEXT NOT NULL DEFAULT ''
);

-- 来源: extension_workshop.go
CREATE TABLE IF NOT EXISTS extension_workshop_revisions (
id TEXT PRIMARY KEY,
session_id TEXT NOT NULL,
revision INTEGER NOT NULL,
raw_model_output TEXT NOT NULL DEFAULT '{}',
plan_json TEXT NOT NULL DEFAULT '{}',
raw_draft_json TEXT NOT NULL,
normalized_draft_json TEXT NOT NULL,
manifest_json TEXT NOT NULL,
input_schema_json TEXT NOT NULL,
output_schema_json TEXT NOT NULL,
config_schema_json TEXT NOT NULL DEFAULT '{}',
workflow_json TEXT NOT NULL,
compiled_workflow_json TEXT NOT NULL,
capability_analysis_json TEXT NOT NULL DEFAULT '{}',
risk_analysis_json TEXT NOT NULL DEFAULT '{}',
validation_result_json TEXT NOT NULL DEFAULT '{}',
workflow_checksum TEXT NOT NULL,
model_provider TEXT NOT NULL DEFAULT '',
model_name TEXT NOT NULL DEFAULT '',
model_input_summary_json TEXT NOT NULL DEFAULT '{}',
model_output_summary_json TEXT NOT NULL DEFAULT '{}',
created_at TEXT NOT NULL,
UNIQUE(session_id, revision)
);

-- 来源: extension_workshop.go
CREATE TABLE IF NOT EXISTS extension_workshop_test_runs (
id TEXT PRIMARY KEY,
test_run_id TEXT NOT NULL UNIQUE,
user_id TEXT NOT NULL,
character_id TEXT NOT NULL DEFAULT '',
session_id TEXT NOT NULL,
revision INTEGER NOT NULL,
workflow_checksum TEXT NOT NULL,
mode TEXT NOT NULL,
status TEXT NOT NULL,
input_summary TEXT NOT NULL DEFAULT '{}',
output_summary TEXT NOT NULL DEFAULT '{}',
step_results_json TEXT NOT NULL DEFAULT '[]',
assertion_results_json TEXT NOT NULL DEFAULT '[]',
side_effects_json TEXT NOT NULL DEFAULT '[]',
capabilities_json TEXT NOT NULL DEFAULT '[]',
warnings_json TEXT NOT NULL DEFAULT '[]',
error_code TEXT NOT NULL DEFAULT '',
error_detail TEXT NOT NULL DEFAULT '',
trace_id TEXT NOT NULL DEFAULT '',
started_at TEXT NOT NULL,
finished_at TEXT NOT NULL,
duration_ms INTEGER NOT NULL DEFAULT 0,
created_at TEXT NOT NULL
);

-- 来源: extension_workshop.go
CREATE TABLE IF NOT EXISTS extension_artifacts (
id TEXT PRIMARY KEY,
artifact_id TEXT NOT NULL UNIQUE,
extension_id TEXT NOT NULL,
extension_version TEXT NOT NULL,
source TEXT NOT NULL DEFAULT 'workshop',
session_id TEXT NOT NULL,
revision INTEGER NOT NULL,
manifest_json TEXT NOT NULL,
workflow_json TEXT NOT NULL,
schemas_json TEXT NOT NULL,
compiled_workflow_json TEXT NOT NULL,
tests_json TEXT NOT NULL DEFAULT '[]',
readme_text TEXT NOT NULL DEFAULT '',
checksum TEXT NOT NULL,
size_bytes INTEGER NOT NULL,
created_at TEXT NOT NULL,
archived_at TEXT NOT NULL DEFAULT '',
    artifact_kind TEXT NOT NULL DEFAULT 'workflow',
    content_blob BLOB,
    resource_index_json TEXT NOT NULL DEFAULT '[]',
    artifact_status TEXT NOT NULL DEFAULT 'active',
    operation_id TEXT NOT NULL DEFAULT '',
UNIQUE(extension_id, extension_version)
);

-- 来源: interaction_records_create.go
CREATE TABLE IF NOT EXISTS interaction_records (
				id TEXT PRIMARY KEY,
				user_id TEXT,
				character_id TEXT,
				conversation_id TEXT,
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
				causation_id TEXT DEFAULT ''
			);

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
response_group_id TEXT NOT NULL DEFAULT '',
delivery_sequence INTEGER NOT NULL DEFAULT 0
);

-- 来源: plugin_runtime.go
CREATE TABLE IF NOT EXISTS extension_states (
id TEXT PRIMARY KEY,
extension_id TEXT NOT NULL,
scope_type TEXT NOT NULL DEFAULT 'global',
scope_id TEXT NOT NULL DEFAULT '',
schema_version TEXT NOT NULL,
revision INTEGER NOT NULL DEFAULT 1,
state_json TEXT NOT NULL DEFAULT '{}',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
UNIQUE(extension_id, scope_type, scope_id)
);

-- 来源: plugin_runtime.go
CREATE TABLE IF NOT EXISTS extension_state_revisions (
id TEXT PRIMARY KEY,
extension_id TEXT NOT NULL,
scope_type TEXT NOT NULL,
scope_id TEXT NOT NULL DEFAULT '',
schema_version TEXT NOT NULL,
revision INTEGER NOT NULL,
state_json TEXT NOT NULL DEFAULT '{}',
created_at TEXT NOT NULL DEFAULT '',
UNIQUE(extension_id, scope_type, scope_id, revision)
);

-- 来源: plugin_runtime.go
CREATE TABLE IF NOT EXISTS extension_events (
id TEXT PRIMARY KEY,
event_id TEXT NOT NULL UNIQUE,
source TEXT NOT NULL,
type TEXT NOT NULL,
subject TEXT NOT NULL DEFAULT '',
data_json TEXT NOT NULL DEFAULT '{}',
trace_id TEXT NOT NULL DEFAULT '',
correlation_id TEXT NOT NULL DEFAULT '',
causation_id TEXT NOT NULL DEFAULT '',
depth INTEGER NOT NULL DEFAULT 0,
created_at TEXT NOT NULL DEFAULT ''
);

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
CREATE TABLE IF NOT EXISTS extension_schedules (
id TEXT PRIMARY KEY,
plugin_id TEXT NOT NULL,
schedule_id TEXT NOT NULL,
scope_type TEXT NOT NULL DEFAULT 'global',
scope_id TEXT NOT NULL DEFAULT '',
schedule_type TEXT NOT NULL,
expression TEXT NOT NULL,
timezone TEXT NOT NULL DEFAULT 'UTC',
payload_json TEXT NOT NULL DEFAULT '{}',
enabled INTEGER NOT NULL DEFAULT 1,
next_run_at TEXT NOT NULL DEFAULT '',
last_run_at TEXT NOT NULL DEFAULT '',
last_status TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT '',
UNIQUE(plugin_id, schedule_id, scope_type, scope_id)
);

-- 来源: plugin_runtime.go
CREATE TABLE IF NOT EXISTS extension_plugin_runs (
run_id TEXT PRIMARY KEY,
plugin_id TEXT NOT NULL,
plugin_version TEXT NOT NULL,
hook TEXT NOT NULL,
character_id TEXT NOT NULL DEFAULT '',
conversation_id TEXT NOT NULL DEFAULT '',
channel TEXT NOT NULL DEFAULT '',
status TEXT NOT NULL,
duration_ms INTEGER NOT NULL DEFAULT 0,
error_code TEXT NOT NULL DEFAULT '',
trace_id TEXT NOT NULL DEFAULT '',
circuit_state TEXT NOT NULL DEFAULT 'closed',
created_at TEXT NOT NULL DEFAULT ''
);

-- 来源: plugin_runtime.go
CREATE TABLE IF NOT EXISTS extension_audits (
id TEXT PRIMARY KEY,
extension_id TEXT NOT NULL,
action TEXT NOT NULL,
scope_type TEXT NOT NULL DEFAULT 'global',
scope_id TEXT NOT NULL DEFAULT '',
detail_json TEXT NOT NULL DEFAULT '{}',
trace_id TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT ''
);

-- 来源: qdrant_collections.go
CREATE TABLE IF NOT EXISTS qdrant_collection_versions (
				collection_name TEXT PRIMARY KEY,
				vector_dim INTEGER NOT NULL,
				distance TEXT NOT NULL DEFAULT 'Cosine',
				created_at TEXT DEFAULT (datetime('now'))
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
				schema_version TEXT PRIMARY KEY,
				entity_types TEXT NOT NULL DEFAULT '',
				edge_types TEXT NOT NULL DEFAULT '',
				created_at TEXT DEFAULT (datetime('now'))
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
scope_type TEXT NOT NULL DEFAULT 'user',
user_id TEXT NOT NULL DEFAULT '',
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
user_id TEXT NOT NULL DEFAULT '',
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
);

-- 来源: temporal_relationship_time.go
CREATE TABLE IF NOT EXISTS temporal_relationship_presence_states (
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
);

-- 来源: temporal_relationship_time.go
CREATE TABLE IF NOT EXISTS temporal_cadence_samples (
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
);

-- 来源: temporal_relationship_time.go
CREATE TABLE IF NOT EXISTS temporal_reunion_episodes (
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
);

-- 来源: temporal_relationship_time.go
CREATE TABLE IF NOT EXISTS temporal_interaction_receipts (
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
);

-- 来源: temporal_relationship_time.go
CREATE TABLE IF NOT EXISTS temporal_effect_ledger (
id TEXT PRIMARY KEY,
effect_key TEXT NOT NULL DEFAULT '',
effect_type TEXT NOT NULL DEFAULT '',
user_id TEXT NOT NULL DEFAULT 'default',
character_id TEXT NOT NULL DEFAULT '',
reunion_episode_id TEXT NOT NULL DEFAULT '',
interaction_id TEXT NOT NULL DEFAULT '',
payload_json TEXT NOT NULL DEFAULT '{}',
applied_at_utc TEXT NOT NULL DEFAULT ''
);

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

-- 来源: trigger_history.go
CREATE TABLE IF NOT EXISTS trigger_histories (
				id TEXT PRIMARY KEY,
				trigger_id TEXT NOT NULL DEFAULT '',
				trigger_type TEXT NOT NULL DEFAULT '',
				title TEXT NOT NULL DEFAULT '',
				channel TEXT NOT NULL DEFAULT 'web',
				state TEXT NOT NULL DEFAULT 'pending',
				priority TEXT NOT NULL DEFAULT 'normal',
				reason TEXT NOT NULL DEFAULT '',
				attempt_count INTEGER DEFAULT 0,
				last_error TEXT DEFAULT '',
				created_at TEXT DEFAULT '',
				updated_at TEXT DEFAULT ''
			);

--- 来源: backup.go + backup_tables_upgrade.go
CREATE TABLE IF NOT EXISTS backup_records (
id TEXT PRIMARY KEY,
backup_path TEXT NOT NULL DEFAULT '',
backup_size INTEGER DEFAULT 0,
checksum TEXT DEFAULT '',
status TEXT NOT NULL DEFAULT 'pending',
started_at TEXT DEFAULT '',
finished_at TEXT DEFAULT '',
error_message TEXT DEFAULT '',
purpose TEXT DEFAULT 'user',
format_version INTEGER DEFAULT 1,
profile TEXT DEFAULT 'full',
scope TEXT DEFAULT 'all',
encrypted INTEGER DEFAULT 0,
manifest_checksum TEXT DEFAULT '',
app_version TEXT DEFAULT '',
schema_fingerprint TEXT DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_backup_records_purpose ON backup_records(purpose);
CREATE INDEX IF NOT EXISTS idx_backup_records_created ON backup_records(started_at);

--- 来源: backup.go + backup_tables_upgrade.go
CREATE TABLE IF NOT EXISTS backup_contents (
id TEXT PRIMARY KEY,
backup_id TEXT NOT NULL DEFAULT '',
table_name TEXT NOT NULL DEFAULT '',
row_count INTEGER DEFAULT 0,
checksum TEXT DEFAULT '',
component_id TEXT DEFAULT '',
kind TEXT DEFAULT '',
logical_name TEXT DEFAULT '',
size_bytes INTEGER DEFAULT 0,
item_count INTEGER DEFAULT 0,
required INTEGER DEFAULT 1,
source_of_truth INTEGER DEFAULT 0,
rebuildable INTEGER DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_backup_contents_backup_id ON backup_contents(backup_id);
CREATE INDEX IF NOT EXISTS idx_backup_contents_component ON backup_contents(component_id);

--- 来源: legacy_data_migration.go
CREATE TABLE IF NOT EXISTS migration_checkpoints (
id TEXT PRIMARY KEY,
check_type TEXT NOT NULL DEFAULT '',
scope TEXT NOT NULL DEFAULT '',
status TEXT NOT NULL DEFAULT 'pending',
total_count INTEGER DEFAULT 0,
migrated_count INTEGER DEFAULT 0,
error_count INTEGER DEFAULT 0,
started_at TEXT DEFAULT '',
finished_at TEXT DEFAULT '',
description TEXT DEFAULT ''
);

--- 来源: message_sequence_checkpoint.go
CREATE TABLE IF NOT EXISTS message_sequence_checkpoints (
id TEXT PRIMARY KEY,
character_id TEXT NOT NULL DEFAULT '',
conversation_id TEXT NOT NULL DEFAULT '',
last_sequence INTEGER DEFAULT 0,
last_message_id TEXT NOT NULL DEFAULT '',
processed_count INTEGER DEFAULT 0,
status TEXT NOT NULL DEFAULT 'active',
updated_at TEXT DEFAULT ''
);

--- 来源: migrations.go
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

--- 来源: migrations.go
CREATE TABLE IF NOT EXISTS psyche_events (
id TEXT PRIMARY KEY,
character_id TEXT NOT NULL DEFAULT '',
event_type TEXT NOT NULL DEFAULT '',
event_data TEXT DEFAULT '{}',
created_at TEXT DEFAULT ''
);

--- 来源: migrations.go
CREATE TABLE IF NOT EXISTS psyche_snapshots (
id TEXT PRIMARY KEY,
character_id TEXT NOT NULL DEFAULT '',
snapshot_data TEXT DEFAULT '{}',
created_at TEXT DEFAULT ''
);

--- 来源: migrations.go
CREATE TABLE IF NOT EXISTS relationship_states (
id TEXT PRIMARY KEY,
character_id TEXT NOT NULL DEFAULT '',
relation_type TEXT NOT NULL DEFAULT '',
relation_data TEXT DEFAULT '{}',
created_at TEXT DEFAULT '',
updated_at TEXT DEFAULT ''
);

--- 来源: migrations.go
CREATE TABLE IF NOT EXISTS relationship_events (
id TEXT PRIMARY KEY,
character_id TEXT NOT NULL DEFAULT '',
event_type TEXT NOT NULL DEFAULT '',
event_data TEXT DEFAULT '{}',
created_at TEXT DEFAULT ''
);

--- 来源: tombstone_rebuild.go
CREATE TABLE IF NOT EXISTS tombstone_rebuild_tracker (
id TEXT PRIMARY KEY,
target_id TEXT NOT NULL DEFAULT '',
target_type TEXT NOT NULL DEFAULT '',
rebuild_type TEXT NOT NULL DEFAULT 'full',
status TEXT NOT NULL DEFAULT 'pending',
attempts INTEGER DEFAULT 0,
last_error TEXT DEFAULT '',
started_at TEXT DEFAULT '',
finished_at TEXT DEFAULT '',
created_at TEXT DEFAULT ''
);

--- 来源: mcp_client.go
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
normalized_identity TEXT NOT NULL UNIQUE,
configuration_hash TEXT NOT NULL DEFAULT '',
last_connected_at TEXT NOT NULL DEFAULT '',
last_error_code TEXT NOT NULL DEFAULT '',
last_error_message TEXT NOT NULL DEFAULT '',
created_at TEXT NOT NULL DEFAULT '',
updated_at TEXT NOT NULL DEFAULT ''
);

--- 来源: mcp_client.go
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

--- 来源: mcp_client.go
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

--- 来源: mcp_client.go
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

--- 来源: mcp_client.go
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

--- 来源: mcp_client.go
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

--- 来源: mcp_client.go
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

--- 来源: mcp_client.go
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

--- 来源: mcp_client.go
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

--- 来源: mcp_client.go
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

--- 来源: mcp_client.go
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

--- 来源: mcp_client.go
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

--- 来源: mcp_client.go
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

CREATE TABLE IF NOT EXISTS desktop_pet_active_quality_evaluation_bindings (
  id TEXT PRIMARY KEY,
  action_revision_id TEXT NOT NULL,
  profile_id TEXT NOT NULL DEFAULT '',
  active_evaluation_id TEXT NOT NULL,
  binding_revision INTEGER NOT NULL DEFAULT 1,
  bound_at TEXT NOT NULL DEFAULT '',
  created_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dpaqeb_revision_profile ON desktop_pet_active_quality_evaluation_bindings(action_revision_id, profile_id);

CREATE TABLE IF NOT EXISTS desktop_pet_quality_commit_journals (
  id TEXT PRIMARY KEY,
  evaluation_id TEXT NOT NULL,
  commit_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  steps_completed TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  created_at TEXT DEFAULT (datetime('now')),
  completed_at TEXT DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_dpqcj_eval ON desktop_pet_quality_commit_journals(evaluation_id);

CREATE TABLE IF NOT EXISTS desktop_pet_quality_review_decisions (
  id TEXT PRIMARY KEY,
  evaluation_id TEXT NOT NULL,
  action_revision_id TEXT NOT NULL,
  decision TEXT NOT NULL,
  reason TEXT NOT NULL DEFAULT '',
  reviewer TEXT NOT NULL DEFAULT '',
  reviewed_at TEXT NOT NULL DEFAULT '',
  created_at TEXT DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dpqrd_eval ON desktop_pet_quality_review_decisions(evaluation_id);
CREATE INDEX IF NOT EXISTS idx_dpqrd_revision ON desktop_pet_quality_review_decisions(action_revision_id);

CREATE TABLE IF NOT EXISTS desktop_pet_quality_measurement_cache (
  id TEXT PRIMARY KEY,
  frame_artifact_id TEXT NOT NULL,
  content_hash TEXT NOT NULL,
  measurement_version TEXT NOT NULL DEFAULT '',
  width INTEGER NOT NULL DEFAULT 0,
  height INTEGER NOT NULL DEFAULT 0,
  has_alpha_channel INTEGER NOT NULL DEFAULT 0,
  alpha_coverage REAL NOT NULL DEFAULT 0,
  fully_transparent_ratio REAL NOT NULL DEFAULT 0,
  semi_transparent_ratio REAL NOT NULL DEFAULT 0,
  opaque_ratio REAL NOT NULL DEFAULT 0,
  decodable INTEGER NOT NULL DEFAULT 0,
  mime_type TEXT NOT NULL DEFAULT '',
  pixel_hash TEXT NOT NULL DEFAULT '',
  measurements_json TEXT NOT NULL DEFAULT '{}',
  created_at TEXT DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dpqmc_artifact_hash_ver ON desktop_pet_quality_measurement_cache(frame_artifact_id, content_hash, measurement_version);

CREATE TABLE IF NOT EXISTS desktop_pet_quality_outbox_events (
  id TEXT PRIMARY KEY,
  event_type TEXT NOT NULL,
  payload_json TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  created_at TEXT DEFAULT (datetime('now')),
  published_at TEXT DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_dpqoe_status ON desktop_pet_quality_outbox_events(status);

--- 来源: desktop_pet_runtime_protocol_v2.go
CREATE TABLE IF NOT EXISTS desktop_pet_runtime_sessions (
  id TEXT DEFAULT '',
  runtime_instance_id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL DEFAULT '',
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
  last_event_sequence INTEGER NOT NULL DEFAULT 0
);

--- 来源: desktop_pet_runtime_protocol_v2.go
CREATE TABLE IF NOT EXISTS desktop_pet_runtime_command_acks (
  ack_id TEXT PRIMARY KEY,
  command_id TEXT NOT NULL DEFAULT '',
  runtime_instance_id TEXT NOT NULL DEFAULT '',
  command_sequence INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT '',
  reject_reason TEXT NOT NULL DEFAULT '',
  received_at TEXT NOT NULL DEFAULT '',
  created_at TEXT DEFAULT (datetime('now')),
  UNIQUE(runtime_instance_id, command_id)
);

--- 来源: desktop_pet_runtime_protocol_v2.go
CREATE TABLE IF NOT EXISTS desktop_pet_runtime_event_inbox (
  inbox_id TEXT PRIMARY KEY,
  event_id TEXT NOT NULL DEFAULT '',
  runtime_instance_id TEXT NOT NULL DEFAULT '',
  event_sequence INTEGER NOT NULL DEFAULT 0,
  event_type TEXT NOT NULL DEFAULT '',
  payload_hash TEXT NOT NULL DEFAULT '',
  processed_at TEXT NOT NULL DEFAULT '',
  created_at TEXT DEFAULT (datetime('now')),
  UNIQUE(runtime_instance_id, event_id),
  UNIQUE(runtime_instance_id, event_sequence)
);

--- 来源: desktop_pet_runtime_protocol_v2.go
CREATE TABLE IF NOT EXISTS desktop_pet_runtime_protocol_errors (
  error_id TEXT PRIMARY KEY,
  runtime_instance_id TEXT NOT NULL DEFAULT '',
  error_code TEXT NOT NULL DEFAULT '',
  error_message TEXT NOT NULL DEFAULT '',
  component TEXT NOT NULL DEFAULT '',
  recoverable INTEGER NOT NULL DEFAULT 0,
  command_id TEXT NOT NULL DEFAULT '',
  playback_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  occurred_at TEXT NOT NULL DEFAULT '',
  created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS desktop_pet_runtime_desired_states (
    id TEXT PRIMARY KEY,
    installation_id TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
    device_id TEXT NOT NULL DEFAULT '',
    desired_enabled INTEGER NOT NULL DEFAULT 0,
    desired_visible INTEGER NOT NULL DEFAULT 0,
    desired_release_id TEXT NOT NULL DEFAULT '',
    desired_action_key TEXT NOT NULL DEFAULT '',
    position_x REAL,
    position_y REAL,
    scale REAL NOT NULL DEFAULT 1.0,
    opacity REAL NOT NULL DEFAULT 1.0,
    always_on_top INTEGER NOT NULL DEFAULT 1,
    click_through_mode TEXT NOT NULL DEFAULT 'off',
    position_policy TEXT NOT NULL DEFAULT '',
    revision INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT DEFAULT (datetime('now')),
    created_at TEXT DEFAULT (datetime('now')),
    UNIQUE(installation_id)
);
CREATE INDEX IF NOT EXISTS idx_dprds_installation ON desktop_pet_runtime_desired_states(installation_id);
CREATE INDEX IF NOT EXISTS idx_dprds_user ON desktop_pet_runtime_desired_states(user_id);

CREATE TABLE IF NOT EXISTS desktop_pet_installation_commit_journals (
    id TEXT PRIMARY KEY,
    operation_id TEXT NOT NULL DEFAULT '',
    installation_id TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
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
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_dpicj_operation ON desktop_pet_installation_commit_journals(operation_id);
CREATE INDEX IF NOT EXISTS idx_dpicj_installation ON desktop_pet_installation_commit_journals(installation_id);
CREATE INDEX IF NOT EXISTS idx_dpicj_state ON desktop_pet_installation_commit_journals(state);

CREATE TABLE IF NOT EXISTS desktop_pet_installation_switch_journals (
    id TEXT PRIMARY KEY,
    operation_id TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
    device_id TEXT NOT NULL DEFAULT '',
    old_installation_id TEXT NOT NULL DEFAULT '',
    new_installation_id TEXT NOT NULL DEFAULT '',
    old_desired_revision INTEGER NOT NULL DEFAULT 0,
    new_desired_revision INTEGER NOT NULL DEFAULT 0,
    binding_revision INTEGER NOT NULL DEFAULT 0,
    state TEXT NOT NULL DEFAULT 'pending',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_dpisj_operation ON desktop_pet_installation_switch_journals(operation_id);

CREATE TABLE IF NOT EXISTS desktop_pet_legacy_installation_mappings (
    id TEXT PRIMARY KEY,
    legacy_installation_id TEXT NOT NULL DEFAULT '',
    new_installation_id TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
    legacy_package_id TEXT NOT NULL DEFAULT '',
    pet_id TEXT NOT NULL DEFAULT '',
    release_id TEXT NOT NULL DEFAULT '',
    migration_status TEXT NOT NULL DEFAULT 'pending',
    source_content_hash TEXT NOT NULL DEFAULT '',
    error_code TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    UNIQUE(legacy_installation_id)
);
CREATE INDEX IF NOT EXISTS idx_dplim_legacy ON desktop_pet_legacy_installation_mappings(legacy_installation_id);
CREATE INDEX IF NOT EXISTS idx_dplim_user ON desktop_pet_legacy_installation_mappings(user_id);
CREATE INDEX IF NOT EXISTS idx_dplim_status ON desktop_pet_legacy_installation_mappings(migration_status);

CREATE TABLE IF NOT EXISTS desktop_pet_generation_task_plans (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL DEFAULT '',
    schema_version INTEGER NOT NULL DEFAULT 1,
    plan_hash TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    config_id INTEGER NOT NULL DEFAULT 0,
    config_revision TEXT NOT NULL DEFAULT '',
    capability_snapshot_json TEXT NOT NULL DEFAULT '{}',
    capability_snapshot_hash TEXT NOT NULL DEFAULT '',
    reference_asset_id TEXT NOT NULL DEFAULT '',
    cost_estimate_json TEXT NOT NULL DEFAULT '{}',
    planned_primary_request_count INTEGER NOT NULL DEFAULT 0,
    planned_max_provider_call_count INTEGER NOT NULL DEFAULT 0,
    plan_json TEXT NOT NULL DEFAULT '{}',
    frozen_at TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_task_plans_task_id ON desktop_pet_generation_task_plans(task_id);

CREATE TABLE IF NOT EXISTS desktop_pet_generation_action_plans (
    id TEXT PRIMARY KEY,
    task_plan_id TEXT NOT NULL DEFAULT '',
    task_id TEXT NOT NULL DEFAULT '',
    task_action_id TEXT NOT NULL DEFAULT '',
    action_key TEXT NOT NULL DEFAULT '',
    schema_version INTEGER NOT NULL DEFAULT 1,
    plan_hash TEXT NOT NULL DEFAULT '',
    mode TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    config_id INTEGER NOT NULL DEFAULT 0,
    config_revision TEXT NOT NULL DEFAULT '',
    capability_hash TEXT NOT NULL DEFAULT '',
    reference_asset_id TEXT NOT NULL DEFAULT '',
    layout_json TEXT NOT NULL DEFAULT '{}',
    layout_hash TEXT NOT NULL DEFAULT '',
    prompt_snapshot TEXT NOT NULL DEFAULT '',
    prompt_hash TEXT NOT NULL DEFAULT '',
    negative_prompt_snapshot TEXT NOT NULL DEFAULT '',
    negative_prompt_hash TEXT NOT NULL DEFAULT '',
    seed_policy TEXT NOT NULL DEFAULT '',
    seed_value INTEGER,
    output_count INTEGER NOT NULL DEFAULT 1,
    target_frame_count INTEGER NOT NULL DEFAULT 0,
    planned_segment_count INTEGER NOT NULL DEFAULT 0,
    planned_primary_request_count INTEGER NOT NULL DEFAULT 0,
    planned_max_provider_call_count INTEGER NOT NULL DEFAULT 0,
    planned_call_count INTEGER NOT NULL DEFAULT 0,
    sheet_width INTEGER NOT NULL DEFAULT 0,
    sheet_height INTEGER NOT NULL DEFAULT 0,
    cell_width INTEGER NOT NULL DEFAULT 0,
    cell_height INTEGER NOT NULL DEFAULT 0,
    fallback_mode TEXT NOT NULL DEFAULT '',
    action_spec_version TEXT NOT NULL DEFAULT '',
    action_catalog_hash TEXT NOT NULL DEFAULT '',
    provider_config_hash TEXT NOT NULL DEFAULT '',
    safety_policy_version TEXT NOT NULL DEFAULT '',
    output_format TEXT NOT NULL DEFAULT '',
    plan_json TEXT NOT NULL DEFAULT '{}',
    frozen_at TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_action_plans_task_action_id ON desktop_pet_generation_action_plans(task_action_id);
CREATE INDEX IF NOT EXISTS idx_action_plans_task_plan_id ON desktop_pet_generation_action_plans(task_plan_id);
CREATE INDEX IF NOT EXISTS idx_action_plans_task_id ON desktop_pet_generation_action_plans(task_id);

CREATE TABLE IF NOT EXISTS desktop_pet_reference_asset_publish_journals (
    id TEXT PRIMARY KEY,
    reference_asset_id TEXT NOT NULL DEFAULT '',
    staging_path TEXT NOT NULL DEFAULT '',
    final_path TEXT NOT NULL DEFAULT '',
    source_storage_key TEXT NOT NULL DEFAULT '',
    normalized_storage_key TEXT NOT NULL DEFAULT '',
    content_hash TEXT NOT NULL DEFAULT '',
    journal_status TEXT NOT NULL DEFAULT 'staging',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT '',
    completed_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_ref_asset_publish_journals_reference_asset_id ON desktop_pet_reference_asset_publish_journals(reference_asset_id);

CREATE TABLE IF NOT EXISTS desktop_pet_generation_outbox (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL DEFAULT '',
    task_action_id TEXT NOT NULL DEFAULT '',
    attempt_id TEXT NOT NULL DEFAULT '',
    event_type TEXT NOT NULL DEFAULT '',
    payload_json TEXT NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'pending',
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 3,
    next_retry_at TEXT NOT NULL DEFAULT '',
    processed_at TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_generation_outbox_status ON desktop_pet_generation_outbox(status);
CREATE INDEX IF NOT EXISTS idx_generation_outbox_task_action_id ON desktop_pet_generation_outbox(task_action_id);
CREATE INDEX IF NOT EXISTS idx_generation_outbox_attempt_id ON desktop_pet_generation_outbox(attempt_id);

CREATE TABLE IF NOT EXISTS desktop_pet_action_streams (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  root_processing_task_id TEXT NOT NULL DEFAULT '',
  stream_key TEXT NOT NULL DEFAULT '',
  next_revision_number INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_das_stream_key ON desktop_pet_action_streams(stream_key);

CREATE TABLE IF NOT EXISTS desktop_pet_revision_bridge_journals (
  id TEXT PRIMARY KEY,
  processing_revision_id TEXT NOT NULL DEFAULT '',
  processing_action_id TEXT NOT NULL DEFAULT '',
  action_revision_id TEXT NOT NULL DEFAULT '',
  target_action_key TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'processing_published',
  last_error TEXT NOT NULL DEFAULT '',
  retry_count INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
  event_id TEXT NOT NULL DEFAULT '',
  user_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  payload_json TEXT NOT NULL DEFAULT '',
  payload_hash TEXT NOT NULL DEFAULT '',
  lease_owner TEXT NOT NULL DEFAULT '',
  lease_expires_at TEXT NOT NULL DEFAULT '',
  processed_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_drbj_status ON desktop_pet_revision_bridge_journals(status);
CREATE UNIQUE INDEX IF NOT EXISTS uq_drbj_proc_rev ON desktop_pet_revision_bridge_journals(processing_revision_id);

CREATE TABLE IF NOT EXISTS desktop_pet_active_action_revision_bindings (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  active_action_revision_id TEXT NOT NULL DEFAULT '',
  binding_revision INTEGER NOT NULL DEFAULT 0,
  bound_reason TEXT NOT NULL DEFAULT '',
  bound_by TEXT NOT NULL DEFAULT '',
  bound_at TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
  action_stream_id TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_daarb_action_stream_id ON desktop_pet_active_action_revision_bindings(action_stream_id);

CREATE TABLE IF NOT EXISTS desktop_pet_action_revision_binding_history (
  id TEXT PRIMARY KEY,
  action_stream_id TEXT NOT NULL DEFAULT '',
  binding_revision INTEGER NOT NULL DEFAULT 0,
  previous_revision_id TEXT NOT NULL DEFAULT '',
  new_revision_id TEXT NOT NULL DEFAULT '',
  reason TEXT NOT NULL DEFAULT '',
  actor TEXT NOT NULL DEFAULT '',
  occurred_at TEXT NOT NULL DEFAULT (datetime('now')),
  correlation_id TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_darbh_stream_rev ON desktop_pet_action_revision_binding_history(action_stream_id, binding_revision);

CREATE TABLE IF NOT EXISTS desktop_pet_action_revision_bridge_inbox (
  id TEXT PRIMARY KEY,
  event_id TEXT NOT NULL DEFAULT '',
  processing_revision_id TEXT NOT NULL DEFAULT '',
  payload_json TEXT NOT NULL DEFAULT '',
  payload_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'received',
  attempt_count INTEGER NOT NULL DEFAULT 0,
  lease_owner TEXT NOT NULL DEFAULT '',
  lease_expires_at TEXT NOT NULL DEFAULT '',
  last_error TEXT NOT NULL DEFAULT '',
  received_at TEXT NOT NULL DEFAULT (datetime('now')),
  processed_at TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_darbi_event_id ON desktop_pet_action_revision_bridge_inbox(event_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_darbi_proc_rev ON desktop_pet_action_revision_bridge_inbox(processing_revision_id);
CREATE INDEX IF NOT EXISTS idx_darbi_status ON desktop_pet_action_revision_bridge_inbox(status);

CREATE TABLE IF NOT EXISTS desktop_pet_action_revision_event_outbox (
  id TEXT PRIMARY KEY,
  event_id TEXT NOT NULL DEFAULT '',
  event_type TEXT NOT NULL DEFAULT '',
  aggregate_type TEXT NOT NULL DEFAULT 'action_revision',
  aggregate_id TEXT NOT NULL DEFAULT '',
  aggregate_sequence INTEGER NOT NULL DEFAULT 0,
  action_stream_id TEXT NOT NULL DEFAULT '',
  action_revision_id TEXT NOT NULL DEFAULT '',
  previous_revision_id TEXT NOT NULL DEFAULT '',
  processing_revision_id TEXT NOT NULL DEFAULT '',
  payload_json TEXT NOT NULL DEFAULT '',
  payload_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  attempt_count INTEGER NOT NULL DEFAULT 0,
  available_at TEXT NOT NULL DEFAULT (datetime('now')),
  last_error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  published_at TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dareo_event_id ON desktop_pet_action_revision_event_outbox(event_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dareo_agg_seq_type ON desktop_pet_action_revision_event_outbox(aggregate_id, aggregate_sequence, event_type);
CREATE INDEX IF NOT EXISTS idx_dareo_status ON desktop_pet_action_revision_event_outbox(status);

CREATE TABLE IF NOT EXISTS desktop_pet_legacy_revision_mappings (
  id TEXT PRIMARY KEY,
  legacy_revision_id TEXT NOT NULL DEFAULT '',
  new_action_revision_id TEXT NOT NULL DEFAULT '',
  action_stream_id TEXT NOT NULL DEFAULT '',
  legacy_revision_number INTEGER NOT NULL DEFAULT 0,
  migrated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dlrm_legacy_rev ON desktop_pet_legacy_revision_mappings(legacy_revision_id);

CREATE TABLE IF NOT EXISTS desktop_pet_legacy_binding_mappings (
  id TEXT PRIMARY KEY,
  legacy_processing_task_id TEXT NOT NULL DEFAULT '',
  legacy_action_key TEXT NOT NULL DEFAULT '',
  legacy_revision_id TEXT NOT NULL DEFAULT '',
  new_binding_id TEXT NOT NULL DEFAULT '',
  action_stream_id TEXT NOT NULL DEFAULT '',
  migrated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dlbm_legacy ON desktop_pet_legacy_binding_mappings(legacy_processing_task_id, legacy_action_key);

CREATE TABLE IF NOT EXISTS desktop_pet_quality_evaluation_request_inbox (
  id TEXT PRIMARY KEY,
  event_id TEXT NOT NULL DEFAULT '',
  action_revision_id TEXT NOT NULL DEFAULT '',
  action_content_hash TEXT NOT NULL DEFAULT '',
  profile_id TEXT NOT NULL DEFAULT '',
  profile_version TEXT NOT NULL DEFAULT '',
  rule_set_version TEXT NOT NULL DEFAULT '',
  idempotency_key TEXT NOT NULL DEFAULT '',
  payload_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'received',
  attempt_count INTEGER NOT NULL DEFAULT 0,
  lease_owner TEXT NOT NULL DEFAULT '',
  lease_expires_at TEXT NOT NULL DEFAULT '',
  last_error TEXT NOT NULL DEFAULT '',
  received_at TEXT NOT NULL DEFAULT '',
  processed_at TEXT NOT NULL DEFAULT '',
  created_at TEXT DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dpeqri_event ON desktop_pet_quality_evaluation_request_inbox(event_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dpeqri_idem ON desktop_pet_quality_evaluation_request_inbox(idempotency_key);

CREATE TABLE IF NOT EXISTS desktop_pet_quality_input_snapshots (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
  action_stream_id TEXT NOT NULL DEFAULT '',
  action_revision_id TEXT NOT NULL DEFAULT '',
  action_content_hash TEXT NOT NULL DEFAULT '',
  frame_set_hash TEXT NOT NULL DEFAULT '',
  binding_revision INTEGER NOT NULL DEFAULT 0,
  processing_revision_id TEXT NOT NULL DEFAULT '',
  action_key TEXT NOT NULL DEFAULT '',
  action_config_hash TEXT NOT NULL DEFAULT '',
  action_spec_hash TEXT NOT NULL DEFAULT '',
  playback_mode TEXT NOT NULL DEFAULT '',
  fps INTEGER NOT NULL DEFAULT 0,
  expected_frame_count INTEGER NOT NULL DEFAULT 0,
  frame_inputs_json TEXT NOT NULL DEFAULT '[]',
  snapshot_hash TEXT NOT NULL DEFAULT '',
  created_at TEXT DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dpqis_snap ON desktop_pet_quality_input_snapshots(action_revision_id, snapshot_hash);

CREATE TABLE IF NOT EXISTS desktop_pet_quality_measurement_sets (
  id TEXT PRIMARY KEY,
  action_revision_id TEXT NOT NULL DEFAULT '',
  action_content_hash TEXT NOT NULL DEFAULT '',
  frame_set_hash TEXT NOT NULL DEFAULT '',
  measurement_version TEXT NOT NULL DEFAULT '',
  measurement_profile_hash TEXT NOT NULL DEFAULT '',
  frame_count INTEGER NOT NULL DEFAULT 0,
  canvas_width INTEGER NOT NULL DEFAULT 0,
  canvas_height INTEGER NOT NULL DEFAULT 0,
  measurement_set_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'building',
  created_at TEXT DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dpqms_set ON desktop_pet_quality_measurement_sets(action_revision_id, measurement_set_hash);

CREATE TABLE IF NOT EXISTS desktop_pet_quality_frame_measurements (
  id TEXT PRIMARY KEY,
  measurement_set_id TEXT NOT NULL DEFAULT '',
  frame_artifact_id TEXT NOT NULL DEFAULT '',
  frame_index INTEGER NOT NULL DEFAULT 0,
  file_hash TEXT NOT NULL DEFAULT '',
  pixel_hash TEXT NOT NULL DEFAULT '',
  width INTEGER NOT NULL DEFAULT 0,
  height INTEGER NOT NULL DEFAULT 0,
  mime_type TEXT NOT NULL DEFAULT '',
  file_bytes INTEGER NOT NULL DEFAULT 0,
  has_alpha_channel INTEGER NOT NULL DEFAULT 0,
  alpha_coverage REAL NOT NULL DEFAULT 0,
  fully_transparent_ratio REAL NOT NULL DEFAULT 0,
  semi_transparent_ratio REAL NOT NULL DEFAULT 0,
  opaque_ratio REAL NOT NULL DEFAULT 0,
  decodable INTEGER NOT NULL DEFAULT 0,
  subject_box_json TEXT NOT NULL DEFAULT '{}',
  transform_hash TEXT NOT NULL DEFAULT '',
  created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_dpqfm_set ON desktop_pet_quality_frame_measurements(measurement_set_id);
CREATE INDEX IF NOT EXISTS idx_dpqfm_frame ON desktop_pet_quality_frame_measurements(frame_artifact_id);

CREATE TABLE IF NOT EXISTS desktop_pet_quality_sequence_measurements (
  id TEXT PRIMARY KEY,
  measurement_set_id TEXT NOT NULL DEFAULT '',
  frame_index INTEGER NOT NULL DEFAULT 0,
  subject_area_ratio REAL NOT NULL DEFAULT 0,
  connected_component_count INTEGER NOT NULL DEFAULT 0,
  largest_component_ratio REAL NOT NULL DEFAULT 0,
  border_foreground_coverage REAL NOT NULL DEFAULT 0,
  edge_contact_json TEXT NOT NULL DEFAULT '[]',
  centroid_x REAL NOT NULL DEFAULT 0,
  centroid_y REAL NOT NULL DEFAULT 0,
  created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_dpqsm_set ON desktop_pet_quality_sequence_measurements(measurement_set_id);

CREATE TABLE IF NOT EXISTS desktop_pet_quality_report_artifacts (
  id TEXT PRIMARY KEY,
  evaluation_id TEXT NOT NULL DEFAULT '',
  storage_key TEXT NOT NULL DEFAULT '',
  content_hash TEXT NOT NULL DEFAULT '',
  byte_size INTEGER NOT NULL DEFAULT 0,
  schema_version TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'staging',
  created_at TEXT DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dpqra_eval ON desktop_pet_quality_report_artifacts(evaluation_id);

CREATE TABLE IF NOT EXISTS desktop_pet_active_quality_binding_history (
  id TEXT PRIMARY KEY,
  action_revision_id TEXT NOT NULL DEFAULT '',
  profile_hash TEXT NOT NULL DEFAULT '',
  binding_revision INTEGER NOT NULL DEFAULT 0,
  previous_evaluation_id TEXT NOT NULL DEFAULT '',
  new_evaluation_id TEXT NOT NULL DEFAULT '',
  reason TEXT NOT NULL DEFAULT '',
  actor TEXT NOT NULL DEFAULT '',
  occurred_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_dpaqbh_rev ON desktop_pet_active_quality_binding_history(action_revision_id);

CREATE TABLE IF NOT EXISTS desktop_pet_quality_gate_snapshots (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL DEFAULT '',
  character_id TEXT NOT NULL DEFAULT '',
  processing_task_id TEXT NOT NULL DEFAULT '',
  active_revision_set_hash TEXT NOT NULL DEFAULT '',
  evaluation_set_hash TEXT NOT NULL DEFAULT '',
  gate_profile_id TEXT NOT NULL DEFAULT '',
  gate_profile_version TEXT NOT NULL DEFAULT '',
  rule_set_version TEXT NOT NULL DEFAULT '',
  rule_set_content_hash TEXT NOT NULL DEFAULT '',
  gate_status TEXT NOT NULL DEFAULT '',
  required_action_keys_json TEXT NOT NULL DEFAULT '[]',
  included_action_keys_json TEXT NOT NULL DEFAULT '[]',
  excluded_action_keys_json TEXT NOT NULL DEFAULT '[]',
  action_verdicts_json TEXT NOT NULL DEFAULT '[]',
  required_action_count INTEGER NOT NULL DEFAULT 0,
  accepted_action_count INTEGER NOT NULL DEFAULT 0,
  warning_action_count INTEGER NOT NULL DEFAULT 0,
  review_action_count INTEGER NOT NULL DEFAULT 0,
  rejected_action_count INTEGER NOT NULL DEFAULT 0,
  failed_evaluation_count INTEGER NOT NULL DEFAULT 0,
  snapshot_hash TEXT NOT NULL DEFAULT '',
  gate_hash TEXT NOT NULL DEFAULT '',
  invalidated_at TEXT NOT NULL DEFAULT '',
  invalidation_reason TEXT NOT NULL DEFAULT '',
  created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_dpqgs_task ON desktop_pet_quality_gate_snapshots(processing_task_id);
CREATE INDEX IF NOT EXISTS idx_dpqgs_rsh ON desktop_pet_quality_gate_snapshots(active_revision_set_hash);

CREATE TABLE IF NOT EXISTS desktop_pet_active_quality_gate_bindings (
  id TEXT PRIMARY KEY,
  processing_task_id TEXT NOT NULL DEFAULT '',
  gate_profile_hash TEXT NOT NULL DEFAULT '',
  active_gate_id TEXT NOT NULL DEFAULT '',
  active_revision_set_hash TEXT NOT NULL DEFAULT '',
  evaluation_set_hash TEXT NOT NULL DEFAULT '',
  binding_revision INTEGER NOT NULL DEFAULT 0,
  created_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dpaqgbind ON desktop_pet_active_quality_gate_bindings(processing_task_id, gate_profile_hash);

CREATE TABLE IF NOT EXISTS desktop_pet_quality_gate_rebuild_requests (
  id TEXT PRIMARY KEY,
  processing_task_id TEXT NOT NULL DEFAULT '',
  source_event_type TEXT NOT NULL DEFAULT '',
  source_event_id TEXT NOT NULL DEFAULT '',
  reason TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_dpqgrr_task ON desktop_pet_quality_gate_rebuild_requests(processing_task_id);

CREATE TABLE IF NOT EXISTS desktop_pet_quality_outbox_events_v2 (
  id TEXT PRIMARY KEY,
  event_id TEXT NOT NULL DEFAULT '',
  event_type TEXT NOT NULL DEFAULT '',
  aggregate_id TEXT NOT NULL DEFAULT '',
  aggregate_sequence INTEGER NOT NULL DEFAULT 0,
  payload_json TEXT NOT NULL DEFAULT '',
  payload_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  attempt_count INTEGER NOT NULL DEFAULT 0,
  available_at TEXT NOT NULL DEFAULT (datetime('now')),
  last_error TEXT NOT NULL DEFAULT '',
  created_at TEXT DEFAULT (datetime('now')),
  published_at TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dpqoev2_event ON desktop_pet_quality_outbox_events_v2(event_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dpqoev2_agg ON desktop_pet_quality_outbox_events_v2(aggregate_id, aggregate_sequence, event_type);

CREATE TABLE IF NOT EXISTS desktop_pet_quality_commit_journals_v2 (
  id TEXT PRIMARY KEY,
  commit_hash TEXT NOT NULL DEFAULT '',
  evaluation_id TEXT NOT NULL DEFAULT '',
  action_revision_id TEXT NOT NULL DEFAULT '',
  action_content_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'created',
  steps_json TEXT NOT NULL DEFAULT '',
  report_staging_key TEXT NOT NULL DEFAULT '',
  report_final_key TEXT NOT NULL DEFAULT '',
  report_hash TEXT NOT NULL DEFAULT '',
  result_hash TEXT NOT NULL DEFAULT '',
  last_error TEXT NOT NULL DEFAULT '',
  created_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now')),
  completed_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_dpqcjv2_eval ON desktop_pet_quality_commit_journals_v2(evaluation_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dpqcjv2_hash ON desktop_pet_quality_commit_journals_v2(commit_hash);

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
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
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
  available_at TEXT NOT NULL DEFAULT (datetime('now')),
  last_error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  published_at TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_drevo_event_id ON desktop_pet_release_event_outbox(event_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_drevo_agg_seq_type ON desktop_pet_release_event_outbox(aggregate_id, aggregate_sequence, event_type);
CREATE INDEX IF NOT EXISTS idx_drevo_status ON desktop_pet_release_event_outbox(status);
CREATE INDEX IF NOT EXISTS idx_drevo_available ON desktop_pet_release_event_outbox(available_at) WHERE status='pending';

CREATE TABLE IF NOT EXISTS desktop_pet_release_build_request_inbox (
  id TEXT PRIMARY KEY,
  request_id TEXT NOT NULL DEFAULT '',
  user_id TEXT NOT NULL DEFAULT '',
  idempotency_key TEXT NOT NULL DEFAULT '',
  input_hash TEXT NOT NULL DEFAULT '',
  payload_json TEXT NOT NULL DEFAULT '',
  payload_hash TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'pending',
  operation_id TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  processed_at TEXT NOT NULL DEFAULT '',
  last_error TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_drbrbi_request_id ON desktop_pet_release_build_request_inbox(request_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_drbrbi_idempotent ON desktop_pet_release_build_request_inbox(user_id, idempotency_key);
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
  user_id TEXT NOT NULL DEFAULT '',
  pet_id TEXT NOT NULL DEFAULT '',
  release_id TEXT NOT NULL DEFAULT '',
  operation_id TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'preparing',
  last_error TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_dips_staging ON desktop_pet_import_package_snapshots(import_staging_id);
CREATE INDEX IF NOT EXISTS idx_dips_release ON desktop_pet_import_package_snapshots(release_id);
CREATE INDEX IF NOT EXISTS idx_dips_operation ON desktop_pet_import_package_snapshots(operation_id);
CREATE INDEX IF NOT EXISTS idx_dips_status ON desktop_pet_import_package_snapshots(status);

CREATE TABLE IF NOT EXISTS desktop_pet_device_active_installation_bindings (
    user_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    installation_id TEXT NOT NULL DEFAULT '',
    pet_id TEXT NOT NULL DEFAULT '',
    release_id TEXT NOT NULL DEFAULT '',
    binding_revision INTEGER NOT NULL DEFAULT 0,
    bound_reason TEXT NOT NULL DEFAULT 'install_bound',
    bound_at TEXT NOT NULL DEFAULT '',
    bound_by TEXT NOT NULL DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    PRIMARY KEY(user_id, device_id)
);
CREATE INDEX IF NOT EXISTS idx_dpdainst_installation ON desktop_pet_device_active_installation_bindings(installation_id);
CREATE INDEX IF NOT EXISTS idx_dpdainst_pet ON desktop_pet_device_active_installation_bindings(pet_id);

CREATE TABLE IF NOT EXISTS desktop_pet_device_installation_binding_history (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    previous_installation_id TEXT NOT NULL DEFAULT '',
    new_installation_id TEXT NOT NULL DEFAULT '',
    binding_revision INTEGER NOT NULL DEFAULT 0,
    reason TEXT NOT NULL DEFAULT '',
    actor TEXT NOT NULL DEFAULT '',
    operation_id TEXT NOT NULL DEFAULT '',
    occurred_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_dpbih_user_device ON desktop_pet_device_installation_binding_history(user_id, device_id);
CREATE INDEX IF NOT EXISTS idx_dpbih_operation ON desktop_pet_device_installation_binding_history(operation_id);

CREATE TABLE IF NOT EXISTS desktop_pet_device_desired_revision_counters (
    user_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    current_revision INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT DEFAULT (datetime('now')),
    PRIMARY KEY(user_id, device_id)
);

CREATE TABLE IF NOT EXISTS desktop_pet_installation_runtime_projections (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
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
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_drirp_user_device ON desktop_pet_installation_runtime_projections(user_id, device_id);
CREATE INDEX IF NOT EXISTS idx_drirp_installation ON desktop_pet_installation_runtime_projections(installation_id);
CREATE INDEX IF NOT EXISTS idx_drirp_runtime ON desktop_pet_installation_runtime_projections(runtime_id);

CREATE TABLE IF NOT EXISTS desktop_pet_runtime_desired_state_outbox (
    event_id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL DEFAULT 'desired_state_changed',
    user_id TEXT NOT NULL,
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
    created_at TEXT DEFAULT (datetime('now')),
    published_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_drdso_user_device_status ON desktop_pet_runtime_desired_state_outbox(user_id, device_id, status);
CREATE INDEX IF NOT EXISTS idx_drdso_available ON desktop_pet_runtime_desired_state_outbox(status, available_at);

--- 来源: desktop_pet_runtime_v2_tables.go ---
CREATE TABLE IF NOT EXISTS desktop_pet_runtime_sessions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL DEFAULT '',
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
  created_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_rtsessv2_user_device ON desktop_pet_runtime_sessions(user_id, device_id);
CREATE INDEX IF NOT EXISTS idx_rtsessv2_user_device_runtime_status ON desktop_pet_runtime_sessions(user_id, device_id, runtime_id, status);

--- 来源: desktop_pet_runtime_v2_tables.go ---
CREATE TABLE IF NOT EXISTS desktop_pet_runtime_commands_v2 (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL DEFAULT '',
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
  inserted_at TEXT DEFAULT (datetime('now')),
  created_at TEXT NOT NULL DEFAULT '',
  updated_at TEXT DEFAULT (datetime('now')),
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
  UNIQUE(user_id, device_id, idempotency_key)
);
CREATE INDEX IF NOT EXISTS idx_rtcv2_user_device_type ON desktop_pet_runtime_commands_v2(user_id, device_id, command_type);
CREATE INDEX IF NOT EXISTS idx_rtcv2_status ON desktop_pet_runtime_commands_v2(status, inserted_at);
CREATE INDEX IF NOT EXISTS idx_rtcv2_device_seq ON desktop_pet_runtime_commands_v2(user_id, device_id, device_sequence);
CREATE INDEX IF NOT EXISTS idx_rtcv2_user_device_runtime_status ON desktop_pet_runtime_commands_v2(user_id, device_id, runtime_id, status);

--- 来源: desktop_pet_import_stagings.go ---
CREATE TABLE IF NOT EXISTS desktop_pet_import_stagings (
  id TEXT PRIMARY KEY,
  owner_user_id TEXT NOT NULL,
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
  rejected_reason TEXT NOT NULL DEFAULT '',
  correlation_id TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_dpis_owner_status ON desktop_pet_import_stagings(owner_user_id, status);

--- 来源: desktop_pet_migration_control.go ---
CREATE TABLE IF NOT EXISTS desktop_pet_migration_operations (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  status TEXT NOT NULL,
  started_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  completed_at TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  metadata TEXT NOT NULL DEFAULT '{}'
);
CREATE TABLE IF NOT EXISTS desktop_pet_migration_checkpoints (
  id TEXT PRIMARY KEY,
  operation_id TEXT NOT NULL,
  step_name TEXT NOT NULL,
  last_primary_key TEXT NOT NULL DEFAULT '',
  processed_count INTEGER NOT NULL DEFAULT 0,
  input_hash TEXT NOT NULL DEFAULT '',
  output_hash TEXT NOT NULL DEFAULT '',
  conflict_count INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS desktop_pet_migration_conflicts (
  id TEXT PRIMARY KEY,
  operation_id TEXT NOT NULL,
  entity_kind TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  conflict_reason TEXT NOT NULL,
  detected_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS desktop_pet_migration_locks (
  lock_name TEXT PRIMARY KEY,
  owner_instance_id TEXT NOT NULL,
  lease_expires_at TEXT NOT NULL,
  heartbeat_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS desktop_pet_read_cutovers (
  id TEXT PRIMARY KEY,
  operation_id TEXT NOT NULL,
  step_name TEXT NOT NULL,
  cutover_at TEXT NOT NULL,
  verified INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS desktop_pet_write_cutovers (
  id TEXT PRIMARY KEY,
  operation_id TEXT NOT NULL,
  step_name TEXT NOT NULL,
  cutover_at TEXT NOT NULL,
  verified INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_dpmc_operation ON desktop_pet_migration_checkpoints(operation_id);
CREATE INDEX IF NOT EXISTS idx_dpmc_conflict_op ON desktop_pet_migration_conflicts(operation_id);
CREATE INDEX IF NOT EXISTS idx_dpmc_lock_expiry ON desktop_pet_migration_locks(lease_expires_at);
CREATE INDEX IF NOT EXISTS idx_dprc_operation ON desktop_pet_read_cutovers(operation_id);
CREATE INDEX IF NOT EXISTS idx_dpmc_write_op ON desktop_pet_write_cutovers(operation_id);

--- 来源: desktop_pet_runtime_v2_tables.go ---
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
  inserted_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_rtca_command ON desktop_pet_runtime_command_attempts(command_id);

--- 来源: desktop_pet_runtime_v2_tables.go ---
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
  inserted_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now')),
  UNIQUE(command_id, runtime_id)
);
CREATE INDEX IF NOT EXISTS idx_rtcr_command ON desktop_pet_runtime_command_results(command_id);
CREATE INDEX IF NOT EXISTS idx_rtcr_runtime ON desktop_pet_runtime_command_results(runtime_id);

--- 来源: desktop_pet_runtime_v2_tables.go ---
CREATE TABLE IF NOT EXISTS desktop_pet_runtime_device_command_sequences (
  user_id TEXT NOT NULL DEFAULT '',
  device_id TEXT NOT NULL DEFAULT '',
  sequence INTEGER NOT NULL DEFAULT 0,
  last_reserved_at TEXT NOT NULL DEFAULT '',
  inserted_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now')),
  PRIMARY KEY(user_id, device_id)
);

--- 来源: desktop_pet_runtime_v2_tables.go ---
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
  inserted_at TEXT DEFAULT (datetime('now')),
  UNIQUE(runtime_session_id, sequence)
);
CREATE INDEX IF NOT EXISTS idx_rter_session_seq ON desktop_pet_runtime_event_records(runtime_session_id, sequence);
CREATE INDEX IF NOT EXISTS idx_rter_session_delivered ON desktop_pet_runtime_event_records(runtime_session_id, delivered, sequence);

--- 来源: desktop_pet_runtime_v2_tables.go ---
CREATE TABLE IF NOT EXISTS desktop_pet_runtime_actual_states_v2 (
  user_id TEXT NOT NULL DEFAULT '',
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
  stable_action_key TEXT NOT NULL DEFAULT '',
  current_action_key TEXT NOT NULL DEFAULT '',
  playback_instance_id TEXT NOT NULL DEFAULT '',
  current_command_id TEXT NOT NULL DEFAULT '',
  actual_state_hash TEXT NOT NULL DEFAULT '',
  health_status TEXT NOT NULL DEFAULT '',
  last_error_code TEXT NOT NULL DEFAULT '',
  updated_at TEXT DEFAULT (datetime('now')),
  PRIMARY KEY(user_id, device_id, runtime_id)
);
CREATE INDEX IF NOT EXISTS idx_rtasv2_user_device ON desktop_pet_runtime_actual_states_v2(user_id, device_id);

--- 来源: desktop_pet_runtime_v2_tables.go ---
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
  inserted_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now')),
  published_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_dteo_status ON desktop_pet_runtime_domain_event_outbox(status, inserted_at);

--- 来源: desktop_pet_runtime_v2_tables.go ---
CREATE TABLE IF NOT EXISTS desktop_pet_runtime_command_dedup (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL DEFAULT '',
  device_id TEXT NOT NULL DEFAULT '',
  idempotency_key TEXT NOT NULL DEFAULT '',
  nak_count INTEGER NOT NULL DEFAULT 0,
  last_nak_at TEXT NOT NULL DEFAULT '',
  inserted_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_rtcdd_user_device_idem ON desktop_pet_runtime_command_dedup(user_id, device_id, idempotency_key);

--- 来源: desktop_pet_runtime_v2_tables.go ---
CREATE TABLE IF NOT EXISTS desktop_pet_runtime_reconcile_leases (
  reconciler_id TEXT PRIMARY KEY,
  last_heartbeat_at TEXT NOT NULL DEFAULT '',
  inserted_at TEXT DEFAULT (datetime('now')),
  updated_at TEXT DEFAULT (datetime('now'))
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
    created_at TEXT DEFAULT (datetime('now')),
    purged_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_dptein_installation ON desktop_pet_installation_trash_entries(installation_id);
CREATE INDEX IF NOT EXISTS idx_dptein_retain ON desktop_pet_installation_trash_entries(retain_until);

--- 来源: resource_snapshot_store.go ---
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

--- 来源: user_data_snapshot_store.go ---
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

CREATE UNIQUE INDEX IF NOT EXISTS uq_dpinst_user_device_pet ON desktop_pet_installations(user_id, device_id, pet_id);

--- 来源: desktop_session.go ---
CREATE TABLE IF NOT EXISTS desktop_pet_local_sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL DEFAULT '',
    desktop_instance_id TEXT NOT NULL DEFAULT '',
    token_hash TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    created_at TEXT NOT NULL DEFAULT '',
    expires_at TEXT NOT NULL DEFAULT '',
    last_used_at TEXT NOT NULL DEFAULT '',
    revoked_at TEXT DEFAULT NULL
);
CREATE INDEX IF NOT EXISTS idx_dpls_token ON desktop_pet_local_sessions(token_hash, status);
CREATE INDEX IF NOT EXISTS idx_dpls_user ON desktop_pet_local_sessions(user_id, status);

CREATE TABLE IF NOT EXISTS desktop_pet_runtime_bootstrap_tickets (
    id TEXT PRIMARY KEY,
    ticket_hash TEXT UNIQUE NOT NULL,
    user_id TEXT NOT NULL DEFAULT '',
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
CREATE INDEX IF NOT EXISTS idx_bt_user ON desktop_pet_runtime_bootstrap_tickets(user_id, status);
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



CREATE TABLE IF NOT EXISTS desktop_pet_devices (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    desktop_instance_id TEXT NOT NULL,
    platform TEXT NOT NULL DEFAULT '',
    app_version TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    first_seen_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    revoked_at TEXT NOT NULL DEFAULT '',
    UNIQUE(user_id, device_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_dpd_user_device ON desktop_pet_devices(user_id, device_id);
CREATE INDEX IF NOT EXISTS idx_dpd_status ON desktop_pet_devices(status);

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
    updated_at DATETIME NOT NULL
);

--- 来源: model_config_protocol.go ---
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

CREATE TABLE IF NOT EXISTS voice_profiles (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    asr_config_id INTEGER DEFAULT 0,
    tts_config_id INTEGER DEFAULT 0,
    realtime_provider_id TEXT DEFAULT '',
    wake_config_id TEXT DEFAULT '',
    vad_preset TEXT DEFAULT 'default',
    interrupt_policy TEXT DEFAULT 'immediate',
    privacy_mode TEXT DEFAULT 'standard',
    is_default INTEGER DEFAULT 0,
    created_at TEXT DEFAULT '',
    updated_at TEXT DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_voice_profiles_is_default ON voice_profiles(is_default);

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

--- 来源: production_cutover.go ---
CREATE TABLE IF NOT EXISTS production_cutover_state (
	operation_id TEXT PRIMARY KEY,
	phase TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT '',
	phase_status TEXT NOT NULL DEFAULT '',
	snapshot_id TEXT NOT NULL DEFAULT '',
	error_message TEXT NOT NULL DEFAULT '',
	started_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	completed_at TEXT NOT NULL DEFAULT '',
	canonical_generation INTEGER NOT NULL DEFAULT 0,
	plan_version INTEGER NOT NULL DEFAULT 1
);

--- 来源: device_runtime_session.go ---
CREATE TABLE IF NOT EXISTS kernel_device_runtime_sessions (
    runtime_session_id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
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
CREATE INDEX IF NOT EXISTS idx_kernel_device_runtime_sessions_identity ON kernel_device_runtime_sessions(user_id, device_id, runtime_id);
CREATE INDEX IF NOT EXISTS idx_kernel_device_runtime_sessions_status ON kernel_device_runtime_sessions(status);
CREATE INDEX IF NOT EXISTS idx_kernel_device_runtime_sessions_heartbeat ON kernel_device_runtime_sessions(last_heartbeat_at);

--- 来源: artifact/migration.go ---
CREATE TABLE IF NOT EXISTS artifacts (
    artifact_id TEXT PRIMARY KEY,
    owner_user_id TEXT NOT NULL,
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
CREATE INDEX IF NOT EXISTS idx_artifacts_owner ON artifacts(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_artifacts_blob_digest ON artifacts(blob_digest);
CREATE INDEX IF NOT EXISTS idx_artifacts_status ON artifacts(status);
CREATE INDEX IF NOT EXISTS idx_artifacts_created_at ON artifacts(created_at);

