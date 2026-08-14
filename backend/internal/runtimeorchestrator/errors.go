package runtimeorchestrator

import (
	"errors"
	"fmt"
)

var (
	ErrDuplicateComponent         = errors.New("component already registered")
	ErrUnknownDependency          = errors.New("unknown dependency")
	ErrDependencyCycle            = errors.New("dependency cycle detected")
	ErrInvalidDescriptor          = errors.New("invalid component descriptor")
	ErrPhaseOrder                 = errors.New("invalid phase ordering or phase not ready")
	ErrRequiredComponentFailed    = errors.New("required component failed")
	ErrProviderAlreadyRegistered  = errors.New("provider already registered for slot")
	ErrProviderNotFound           = errors.New("provider not found")
	ErrProviderSlotMismatch       = errors.New("provider slot mismatch")
	ErrProviderProfileUnsupported = errors.New("provider profile unsupported")
	ErrOrchestratorStopped        = errors.New("orchestrator is stopping or stopped")
	ErrUnknownComponent           = errors.New("unknown component")
	ErrComponentDisabled          = errors.New("component is disabled")
	ErrRestartFailed              = errors.New("component restart failed")
	ErrDependencyNotReady         = errors.New("dependency not ready")
)

type componentError struct {
	base     error
	id       ComponentID
	provider string
	detail   string
}

func (e componentError) Error() string {
	msg := e.base.Error()
	if e.provider != "" {
		msg = e.provider + ": " + msg
	}
	if e.id != "" {
		msg = string(e.id) + ": " + msg
	}
	if e.detail != "" {
		msg = msg + ": " + e.detail
	}
	return msg
}

func (e componentError) Unwrap() error {
	return e.base
}

func wrapComponentErr(base error, id ComponentID, provider, detail string) error {
	return componentError{base: base, id: id, provider: provider, detail: detail}
}

func duplicateComponentErr(id ComponentID) error {
	return wrapComponentErr(ErrDuplicateComponent, id, "", "")
}

func unknownDependencyErr(dep ComponentID) error {
	return wrapComponentErr(ErrUnknownDependency, dep, "", "")
}

func dependencyCycleErr(id ComponentID, dep ComponentID) error {
	return wrapComponentErr(ErrDependencyCycle, id, "", fmt.Sprintf("depends on %s which creates a cycle", dep))
}

func invalidDescriptorErr(detail string) error {
	return wrapComponentErr(ErrInvalidDescriptor, "", "", detail)
}

func phaseOrderErr(phase ComponentPhase) error {
	return wrapComponentErr(ErrPhaseOrder, "", "", string(phase))
}

func requiredComponentFailedErr(id ComponentID, phase ComponentPhase, detail string) error {
	return wrapComponentErr(ErrRequiredComponentFailed, id, "", fmt.Sprintf("@%s: %s", phase, detail))
}

func providerAlreadyRegisteredErr(slot, providerID string) error {
	return wrapComponentErr(ErrProviderAlreadyRegistered, "", providerID, "slot="+slot)
}

func providerNotFoundErr(slot, providerID string) error {
	return wrapComponentErr(ErrProviderNotFound, "", providerID, "slot="+slot)
}

func providerSlotMismatchErr(slot, providerID string) error {
	return wrapComponentErr(ErrProviderSlotMismatch, "", providerID, "slot="+slot)
}

func providerProfileUnsupportedErr(slot, providerID, profile string) error {
	return wrapComponentErr(ErrProviderProfileUnsupported, "", providerID, "slot="+slot+", profile="+profile)
}

func newErrOrchestratorStopped() error {
	return ErrOrchestratorStopped
}

func DescriptorFailure(id ComponentID, detail string) error {
	if id == "" {
		return wrapComponentErr(ErrInvalidDescriptor, "", "", detail)
	}
	return wrapComponentErr(ErrInvalidDescriptor, id, "", detail)
}

func DependencyUnknown(refID ComponentID, depID ComponentID) error {
	if refID == "" {
		return wrapComponentErr(ErrUnknownDependency, depID, "", "")
	}
	return wrapComponentErr(ErrUnknownDependency, refID, "", string(depID))
}

func OrchestratorAlreadyStopped() error {
	return ErrOrchestratorStopped
}

func unknownComponentErr(id ComponentID) error {
	return wrapComponentErr(ErrUnknownComponent, id, "", "")
}

func componentDisabledErr(id ComponentID) error {
	return wrapComponentErr(ErrComponentDisabled, id, "", "")
}

func restartFailedErr(id ComponentID, detail string) error {
	return wrapComponentErr(ErrRestartFailed, id, "", detail)
}

func dependencyNotReadyErr(id ComponentID, detail string) error {
	return wrapComponentErr(ErrDependencyNotReady, id, "", detail)
}
