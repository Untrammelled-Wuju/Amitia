package display

import (
	"context"
	"sync"
)

type DisplayListenerRegistry interface {
	Register(DisplayEvent)
	Subscribe(id string) (<-chan DisplayEvent, func())
}

type Listener struct {
	mu        sync.RWMutex
	subs      map[string]chan DisplayEvent
	nextID    uint64
}

func NewListener() *Listener {
	return &Listener{
		subs: make(map[string]chan DisplayEvent),
	}
}

func (l *Listener) Emit(evt DisplayEvent) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	for _, ch := range l.subs {
		select {
		case ch <- evt:
		default:
		}
	}
}

func (l *Listener) Subscribe() (<-chan DisplayEvent, func()) {
	l.mu.Lock()
	defer l.mu.Unlock()
	id := l.nextID
	l.nextID++
	ch := make(chan DisplayEvent, 8)
	subID := string(rune('s'+id))
	l.subs[subID] = ch
	cancel := func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		delete(l.subs, subID)
		close(ch)
	}
	return ch, cancel
}

func (l *Listener) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for id, ch := range l.subs {
		delete(l.subs, id)
		close(ch)
	}
}

func (l *Listener) CountSubscribers() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.subs)
}

func BuildEventFromRemove(rec *DisplayRecord, observedAt int64) DisplayEvent {
	return DisplayEvent{
		Type:       string(EventTypeRemoved),
		DisplayID:  rec.Info.DisplayID,
		Generation: rec.Info.Generation,
		Snapshot:   rec.Info,
		ObservedAt: observedAt,
	}
}

func BuildEventFromAdd(info DisplayInfo, observedAt int64) DisplayEvent {
	return DisplayEvent{
		Type:       string(EventTypeAdded),
		DisplayID:  info.DisplayID,
		Generation: info.Generation,
		Snapshot:   info,
		ObservedAt: observedAt,
	}
}

func BuildEventFromChange(prev, next DisplayInfo, changedFields []string, observedAt int64) DisplayEvent {
	return DisplayEvent{
		Type:          string(EventTypeChanged),
		DisplayID:     next.DisplayID,
		Generation:    next.Generation,
		Snapshot:      next,
		ChangedFields: changedFields,
		ObservedAt:    observedAt,
	}
}

func ChangedFieldsDisplay(prev, next DisplayInfo) []string {
	fields := []string{}
	if prev.Width != next.Width || prev.Height != next.Height {
		fields = append(fields, "size")
	}
	if prev.Rotation != next.Rotation {
		fields = append(fields, "rotation")
	}
	if prev.DensityDPI != next.DensityDPI {
		fields = append(fields, "density")
	}
	if prev.State != next.State {
		fields = append(fields, "state")
	}
	if prev.RefreshRate != next.RefreshRate {
		fields = append(fields, "refreshRate")
	}
	if prev.IsValid != next.IsValid {
		fields = append(fields, "valid")
	}
	return fields
}

func DisplayFromNative(ctx context.Context, payload map[string]any) (DisplayInfo, error) {
	return DisplayInfo{}, nil
}
