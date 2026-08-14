package host_registry

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type RegistryEntryKind string

const (
	RegistryEntryKindRuntime RegistryEntryKind = "runtime"
	RegistryEntryKindUIHost  RegistryEntryKind = "ui_host"
)

func (k RegistryEntryKind) String() string {
	return string(k)
}

func (k RegistryEntryKind) IsValid() bool {
	switch k {
	case RegistryEntryKindRuntime, RegistryEntryKindUIHost:
		return true
	default:
		return false
	}
}

func ParseRegistryEntryKind(raw string) RegistryEntryKind {
	switch RegistryEntryKind(raw) {
	case RegistryEntryKindRuntime, RegistryEntryKindUIHost:
		return RegistryEntryKind(raw)
	default:
		return RegistryEntryKindUIHost
	}
}

type PresenceState string

const (
	PresenceStateReady        PresenceState = "ready"
	PresenceStateDisconnected PresenceState = "disconnected"
	PresenceStateExpired      PresenceState = "expired"
)

type ConnectionState = PresenceState

const (
	HostStateReady        = PresenceStateReady
	HostStateDisconnected = PresenceStateDisconnected
	HostStateExpired      = PresenceStateExpired
)

type EndpointFeature string

const (
	EndpointFeatureUINotify       EndpointFeature = "ui.notify"
	EndpointFeatureUIDialog       EndpointFeature = "ui.dialog"
	EndpointFeatureUINavigate     EndpointFeature = "ui.navigate"
	EndpointFeatureClipboardWrite EndpointFeature = "clipboard.write"
	EndpointFeatureClipboardRead  EndpointFeature = "clipboard.read"
	EndpointFeatureDesktopMenu    EndpointFeature = "desktop.menu"
	EndpointFeatureDesktopTray    EndpointFeature = "desktop.tray"
)

type HostCapability = EndpointFeature

const (
	CapUINotify       = EndpointFeatureUINotify
	CapUIDialog       = EndpointFeatureUIDialog
	CapUINavigate     = EndpointFeatureUINavigate
	CapClipboardWrite = EndpointFeatureClipboardWrite
	CapClipboardRead  = EndpointFeatureClipboardRead
	CapDesktopMenu    = EndpointFeatureDesktopMenu
	CapDesktopTray    = EndpointFeatureDesktopTray
)

type RuntimeEntry struct {
	EntryID         string
	Kind            RegistryEntryKind
	UserID          runtimeidentity.UserID
	DeviceID        runtimeidentity.DeviceID
	RuntimeID       runtimeidentity.RuntimeID
	Platform        runtimeidentity.Platform
	Features        []EndpointFeature
	AuthenticatedAt time.Time
	LastHeartbeat   time.Time
	PresenceState   PresenceState
	SessionToken    string
	CreatedAt       time.Time
	ExpiresAt       time.Time
	HostClientID    string
	HostSessionID   string
	WindowID        string

	RuntimeSessionID      runtimeidentity.RuntimeSessionID
	ConnectionGeneration  int64
}

type HostEntry = RuntimeEntry

func (e *RuntimeEntry) NormalizeCompatibility() {
	if e.EntryID == "" && e.HostClientID != "" {
		e.EntryID = e.HostClientID
	}
	if e.Kind == "" {
		e.Kind = RegistryEntryKindUIHost
	}
}

func (e RuntimeEntry) HasDeviceIdentity() bool {
	return e.UserID != "" && e.DeviceID != ""
}

func (e RuntimeEntry) HasRuntimeIdentity() bool {
	return e.UserID != "" && e.DeviceID != "" && e.RuntimeID != ""
}

func (e RuntimeEntry) IsExpired() bool {
	if e.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().UTC().After(e.ExpiresAt)
}

func (e RuntimeEntry) IsHeartbeatValid() bool {
	return time.Since(e.LastHeartbeat) <= defaultHeartbeatValidity
}

func (e RuntimeEntry) IsReady() bool {
	return e.PresenceState == PresenceStateReady && e.IsHeartbeatValid() && !e.IsExpired()
}

func (e RuntimeEntry) IsExpiredAt(now time.Time) bool {
	if e.ExpiresAt.IsZero() {
		return false
	}
	return now.After(e.ExpiresAt)
}

func (e RuntimeEntry) IsHeartbeatValidAt(now time.Time, validity time.Duration) bool {
	return now.Sub(e.LastHeartbeat) <= validity
}

func (e RuntimeEntry) IsReadyAt(now time.Time, validity time.Duration) bool {
	return e.PresenceState == PresenceStateReady && e.IsHeartbeatValidAt(now, validity) && !e.IsExpiredAt(now)
}

func (e RuntimeEntry) HasFeature(feature EndpointFeature) bool {
	for _, f := range e.Features {
		if f == feature {
			return true
		}
	}
	return false
}

func (e RuntimeEntry) HasCapability(cap HostCapability) bool {
	return e.HasFeature(EndpointFeature(cap))
}

type DevicePresence struct {
	UserID        runtimeidentity.UserID
	DeviceID      runtimeidentity.DeviceID
	Platform      runtimeidentity.Platform
	State         PresenceState
	LastHeartbeat time.Time
	RuntimeIDs    []runtimeidentity.RuntimeID
}

