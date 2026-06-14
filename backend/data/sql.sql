-- ============================================
-- U-Ai 数据库初始化脚本
-- ============================================

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
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL,
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
    tool_call_id TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

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
    voice_type TEXT DEFAULT 'zh_female_cancan_mars_bigtts',
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
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS fixed_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    character_id TEXT DEFAULT '',
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    week_day INTEGER DEFAULT -1,
    start_time TEXT DEFAULT '',
    end_time TEXT DEFAULT '',
    event_type TEXT DEFAULT 'custom',
    location TEXT DEFAULT '',
    enabled INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS special_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    character_id TEXT DEFAULT '',
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    event_date TEXT DEFAULT '',
    start_time TEXT DEFAULT '',
    end_time TEXT DEFAULT '',
    event_type TEXT DEFAULT 'custom',
    location TEXT DEFAULT '',
    enabled INTEGER DEFAULT 1,
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
    activity TEXT DEFAULT '',
    intensity INTEGER DEFAULT 50,
    schedule TEXT DEFAULT '',
    preference TEXT DEFAULT '',
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS work_profiles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    character_id TEXT DEFAULT '',
    job_title TEXT DEFAULT '',
    work_hours TEXT DEFAULT '',
    work_days TEXT DEFAULT '',
    description TEXT DEFAULT '',
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
    canceled_at TEXT,
    source TEXT DEFAULT 'schedule_based',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS proactive_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT DEFAULT '',
    description TEXT DEFAULT '',
    rule_type TEXT DEFAULT '',
    trigger_condition TEXT DEFAULT '',
    action TEXT DEFAULT '',
    enabled INTEGER DEFAULT 1,
    character_id TEXT DEFAULT '',
    sent_count_today INTEGER DEFAULT 0,
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
    remind_at TEXT DEFAULT '',
    enabled INTEGER DEFAULT 1,
    character_id TEXT DEFAULT '',
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
    source TEXT DEFAULT 'manual',
    character_id TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS memory_events (
    id TEXT PRIMARY KEY,
    memory_id TEXT DEFAULT '',
    event_type TEXT DEFAULT '',
    key TEXT DEFAULT '',
    value TEXT DEFAULT '',
    memory_type TEXT DEFAULT '',
    importance INTEGER DEFAULT 0,
    source TEXT DEFAULT '',
    character_id TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS memory_embeddings (
    memory_id TEXT PRIMARY KEY,
    created_at TEXT DEFAULT (datetime('now'))
);

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
    level INTEGER DEFAULT 50,
    created_at TEXT DEFAULT (datetime('now'))
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
CREATE INDEX IF NOT EXISTS idx_conversations_character ON conversations(character_id);
