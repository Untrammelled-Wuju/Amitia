package notification

import "context"

type NotificationSink interface {
	Publish(
		ctx context.Context,
		notification Notification,
	) error
}
