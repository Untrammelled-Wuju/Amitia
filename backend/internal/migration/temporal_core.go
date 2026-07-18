package migration

func TemporalCoreMigration() Migration {
	return Migration{
		Version:"202607180004",
		Name:"add_temporal_core",
		Up:func(s *Step)error{
			s.CreateTable(`CREATE TABLE IF NOT EXISTS temporal_profiles (
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
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS temporal_anchors (
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
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS temporal_events (
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
)`)
			s.CreateTable(`CREATE TABLE IF NOT EXISTS memory_temporal_metadata (
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
)`)
			if err:=s.CreateIndex("idx_temporal_anchor_scope","temporal_anchors",[]string{"user_id","character_id","status"},false);err!=nil{return err}
			if err:=s.CreateIndex("idx_temporal_anchor_next","temporal_anchors",[]string{"next_occurrence_at_utc"},false);err!=nil{return err}
			if err:=s.CreateIndex("idx_temporal_event_scope_time","temporal_events",[]string{"user_id","character_id","occurred_at_utc"},false);err!=nil{return err}
			if err:=s.CreateIndex("idx_memory_temporal_occurred","memory_temporal_metadata",[]string{"occurred_at_utc"},false);err!=nil{return err}
			return nil
		},
	}
}
