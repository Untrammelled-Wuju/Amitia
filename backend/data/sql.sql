-- ============================================
-- U-Ai 数据库初始化脚本
-- ============================================

CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    checksum TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    error_message TEXT NOT NULL DEFAULT '',
    started_at TEXT NOT NULL DEFAULT '',
    finished_at TEXT NOT NULL DEFAULT ''
);

-- 用户认证
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

-- 角色
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
    silence_duration INTEGER DEFAULT 0
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

-- 对话
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
    updated_at TEXT DEFAULT (datetime('now'))
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
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

-- TTS / ASR / Vision
CREATE TABLE IF NOT EXISTS tts_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    api_key TEXT DEFAULT '',
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
    api_key TEXT DEFAULT '',
    resource_id TEXT DEFAULT 'volc.seedasr.auc',
    is_active INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS vision_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    api_key TEXT DEFAULT '',
    model_name TEXT DEFAULT 'doubaoseed2.0lite',
    base_url TEXT DEFAULT 'https://ark.cn-beijing.volces.com/api/v3',
    is_active INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

-- 陪伴设置
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

-- 主动消息
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
    updated_at TEXT DEFAULT (datetime('now'))
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

-- 记忆

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
    last_used_at TEXT
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
    created_at TEXT DEFAULT (datetime('now'))
);

ALTER TABLE memories ADD COLUMN confidence INTEGER DEFAULT 50;
ALTER TABLE memories ADD COLUMN expires_at TEXT DEFAULT NULL;
ALTER TABLE memories ADD COLUMN entity_id TEXT DEFAULT '';
ALTER TABLE memories ADD COLUMN entity_type TEXT DEFAULT '';
ALTER TABLE memories ADD COLUMN source_msg_id TEXT DEFAULT '';
ALTER TABLE memories ADD COLUMN source_conv_id TEXT DEFAULT '';
ALTER TABLE memories ADD COLUMN verified_status TEXT DEFAULT 'unverified';
ALTER TABLE memories ADD COLUMN last_verified_at TEXT DEFAULT NULL;
ALTER TABLE memories ADD COLUMN scope TEXT DEFAULT 'character';
ALTER TABLE memories ADD COLUMN scope_type TEXT DEFAULT 'user_character';
UPDATE memories SET scope_type = CASE WHEN scope_type IN ('character_self', 'world') THEN scope_type WHEN scope = 'user' THEN 'user_global' WHEN scope_type = 'user_global' THEN 'user_global' WHEN scope = 'character' THEN 'user_character' ELSE 'user_character' END WHERE scope_type IS NULL OR scope_type = '' OR scope_type IN ('user', 'character', 'user_character');

