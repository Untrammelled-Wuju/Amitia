package lifecycle

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"time"
)

func now() time.Time {
	return time.Now().UTC()
}

func computePlanHash(plan *BootstrapPlan) string {
	if plan == nil {
		return ""
	}
	ids := make([]string, 0, len(plan.Components))
	for _, c := range plan.Components {
		ids = append(ids, c.ID+":"+string(c.Phase))
	}
	sort.Strings(ids)
	h := sha256.New()
	for _, id := range ids {
		h.Write([]byte(id))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

type StartupStatus string

const (
	StartupStatusPending    StartupStatus = "pending"
	StartupStatusStarting   StartupStatus = "starting"
	StartupStatusStarted    StartupStatus = "started"
	StartupStatusReady      StartupStatus = "ready"
	StartupStatusDegraded   StartupStatus = "degraded"
	StartupStatusFailed     StartupStatus = "failed"
	StartupStatusSkipped    StartupStatus = "skipped"
	StartupStatusRolledBack StartupStatus = "rolled_back"
)

type ShutdownStatus string

const (
	ShutdownStatusPending          ShutdownStatus = "pending"
	ShutdownStatusStopping         ShutdownStatus = "stopping"
	ShutdownStatusStopped          ShutdownStatus = "stopped"
	ShutdownStatusTimedOut         ShutdownStatus = "timed_out"
	ShutdownStatusFailed           ShutdownStatus = "failed"
	ShutdownStatusForced           ShutdownStatus = "forced"
	ShutdownStatusRecoveryRequired ShutdownStatus = "recovery_required"
)

type ActiveOperation struct {
	ID            string
	ComponentID   string
	Kind          string
	StartedAt     time.Time
	Cancelable    bool
	RequiresFlush bool
	Metadata      map[string]any
}
