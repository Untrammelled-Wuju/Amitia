package hostapi

import "context"

type PermissionSnapshotIDProvider interface {
	CurrentSnapshotID(
		ctx context.Context,
		extensionID string,
		moduleID string,
		generation int64,
	) (snapshotID string, ok bool, err error)
}

type ScopeSnapshotIDProvider interface {
	CurrentSnapshotID(
		ctx context.Context,
		extensionID string,
		moduleID string,
		generation int64,
	) (snapshotID string, ok bool, err error)
}
