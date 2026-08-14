package runtimeorchestrator

import (
	"time"

	"github.com/u-ai/backend/internal/runtimeprofile"
	"github.com/u-ai/backend/pkg/platform"
)

type RuntimeSnapshot struct {
	State         OrchestratorState
	Runtime       platform.RuntimeDescriptor
	Profile       runtimeprofile.Profile `json:"profile"`
	Components    map[ComponentID]ComponentStatus
	ReadyCount    int
	DisabledCount int
	DegradedCount int
	FailedCount   int
	BlockingCount int
	Timestamp     time.Time
}

func (s RuntimeSnapshot) IsReady() bool {
	return s.State == OrchestratorReady || s.State == OrchestratorDegraded
}

func (s RuntimeSnapshot) IsBlocked() bool {
	return s.State == OrchestratorBlocked || s.State == OrchestratorStopping || s.State == OrchestratorStopped
}
