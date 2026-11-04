package main

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrMCPDuplicateProviderPresent = errors.New("canonical startup violation: unresolved MCP duplicate registrations must not be present in production")
	ErrToolFacadeMissing           = errors.New("canonical startup violation: ToolFacade must be the unique tool authority")
	ErrPermissionBrokerMissing     = errors.New("canonical startup violation: PermissionBroker must be the unique permission authority")
	ErrTaskRuntimeMissing          = errors.New("canonical startup violation: TaskRuntime must be the unique task authority")
	ErrEventServiceMissing         = errors.New("canonical startup violation: EventService must be the unique event authority")
	ErrScheduleServiceMissing      = errors.New("canonical startup violation: ScheduleService must be the unique schedule authority")
	ErrChannelResolverMissing      = errors.New("canonical startup violation: CapabilityChannelResolver must be the delivery authority")
	ErrBackgroundRuntimeMissing    = errors.New("canonical startup violation: canonical background removal runtime must be configured")
)

type CanonicalRuntimeSnapshot struct {
	ToolFacadeCount       int
	PermissionBrokerCount int
	TaskRuntimeCount      int
	EventServiceCount     int
	ScheduleServiceCount  int
}

func runCanonicalStartupAssertions(services *AppServices) error {
	if services == nil {
		return nil
	}
	duplicateMCP, err := services.hasDuplicateMCPRegistrations()
	if err != nil {
		return err
	}
	if duplicateMCP {
		return ErrMCPDuplicateProviderPresent
	}
	if services.KernelContainer == nil {
		return fmt.Errorf("%w: KernelContainer is nil", ErrToolFacadeMissing)
	}
	policy := services.RuntimePolicy
	if policy.ExtensionKernel {
		if services.KernelContainer.ToolFacade == nil {
			return ErrToolFacadeMissing
		}
		if services.KernelContainer.PermissionBroker == nil {
			return ErrPermissionBrokerMissing
		}
		if services.KernelContainer.ChannelResolver == nil {
			return ErrChannelResolverMissing
		}
		if services.KernelContainer.BackgroundRemovalRegistry == nil {
			return ErrBackgroundRuntimeMissing
		}
	}
	if policy.TaskRuntime {
		if services.KernelContainer.TaskRuntimeService == nil {
			return ErrTaskRuntimeMissing
		}
	}
	if policy.DurableEvents {
		if services.KernelContainer.EventService == nil {
			return ErrEventServiceMissing
		}
	}
	if policy.TaskRuntime {
		if services.KernelContainer.ScheduleService == nil {
			return ErrScheduleServiceMissing
		}
	}
	return nil
}

func runCanonicalRuntimeAssertions(services *AppServices) error {
	if services == nil {
		return fmt.Errorf("services is nil")
	}
	duplicateMCP, err := services.hasDuplicateMCPRegistrations()
	if err != nil {
		return err
	}
	if duplicateMCP {
		return ErrMCPDuplicateProviderPresent
	}
	return nil
}

func (s *AppServices) CanonicalRuntimeSnapshot() CanonicalRuntimeSnapshot {
	snap := CanonicalRuntimeSnapshot{}
	if s == nil {
		return snap
	}
	if s.KernelContainer == nil {
		return snap
	}
	if s.KernelContainer.ToolFacade != nil {
		snap.ToolFacadeCount = 1
	}
	if s.KernelContainer.PermissionBroker != nil {
		snap.PermissionBrokerCount = 1
	}
	if s.KernelContainer.TaskRuntimeService != nil {
		snap.TaskRuntimeCount = 1
	}
	if s.KernelContainer.EventService != nil {
		snap.EventServiceCount = 1
	}
	if s.KernelContainer.ScheduleService != nil {
		snap.ScheduleServiceCount = 1
	}

	return snap
}

func (s *AppServices) hasDuplicateMCPRegistrations() (bool, error) {
	if s == nil || s.KernelContainer == nil {
		return false, nil
	}
	if s.KernelContainer.MCPDuplicateProvider == nil {
		return false, nil
	}
	count, err := s.KernelContainer.MCPDuplicateProvider.CountUnresolved(context.Background())
	if err != nil {
		return false, fmt.Errorf("canonical startup: count unresolved MCP duplicate registrations: %w", err)
	}
	return count > 0, nil
}

func verifyCanonicalStartupOrPanic(services *AppServices) {
	if err := runCanonicalBuildAssertions(services); err != nil {
		panic(fmt.Sprintf("Canonical Startup Assertion failed: %v", err))
	}
}

func runCanonicalBuildAssertions(services *AppServices) error {
	if services == nil {
		return fmt.Errorf("services is nil")
	}
	if services.KernelContainer == nil {
		return fmt.Errorf("%w: KernelContainer is nil", ErrToolFacadeMissing)
	}
	policy := services.RuntimePolicy
	if policy.ExtensionKernel {
		if services.KernelContainer.ToolFacade == nil {
			return ErrToolFacadeMissing
		}
		if services.KernelContainer.PermissionBroker == nil {
			return ErrPermissionBrokerMissing
		}
		if services.KernelContainer.ChannelResolver == nil {
			return ErrChannelResolverMissing
		}
		if services.KernelContainer.BackgroundRemovalRegistry == nil {
			return ErrBackgroundRuntimeMissing
		}
	}
	if policy.TaskRuntime {
		if services.KernelContainer.TaskRuntimeService == nil {
			return ErrTaskRuntimeMissing
		}
	}
	if policy.DurableEvents {
		if services.KernelContainer.EventService == nil {
			return ErrEventServiceMissing
		}
	}
	if policy.TaskRuntime {
		if services.KernelContainer.ScheduleService == nil {
			return ErrScheduleServiceMissing
		}
	}
	return nil
}