CREATE TABLE IF NOT EXISTS memory_candidates (
    id TEXT PRIMARY KEY,
    key TEXT NOT NULL DEFAULT '',
    value TEXT NOT NULL DEFAULT '',
    memory_type TEXT DEFAULT 'custom',
    importance INTEGER DEFAULT 5,
    source_text TEXT DEFAULT '',
    conversation_id TEXT DEFAULT '',
    character_id TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_memories_confidence ON memories(character_id, confidence);
CREATE INDEX IF NOT EXISTS idx_memories_verified ON memories(character_id, verified_status);
CREATE INDEX IF NOT EXISTS idx_memories_entity ON memories(entity_id, entity_type);
CREATE INDEX IF NOT EXISTS idx_memories_importance_conf ON memories(character_id, importance, confidence);
CREATE INDEX IF NOT EXISTS idx_memories_scope_type ON memories(scope_type);
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
    updated_at TEXT DEFAULT (datetime('now'))
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
-- 反馈
CREATE TABLE IF NOT EXISTS message_feedback (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id TEXT DEFAULT '',
    rating INTEGER DEFAULT 0,
    comment TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now'))
);

-- 安全
CREATE TABLE IF NOT EXISTS safety_events (
    id TEXT PRIMARY KEY,
    conversation_id TEXT DEFAULT '',
    event_type TEXT DEFAULT '',
    description TEXT DEFAULT '',
    direction TEXT DEFAULT '',
    handled INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now'))
);

-- 心情
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

-- 应用设置
CREATE TABLE IF NOT EXISTS app_settings (
    key TEXT PRIMARY KEY,
    value TEXT DEFAULT '',
    updated_at TEXT DEFAULT (datetime('now'))
);

-- 索引
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


-- ============================================
-- 数据库迁移: 为旧表添加缺失字段 (忽略已存在的列)
-- ============================================

INSERT OR IGNORE INTO sleep_settings (id, character_id, bed_time, wake_time, enabled, sleep_reply_enabled, sleep_reply_mode) SELECT id, character_id, bed_time, wake_time, enabled, 0, 'NO_REPLY' FROM sleep_settings WHERE sleep_reply_mode IS NULL;

UPDATE fixed_events SET repeat_type = 'weekly' WHERE repeat_type IS NULL;
UPDATE fixed_events SET reply_mode = 'SHORT_REPLY' WHERE reply_mode IS NULL;

UPDATE special_events SET reply_mode = 'SHORT_REPLY' WHERE reply_mode IS NULL;
-- ============================================
-- 迁移: 添加缺失字段
-- ============================================
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
ALTER TABLE retrieval_logs ADD COLUMN character_id TEXT NOT NULL DEFAULT '';
ALTER TABLE retrieval_logs ADD COLUMN request_id TEXT NOT NULL DEFAULT '';
ALTER TABLE retrieval_logs ADD COLUMN channel TEXT NOT NULL DEFAULT '';
ALTER TABLE retrieval_logs ADD COLUMN retrieval_version TEXT NOT NULL DEFAULT '';
ALTER TABLE retrieval_logs ADD COLUMN legacy INTEGER NOT NULL DEFAULT 0;
UPDATE retrieval_logs
SET legacy = 1
WHERE conversation_id = ''
   OR conversation_id NOT IN (SELECT id FROM conversations);
ALTER TABLE memories ADD COLUMN scope TEXT DEFAULT "character";

ALTER TABLE memories ADD COLUMN confidence INTEGER DEFAULT 50;
ALTER TABLE memories ADD COLUMN expires_at TEXT DEFAULT NULL;
ALTER TABLE memories ADD COLUMN entity_id TEXT DEFAULT '';
ALTER TABLE memories ADD COLUMN entity_type TEXT DEFAULT '';
ALTER TABLE memories ADD COLUMN source_msg_id TEXT DEFAULT '';
ALTER TABLE memories ADD COLUMN source_conv_id TEXT DEFAULT '';
ALTER TABLE memories ADD COLUMN verified_status TEXT DEFAULT 'unverified';
ALTER TABLE memories ADD COLUMN last_verified_at TEXT DEFAULT NULL;
ALTER TABLE messages ADD COLUMN request_id TEXT DEFAULT '';
ALTER TABLE messages ADD COLUMN sequence INTEGER NOT NULL DEFAULT 0;
UPDATE messages SET sequence = (
    SELECT rn FROM (
        SELECT id, ROW_NUMBER() OVER (PARTITION BY conversation_id ORDER BY created_at, id) AS rn
        FROM messages
    ) ranked WHERE ranked.id = messages.id
) WHERE sequence IS NULL OR sequence <= 0;
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
ALTER TABLE memories ADD COLUMN scope_type TEXT DEFAULT 'user_character';
UPDATE memories SET scope_type = CASE WHEN scope_type IN ('character_self', 'world') THEN scope_type WHEN scope = 'user' THEN 'user_global' WHEN scope_type = 'user_global' THEN 'user_global' WHEN scope = 'character' THEN 'user_character' ELSE 'user_character' END WHERE scope_type IS NULL OR scope_type = '' OR scope_type IN ('user', 'character', 'user_character');
CREATE INDEX IF NOT EXISTS idx_messages_request ON messages(conversation_id, role, request_id);
CREATE INDEX IF NOT EXISTS idx_messages_conversation_sequence ON messages(conversation_id, sequence);
CREATE UNIQUE INDEX IF NOT EXISTS idx_messages_conv_sequence_unique ON messages(conversation_id, sequence);
CREATE INDEX IF NOT EXISTS idx_pipeline_checkpoints_conversation ON pipeline_checkpoints(conversation_id);
CREATE INDEX IF NOT EXISTS idx_pipeline_checkpoints_updated ON pipeline_checkpoints(updated_at);
CREATE INDEX IF NOT EXISTS idx_memories_scope_type ON memories(scope_type);

CREATE TABLE IF NOT EXISTS memory_candidates (
    id TEXT PRIMARY KEY,
    key TEXT NOT NULL DEFAULT '',
    value TEXT NOT NULL DEFAULT '',
    memory_type TEXT DEFAULT 'custom',
    importance INTEGER DEFAULT 5,
    source_text TEXT DEFAULT '',
    conversation_id TEXT DEFAULT '',
    character_id TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now'))
);



CREATE TABLE IF NOT EXISTS embedding_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL DEFAULT 'default',
    api_key TEXT DEFAULT '',
    model_name TEXT DEFAULT 'doubao-embedding-vision-251215',
    base_url TEXT DEFAULT 'https://ark.cn-beijing.volces.com/api/v3',
    is_active INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

ALTER TABLE memories ADD COLUMN source_start INTEGER DEFAULT 0;
ALTER TABLE memories ADD COLUMN source_end INTEGER DEFAULT 0;
ALTER TABLE memory_candidates ADD COLUMN source_start INTEGER DEFAULT 0;
ALTER TABLE memory_candidates ADD COLUMN source_end INTEGER DEFAULT 0;

ALTER TABLE conversations ADD COLUMN state_version TEXT DEFAULT '';

ALTER TABLE moods ADD COLUMN mood_value TEXT DEFAULT '';

ALTER TABLE messages ADD COLUMN reply_to_message_id TEXT DEFAULT '';
ALTER TABLE messages ADD COLUMN reply_to_role TEXT DEFAULT '';
ALTER TABLE messages ADD COLUMN reply_to_excerpt TEXT DEFAULT '';


ALTER TABLE user_profiles ADD COLUMN character_id TEXT NOT NULL DEFAULT '';

ALTER TABLE user_profiles ADD COLUMN source TEXT NOT NULL DEFAULT '';