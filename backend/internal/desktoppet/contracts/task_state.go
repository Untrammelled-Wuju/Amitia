// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package contracts

type LifecycleStatus string

const (
	StatusPending            LifecycleStatus = "pending"
	StatusQueued             LifecycleStatus = "queued"
	StatusProcessing         LifecycleStatus = "processing"
	StatusCancelling         LifecycleStatus = "cancelling"
	StatusSucceeded          LifecycleStatus = "succeeded"
	StatusPartiallySucceeded LifecycleStatus = "partially_succeeded"
	StatusFailed             LifecycleStatus = "failed"
	StatusCancelled          LifecycleStatus = "cancelled"
)

func (s LifecycleStatus) String() string { return string(s) }

func (s LifecycleStatus) IsTerminal() bool {
	switch s {
	case StatusSucceeded, StatusPartiallySucceeded, StatusFailed, StatusCancelled:
		return true
	default:
		return false
	}
}

func (s LifecycleStatus) IsActive() bool {
	switch s {
	case StatusQueued, StatusProcessing, StatusCancelling:
		return true
	default:
		return false
	}
}

func (s LifecycleStatus) IsCancellable() bool {
	switch s {
	case StatusPending, StatusQueued, StatusProcessing:
		return true
	default:
		return false
	}
}

func (s LifecycleStatus) IsRetryable() bool {
	switch s {
	case StatusFailed, StatusPartiallySucceeded, StatusCancelled:
		return true
	default:
		return false
	}
}

func (s LifecycleStatus) IsRestartable() bool {
	return s == StatusSucceeded
}

var allLifecycleStatuses = []LifecycleStatus{
	StatusPending, StatusQueued, StatusProcessing, StatusCancelling,
	StatusSucceeded, StatusPartiallySucceeded, StatusFailed, StatusCancelled,
}

func AllLifecycleStatuses() []LifecycleStatus {
	return allLifecycleStatuses
}

func IsValidLifecycleStatus(s string) bool {
	for _, v := range allLifecycleStatuses {
		if string(v) == s {
			return true
		}
	}
	return false
}

var deprecatedStatuses = map[string]string{
	"skipped":    "",
	"warning":    "",
	"submitted":  "",
	"polling":    "",
	"completed":  "",
	"draft":      "",
	"running":    "",
}

func IsDeprecatedStatus(s string) bool {
	_, ok := deprecatedStatuses[s]
	return ok
}

func AllowedStatusesFor(et EntityType) []LifecycleStatus {
	switch et {
	case EntityGenerationTask, EntityGenerationAction:
		return []LifecycleStatus{
			StatusPending, StatusQueued, StatusProcessing, StatusCancelling,
			StatusSucceeded, StatusPartiallySucceeded, StatusFailed, StatusCancelled,
		}
	case EntityProcessingTask:
		return []LifecycleStatus{
			StatusPending, StatusQueued, StatusProcessing, StatusCancelling,
			StatusSucceeded, StatusPartiallySucceeded, StatusFailed, StatusCancelled,
		}
	case EntityGenerationFrame, EntityProcessedFrame:
		return []LifecycleStatus{
			StatusPending, StatusQueued, StatusProcessing, StatusCancelling,
			StatusSucceeded, StatusFailed, StatusCancelled,
		}
	case EntityProcessingAction:
		return []LifecycleStatus{
			StatusPending, StatusQueued, StatusProcessing, StatusCancelling,
			StatusSucceeded, StatusFailed, StatusCancelled,
		}
	case EntityPackage:
		return []LifecycleStatus{
			StatusPending, StatusProcessing, StatusSucceeded, StatusFailed, StatusCancelled,
		}
	default:
		return nil
	}
}

func IsAllowedStatusFor(et EntityType, s LifecycleStatus) bool {
	for _, v := range AllowedStatusesFor(et) {
		if v == s {
			return true
		}
	}
	return false
}
