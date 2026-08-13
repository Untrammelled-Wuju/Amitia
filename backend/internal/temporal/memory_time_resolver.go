// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package temporal

import (
	"context"
	"fmt"
	"time"
)

type MemoryTimeResolver struct {
	temporal *Service
	clock    Clock
}

func NewMemoryTimeResolver(temporalSvc *Service, clock Clock) *MemoryTimeResolver {
	if clock == nil {
		clock = SystemClock{}
	}
	return &MemoryTimeResolver{temporal: temporalSvc, clock: clock}
}

type RelativeMemoryTimeInput struct {
	Expression       string
	UserID           string
	CharacterID      string
	ReferenceTimeUTC *time.Time
}

func (r *MemoryTimeResolver) Resolve(input RelativeMemoryTimeInput) (ResolvedMemoryTimeRange, error) {
	refTime := r.clock.Now()
	if input.ReferenceTimeUTC != nil {
		refTime = *input.ReferenceTimeUTC
	}
	loc, timezone, err := r.resolveLocation(input.UserID, input.CharacterID)
	if err != nil {
		return ResolvedMemoryTimeRange{}, err
	}
	localRef := refTime.In(loc)
	expr := normalizeExpression(input.Expression)
	return r.resolveExpression(expr, localRef, refTime, timezone, input.Expression)
}

func (r *MemoryTimeResolver) resolveExpression(expr string, localRef time.Time, refUTC time.Time, timezone string, rawExpr string) (ResolvedMemoryTimeRange, error) {
	year, month, day := localRef.Date()
	localMidnight := time.Date(year, month, day, 0, 0, 0, 0, localRef.Location())
	switch expr {
	case "today":
		from := localMidnight
		to := from.AddDate(0, 0, 1)
		return rangeResponse(from, to, timezone, "", "", "day", "exact", rawExpr), nil
	case "yesterday":
		from := localMidnight.AddDate(0, 0, -1)
		to := localMidnight
		return rangeResponse(from, to, timezone, "", "", "day", "exact", rawExpr), nil
	case "this_week":
		weekStart := weekStartLocal(localRef, r.weekStartOffset())
		from := weekStart
		to := from.AddDate(0, 0, 7)
		return rangeResponse(from, to, timezone, "", "", "week", "exact", rawExpr), nil
	case "last_week":
		weekStart := weekStartLocal(localRef, r.weekStartOffset())
		from := weekStart.AddDate(0, 0, -7)
		to := weekStart
		return rangeResponse(from, to, timezone, "", "", "week", "exact", rawExpr), nil
	case "this_month":
		from := time.Date(year, month, 1, 0, 0, 0, 0, localRef.Location())
		to := from.AddDate(0, 1, 0)
		return rangeResponse(from, to, timezone, "", "", "month", "exact", rawExpr), nil
	case "last_month":
		firstOfThisMonth := time.Date(year, month, 1, 0, 0, 0, 0, localRef.Location())
		from := firstOfThisMonth.AddDate(0, -1, 0)
		to := firstOfThisMonth
		return rangeResponse(from, to, timezone, "", "", "month", "exact", rawExpr), nil
	case "this_year":
		from := time.Date(year, 1, 1, 0, 0, 0, 0, localRef.Location())
		to := from.AddDate(1, 0, 0)
		return rangeResponse(from, to, timezone, "", "", "year", "exact", rawExpr), nil
	case "last_year":
		from := time.Date(year-1, 1, 1, 0, 0, 0, 0, localRef.Location())
		to := time.Date(year, 1, 1, 0, 0, 0, 0, localRef.Location())
		return rangeResponse(from, to, timezone, "", "", "year", "exact", rawExpr), nil
	case "past_7_days":
		from := localMidnight.AddDate(0, 0, -6)
		to := localMidnight.AddDate(0, 0, 1)
		return rangeResponse(from, to, timezone, "", "", "range", "exact", rawExpr), nil
	case "past_30_days":
		from := localMidnight.AddDate(0, 0, -29)
		to := localMidnight.AddDate(0, 0, 1)
		return rangeResponse(from, to, timezone, "", "", "range", "exact", rawExpr), nil
	default:
		if d, ok := parsePastNDays(expr); ok {
			from := localMidnight.AddDate(0, 0, -(d - 1))
			to := localMidnight.AddDate(0, 0, 1)
			return rangeResponse(from, to, timezone, "", "", "range", "exact", rawExpr), nil
		}
		return ResolvedMemoryTimeRange{SourceExpression: rawExpr, Confidence: "unsupported"}, fmt.Errorf("MEMORY_TIME_PRESET_UNSUPPORTED")
	}
}

func (r *MemoryTimeResolver) resolveLocation(userID, characterID string) (*time.Location, string, error) {
	if r.temporal == nil || r.temporal.repo == nil {
		return time.UTC, "UTC", nil
	}
	profile, err := r.temporal.GetProfile(context.Background(), OwnerUser, userID)
	if err != nil || profile == nil {
		return time.UTC, "UTC", nil
	}
	loc, err := time.LoadLocation(profile.Timezone)
	if err != nil {
		return time.UTC, "UTC", nil
	}
	return loc, profile.Timezone, nil
}

func (r *MemoryTimeResolver) weekStartOffset() int {
	return 1
}

func rangeResponse(fromLocal, toLocal time.Time, timezone, localFrom, localTo, precision, confidence, rawExpr string) ResolvedMemoryTimeRange {
	f := utc(fromLocal)
	t := utc(toLocal)
	if localFrom == "" {
		localFrom = fromLocal.Format("2006-01-02")
	}
	if localTo == "" {
		localTo = toLocal.Format("2006-01-02")
	}
	return ResolvedMemoryTimeRange{
		FromUTC:          &f,
		ToUTC:            &t,
		Timezone:         timezone,
		LocalDateFrom:    localFrom,
		LocalDateTo:      localTo,
		Precision:        precision,
		SourceExpression: rawExpr,
		Confidence:       confidence,
	}
}

func weekStartLocal(t time.Time, weekStartIdx int) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	offset := weekday - weekStartIdx
	if offset < 0 {
		offset += 7
	}
	y, m, d := t.AddDate(0, 0, -offset).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func normalizeExpression(expr string) string {
	return expr
}

func parsePastNDays(expr string) (int, bool) {
	var n int
	if _, err := fmt.Sscanf(expr, "past_%d_days", &n); err == nil && n > 0 {
		return n, true
	}
	return 0, false
}
