package main

import (
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/companion"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/internal/qq"
	"github.com/u-ai/backend/internal/proactive"
	"fmt"
	"os"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/log"
	"github.com/u-ai/backend/pkg/app"
	"github.com/u-ai/backend/pkg/database/mysql"

	agenttool "github.com/u-ai/backend/internal/agent/tool"
)

func main() {
	
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config"
	}
	config.InitConfig(configPath)

	
	log.InitLogger("logs")

	
	db := mysql.NewSQLite(config.AppCfg.Storage.DataDir)

	

	sqlDB, _ := db.DB()
	agenttool.SetDB(sqlDB)
	db.Exec("CREATE TABLE IF NOT EXISTS active_message_task (id INTEGER PRIMARY KEY AUTOINCREMENT, task_type TEXT DEFAULT '', due_time TEXT, prompt TEXT DEFAULT '', status TEXT DEFAULT 'PENDING', reason TEXT DEFAULT '', retry_count INTEGER DEFAULT 0, max_retry INTEGER DEFAULT 3, last_error TEXT DEFAULT '', sent_at TEXT, created_at TEXT DEFAULT (datetime('now')), updated_at TEXT DEFAULT (datetime('now')), canceled_at TEXT, source TEXT DEFAULT 'schedule_based')")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_active_task_due ON active_message_task(due_time)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_active_task_status_due ON active_message_task(status, due_time)")
	db.Exec("ALTER TABLE active_message_task ADD COLUMN source TEXT DEFAULT 'schedule_based'")
	db.Exec("ALTER TABLE active_message_task ADD COLUMN max_retry INTEGER DEFAULT 3")
	db.Exec("ALTER TABLE active_message_task ADD COLUMN last_error TEXT DEFAULT '')")
	db.Exec("ALTER TABLE active_message_task ADD COLUMN reason TEXT DEFAULT '')")
	db.Exec("ALTER TABLE active_message_task ADD COLUMN canceled_at TEXT")
	db.Exec("CREATE TABLE IF NOT EXISTS active_message_settings (enabled INTEGER DEFAULT 1, active_level INTEGER DEFAULT 50, quiet_minutes TEXT DEFAULT '', min_interval INTEGER DEFAULT 60, max_daily INTEGER DEFAULT 6, max_daily_calls INTEGER DEFAULT 10, channel TEXT DEFAULT 'all', created_at TEXT DEFAULT (datetime('now')), updated_at TEXT DEFAULT (datetime('now')))")
	db.Exec("ALTER TABLE active_message_settings ADD COLUMN active_level INTEGER DEFAULT 50")
	db.Exec("ALTER TABLE active_message_settings ADD COLUMN quiet_minutes TEXT DEFAULT '')")
	db.Exec("ALTER TABLE active_message_settings ADD COLUMN max_daily_calls INTEGER DEFAULT 10")
	db.Exec("ALTER TABLE active_message_settings ADD COLUMN created_at TEXT DEFAULT (datetime('now'))")
	db.Exec("ALTER TABLE active_message_settings ADD COLUMN updated_at TEXT DEFAULT (datetime('now'))")
	db.Exec("CREATE TABLE IF NOT EXISTS proactive_messages (id INTEGER PRIMARY KEY AUTOINCREMENT, rule_id INTEGER, conversation_id TEXT, message_content TEXT, channel TEXT, status TEXT, task_type TEXT DEFAULT '', prompt TEXT DEFAULT '', error TEXT DEFAULT '', created_at TEXT DEFAULT (datetime('now')), sent_at TEXT, updated_at TEXT)")
	db.Exec("ALTER TABLE proactive_messages ADD COLUMN task_type TEXT DEFAULT '')")
	db.Exec("ALTER TABLE proactive_messages ADD COLUMN prompt TEXT DEFAULT '')")
	db.Exec("ALTER TABLE proactive_messages ADD COLUMN error TEXT DEFAULT '')")
	db.Exec("ALTER TABLE proactive_messages ADD COLUMN sent_at TEXT")
	db.Exec("ALTER TABLE proactive_messages ADD COLUMN updated_at TEXT")
	db.Exec("ALTER TABLE messages ADD COLUMN tool_call_id TEXT DEFAULT '')")
	db.Exec("ALTER TABLE messages ADD COLUMN image_url TEXT DEFAULT ''")
	db.Exec("CREATE TABLE IF NOT EXISTS app_settings (key TEXT PRIMARY KEY, value TEXT, updated_at TEXT)")
	db.Exec("CREATE TABLE IF NOT EXISTS memory_events (id TEXT PRIMARY KEY, memory_id TEXT, event_type TEXT, key TEXT, value TEXT, memory_type TEXT, importance INTEGER, source TEXT, character_id TEXT, created_at TEXT)")
		db.Exec("ALTER TABLE safety_events ADD COLUMN direction TEXT DEFAULT ''")
	db.Exec("CREATE TABLE IF NOT EXISTS tts_configs (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, api_key TEXT DEFAULT '', resource_id TEXT DEFAULT 'seed-tts-2.0', voice_type TEXT DEFAULT 'zh_female_cancan_mars_bigtts', emotion TEXT DEFAULT '', speed REAL DEFAULT 1.0, pitch REAL DEFAULT 1.0, volume REAL DEFAULT 1.0, is_active INTEGER DEFAULT 0, is_custom INTEGER DEFAULT 0, custom_voice_id TEXT DEFAULT '', created_at TEXT DEFAULT (datetime('now')), updated_at TEXT DEFAULT (datetime('now')))")
	db.Exec("CREATE TABLE IF NOT EXISTS asr_configs (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, api_key TEXT DEFAULT '', resource_id TEXT DEFAULT 'volc.seedasr.auc', is_active INTEGER DEFAULT 0, created_at TEXT DEFAULT (datetime('now')), updated_at TEXT DEFAULT (datetime('now')))")
	db.Exec("ALTER TABLE characters ADD COLUMN voice_config_id TEXT DEFAULT ''")
	db.Exec("ALTER TABLE characters ADD COLUMN voice_type TEXT DEFAULT ''")
	db.Exec("ALTER TABLE characters ADD COLUMN voice_speed REAL DEFAULT 1.0")
	db.Exec("ALTER TABLE characters ADD COLUMN voice_pitch REAL DEFAULT 1.0")
	db.Exec("ALTER TABLE characters ADD COLUMN voice_volume REAL DEFAULT 1.0")
	db.Exec("ALTER TABLE characters ADD COLUMN custom_voice_id TEXT DEFAULT ''")
	db.Exec("ALTER TABLE characters ADD COLUMN voice_mode TEXT DEFAULT 'preset'")
	db.Exec("ALTER TABLE characters ADD COLUMN emotion TEXT DEFAULT ''")  
	db.Exec("ALTER TABLE characters ADD COLUMN emotion_scale INTEGER DEFAULT 0")
	db.Exec("ALTER TABLE characters ADD COLUMN silence_duration INTEGER DEFAULT 0")
	db.Exec("ALTER TABLE tts_configs ADD COLUMN realtime_app_id TEXT DEFAULT ''")
	db.Exec("ALTER TABLE tts_configs ADD COLUMN realtime_access_token TEXT DEFAULT ''")
	db.Exec("ALTER TABLE tts_configs ADD COLUMN realtime_secret_key TEXT DEFAULT ''")

	db.Exec("CREATE TABLE IF NOT EXISTS safety_events (id TEXT PRIMARY KEY, conversation_id TEXT, event_type TEXT, description TEXT, direction TEXT DEFAULT '', handled INTEGER DEFAULT 0, created_at TEXT DEFAULT (datetime('now'))))")

	db.Exec("UPDATE characters SET conversation_id = (SELECT c.id FROM conversations c WHERE c.character_id = characters.id ORDER BY c.updated_at DESC LIMIT 1) WHERE conversation_id IS NULL OR conversation_id = ''")

	
	ctx := app.NewAppContext(db, nil)

	
	var env *Environment
	if os.Getenv("START_SIDECAR") != "0" {
		env = startEnvironment()
		defer func() {
			if env != nil {
				env.StopAll()
			}
		}()
	}

	
	compSvc := companion.NewService(ctx)
	cron := NewProactiveCron(db, compSvc)
	cron.Start()
	proactive.SchedulerRunning = true
	defer func() { proactive.SchedulerRunning = false; cron.Stop() }()

	
	serverAddr := config.AppCfg.Server.Addr()
	fmt.Printf("\n  ========================================\n")
	fmt.Printf("    %s Backend Server\n", config.AppCfg.App.Name)
	fmt.Printf("    Version:     %s\n", config.AppCfg.App.Version)
	fmt.Printf("    Listen:      http://%s\n", serverAddr)
	fmt.Printf("    Deploy Mode: %s\n", config.AppCfg.App.DeployMode)
	fmt.Printf("    Database:    %s/app.db\n", config.AppCfg.Storage.DataDir)
	fmt.Printf("  ========================================\n\n")


	qqMgr := qq.NewManager("http://127.0.0.1:9877")
	qq.SetManager(qqMgr)

	chatRepo := chat.NewRepository(ctx)
	memRepo := memory.NewRepository(ctx)
	memSvc := memory.NewService(memRepo, ctx)
	chatSvc := chat.NewService(chatRepo, ctx, memSvc)
	go func() {
		time.Sleep(3 * time.Second)
		chatSvc.EnsureChannelConversation("wechat")
		chatSvc.EnsureChannelConversation("qq")
		log.Info("频道对话已确保创建")
	}()
		count, err := chatSvc.RecalculateMessageCounts()
		if err != nil {
			log.Error("重算消息计数失败:", err)
		} else {
			log.Info("消息计数已修复，影响", count, "条对话")
		}
		compSvc.ScheduleBasedGenerator(time.Now().Format("2006-01-02"))
		log.Info("今日主动消息任务已生成")

	r := setupRouter(ctx)
	if err := r.Run(serverAddr); err != nil {
		log.Error("服务启动失败:", err)
		os.Exit(1)
	}
}
