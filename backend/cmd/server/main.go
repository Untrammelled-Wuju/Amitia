package main

import (
	"path/filepath"
	"strings"
	"gorm.io/gorm"
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
	qdrantDB "github.com/u-ai/backend/pkg/database/qdrant"

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
	initDatabase(db)
	ctx := app.NewAppContext(db, nil)

	env := startEnvironment()
	env.SetOnShutdown(func() { qdrantDB.StopQdrant() })

	cleanup := func() {
		if env != nil {
			env.StopAll()
		}
		qdrantDB.StopQdrant()
	}
	defer cleanup()

	startQdrant()

	compSvc := companion.NewService(ctx)
	cron := NewProactiveCron(db, compSvc)
	cron.Start()
	proactive.SchedulerRunning = true
	defer func() {
		proactive.SchedulerRunning = false
		cron.Stop()
	}()

	serverAddr := config.AppCfg.Server.Addr()
	fmt.Printf("\n  ========================================\n")
	fmt.Printf("    %s Backend Server\n", config.AppCfg.App.Name)
	fmt.Printf("    Version:     %s\n", config.AppCfg.App.Version)
	fmt.Printf("    Listen:      http://%s\n", serverAddr)
	fmt.Printf("    Deploy Mode: %s\n", config.AppCfg.App.DeployMode)
	fmt.Printf("    Database:    %s/app.db\n", config.AppCfg.Storage.DataDir)
	fmt.Printf("    Qdrant:      %s:%d\n", config.AppCfg.Qdrant.Host, config.AppCfg.Qdrant.Port)
	fmt.Printf("  ========================================\n\n")

	qqMgr := qq.NewManager("http://127.0.0.1:9877")
	qq.SetManager(qqMgr)

	chatRepo := chat.NewRepository(ctx)
	memRepo := memory.NewRepository(ctx)
	memSvc := memory.NewService(memRepo, ctx)

	agenttool.SetOnMemorySaved(func(id, key, value, memoryType, characterID string) {
		memSvc.SyncEmbedding(id, key, value, characterID, memoryType)
	})
	chatSvc := chat.NewService(chatRepo, ctx, memSvc)
	chat.InitBuffer(config.AppCfg.Chat.MergeWindowMs)
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
	compSvc.ScheduleBasedGenerator(time.Now().Format("2006-01-02"), "")
	log.Info("今日主动消息任务已生成")

	r := setupRouter(ctx)
	if err := r.Run(serverAddr); err != nil {
		log.Error("服务启动失败:", err)
		cleanup()
		os.Exit(1)
	}
}

func initDatabase(db *gorm.DB) {
	sqlPath := filepath.Join(config.AppCfg.Storage.DataDir, "sql.sql")
	data, err := os.ReadFile(sqlPath)
	if err != nil {
		log.Warn("sql.sql未找到，跳过建表:", err)
		return
	}
	statements := strings.Split(string(data), ";")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}
		db.Exec(stmt)
	}
	log.Info("sql.sql建表完成")
}

func startQdrant() {
	qcfg := config.AppCfg.Qdrant
	log.Info("正在启动Qdrant...")
	if err := qdrantDB.StartQdrant(); err != nil {
		log.Error("Qdrant启动失败:", err)
		log.Warn("向量检索功能不可用，将回退到关键词搜索")
		return
	}
	if err := qdrantDB.WaitForQdrant(qcfg.Port); err != nil {
		log.Error("等待Qdrant就绪超时:", err)
		qdrantDB.StopQdrant()
		log.Warn("向量检索功能不可用，将回退到关键词搜索")
		return
	}
	if err := qdrantDB.InitClient(); err != nil {
		log.Error("Qdrant客户端初始化失败:", err)
		qdrantDB.StopQdrant()
		log.Warn("向量检索功能不可用，将回退到关键词搜索")
		return
	}
	if err := qdrantDB.EnsureCollection(); err != nil {
		log.Error("Qdrant集合创建失败:", err)
		qdrantDB.StopQdrant()
		log.Warn("向量检索功能不可用，将回退到关键词搜索")
		return
	}
	log.Info("Qdrant就绪，向量检索功能已启用")
}
