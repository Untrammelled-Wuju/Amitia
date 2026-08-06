// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtimehost

import (
	"context"

	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

type restrictedHost struct {
	descriptor   platform.RuntimeDescriptor
	capabilities *HostCapabilities
	paths        util.RuntimePaths
	instanceID   string
	processes    *noopProcessSupervisor
}

func newRestrictedHost(descriptor platform.RuntimeDescriptor, paths util.RuntimePaths) *restrictedHost {
	host := &restrictedHost{
		descriptor:   descriptor,
		capabilities: newHostCapabilities(),
		paths:        paths,
		instanceID:   generateInstanceID(),
	}
	host.processes = newNoopSupervisor(host)
	host.configureCapabilities()
	return host
}

func (h *restrictedHost) Descriptor() platform.RuntimeDescriptor { return h.descriptor }
func (h *restrictedHost) Capabilities() *HostCapabilities        { return h.capabilities }
func (h *restrictedHost) Paths() util.RuntimePaths               { return h.paths }
func (h *restrictedHost) Processes() ProcessSupervisor             { return h.processes }
func (h *restrictedHost) RuntimeInstanceID() string                { return h.instanceID }

func (h *restrictedHost) configureCapabilities() {
	h.capabilities.set(CapProcessSpawn, SupportUnsupported)
	h.capabilities.set(CapProcessTreeControl, SupportUnsupported)
	h.capabilities.set(CapProcessGracefulStop, SupportUnsupported)
	h.capabilities.set(CapProcessForceStop, SupportUnsupported)
	h.capabilities.set(CapProcessRestart, SupportUnsupported)
	h.capabilities.set(CapProcessHealthMonitor, SupportUnsupported)
	h.capabilities.set(CapFilesystemLocal, SupportLimited, "local filesystem access is limited")
	h.capabilities.set(CapFilesystemExecutable, SupportUnsupported)
	h.capabilities.set(CapNetworkLoopback, SupportLimited, "loopback network is restricted")
	h.capabilities.set(CapRuntimeNativeOffload, SupportLimited, "native offload not implemented")
	h.capabilities.set(CapRuntimeBgPersistence, SupportLimited, "background persistence is restricted")
	h.capabilities.set(CapRuntimeSandboxedExec, SupportSupported)
}

type noopProcessSupervisor struct {
	host *restrictedHost
}

func newNoopSupervisor(host *restrictedHost) *noopProcessSupervisor {
	return &noopProcessSupervisor{host: host}
}

func (s *noopProcessSupervisor) Register(spec ProcessSpec) error {
	return ErrHostProcessUnsupported
}

func (s *noopProcessSupervisor) Unregister(id ProcessID) error {
	return nil
}

func (s *noopProcessSupervisor) Start(ctx context.Context, id ProcessID) error {
	return ErrHostProcessUnsupported
}

func (s *noopProcessSupervisor) WaitReady(ctx context.Context, id ProcessID) error {
	return ErrHostProcessUnsupported
}

func (s *noopProcessSupervisor) Restart(ctx context.Context, id ProcessID) error {
	return ErrHostProcessUnsupported
}

func (s *noopProcessSupervisor) Stop(ctx context.Context, id ProcessID) error {
	return nil
}

func (s *noopProcessSupervisor) StopAll(ctx context.Context) error {
	return nil
}

func (s *noopProcessSupervisor) Snapshot(id ProcessID) (ProcessSnapshot, bool) {
	return ProcessSnapshot{}, false
}

func (s *noopProcessSupervisor) List() []ProcessSnapshot {
	return nil
}

func (s *noopProcessSupervisor) Subscribe(fn func(ProcessEvent)) func() {
	return func() {}
}
