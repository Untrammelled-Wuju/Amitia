// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"log"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/companion"
	"github.com/u-ai/backend/internal/proactive"
	"gorm.io/gorm"
)

type ProactiveCron struct {
	db        *gorm.DB
	compSvc   companion.Service
	executor  *proactive.Executor
	scheduler *proactive.SafeScheduler
	running   bool
	mu        sync.Mutex
	stopCh    chan struct{}

	scheduled          map[int]int
	lastClean          string
	lastRegenerateDate string
	lastBurstAt        time.Time
	todayBurstCount    int
}

func NewProactiveCron(db *gorm.DB, compSvc companion.Service) *ProactiveCron {
	exec := proactive.NewExecutor(db)
	return &ProactiveCron{
		db:                 db,
		compSvc:            compSvc,
		executor:           exec,
		scheduler:          proactive.NewSafeScheduler(db, exec),
		scheduled:          make(map[int]int),
		lastRegenerateDate: time.Now().Format("2006-01-02"),
	}
}

func (c *ProactiveCron) Start() {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	c.running = true
	c.stopCh = make(chan struct{})
	c.mu.Unlock()

	c.scheduler.Start()
	go c.runReminderScanner()
	go c.runActiveTaskScanner()
	go c.runDailyRegenerator()
	go c.runRandomBurstTrigger()

	log.Println("[ProactiveCron] 规则扫描已启动（SafeScheduler Timer模式）")
	log.Println("[ProactiveCron] 提醒扫描已启动（每 10s）")
	log.Println("[ProactiveCron] 主动任务扫描已启动（每 30s）")
	log.Println("[ProactiveCron] 每日重生成已启动（每 60s）")
	log.Println("[ProactiveCron] 随机突发已启动（每 60s）")
}

func (c *ProactiveCron) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running {
		return
	}
	c.running = false
	c.scheduler.Stop()
	close(c.stopCh)
	log.Println("[ProactiveCron] 所有扫描器已停止")
}

func (c *ProactiveCron) runReminderScanner() {
	defer func() {
		if r := recover(); r != nil {
			log.Println("[ProactiveCron] 提醒扫描器 panic 恢复:", r)
			go c.runReminderScanner()
		}
	}()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.cleanupOldReminders()
		case <-c.stopCh:
			return
		}
	}
}

func (c *ProactiveCron) runActiveTaskScanner() {
	defer func() {
		if r := recover(); r != nil {
			log.Println("[ProactiveCron] 主动任务扫描器 panic 恢复:", r)
			go c.runActiveTaskScanner()
		}
	}()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			var charIDs []string
			c.db.Table("characters").Pluck("id", &charIDs)
			for _, cid := range charIDs {
				result := c.compSvc.ProcessDueActiveMessageTasks(cid)
				processed, _ := result["processed"].(int)
				sent, _ := result["sent"].(int)
				if processed > 0 {
					log.Printf("[ProactiveCron] 主动任务处理 char=%s: processed=%d sent=%d", cid, processed, sent)
				}
			}
		case <-c.stopCh:
			return
		}
	}
}

func (c *ProactiveCron) runDailyRegenerator() {
	defer func() {
		if r := recover(); r != nil {
			log.Println("[ProactiveCron] 每日重生成器 panic 恢复:", r)
			go c.runDailyRegenerator()
		}
	}()
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.dailyRegenerate()
		case <-c.stopCh:
			return
		}
	}
}

func (c *ProactiveCron) dailyRegenerate() {
	now := time.Now()
	today := now.Format("2006-01-02")
	if now.Hour() == 0 && now.Minute() < 5 && c.lastRegenerateDate != today {
		log.Println("[ProactiveCron] 开始每日任务重生成...")
		var charIDs []string
		c.db.Table("characters").Pluck("id", &charIDs)
		for _, cid := range charIDs {
			result := c.compSvc.ScheduleBasedGenerator(today, cid)
			taskCount, _ := result["taskCount"].(int)
			log.Printf("[ProactiveCron] 每日任务重生成 char=%s: tasks=%d", cid, taskCount)
		}
		c.lastRegenerateDate = today
		c.lastBurstAt = time.Time{}
		c.todayBurstCount = 0
	}
}

func (c *ProactiveCron) runRandomBurstTrigger() {
	defer func() {
		if r := recover(); r != nil {
			log.Println("[ProactiveCron] 随机突发触发器 panic 恢复:", r)
			go c.runRandomBurstTrigger()
		}
	}()
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if c.compSvc != nil {
				var charIDs []string
				c.db.Table("characters").Pluck("id", &charIDs)
				for _, cid := range charIDs {
					c.compSvc.RandomBurstTrigger(cid)
				}
			}
		case <-c.stopCh:
			return
		}
	}
}

func (c *ProactiveCron) cleanStaleSchedules() {
	today := time.Now().Format("2006-01-02")
	if c.lastClean == today {
		return
	}
	c.lastClean = today
	c.scheduled = make(map[int]int)
	c.db.Exec("UPDATE proactive_rules SET sent_count_today=0")
	log.Println("[ProactiveCron] 每日计数器已重置")
}

func (c *ProactiveCron) cleanupOldReminders() {
	var daysStr string
	c.db.Raw("SELECT value FROM app_settings WHERE key = 'reminder_cleanup_days' LIMIT 1").Row().Scan(&daysStr)
	days := 0
	if daysStr != "" {
		_ = daysStr
		for _, ch := range daysStr {
			if ch >= '0' && ch <= '9' {
				days = days*10 + int(ch-'0')
			}
		}
	}
	if days <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")
	c.db.Exec("DELETE FROM reminders WHERE enabled = 0 AND last_triggered_at < ?", cutoff)
}
