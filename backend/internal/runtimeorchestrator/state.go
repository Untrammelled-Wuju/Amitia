package runtimeorchestrator

import "time"

type ComponentState string

const (
	StateRegistered ComponentState = "registered"
	StateDisabled   ComponentState = "disabled"
	StateStarting   ComponentState = "starting"
	StateReady      ComponentState = "ready"
	StateDegraded   ComponentState = "degraded"
	StateFailed     ComponentState = "failed"
	StateStopping   ComponentState = "stopping"
	StateStopped    ComponentState = "stopped"
)

type OrchestratorState string

const (
	OrchestratorCreated  OrchestratorState = "created"
	OrchestratorStarting OrchestratorState = "starting"
	OrchestratorReady    OrchestratorState = "ready"
	OrchestratorDegraded OrchestratorState = "degraded"
	OrchestratorBlocked  OrchestratorState = "blocked"
	OrchestratorStopping OrchestratorState = "stopping"
	OrchestratorStopped  OrchestratorState = "stopped"
)

type ComponentStatus struct {
	ID           ComponentID
	Phase        ComponentPhase
	State        ComponentState
	Enabled      bool
	Required     bool
	ProviderID   string
	Capabilities []string
	Dependencies []ComponentID
	StartedAt    time.Time
	ReadyAt      time.Time
	StoppedAt    time.Time
	LastError    string
}

func (s ComponentStatus) clone() ComponentStatus {
	s.Capabilities = cloneCapabilities(s.Capabilities)
	s.Dependencies = cloneDependencies(s.Dependencies)
	return s
}
