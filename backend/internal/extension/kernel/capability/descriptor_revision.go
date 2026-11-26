package capability

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
)

type DescriptorEventKind string

const (
	DescriptorRegister   DescriptorEventKind = "register"
	DescriptorUnregister DescriptorEventKind = "unregister"
	DescriptorUpdate     DescriptorEventKind = "update"
)

func (k DescriptorEventKind) Valid() bool {
	return k == DescriptorRegister || k == DescriptorUnregister || k == DescriptorUpdate
}

type DescriptorEvent struct {
	Revision uint64
	Kind     DescriptorEventKind
	Tool     ToolDefinition
	ToolID   string
}

var (
	ErrDescriptorRevisionGap   = errors.New("能力索引：生命周期事件 revision 断档，必须用权威快照修复")
	ErrDescriptorRevisionStale = errors.New("能力索引：修复快照的 revision 不比当前索引新")
	ErrDescriptorEventInvalid  = errors.New("能力索引：生命周期事件缺少必填字段或类型未知")
)

type RevisionGap struct {
	Expected uint64
	Received uint64
}

func (g *RevisionGap) Error() string {
	return fmt.Sprintf("能力索引：预期 revision=%d 实际收到 revision=%d", g.Expected, g.Received)
}

func (g *RevisionGap) Unwrap() error {
	return ErrDescriptorRevisionGap
}

type ApplyReport struct {
	Applied    int
	Skipped    int
	Added      int
	Removed    int
	Updated    int
	Revision   uint64
	Gap        *RevisionGap
	SkippedIDs []string
}

func (r ApplyReport) DuplicateFree() bool {
	return r.Skipped == 0
}

type RepairReport struct {
	Added    int
	Removed  int
	Kept     int
	Revision uint64
}

func (i *DescriptorIndex) Apply(ctx context.Context, store *SchemaStore, events []DescriptorEvent) (ApplyReport, error) {
	report := ApplyReport{}
	if i == nil {
		return report, fmt.Errorf("%w: index is nil", ErrDescriptorInvalid)
	}
	if store == nil {
		return report, fmt.Errorf("%w: schema store is required", ErrDescriptorInvalid)
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	report.Revision = i.revision
	for _, event := range events {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		if event.Revision == 0 || !event.Kind.Valid() {
			return report, fmt.Errorf("%w: revision=%d kind=%q", ErrDescriptorEventInvalid, event.Revision, event.Kind)
		}
		if event.Revision <= i.revision {
			i.skipped++
			report.Skipped++
			report.SkippedIDs = append(report.SkippedIDs, event.ToolID)
			continue
		}
		if event.Revision > i.revision+1 {
			gap := &RevisionGap{Expected: i.revision + 1, Received: event.Revision}
			report.Gap = gap
			return report, gap
		}
		added, removed, updated, consumed, err := i.applyEvent(store, event)
		if err != nil {
			return report, err
		}
		if !consumed {
			report.Skipped++
			report.SkippedIDs = append(report.SkippedIDs, event.ToolID)
			i.revision = event.Revision
			report.Revision = i.revision
			continue
		}
		if added {
			report.Added++
		}
		if removed {
			report.Removed++
		}
		if updated {
			report.Updated++
		}
		report.Applied++
		i.mutations++
		i.revision = event.Revision
		report.Revision = i.revision
	}
	return report, nil
}

func (i *DescriptorIndex) applyEvent(store *SchemaStore, event DescriptorEvent) (bool, bool, bool, bool, error) {
	if event.Kind == DescriptorUnregister {
		toolID := strings.TrimSpace(event.ToolID)
		if toolID == "" {
			toolID = strings.TrimSpace(event.Tool.ID)
		}
		if toolID == "" {
			return false, false, false, false, fmt.Errorf("%w: unregister without tool id", ErrDescriptorEventInvalid)
		}
		position, ok := i.byID[toolID]
		if !ok {
			i.skipped++
			return false, false, false, false, nil
		}
		i.entries = append(i.entries[:position], i.entries[position+1:]...)
		i.reindex()
		return false, true, false, true, nil
	}
	if strings.TrimSpace(event.Tool.ID) == "" {
		return false, false, false, false, fmt.Errorf("%w: %s without tool id", ErrDescriptorEventInvalid, event.Kind)
	}
	entry, err := descriptorEntryFor(store, event.Tool)
	if err != nil {
		return false, false, false, false, err
	}
	if position, ok := i.byID[entry.ToolID]; ok {
		if i.entries[position] == entry {
			i.skipped++
			return false, false, false, false, nil
		}
		i.entries[position] = entry
		return false, false, true, true, nil
	}
	i.entries = append(i.entries, entry)
	i.reindex()
	return true, false, false, true, nil
}

func (i *DescriptorIndex) reindex() {
	sortEntries(i.entries)
	i.byID = make(map[string]int, len(i.entries))
	for position, entry := range i.entries {
		i.byID[entry.ToolID] = position
	}
}

func sortEntries(entries []DescriptorEntry) {
	sort.Slice(entries, func(left, right int) bool {
		return entries[left].ToolID < entries[right].ToolID
	})
}

func (i *DescriptorIndex) Repair(ctx context.Context, store *SchemaStore, snapshot []ToolDefinition, revision uint64) (RepairReport, error) {
	report := RepairReport{}
	if i == nil {
		return report, fmt.Errorf("%w: index is nil", ErrDescriptorInvalid)
	}
	if err := ctx.Err(); err != nil {
		return report, err
	}
	if revision == 0 {
		return report, fmt.Errorf("%w: repair revision must be positive", ErrDescriptorEventInvalid)
	}
	i.mu.RLock()
	current := i.revision
	before := make(map[string]DescriptorEntry, len(i.entries))
	for _, entry := range i.entries {
		before[entry.ToolID] = entry
	}
	i.mu.RUnlock()
	if revision <= current {
		return report, fmt.Errorf("%w: snapshot=%d current=%d", ErrDescriptorRevisionStale, revision, current)
	}
	entries, byID, err := assembleDescriptors(store, snapshot, nil)
	if err != nil {
		return report, err
	}
	after := make(map[string]DescriptorEntry, len(entries))
	for _, entry := range entries {
		after[entry.ToolID] = entry
	}
	for id := range after {
		if _, ok := before[id]; ok {
			report.Kept++
		} else {
			report.Added++
		}
	}
	for id := range before {
		if _, ok := after[id]; !ok {
			report.Removed++
		}
	}
	i.mu.Lock()
	i.entries = entries
	i.byID = byID
	i.revision = revision
	i.rebuilds++
	i.mu.Unlock()
	report.Revision = revision
	return report, nil
}
