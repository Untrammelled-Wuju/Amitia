package behavior

import "time"

type SemanticHistory struct {
	records []RecentSemanticRecord
	maxSize int
}

func NewSemanticHistory(maxSize int) *SemanticHistory {
	if maxSize <= 0 {
		maxSize = MaxRecentSemantics
	}
	return &SemanticHistory{
		records: make([]RecentSemanticRecord, 0, maxSize),
		maxSize: maxSize,
	}
}

func (h *SemanticHistory) Add(semantic, actionKey string, at time.Time) {
	h.records = append(h.records, RecentSemanticRecord{
		Semantic:  semantic,
		ActionKey: actionKey,
		At:        at,
	})
	if len(h.records) > h.maxSize {
		h.records = h.records[len(h.records)-h.maxSize:]
	}
}

func (h *SemanticHistory) LastSemantic() string {
	if len(h.records) == 0 {
		return ""
	}
	return h.records[len(h.records)-1].Semantic
}

func (h *SemanticHistory) LastAction() string {
	if len(h.records) == 0 {
		return ""
	}
	return h.records[len(h.records)-1].ActionKey
}

func (h *SemanticHistory) IsSameSemantic(semantic string) bool {
	return h.LastSemantic() == semantic && semantic != ""
}

func (h *SemanticHistory) CountSince(semantic string, since time.Time) int {
	count := 0
	for _, r := range h.records {
		if r.Semantic == semantic && !r.At.Before(since) {
			count++
		}
	}
	return count
}

func (h *SemanticHistory) Recent() []RecentSemanticRecord {
	result := make([]RecentSemanticRecord, len(h.records))
	copy(result, h.records)
	return result
}

func (h *SemanticHistory) Clear() {
	h.records = h.records[:0]
}

func ContextFromHistory(ctx *BehaviorContextSnapshot) *SemanticHistory {
	h := NewSemanticHistory(MaxRecentSemantics)
	h.records = make([]RecentSemanticRecord, len(ctx.RecentSemantics))
	copy(h.records, ctx.RecentSemantics)
	return h
}

func SyncHistoryToContext(ctx *BehaviorContextSnapshot, h *SemanticHistory) {
	ctx.RecentSemantics = h.Recent()
}
