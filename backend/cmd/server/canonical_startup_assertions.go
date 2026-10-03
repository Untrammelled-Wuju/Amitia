package main

import (
	"errors"
	"fmt"
)

var (
	ErrLegacyMCPManagerPresent  = errors.New("canonical startup violation: Legacy MCP manager must not be present in production")
	ErrToolFacadeMissing        = errors.New("canonical startup violation: ToolFacade must be the unique tool authority")
	ErrPermissionBrokerMissing  = errors.New("canonical startup violation: PermissionBroker must be the unique permission authority")
	ErrTaskRuntimeMissing       = errors.New("canonical startup violation: TaskRuntime must be the unique task authority")
	ErrEventServiceMissing      = errors.New("canonical startup violation: EventService must be the unique event authority")
	ErrScheduleServiceMissing   = errors.New("canonical startup violation: ScheduleService must be the unique schedule authority")
	ErrMemoryRawWriterPresent   = errors.New("canonical startup violation: Memory raw writer must not be present in production")
	ErrChannelResolverMissing   = errors.New("canonical startup violation: CapabilityChannelResolver must be the delivery authority")
	ErrBackgroundRuntimeMissing = errors.New("canonical startup violation: canonical background removal runtime must be configured")
)

type CanonicalRuntimeSnapshot struct {
	LegacyMCPManagerPresent bool
	MemoryRawWriterPresent  bool

	ToolFacadeCount       int
	PermissionBrokerCount int
	TaskRuntimeCount      int
	EventServiceCount     int
	ScheduleServiceCount  int

	LegacyRuntimeActive int
	LegacyWriteEnabled  int
}

type LegacyRuntimeSnapshot struct {
	LegacyMCPManagerActive bool
	MemoryRawWriterActive  bool
}

func runCanonicalStartupAssertions(services *AppServices) error {
	if services == nil {
		return nil
	}
	if !services.hasNoLegacyMCPManager() {
		return ErrLegacyMCPManagerPresent
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
	if !services.hasNoMemoryRawWriter() {
		return ErrMemoryRawWriterPresent
	}
	return nil
}

func runCanonicalRuntimeAssertions(services *AppServices) error {
	if services == nil {
		return fmt.Errorf("services is nil")
	}
	snapshot := services.LegacyRuntimeSnapshot()
	if snapshot.LegacyMCPManagerActive {
		return fmt.Errorf("%w: legacy MCP manager is active", ErrLegacyMCPManagerPresent)
	}
	if snapshot.MemoryRawWriterActive {
		return fmt.Errorf("%w: memory raw writer is active", ErrMemoryRawWriterPresent)
	}

	if services.KernelContainer != nil {
		if services.KernelContainer.MCPDuplicateProvider != nil {
			if unresolved, err := services.KernelContainer.MCPDuplicateProvider.CountUnresolved(nil); err == nil && unresolved > 0 {
				return fmt.Errorf("canonical startup violation: %d unresolved MCP duplicate registrations", unresolved)
			}
		}
	}
	return nil
}

func (s *AppServices) LegacyRuntimeSnapshot() LegacyRuntimeSnapshot {
	snap := LegacyRuntimeSnapshot{}
	if s == nil {
		return snap
	}
	if s.hasLegacyMCPManager() {
		snap.LegacyMCPManagerActive = true
	}
	if s.hasMemoryRawWriter() {
		snap.MemoryRawWriterActive = true
	}
	return snap
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

	legacy := s.LegacyRuntimeSnapshot()
	snap.LegacyMCPManagerPresent = legacy.LegacyMCPManagerActive
	snap.MemoryRawWriterPresent = legacy.MemoryRawWriterActive
	return snap
}

func (s *AppServices) hasNoLegacyMCPManager() bool {
	return !s.hasLegacyMCPManager()
}

func (s *AppServices) hasLegacyMCPManager() bool {
	if s == nil || s.KernelContainer == nil {
		return false
	}
	if s.KernelContainer.MCPDuplicateProvider == nil {
		return false
	}
	count, err := s.KernelContainer.MCPDuplicateProvider.CountUnresolved(nil)
	if err != nil {
		return false
	}
	return count > 0
}

func (s *AppServices) hasNoMemoryRawWriter() bool {
	return !s.hasMemoryRawWriter()
}

func (s *AppServices) hasMemoryRawWriter() bool {
	if s == nil || s.Memory == nil {
		return false
	}
	authority := s.Memory.WriteAuthoritySnapshot()
	return authority.LegacyRawWriterEnabled
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
