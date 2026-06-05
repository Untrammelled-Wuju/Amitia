package main

import (
	"github.com/u-ai/backend/internal/chat"
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
	db.Exec("ALTER TABLE messages ADD COLUMN tool_call_id TEXT DEFAULT ''"); db.Exec("CREATE TABLE IF NOT EXISTS app_settings (key TEXT PRIMARY KEY, value TEXT, updated_at TEXT)"); db.Exec("CREATE TABLE IF NOT EXISTS memory_events (id TEXT PRIMARY KEY, memory_id TEXT, event_type TEXT, key TEXT, value TEXT, memory_type TEXT, importance INTEGER, source TEXT, character_id TEXT, created_at TEXT)")

	
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

	
	cron := NewProactiveCron(db)
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

	r := setupRouter(ctx)
	if err := r.Run(serverAddr); err != nil {
		log.Error("服务启动失败:", err)
		os.Exit(1)
	}
}
