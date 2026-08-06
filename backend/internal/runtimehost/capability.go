// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtimehost

import "sync"

type HostCapabilityID string

const (
	CapProcessSpawn         HostCapabilityID = "process.spawn"
	CapProcessTreeControl   HostCapabilityID = "process.tree-control"
	CapProcessGracefulStop  HostCapabilityID = "process.graceful-stop"
	CapProcessForceStop     HostCapabilityID = "process.force-stop"
	CapProcessRestart       HostCapabilityID = "process.restart"
	CapProcessHealthMonitor HostCapabilityID = "process.health-monitor"
	CapFilesystemLocal      HostCapabilityID = "filesystem.local"
	CapFilesystemExecutable HostCapabilityID = "filesystem.executable"
	CapNetworkLoopback      HostCapabilityID = "network.loopback"
	CapRuntimeNativeOffload HostCapabilityID = "runtime.native-offload"
	CapRuntimeBgPersistence HostCapabilityID = "runtime.background-persistence"
	CapRuntimeSandboxedExec HostCapabilityID = "runtime.sandboxed-execution"
)

const (
	SupportUnsupported CapabilitySupport = iota
	SupportLimited
	SupportSupported
)

type CapabilitySupport int

type CapabilityRequirement struct {
	ID      HostCapabilityID
	Minimum CapabilitySupport
}

type HostCapabilities struct {
	mu      sync.RWMutex
	support map[HostCapabilityID]CapabilitySupport
	limits  map[HostCapabilityID]string
}

func newHostCapabilities() *HostCapabilities {
	return &HostCapabilities{
		support: make(map[HostCapabilityID]CapabilitySupport),
		limits:  make(map[HostCapabilityID]string),
	}
}

func (c *HostCapabilities) set(id HostCapabilityID, level CapabilitySupport, limits ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.support[id] = level
	if len(limits) > 0 && limits[0] != "" {
		c.limits[id] = limits[0]
	}
}

func (c *HostCapabilities) Support(id HostCapabilityID) CapabilitySupport {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.support[id]
}

func (c *HostCapabilities) Supports(id HostCapabilityID) bool {
	return c.Support(id) == SupportSupported
}

func (c *HostCapabilities) RequirementSatisfied(req CapabilityRequirement) bool {
	actual := c.Support(req.ID)
	return actual >= req.Minimum
}

func (c *HostCapabilities) Snapshot() HostCapabilitySnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	support := make(map[HostCapabilityID]CapabilitySupport, len(c.support))
	for k, v := range c.support {
		support[k] = v
	}
	limits := make(map[HostCapabilityID]string, len(c.limits))
	for k, v := range c.limits {
		limits[k] = v
	}
	return HostCapabilitySnapshot{Support: support, Limits: limits}
}

type HostCapabilitySnapshot struct {
	Support map[HostCapabilityID]CapabilitySupport `json:"support"`
	Limits  map[HostCapabilityID]string            `json:"limits"`
}
