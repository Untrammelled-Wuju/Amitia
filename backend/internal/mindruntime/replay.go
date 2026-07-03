package mindruntime

import (
	"sort"
	"strings"
	"time"
)

type RuntimeReplayRecord struct {
	ID             string                     `json:"id"`
	Report         RuntimeObservabilityReport `json:"report"`
	Index          RuntimeReplayIndex         `json:"index"`
	RetentionUntil time.Time                  `json:"retentionUntil"`
	Redacted       bool                       `json:"redacted"`
}

type RuntimeReplayIndex struct {
	SnapshotID    string `json:"snapshotId"`
	RequestID     string `json:"requestId,omitempty"`
	EventID       string `json:"eventId,omitempty"`
	InteractionID string `json:"interactionId,omitempty"`
	DeliveryID    string `json:"deliveryId,omitempty"`
	ToolID        string `json:"toolId,omitempty"`
	CharacterID   string `json:"characterId,omitempty"`
	Scope         string `json:"scope,omitempty"`
}

type RuntimeReplayQuery struct {
	SnapshotID    string
	RequestID     string
	EventID       string
	InteractionID string
	DeliveryID    string
	ToolID        string
	CharacterID   string
	Scope         string
}

type ReconstructedCausalChain struct {
	RootEvent  RuntimeCausalEvent              `json:"rootEvent"`
	Children   map[string][]RuntimeCausalEvent `json:"children"`
	Ancestors  map[string][]RuntimeCausalEvent `json:"ancestors"`
	TotalDepth int                             `json:"totalDepth"`
	EventCount int                             `json:"eventCount"`
}

type ReplaySideEffect struct {
	EventID    string         `json:"eventId"`
	Kind       TraceEventKind `json:"kind"`
	Applied    bool           `json:"applied"`
	RolledBack bool           `json:"rolledBack"`
	RecordedAt time.Time      `json:"recordedAt"`
}

type ReplaySideEffectLog struct {
	effects []ReplaySideEffect
}

func (l *ReplaySideEffectLog) Record(eventID string, kind TraceEventKind, applied bool) {
	l.effects = append(l.effects, ReplaySideEffect{
		EventID:    eventID,
		Kind:       kind,
		Applied:    applied,
		RolledBack: false,
		RecordedAt: time.Now().UTC(),
	})
}

func (l *ReplaySideEffectLog) Rollback(eventID string) {
	for i := range l.effects {
		if l.effects[i].EventID == eventID {
			l.effects[i].RolledBack = true
		}
	}
}

func (l *ReplaySideEffectLog) AppliedEffects() []ReplaySideEffect {
	result := make([]ReplaySideEffect, 0)
	for _, e := range l.effects {
		if e.Applied {
			result = append(result, e)
		}
	}
	return result
}

func NewReplaySideEffectLog() *ReplaySideEffectLog {
	return &ReplaySideEffectLog{
		effects: make([]ReplaySideEffect, 0),
	}
}

func BuildRuntimeReplayRecord(input RuntimeObservabilityInput, retention time.Duration) RuntimeReplayRecord {
	report := BuildRuntimeObservabilityReport(input)
	record := RuntimeReplayRecord{
		ID:     report.SnapshotID,
		Report: report,
		Index: RuntimeReplayIndex{
			SnapshotID:    report.SnapshotID,
			RequestID:     report.RequestID,
			EventID:       report.EventID,
			InteractionID: report.InteractionID,
			DeliveryID:    strings.TrimSpace(input.DeliveryID),
			ToolID:        strings.TrimSpace(input.ToolID),
			CharacterID:   report.CharacterID,
			Scope:         strings.TrimSpace(input.Scope),
		},
		Redacted: true,
	}
	if retention > 0 {
		record.RetentionUntil = report.CreatedAt.Add(retention).UTC()
	}
	return record
}

func (record RuntimeReplayRecord) Matches(query RuntimeReplayQuery) bool {
	if query.SnapshotID != "" && query.SnapshotID != record.Index.SnapshotID {
		return false
	}
	if query.RequestID != "" && query.RequestID != record.Index.RequestID {
		return false
	}
	if query.EventID != "" && query.EventID != record.Index.EventID {
		return false
	}
	if query.InteractionID != "" && query.InteractionID != record.Index.InteractionID {
		return false
	}
	if query.DeliveryID != "" && query.DeliveryID != record.Index.DeliveryID {
		return false
	}
	if query.ToolID != "" && query.ToolID != record.Index.ToolID {
		return false
	}
	if query.CharacterID != "" && query.CharacterID != record.Index.CharacterID {
		return false
	}
	if query.Scope != "" && query.Scope != record.Index.Scope {
		return false
	}
	return query.SnapshotID != "" || query.RequestID != "" || query.EventID != "" || query.InteractionID != "" || query.DeliveryID != "" || query.ToolID != "" || query.CharacterID != "" || query.Scope != ""
}

