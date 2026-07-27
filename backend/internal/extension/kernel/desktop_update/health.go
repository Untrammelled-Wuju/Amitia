package desktop_update

import (
	"context"
	"fmt"
	"sync"
)

type HealthCheckResult struct {
	Passed bool
	Checks []HealthCheck
}

type HealthCheck struct {
	Name   string
	Passed bool
	Detail string
}

func (r *HealthCheckResult) AddCheck(name string, passed bool, detail string) {
	r.Checks = append(r.Checks, HealthCheck{
		Name:   name,
		Passed: passed,
		Detail: detail,
	})
	if !passed {
		r.Passed = false
	}
}

type HealthChecker struct {
	mu                  sync.RWMutex
	extensionStates     map[string]string
	runtimeStates       map[string]string
	contributionReg     map[string][]string
	toolRegistry        map[string]bool
	hookEventRegistered map[string]bool
	scheduleRegistered  map[string]bool
	snapshotAppliable   map[string]bool
	shortcutConflicts   map[string]bool
	uiLoadable          map[string]bool
	storageAccessible   map[string]bool
}

func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		extensionStates:     make(map[string]string),
		runtimeStates:       make(map[string]string),
		contributionReg:     make(map[string][]string),
		toolRegistry:        make(map[string]bool),
		hookEventRegistered: make(map[string]bool),
		scheduleRegistered:  make(map[string]bool),
		snapshotAppliable:   make(map[string]bool),
		shortcutConflicts:   make(map[string]bool),
		uiLoadable:          make(map[string]bool),
		storageAccessible:   make(map[string]bool),
	}
}

func (h *HealthChecker) SetExtensionState(extensionID, state string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.extensionStates[extensionID] = state
}

func (h *HealthChecker) SetRuntimeState(extensionID, state string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.runtimeStates[extensionID] = state
}

func (h *HealthChecker) SetContributions(extensionID string, contribs []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.contributionReg[extensionID] = contribs
}

func (h *HealthChecker) SetToolResolvable(toolID string, resolvable bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.toolRegistry[toolID] = resolvable
}

func (h *HealthChecker) SetHookEventRegistered(extensionID string, registered bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.hookEventRegistered[extensionID] = registered
}

func (h *HealthChecker) SetScheduleRegistered(extensionID string, registered bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.scheduleRegistered[extensionID] = registered
}

func (h *HealthChecker) SetSnapshotAppliable(extensionID string, appliable bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.snapshotAppliable[extensionID] = appliable
}

func (h *HealthChecker) SetShortcutConflict(extensionID string, hasConflict bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.shortcutConflicts[extensionID] = hasConflict
}

func (h *HealthChecker) SetUILoadable(extensionID string, loadable bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.uiLoadable[extensionID] = loadable
}

func (h *HealthChecker) SetStorageAccessible(extensionID string, accessible bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.storageAccessible[extensionID] = accessible
}

func (h *HealthChecker) Check(ctx context.Context, extensionID string, generation int64) (*HealthCheckResult, error) {
	result := &HealthCheckResult{Passed: true}

	h.mu.RLock()
	extState := h.extensionStates[extensionID]
	rtState := h.runtimeStates[extensionID]
	contribs := h.contributionReg[extensionID]
	hookReg := h.hookEventRegistered[extensionID]
	schedReg := h.scheduleRegistered[extensionID]
	snapApp := h.snapshotAppliable[extensionID]
	shortcutConflict := h.shortcutConflicts[extensionID]
	uiLoad := h.uiLoadable[extensionID]
	storageAcc := h.storageAccessible[extensionID]
	h.mu.RUnlock()

	h.checkEffectiveState(result, extensionID, extState)
	h.checkRuntimeReady(result, extensionID, rtState)
	h.checkContributionRegistry(result, extensionID, contribs)
	h.checkToolsResolvable(result, extensionID, contribs)
	h.checkHookEventSchedule(result, extensionID, hookReg, schedReg)
	h.checkSnapshotAppliable(result, extensionID, snapApp)
	h.checkShortcutConflict(result, extensionID, shortcutConflict)
	h.checkUILoadable(result, extensionID, uiLoad)
	h.checkStorageAccessible(result, extensionID, storageAcc)

	return result, nil
}

