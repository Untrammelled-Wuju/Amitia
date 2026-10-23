package main

import (
	"context"
	"sort"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (p *meshLocalDataPort) BusinessUsage(ctx context.Context, scope coordination.ExecutionScope, since string) (coordination.BusinessUsage, error) {
	result := coordination.BusinessUsage{OwnerID: p.ownerID, Daily: []coordination.BusinessUsageDay{}}
	if scope.ResourceOwnerID != p.ownerID || scope.RoleOwnerID != p.ownerID {
		return result, coordination.ErrWrongOwner
	}
	if since != "" {
		if _, err := time.Parse(time.RFC3339Nano, since); err != nil {
			return result, err
		}
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return result, err
	}
	days := map[string]*coordination.BusinessUsageDay{}
	dayFor := func(day string) *coordination.BusinessUsageDay {
		if days[day] == nil {
			days[day] = &coordination.BusinessUsageDay{Date: day}
		}
		return days[day]
	}
	db := p.services.KernelContainer.DeviceRegistry.Database()
	rows, err := db.QueryContext(ctx, `SELECT strftime('%Y-%m-%d',json_extract(body,'$.createdAt')),kind,COUNT(*) FROM kernel_device_owned_resources WHERE owner_id=? AND deleted=0 AND kind IN ('message','conversation','memory') AND json_valid(body) AND json_extract(body,'$.createdAt') IS NOT NULL AND (?='' OR julianday(json_extract(body,'$.createdAt'))>=julianday(?)) GROUP BY 1,kind`, p.ownerID, since, since)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var day, kind string
		var count int64
		if err := rows.Scan(&day, &kind, &count); err != nil {
			rows.Close()
			return result, err
		}
		entry := dayFor(day)
		switch kind {
		case "message":
			entry.Messages += count
		case "conversation":
			entry.Conversations += count
		case "memory":
			entry.Memories += count
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	cursor := ""
	for {
		batch, err := db.QueryContext(ctx, `SELECT resource_id FROM kernel_device_owned_resources WHERE owner_id=? AND kind='message' AND deleted=0 AND resource_id>? AND json_extract(body,'$.role')='assistant' ORDER BY resource_id LIMIT 400`, p.ownerID, cursor)
		if err != nil {
			return result, err
		}
		keys := []string{}
		for batch.Next() {
			var id string
			if err := batch.Scan(&id); err != nil {
				batch.Close()
				return result, err
			}
			cursor = id
			keys = append(keys, id)
		}
		err = batch.Err()
		batch.Close()
		if err != nil {
			return result, err
		}
		if len(keys) == 0 {
			break
		}
		var feedback []struct {
			Day   string
			Count int64
		}
		query := p.services.DB.WithContext(ctx).Table("message_feedback").Where("space_id=? AND owner_id=? AND core_id=? AND message_id IN ? AND deleted_at IS NULL", p.legacySpaceID, p.ownerID, scope.CoreID, keys)
		if since != "" {
			query = query.Where("julianday(created_at)>=julianday(?)", since)
		}
		if err := query.Select("strftime('%Y-%m-%d',created_at) AS day,COUNT(*) AS count").Group("strftime('%Y-%m-%d',created_at)").Scan(&feedback).Error; err != nil {
			return result, err
		}
		for _, row := range feedback {
			dayFor(row.Day).Feedback += row.Count
		}
		if len(keys) < 400 {
			break
		}
	}
	for _, day := range days {
		result.Totals.Messages += day.Messages
		result.Totals.Conversations += day.Conversations
		result.Totals.Memories += day.Memories
		result.Totals.Feedback += day.Feedback
		if day.Date >= time.Now().UTC().AddDate(0, 0, -29).Format("2006-01-02") {
			result.Daily = append(result.Daily, *day)
		}
	}
	sort.Slice(result.Daily, func(i, j int) bool { return result.Daily[i].Date > result.Daily[j].Date })
	if len(result.Daily) > 30 {
		result.Daily = result.Daily[:30]
	}
	return result, coordination.ValidateCurrent(ctx)
}
