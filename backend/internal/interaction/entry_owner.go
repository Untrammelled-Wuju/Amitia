package interaction

import "context"

type EntryOwner struct {
	SpaceID        string
	DeviceID       string
	TargetDeviceID string
	CoreID         string
}

type entryOwnerContextKey struct{}

func WithEntryOwner(ctx context.Context, owner EntryOwner) context.Context {
	return context.WithValue(ctx, entryOwnerContextKey{}, owner)
}

func EntryOwnerFromContext(ctx context.Context) (EntryOwner, bool) {
	owner, ok := ctx.Value(entryOwnerContextKey{}).(EntryOwner)
	return owner, ok
}
