// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package temporal

import (
	"time"
)

type MemoryTimeBasis string

const (
	TimeBasisOccurred MemoryTimeBasis = "occurred"
	TimeBasisValid    MemoryTimeBasis = "valid"
	TimeBasisCreated  MemoryTimeBasis = "created"
)

type MemoryTemporalQuery struct {
	MemoryIDs        []string
	Basis            MemoryTimeBasis
	OccurredFromUTC  *time.Time
	OccurredToUTC    *time.Time
	ValidAtUTC       *time.Time
	ValidFromUTC     *time.Time
	ValidToUTC       *time.Time
	LocalDateFrom    string
	LocalDateTo      string
	Dayparts         []string
	Precisions       []string
	AnchorIDs        []string
	Limit            int
	Cursor           string
	Order            string
}

func (r *SQLiteRepository) QueryMemoryIDsByTime(query MemoryTemporalQuery) ([]string, int64, error) {
	db := r.db.Model(&MemoryTemporalMetadata{})
	if len(query.MemoryIDs) > 0 {
		db = db.Where("memory_id IN ?", query.MemoryIDs)
	}
	if query.OccurredFromUTC != nil {
		db = db.Where("(occurred_at_utc >= ? OR (occurred_at_utc IS NULL AND created_at_utc >= ?))", utc(*query.OccurredFromUTC), utc(*query.OccurredFromUTC))
	}
	if query.OccurredToUTC != nil {
		db = db.Where("(occurred_at_utc < ? OR occurred_at_utc IS NULL)", utc(*query.OccurredToUTC))
	}
	if query.ValidAtUTC != nil {
		t := utc(*query.ValidAtUTC)
		db = db.Where("(valid_from_utc IS NULL OR valid_from_utc <= ?) AND (valid_to_utc IS NULL OR valid_to_utc > ?)", t, t)
	}
	if query.ValidFromUTC != nil {
		db = db.Where("(valid_from_utc IS NULL OR valid_from_utc >= ?)", utc(*query.ValidFromUTC))
	}
	if query.ValidToUTC != nil {
		db = db.Where("(valid_to_utc IS NULL OR valid_to_utc <= ?)", utc(*query.ValidToUTC))
	}
	if query.LocalDateFrom != "" {
		db = db.Where("local_date >= ?", query.LocalDateFrom)
	}
	if query.LocalDateTo != "" {
		db = db.Where("local_date <= ?", query.LocalDateTo)
	}
	if len(query.Dayparts) > 0 {
		db = db.Where("daypart IN ?", query.Dayparts)
	}
	if len(query.Precisions) > 0 {
		db = db.Where("temporal_precision IN ?", query.Precisions)
	}
	var total int64
	db.Count(&total)
	order := "occurred_at_utc DESC"
	if query.Order == "asc" {
		order = "occurred_at_utc ASC"
	}
	var items []MemoryTemporalMetadata
	err := db.Order(order).Limit(query.Limit).Find(&items).Error
	if err != nil {
		return nil, 0, err
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.MemoryID)
	}
	return ids, total, nil
}

type RelativeMemoryTimeQuery struct {
	Expression       string
	UserID           string
	CharacterID      string
	ReferenceTimeUTC *time.Time
}

type ResolvedMemoryTimeRange struct {
	FromUTC          *time.Time `json:"fromUtc"`
	ToUTC            *time.Time `json:"toUtc"`
	Timezone         string     `json:"timezone"`
	LocalDateFrom    string     `json:"localDateFrom"`
	LocalDateTo      string     `json:"localDateTo"`
	Precision        string     `json:"precision"`
	SourceExpression string     `json:"sourceExpression"`
	Confidence       string     `json:"confidence"`
}

func (s *Service) ResolveRelativeMemoryTime(query RelativeMemoryTimeQuery) (ResolvedMemoryTimeRange, error) {
	return ResolvedMemoryTimeRange{Confidence: "unsupported"}, nil
}