type RuntimePresence struct {
	UserID        runtimeidentity.UserID
	DeviceID      runtimeidentity.DeviceID
	RuntimeID     runtimeidentity.RuntimeID
	Platform      runtimeidentity.Platform
	State         PresenceState
	LastHeartbeat time.Time
	EntryIDs      []string
}

type PresenceSnapshot struct {
	Devices  []DevicePresence
	Runtimes []RuntimePresence
}

func cloneRuntimeEntry(entry *RuntimeEntry) *RuntimeEntry {
	if entry == nil {
		return nil
	}
	clone := *entry
	if entry.Features != nil {
		clone.Features = make([]EndpointFeature, len(entry.Features))
		copy(clone.Features, entry.Features)
	}
	return &clone
}

func RuntimeEntryID(userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID) string {
	normalized := strings.TrimSpace(userID.String()) + "\x00" +
		strings.TrimSpace(deviceID.String()) + "\x00" +
		strings.TrimSpace(runtimeID.String())
	sum := sha256.Sum256([]byte(normalized))
	return "runtime_" + hex.EncodeToString(sum[:])
}

func aggregateRuntimePresence(entries []*RuntimeEntry) RuntimePresence {
	now := time.Now().UTC()
	bestIdx := -1
	for i, e := range entries {
		if e.RuntimeID == "" {
			continue
		}
		if bestIdx == -1 {
			bestIdx = i
			continue
		}
		if entries[bestIdx].LastHeartbeat.Before(e.LastHeartbeat) {
			bestIdx = i
		}
	}
	if bestIdx == -1 {
		bestIdx = 0
	}
	ref := entries[bestIdx]

	state := PresenceStateDisconnected
	allExpired := true
	hasValid := false
	for _, e := range entries {
		if e.RuntimeID == "" {
			continue
		}
		hasValid = true
		if e.IsReadyAt(now, defaultHeartbeatValidity) {
			state = PresenceStateReady
			break
		}
		if !e.IsExpiredAt(now) {
			allExpired = false
		}
	}
	if hasValid && state != PresenceStateReady && allExpired {
		state = PresenceStateExpired
	}

	entryIDs := make([]string, 0, len(entries))
	seen := make(map[string]bool)
	for _, e := range entries {
		if e.EntryID == "" || seen[e.EntryID] {
			continue
		}
		seen[e.EntryID] = true
		entryIDs = append(entryIDs, e.EntryID)
	}
	sort.Strings(entryIDs)

	platform := ref.Platform
	if !platform.IsKnown() {
		for _, e := range entries {
			if e.Platform.IsKnown() {
				platform = e.Platform
				break
			}
		}
	}

	return RuntimePresence{
		UserID:        ref.UserID,
		DeviceID:      ref.DeviceID,
		RuntimeID:     ref.RuntimeID,
		Platform:      platform,
		State:         state,
		LastHeartbeat: ref.LastHeartbeat,
		EntryIDs:      entryIDs,
	}
}

func aggregateDevicePresence(entries []*RuntimeEntry) DevicePresence {
	now := time.Now().UTC()
	bestIdx := -1
	for i, e := range entries {
		if e.DeviceID == "" {
			continue
		}
		if bestIdx == -1 {
			bestIdx = i
			continue
		}
		if entries[bestIdx].LastHeartbeat.Before(e.LastHeartbeat) {
			bestIdx = i
		}
	}
	if bestIdx == -1 && len(entries) > 0 {
		bestIdx = 0
	}
	var ref *RuntimeEntry
	if bestIdx >= 0 {
		ref = entries[bestIdx]
	}

	state := PresenceStateDisconnected
	allExpired := true
	hasValid := false
	for _, e := range entries {
		if e.DeviceID == "" {
			continue
		}
		hasValid = true
		if e.IsReadyAt(now, defaultHeartbeatValidity) {
			state = PresenceStateReady
			break
		}
		if !e.IsExpiredAt(now) {
			allExpired = false
		}
	}
	if hasValid && state != PresenceStateReady && allExpired {
		state = PresenceStateExpired
	}

	runtimeIDSet := make(map[runtimeidentity.RuntimeID]bool)
	for _, e := range entries {
		if e.RuntimeID != "" {
			runtimeIDSet[e.RuntimeID] = true
		}
	}
	runtimeIDs := make([]runtimeidentity.RuntimeID, 0, len(runtimeIDSet))
	for rid := range runtimeIDSet {
		runtimeIDs = append(runtimeIDs, rid)
	}
	sort.Slice(runtimeIDs, func(i, j int) bool {
		return runtimeIDs[i] < runtimeIDs[j]
	})

	var platform runtimeidentity.Platform
	var lastHeartbeat time.Time
	if ref != nil {
		platform = ref.Platform
		lastHeartbeat = ref.LastHeartbeat
	}

	return DevicePresence{
		UserID:        ref.UserID,
		DeviceID:      ref.DeviceID,
		Platform:      platform,
		State:         state,
		LastHeartbeat: lastHeartbeat,
		RuntimeIDs:    runtimeIDs,
	}
}
