// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"time"
)

func (s *service) GetUsageOverview() map[string]interface{} {
	var totalTokens, totalRequests int64
	var todayTokens, todayCalls int64
	today := time.Now().Format("2006-01-02")
	s.db.Table("messages").Select("COALESCE(SUM(tokens), 0)").Row().Scan(&totalTokens)
	s.db.Table("messages").Count(&totalRequests)
	s.db.Table("messages").Where("date(created_at) = ?", today).Select("COALESCE(SUM(tokens), 0)").Row().Scan(&todayTokens)
	s.db.Table("messages").Where("date(created_at) = ?", today).Count(&todayCalls)
	return map[string]interface{}{"totalTokens": totalTokens, "totalCost": 0, "totalRequests": totalRequests, "todayCalls": todayCalls, "todayTokens": todayTokens}
}

func (s *service) GetUsageDaily() map[string]interface{} {
	var daily []map[string]interface{}
	s.db.Raw("SELECT date(created_at) as date, COUNT(*) as count, COALESCE(SUM(tokens), 0) as tokens FROM messages GROUP BY date(created_at) ORDER BY date DESC LIMIT 30").Scan(&daily)
	if daily == nil {
		daily = []map[string]interface{}{}
	}
	return map[string]interface{}{"daily": daily}
}

func (s *service) GetUsageModels() map[string]interface{} {
	var models []map[string]interface{}
	s.db.Table("model_configs").Select("model_name as name, api_type as provider").Find(&models)
	if models == nil {
		models = []map[string]interface{}{}
	}
	return map[string]interface{}{"models": models}
}

func (s *service) GetUsageSources() map[string]interface{} {
	var sources []map[string]interface{}
	s.db.Raw("SELECT source, COUNT(*) as count FROM messages GROUP BY source").Scan(&sources)
	if sources == nil {
		sources = []map[string]interface{}{}
	}
	return map[string]interface{}{"sources": sources}
}

func (s *service) ClearUsage() map[string]interface{} {
	s.db.Table("messages").Where("tokens > 0").Update("tokens", 0)
	return map[string]interface{}{"cleared": true}
}
