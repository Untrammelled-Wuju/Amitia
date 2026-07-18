package interaction

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/temporal"
)

type TemporalSnapshotResolver interface {
	ResolveSnapshot(ctx context.Context, input temporal.SnapshotInput) (temporal.Snapshot, error)
}

type TemporalContextLoader struct{ resolver TemporalSnapshotResolver }

func NewTemporalContextLoader(resolver TemporalSnapshotResolver) *TemporalContextLoader {
	return &TemporalContextLoader{resolver: resolver}
}
func (l *TemporalContextLoader) Name() string           { return "temporal" }
func (l *TemporalContextLoader) IsRequired() bool       { return false }
func (l *TemporalContextLoader) Timeout() time.Duration { return 800 * time.Millisecond }
func (l *TemporalContextLoader) CacheKey(scope InteractionScope, version string) string {
	return version + ":temporal:" + scope.UserID + ":" + scope.CharacterID + ":" + scope.Channel
}
func (l *TemporalContextLoader) Load(ctx context.Context, scope InteractionScope, version string) (SnapshotField[any], error) {
	if l.resolver == nil {
		return FieldUnavailable[any](l.Name()), nil
	}
	snapshot, err := l.resolver.ResolveSnapshot(ctx, temporal.SnapshotInput{UserID: scope.UserID, CharacterID: scope.CharacterID, Channel: scope.Channel, DeviceTimezone: temporal.DeviceTimezoneFromContext(ctx)})
	if err != nil {
		return FieldUnavailable[any](l.Name()), err
	}
	return FieldReady[any](snapshot, l.Name(), snapshot.Version), nil
}