func (h *HealthChecker) checkEffectiveState(result *HealthCheckResult, extensionID, state string) {
	switch state {
	case "enabled", "active":
		result.AddCheck("effective_state", true, fmt.Sprintf("extension state: %s", state))
	case "":
		result.AddCheck("effective_state", true, "extension state not tracked (assumed ok)")
	default:
		result.AddCheck("effective_state", false, fmt.Sprintf("unexpected extension state: %s", state))
	}
}

func (h *HealthChecker) checkRuntimeReady(result *HealthCheckResult, extensionID, state string) {
	switch state {
	case "ready", "active", "":
		result.AddCheck("runtime_ready", true, fmt.Sprintf("runtime state: %s", state))
	case "starting":
		result.AddCheck("runtime_ready", false, "runtime still starting")
	default:
		result.AddCheck("runtime_ready", false, fmt.Sprintf("runtime not ready: %s", state))
	}
}

func (h *HealthChecker) checkContributionRegistry(result *HealthCheckResult, extensionID string, contribs []string) {
	if len(contribs) == 0 {
		result.AddCheck("contribution_registry", true, "no contributions to register")
		return
	}
	result.AddCheck("contribution_registry", true, fmt.Sprintf("%d contributions registered", len(contribs)))
}

func (h *HealthChecker) checkToolsResolvable(result *HealthCheckResult, extensionID string, contribs []string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	unresolved := []string{}
	for _, c := range contribs {
		if resolved, ok := h.toolRegistry[extensionID+"/"+c]; ok {
			if !resolved {
				unresolved = append(unresolved, c)
			}
		}
	}
	if len(unresolved) > 0 {
		result.AddCheck("tool_resolvable", false, fmt.Sprintf("unresolved tools: %v", unresolved))
	} else {
		result.AddCheck("tool_resolvable", true, "all tools resolvable")
	}
}

func (h *HealthChecker) checkHookEventSchedule(result *HealthCheckResult, extensionID string, hookReg, schedReg bool) {
	if !hookReg {
		result.AddCheck("hook_event_registered", false, "hooks/events not registered")
	} else {
		result.AddCheck("hook_event_registered", true, "hooks/events registered")
	}
	if !schedReg {
		result.AddCheck("schedule_registered", false, "schedules not registered")
	} else {
		result.AddCheck("schedule_registered", true, "schedules registered")
	}
}

func (h *HealthChecker) checkSnapshotAppliable(result *HealthCheckResult, extensionID string, appliable bool) {
	if !appliable {
		result.AddCheck("snapshot_appliable", false, "desktop snapshot not applicable")
	} else {
		result.AddCheck("snapshot_appliable", true, "desktop snapshot applicable")
	}
}

func (h *HealthChecker) checkShortcutConflict(result *HealthCheckResult, extensionID string, hasConflict bool) {
	if hasConflict {
		result.AddCheck("shortcut_conflict", false, "shortcut conflicts detected")
	} else {
		result.AddCheck("shortcut_conflict", true, "no shortcut conflicts")
	}
}

func (h *HealthChecker) checkUILoadable(result *HealthCheckResult, extensionID string, loadable bool) {
	if !loadable {
		result.AddCheck("ui_loadable", false, "UI cannot be loaded")
	} else {
		result.AddCheck("ui_loadable", true, "UI loadable")
	}
}

func (h *HealthChecker) checkStorageAccessible(result *HealthCheckResult, extensionID string, accessible bool) {
	if !accessible {
		result.AddCheck("storage_accessible", false, "storage not accessible")
	} else {
		result.AddCheck("storage_accessible", true, "storage accessible")
	}
}
