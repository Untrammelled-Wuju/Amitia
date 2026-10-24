package task_runtime

import "time"

func remoteLeaseIsCurrent(expiresAt *time.Time, now time.Time) bool {
	return expiresAt != nil && expiresAt.After(now)
}
