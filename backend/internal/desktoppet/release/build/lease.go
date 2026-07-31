package build

import (
	"time"

	"github.com/u-ai/backend/internal/desktoppet/release"
)

const defaultLeaseDuration = 5 * time.Minute

type LeaseManager struct {
	leaseDuration time.Duration
}

func NewLeaseManager() *LeaseManager {
	return &LeaseManager{leaseDuration: defaultLeaseDuration}
}

func (m *LeaseManager) AcquireLease(op *release.ReleaseBuildOperation, owner string) {
	now := time.Now()
	op.LeaseOwner = owner
	op.LeaseExpiresAt = now.Add(m.leaseDuration).Format("2006-01-02 15:04:05")
	op.HeartbeatAt = now.Format("2006-01-02 15:04:05")
}

func (m *LeaseManager) RenewLease(op *release.ReleaseBuildOperation) {
	if op.LeaseOwner == "" {
		return
	}
	now := time.Now()
	op.LeaseExpiresAt = now.Add(m.leaseDuration).Format("2006-01-02 15:04:05")
	op.HeartbeatAt = now.Format("2006-01-02 15:04:05")
}

func (m *LeaseManager) IsLeaseExpired(op *release.ReleaseBuildOperation) bool {
	if op.LeaseExpiresAt == "" {
		return true
	}
	expiresAt, err := time.Parse("2006-01-02 15:04:05", op.LeaseExpiresAt)
	if err != nil {
		return true
	}
	return time.Now().After(expiresAt)
}

func (m *LeaseManager) ReleaseLease(op *release.ReleaseBuildOperation) {
	op.LeaseOwner = ""
	op.LeaseExpiresAt = ""
	op.HeartbeatAt = ""
}

func (m *LeaseManager) CanEnterPublish(op *release.ReleaseBuildOperation, owner string) bool {
	if op.LeaseOwner == "" || op.LeaseOwner == owner {
		return true
	}
	return m.IsLeaseExpired(op)
}
