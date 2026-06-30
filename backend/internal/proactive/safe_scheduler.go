// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package proactive

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"gorm.io/gorm"
)

type SafeScheduler struct {
	db       *gorm.DB
	timer    *time.Timer
	stopCh   chan struct{}
	mu       sync.Mutex
	executor *Executor
}

func NewSafeScheduler(db *gorm.DB, exec *Executor) *SafeScheduler {
	return &SafeScheduler{
		db:       db,
		executor: exec,
		stopCh:   make(chan struct{}),
	}
}

func (s *SafeScheduler) Start() {
	go s.loop()
	log.Println("[SafeScheduler] 定时调度器已启动（Timer模式）")
}

func (s *SafeScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer != nil {
		s.timer.Stop()
	}
	close(s.stopCh)
	log.Println("[SafeScheduler] 定时调度器已停止")
}

func (s *SafeScheduler) loop() {
	for {
		nextFire := s.calcNextFire()
		if nextFire.IsZero() {
			select {
			case <-time.After(60 * time.Second):
			case <-s.stopCh:
				return
			}
			continue
		}

		delay := time.Until(nextFire)
		if delay < 0 {
			delay = 100 * time.Millisecond
		}

		s.mu.Lock()
		s.timer = time.NewTimer(delay)
		s.mu.Unlock()

		select {
		case <-s.timer.C:
			s.fireRules()
		case <-s.stopCh:
			s.mu.Lock()
			if s.timer != nil {
				s.timer.Stop()
			}
			s.mu.Unlock()
			return
		}
	}
}

func (s *SafeScheduler) calcNextFire() time.Time {
	var earliest time.Time

	now := time.Now()
	todayStr := now.Format("2006-01-02")

	rows, err := s.db.Table("proactive_rules").
		Select("id, schedule_cron, random_minutes, enabled, max_per_day, sent_count_today, quiet_start, quiet_end, last_sent_at").
		Where("enabled = 1 AND schedule_cron != \"\"").Rows()
	if err != nil {
		return time.Time{}
	}
	defer rows.Close()

	for rows.Next() {
		var id, enabled, maxPerDay, sentToday, randomMinutes int
		var cron, quietStart, quietEnd, lastSentAt string
		rows.Scan(&id, &cron, &randomMinutes, &enabled, &maxPerDay, &sentToday, &quietStart, &quietEnd, &lastSentAt)

		if sentToday >= maxPerDay {
			continue
		}

		baseMin := parseCronMinute(cron)
		if baseMin < 0 {
			continue
		}

		window := randomMinutes
		if window <= 0 {
			window = 30
		}

		rng := rand.New(rand.NewSource(int64(id) + now.Unix()/86400))
		offset := rng.Intn(window*2+1) - window
		targetMin := baseMin + offset
		if targetMin < 0 {
			targetMin = 0
		}
		if targetMin > 1439 {
			targetMin = 1439
		}
		targetHour := targetMin / 60
		targetMinute := targetMin % 60

		fireTime, _ := time.ParseInLocation("2006-01-02 15:04",
			todayStr+" "+pad2(targetHour)+":"+pad2(targetMinute), time.Local)
		if fireTime.IsZero() {
			continue
		}

		if fireTime.Before(now) {
			fireTime = fireTime.Add(24 * time.Hour)
		}

		if lastSentAt != "" && len(lastSentAt) >= 19 {
			if lastTime, err := time.Parse("2006-01-02 15:04:05", lastSentAt[:19]); err == nil {
				cooldown := time.Duration(window+10) * time.Minute
				if now.Sub(lastTime) < cooldown {
					tomorrow := fireTime.Add(24 * time.Hour)
					if earliest.IsZero() || tomorrow.Before(earliest) {
						earliest = tomorrow
					}
					continue
				}
			}
		}

		if quietStart != "" && quietEnd != "" {
			fireTimeStr := fireTime.Format("15:04")
			if !quietHoursAllow(quietStart, quietEnd, fireTimeStr) {
				fireTime = fireTime.Add(24 * time.Hour)
			}
		}

		if earliest.IsZero() || fireTime.Before(earliest) {
			earliest = fireTime
		}
	}

	return earliest
}

func (s *SafeScheduler) fireRules() {
	now := time.Now()
	todayStr := now.Format("2006-01-02")
	timeStr := now.Format("15:04")

	type rule struct {
		id, enabled, maxPerDay, sentToday, randomMinutes            int
		name, channel, ruleType, cron, quietStart, quietEnd, prompt string
		charID, lastSentAt                                          string
	}

	rows, err := s.db.Table("proactive_rules").
		Select("id, name, enabled, channel, character_id, rule_type, schedule_cron, quiet_start, quiet_end, max_per_day, sent_count_today, prompt_template, random_minutes, COALESCE(last_sent_at,'')").
		Where("enabled = 1 AND schedule_cron != \"\"").Rows()
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var r rule
		rows.Scan(&r.id, &r.name, &r.enabled, &r.channel, &r.charID, &r.ruleType,
			&r.cron, &r.quietStart, &r.quietEnd, &r.maxPerDay, &r.sentToday,
			&r.prompt, &r.randomMinutes, &r.lastSentAt)

		if r.sentToday >= r.maxPerDay {
			continue
		}
		if !quietHoursAllow(r.quietStart, r.quietEnd, timeStr) {
			continue
		}

		baseMin := parseCronMinute(r.cron)
		if baseMin < 0 {
			continue
		}

		window := r.randomMinutes
		if window <= 0 {
			window = 30
		}

		rng := rand.New(rand.NewSource(int64(r.id) + now.Unix()/86400))
		offset := rng.Intn(window*2+1) - window
		targetMin := baseMin + offset
		if targetMin < 0 {
			targetMin = 0
		}
		if targetMin > 1439 {
			targetMin = 1439
		}

		targetFire, _ := time.ParseInLocation("2006-01-02 15:04",
			todayStr+" "+pad2(targetMin/60)+":"+pad2(targetMin%60), time.Local)
		if targetFire.IsZero() {
			continue
		}

		if now.Before(targetFire) || now.Sub(targetFire) > time.Duration(window+5)*time.Minute {
			continue
		}

		if len(r.lastSentAt) >= 19 {
			if lastTime, err := time.Parse("2006-01-02 15:04:05", r.lastSentAt[:19]); err == nil {
				if now.Sub(lastTime) < time.Duration(window+10)*time.Minute {
					continue
				}
			}
		}

		log.Printf("[SafeScheduler] 触发规则 id=%d name=%s", r.id, r.name)
		go s.executor.executeRule(rule{
			id: r.id, enabled: r.enabled, maxPerDay: r.maxPerDay, sentToday: r.sentToday,
			randomMinutes: r.randomMinutes, name: r.name, channel: r.channel,
			ruleType: r.ruleType, cron: r.cron, quietStart: r.quietStart,
			quietEnd: r.quietEnd, prompt: r.prompt, charID: r.charID,
		})
	}
}

func pad2(n int) string {
	return fmt.Sprintf("%02d", n)
}
