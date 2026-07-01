// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"time"
)

func (s *service) GetReplyTimingOverview() map[string]interface{} {
	var total, active int64
	s.db.Table("proactive_rules").Count(&total)
	s.db.Table("proactive_rules").Where("enabled = 1").Count(&active)
	return map[string]interface{}{"totalBuffers": total, "activeBuffers": active}
}

func (s *service) GetReplyTimingBuffers() map[string]interface{} {
	var rules []map[string]interface{}
	s.db.Table("proactive_rules").Order("created_at DESC").Find(&rules)
	if rules == nil {
		rules = []map[string]interface{}{}
	}
	return map[string]interface{}{"buffers": rules}
}

func (s *service) ReplyTimingCancelBuffer(id string) map[string]interface{} {
	s.db.Table("proactive_rules").Where("id = ?", id).Update("enabled", 0)
	return map[string]interface{}{"canceled": true}
}

func (s *service) ReplyTimingForceBuffer(id string) map[string]interface{} {
	s.db.Table("proactive_rules").Where("id = ?", id).Update("last_sent_at", time.Now().Format("2006-01-02 15:04:05"))
	return map[string]interface{}{"forced": true, "id": id, "forcedAt": time.Now().Format(time.DateTime)}
}

func (s *service) ReplyTimingResumeBuffer(id string) map[string]interface{} {
	s.db.Table("proactive_rules").Where("id = ?", id).Update("enabled", 1)
	return map[string]interface{}{"resumed": true}
}

func (s *service) ReplyTimingForce() map[string]interface{} {
	return map[string]interface{}{"forced": true, "forcedAt": time.Now().Format(time.DateTime)}
}
