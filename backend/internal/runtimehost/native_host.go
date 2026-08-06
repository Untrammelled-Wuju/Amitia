// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtimehost

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/platform/process"
	"github.com/u-ai/backend/pkg/platform"
	"github.com/u-ai/backend/pkg/util"
)

type nativeProcessHost struct {
	descriptor       platform.RuntimeDescriptor
	capabilities     *HostCapabilities
	paths            util.RuntimePaths
	instanceID       string
	processes        *defaultProcessSupervisor
	processManager   *process.DefaultProcessManager
	mu               sync.RWMutex
	stopped          bool
}

func newNativeProcessHost(descriptor platform.RuntimeDescriptor, paths util.RuntimePaths) *nativeProcessHost {
	host := &nativeProcessHost{
		descriptor:     descriptor,
		capabilities:   newHostCapabilities(),
		paths:          paths,
		instanceID:     generateInstanceID(),
		processManager: process.NewDefaultProcessManager(),
	}
	host.processes = newProcessSupervisor(host)
	host.configureCapabilities()
	return host
}

func (h *nativeProcessHost) Descriptor() platform.RuntimeDescriptor { return h.descriptor }
func (h *nativeProcessHost) Capabilities() *HostCapabilities        { return h.capabilities }
func (h *nativeProcessHost) Paths() util.RuntimePaths               { return h.paths }
func (h *nativeProcessHost) Processes() ProcessSupervisor             { return h.processes }

func (h *nativeProcessHost) RuntimeInstanceID() string { return h.instanceID }

func (h *nativeProcessHost) configureCapabilities() {
	h.capabilities.set(CapProcessSpawn, SupportSupported)
	h.capabilities.set(CapProcessTreeControl, SupportSupported)
	h.capabilities.set(CapProcessRestart, SupportSupported)
	h.capabilities.set(CapProcessHealthMonitor, SupportSupported)
	h.capabilities.set(CapFilesystemLocal, SupportSupported)
	h.capabilities.set(CapFilesystemExecutable, SupportSupported)
	h.capabilities.set(CapNetworkLoopback, SupportSupported)
	h.capabilities.set(CapRuntimeNativeOffload, SupportUnsupported)
	h.capabilities.set(CapRuntimeBgPersistence, SupportLimited, "background execution depends on OS lifecycle")

	switch h.descriptor.Guest {
	case platform.GuestPlatformLinux:
		if h.descriptor.Kind == platform.RuntimeKindProot || h.descriptor.Kind == platform.RuntimeKindNativeProcess {
			h.capabilities.set(CapProcessGracefulStop, SupportSupported)
			h.capabilities.set(CapProcessForceStop, SupportSupported)
			h.capabilities.set(CapRuntimeSandboxedExec, SupportLimited, "limited by PRoot isolation")
		}
	case platform.GuestPlatformWindows:
		h.capabilities.set(CapProcessGracefulStop, SupportLimited, "Windows relies on Job Object + Ctrl+Break")
		h.capabilities.set(CapProcessForceStop, SupportSupported)
		h.capabilities.set(CapRuntimeSandboxedExec, SupportLimited, "Windows AppContainer not fully implemented")
	case platform.GuestPlatformMacOS:
		h.capabilities.set(CapProcessGracefulStop, SupportSupported)
		h.capabilities.set(CapProcessForceStop, SupportSupported)
		h.capabilities.set(CapRuntimeSandboxedExec, SupportLimited, "macOS Sandbox Profile not fully implemented")
	}
}

func (h *nativeProcessHost) checkPorts(ports []LoopbackPortClaim) error {
	for _, p := range ports {
		if err := h.checkPort(p); err != nil {
			return err
		}
	}
	return nil
}

func (h *nativeProcessHost) checkPort(claim LoopbackPortClaim) error {
	host := claim.Host
	if host == "" {
		host = "127.0.0.1"
	}
	addr := fmt.Sprintf("%s:%d", host, claim.Port)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("%w: %s:%d", ErrPortInUse, host, claim.Port)
	}
	ln.Close()
	return nil
}

func generateInstanceID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("inst-%d", time.Now().UnixNano())
	}
	return "inst-" + hex.EncodeToString(buf)
}
