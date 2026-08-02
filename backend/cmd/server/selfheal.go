package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/log"
	"gorm.io/gorm"
)

type SubsystemHealth struct {
	Name      string
	Healthy   bool
	LastCheck time.Time
	FailCount int
}

type SelfHealMonitor struct {
	db            *gorm.DB
	addr          string
	mu            sync.RWMutex
	subsystems    map[string]*SubsystemHealth
	stopCh        chan struct{}
	checkInterval time.Duration
	maxFailCount  int
}

func NewSelfHealMonitor(db *gorm.DB, addr string) *SelfHealMonitor {
	return &SelfHealMonitor{
		db:            db,
		addr:          addr,
		subsystems:    make(map[string]*SubsystemHealth),
		stopCh:        make(chan struct{}),
		checkInterval: 30 * time.Second,
		maxFailCount:  3,
	}
}

func (m *SelfHealMonitor) Start(ctx context.Context) {
	go m.loop(ctx)
	log.Info("后端自我保活看门狗已启动")
}

func (m *SelfHealMonitor) Stop() {
	close(m.stopCh)
}

func (m *SelfHealMonitor) loop(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Error("自我保活看门狗异常恢复:", r)
			time.AfterFunc(m.checkInterval, func() { go m.loop(ctx) })
		}
	}()

	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkHTTPServer()
			m.checkDatabase()
		case <-m.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (m *SelfHealMonitor) checkHTTPServer() {
	url := "http://" + m.addr + "/api/health"
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	healthy := err == nil && resp.StatusCode == 200
	if resp != nil {
		resp.Body.Close()
	}

	m.recordHealth("http_server", healthy)
	if !healthy {
		log.Warn("后端HTTP服务器健康检查失败，可能无法正常响应请求")
	}
}

func (m *SelfHealMonitor) checkDatabase() {
	sqlDB, err := m.db.DB()
	if err != nil {
		m.recordHealth("database", false)
		log.Warn("数据库连接获取失败:", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = sqlDB.PingContext(ctx)
	healthy := err == nil

	m.recordHealth("database", healthy)
	if !healthy {
		log.Warn("数据库Ping失败，尝试恢复连接:", err)
		m.recoverDatabase(sqlDB)
	}
}

func (m *SelfHealMonitor) recoverDatabase(sqlDB *sql.DB) {
	if err := sqlDB.Close(); err != nil {
		log.Error("数据库关闭失败:", err)
		return
	}
	log.Info("数据库连接已关闭，等待下次操作自动重连")
}

func (m *SelfHealMonitor) recordHealth(name string, healthy bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	h, exists := m.subsystems[name]
	if !exists {
		h = &SubsystemHealth{Name: name}
		m.subsystems[name] = h
	}
	h.LastCheck = time.Now()
	h.Healthy = healthy
	if healthy {
		h.FailCount = 0
	} else {
		h.FailCount++
		if h.FailCount >= m.maxFailCount {
			log.Error(fmt.Sprintf("子系统 %s 连续 %d 次健康检查失败", name, h.FailCount))
		}
	}
}

func (m *SelfHealMonitor) Snapshot() map[string]*SubsystemHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]*SubsystemHealth, len(m.subsystems))
	for k, v := range m.subsystems {
		copy := *v
		result[k] = &copy
	}
	return result
}

func safeGo(name string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error(fmt.Sprintf("后台任务 %s panic恢复: %v", name, r))
				time.AfterFunc(5*time.Second, func() {
					safeGo(name, fn)
				})
			}
		}()
		fn()
	}()
}

func startSelfHealMonitor(ctx context.Context, db *gorm.DB) *SelfHealMonitor {
	monitor := NewSelfHealMonitor(db, config.AppCfg.Server.Addr())
	monitor.Start(ctx)
	return monitor
}
