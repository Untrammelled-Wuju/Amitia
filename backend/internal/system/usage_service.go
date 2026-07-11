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

type dailyRow struct {
	Date          string
	Messages      int64
	ModelCalls    int64
	Tokens        int64
	Conversations int64
	Memories      int64
	Feedback      int64
}

func (s *service) GetUsageDaily() map[string]interface{} {
	rows, err := s.db.Raw(`
		SELECT
			d.date,
			COALESCE(m.messages, 0)      AS messages,
			COALESCE(m.model_calls, 0)   AS model_calls,
			COALESCE(m.tokens, 0)        AS tokens,
			COALESCE(c.conversations, 0) AS conversations,
			COALESCE(mem.memories, 0)    AS memories,
			COALESCE(fb.feedback, 0)     AS feedback
		FROM (
			SELECT date(created_at) AS date FROM messages
			UNION
			SELECT date(created_at) FROM conversations
			UNION
			SELECT date(created_at) FROM memories
			UNION
			SELECT date(created_at) FROM message_feedback
		) d
		LEFT JOIN (
			SELECT date(created_at) AS date,
				COUNT(*)                                             AS messages,
				COALESCE(SUM(CASE WHEN role = 'assistant' THEN 1 ELSE 0 END), 0) AS model_calls,
				COALESCE(SUM(tokens), 0)                            AS tokens
			FROM messages GROUP BY date(created_at)
		) m ON d.date = m.date
		LEFT JOIN (
			SELECT date(created_at) AS date, COUNT(*) AS conversations
			FROM conversations GROUP BY date(created_at)
		) c ON d.date = c.date
		LEFT JOIN (
			SELECT date(created_at) AS date, COUNT(*) AS memories
			FROM memories GROUP BY date(created_at)
		) mem ON d.date = mem.date
		LEFT JOIN (
			SELECT date(created_at) AS date, COUNT(*) AS feedback
			FROM message_feedback GROUP BY date(created_at)
		) fb ON d.date = fb.date
		ORDER BY d.date DESC
		LIMIT 30
	`).Rows()

	var daily []map[string]interface{}
	if err == nil && rows != nil {
		defer rows.Close()
		for rows.Next() {
			var r dailyRow
			if scanErr := rows.Scan(&r.Date, &r.Messages, &r.ModelCalls, &r.Tokens, &r.Conversations, &r.Memories, &r.Feedback); scanErr == nil {
				daily = append(daily, map[string]interface{}{
					"date":          r.Date,
					"messages":      r.Messages,
					"modelCalls":    r.ModelCalls,
					"tokens":        r.Tokens,
					"conversations": r.Conversations,
					"memories":      r.Memories,
					"feedback":      r.Feedback,
				})
			}
		}
	}

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
