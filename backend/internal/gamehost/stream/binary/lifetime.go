package binary

import (
	"time"
)

type BinaryLifetimePolicy struct {
	MessageTTL time.Duration
}

func DefaultLifetimePolicy() BinaryLifetimePolicy {
	return BinaryLifetimePolicy{
		MessageTTL: 5 * time.Minute,
	}
}

func (p BinaryLifetimePolicy) ExpiryTime(lifetime BinaryLifetime, createdAt time.Time) time.Time {
	switch lifetime {
	case BinaryLifetimeMessage:
		return createdAt.Add(p.MessageTTL)
	case BinaryLifetimeRuntime:
		return time.Time{}
	default:
		return time.Time{}
	}
}