func (record RuntimeReplayRecord) ExpiredAt(now time.Time) bool {
	if record.RetentionUntil.IsZero() {
		return false
	}
	return !now.UTC().Before(record.RetentionUntil)
}

func ReconstructCausalChain(events []RuntimeCausalEvent) ReconstructedCausalChain {
	if len(events) == 0 {
		return ReconstructedCausalChain{
			Children:  make(map[string][]RuntimeCausalEvent),
			Ancestors: make(map[string][]RuntimeCausalEvent),
		}
	}
	children := make(map[string][]RuntimeCausalEvent)
	ancestors := make(map[string][]RuntimeCausalEvent)
	parentChildMap := make(map[string][]RuntimeCausalEvent)
	var rootEvent RuntimeCausalEvent
	maxDepth := 0

	sort.SliceStable(events, func(i, j int) bool {
		return events[i].Index < events[j].Index
	})

	for i := range events {
		event := events[i]
		if event.ParentID != "" {
			parentChildMap[event.ParentID] = append(parentChildMap[event.ParentID], event)
		}
		if i == 0 || rootEvent.Index == 0 {
			rootEvent = event
		}
	}

	for _, event := range events {
		for parentID, childList := range parentChildMap {
			for _, child := range childList {
				if child.Index == event.Index {
					continue
				}
				if parentID == event.ID || parentID == event.ParentID {
					continue
				}
			}
		}
	}

	for parentID, childList := range parentChildMap {
		children[parentID] = childList
		depth := computeDepth(parentID, parentChildMap, 0, make(map[string]bool))
		if depth > maxDepth {
			maxDepth = depth
		}
	}

	for _, event := range events {
		ancChain := traceAncestors(event, events)
		if len(ancChain) > 0 {
			ancestors[event.ID] = ancChain
		}
	}

	return ReconstructedCausalChain{
		RootEvent:  rootEvent,
		Children:   children,
		Ancestors:  ancestors,
		TotalDepth: maxDepth,
		EventCount: len(events),
	}
}

func computeDepth(eventID string, children map[string][]RuntimeCausalEvent, current int, visited map[string]bool) int {
	if visited[eventID] {
		return current
	}
	visited[eventID] = true
	childList := children[eventID]
	maxChildDepth := current
	for _, child := range childList {
		childDepth := computeDepth(child.ID, children, current+1, visited)
		if childDepth > maxChildDepth {
			maxChildDepth = childDepth
		}
	}
	return maxChildDepth
}

func traceAncestors(event RuntimeCausalEvent, allEvents []RuntimeCausalEvent) []RuntimeCausalEvent {
	ancestors := make([]RuntimeCausalEvent, 0)
	visited := make(map[string]bool)
	currentParentID := event.ParentID

	for currentParentID != "" && !visited[currentParentID] {
		visited[currentParentID] = true
		for _, e := range allEvents {
			if e.ID == currentParentID {
				ancestors = append(ancestors, e)
				currentParentID = e.ParentID
				break
			}
		}
		if !visited[currentParentID] {
			currentParentID = ""
		}
	}

	return ancestors
}

func FindEventsByKind(events []RuntimeCausalEvent, kind TraceEventKind) []RuntimeCausalEvent {
	result := make([]RuntimeCausalEvent, 0)
	for _, e := range events {
		if e.Kind == kind {
			result = append(result, e)
		}
	}
	return result
}

func FindEventsByStatus(events []RuntimeCausalEvent, status string) []RuntimeCausalEvent {
	result := make([]RuntimeCausalEvent, 0)
	for _, e := range events {
		if strings.EqualFold(strings.TrimSpace(e.Status), strings.TrimSpace(status)) {
			result = append(result, e)
		}
	}
	return result
}

func FindSupersededEvents(events []RuntimeCausalEvent) []RuntimeCausalEvent {
	return FindEventsByKind(events, TraceEventSuperseded)
}

func FindCancelledEvents(events []RuntimeCausalEvent) []RuntimeCausalEvent {
	return FindEventsByKind(events, TraceEventCancel)
}
