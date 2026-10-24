package main

import (
	"fmt"
	"time"
)

const ownedWorkerAttemptTimeout = 30 * time.Minute

func ownedWorkerEffectiveDeadline(now time.Time, coordinationDeadline *time.Time) (time.Time, error) {
	limit := now.Add(ownedWorkerAttemptTimeout)
	if coordinationDeadline != nil && coordinationDeadline.Before(limit) {
		limit = *coordinationDeadline
	}
	if !limit.After(now) {
		return time.Time{}, fmt.Errorf("multi_agent: durable parent coordination deadline has expired")
	}
	return limit, nil
}
