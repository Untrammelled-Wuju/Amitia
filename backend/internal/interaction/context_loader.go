package interaction

import (
	"context"
	"sync"
	"time"
)

type ContextLoader interface {
	Name() string
	IsRequired() bool
	Timeout() time.Duration
	CacheKey(scope InteractionScope, version string) string
	Load(ctx context.Context, scope InteractionScope, version string) (SnapshotField[any], error)
}

type LoaderStats struct {
	Name         string
	Duration     time.Duration
	DataVersion  string
	CancelReason string
	HealthStatus string
	IsRequired   bool
}

type ContextLoaderRegistry struct {
	mu      sync.RWMutex
	loaders []ContextLoader
	stats   []LoaderStats
}

func NewContextLoaderRegistry() *ContextLoaderRegistry {
	return &ContextLoaderRegistry{}
}

func (r *ContextLoaderRegistry) Register(loader ContextLoader) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loaders = append(r.loaders, loader)
}

func (r *ContextLoaderRegistry) Stats() []LoaderStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]LoaderStats, len(r.stats))
	copy(out, r.stats)
	return out
}

func (r *ContextLoaderRegistry) LoadAll(ctx context.Context, scope InteractionScope, version string) ContextSnapshot {
	snapshot := ContextSnapshot{
		Version:     version,
		AssembledAt: time.Now(),
	}

	r.mu.RLock()
	loaders := make([]ContextLoader, len(r.loaders))
	copy(loaders, r.loaders)
	r.mu.RUnlock()

	stats := make([]LoaderStats, len(loaders))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i, loader := range loaders {
		wg.Add(1)
		go func(idx int, l ContextLoader) {
			defer wg.Done()

			start := time.Now()
			st := LoaderStats{
				Name:       l.Name(),
				IsRequired: l.IsRequired(),
			}

			loaderCtx, cancel := context.WithTimeout(ctx, l.Timeout())
			defer cancel()

			field, err := l.Load(loaderCtx, scope, version)
			st.Duration = time.Since(start)

			if ctx.Err() != nil {
				st.CancelReason = ctx.Err().Error()
				st.HealthStatus = "cancelled"
				mu.Lock()
				stats[idx] = st
				mu.Unlock()
				return
			}

			if err != nil {
				st.HealthStatus = "error"
				if l.IsRequired() {
					field = FieldError[any](l.Name())
				} else {
					field = FieldUnavailable[any](l.Name())
				}
			} else {
				st.HealthStatus = "ok"
				st.DataVersion = field.Version
			}

			mu.Lock()
			stats[idx] = st
			mu.Unlock()

			applySnapshotField(&snapshot, l.Name(), field)
		}(i, loader)
	}

	wg.Wait()

	r.mu.Lock()
	r.stats = stats
	r.mu.Unlock()

	return snapshot
}

func applySnapshotField(snapshot *ContextSnapshot, name string, field SnapshotField[any]) {
	switch name {
	case "runtimeProfile":
		setSnapshotField(&snapshot.RuntimeProfile, field)
	case "conversation":
		setSnapshotField(&snapshot.Conversation, field)
	case "psyche":
		setSnapshotField(&snapshot.Psyche, field)
	case "relationship":
		setSnapshotField(&snapshot.Relationship, field)
	case "beliefs":
		setSnapshotField(&snapshot.Beliefs, field)
	case "memories":
		setSnapshotField(&snapshot.Memories, field)
	case "life":
		setSnapshotField(&snapshot.Life, field)
	case "channel":
		setSnapshotField(&snapshot.Channel, field)
	default:
	}
}

func setSnapshotField[T any](target *SnapshotField[T], src SnapshotField[any]) {
	if v, ok := src.Value.(T); ok {
		target.Value = v
	}
	target.Source = src.Source
	target.Status = src.Status
	target.Version = src.Version
}