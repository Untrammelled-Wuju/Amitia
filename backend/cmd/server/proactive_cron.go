package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/proactive"
	"gorm.io/gorm"
)

type ProactiveCron struct {
	db        *gorm.DB
	executor  *proactive.Executor
	running   bool
	mu        sync.Mutex
	stopCh    chan struct{}
	scheduled map[int]int
	lastClean string
}

func NewProactiveCron(db *gorm.DB) *ProactiveCron {
	return &ProactiveCron{
		db:        db,
		executor:  proactive.NewExecutor(db),
		scheduled: make(map[int]int),
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
	go c.loop()
	log.Println("[ProactiveCron] 规则扫描已启动（每 10s）")
	c.executor.ScanAndExecute()
}

func (c *ProactiveCron) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.running {
		return
	}
	c.running = false
	close(c.stopCh)
	log.Println("[ProactiveCron] 已停止")
}

func (c *ProactiveCron) loop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.executor.ScanAndExecute()
			c.cleanupOldReminders()
			c.cleanStaleSchedules()
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
		fmt.Sscanf(daysStr, "%d", &days)
	}
	if days <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")
	c.db.Exec("DELETE FROM reminders WHERE enabled = 0 AND last_triggered_at < ?", cutoff)
}
