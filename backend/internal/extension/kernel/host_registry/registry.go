package host_registry

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

const defaultHeartbeatValidity = 5 * time.Minute

type Registry struct {
	mu                sync.RWMutex
	db                *sql.DB
	repo              *registryRepository
	entries           map[string]*RuntimeEntry
	loaded            bool
	heartbeatValidity time.Duration
}

type HostRegistry = Registry

func NewRegistry(db *sql.DB) *Registry {
	return &Registry{
		db:                db,
		repo:              &registryRepository{db: db},
		entries:           make(map[string]*RuntimeEntry),
		heartbeatValidity: defaultHeartbeatValidity,
	}
}

func NewHostRegistry(db *sql.DB) *HostRegistry {
	return NewRegistry(db)
}

func MigrateSessionTokens(ctx context.Context, db *sql.DB) error {
	return (&registryRepository{db: db}).MigrateSessionTokens(ctx)
}

func MigrateRuntimeSessionColumns(ctx context.Context, db *sql.DB) error {
	return (&registryRepository{db: db}).MigrateRuntimeSessionColumns(ctx)
}

func (r *Registry) RegisterEntry(ctx context.Context, entry *RuntimeEntry) error {
	if entry == nil {
		return ErrInvalidRegistryEntry
	}
	entry.NormalizeCompatibility()
	if entry.EntryID == "" {
		return ErrInvalidRegistryEntry
	}
	if !entry.Kind.IsValid() {
		return ErrInvalidRegistryEntry
	}
	if entry.Kind == RegistryEntryKindRuntime && !entry.HasRuntimeIdentity() {
		return ErrInvalidRegistryEntry
	}
	if entry.HostClientID != "" && entry.EntryID != entry.HostClientID && entry.Kind == RegistryEntryKindUIHost {
		return ErrInvalidRegistryEntry
	}
	now := time.Now().UTC()
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = now
	}
	if entry.AuthenticatedAt.IsZero() {
		entry.AuthenticatedAt = now
	}
	if entry.LastHeartbeat.IsZero() {
		entry.LastHeartbeat = now
	}
	if entry.PresenceState == "" {
		entry.PresenceState = PresenceStateReady
	}
	if err := r.repo.SaveEntry(ctx, entry); err != nil {
		return err
	}
	r.mu.Lock()
	r.entries[entry.EntryID] = cloneRuntimeEntry(entry)
	r.mu.Unlock()
	return nil
}

func (r *Registry) RegisterHost(ctx context.Context, entry *HostEntry) error {
	if entry != nil && entry.Kind == "" {
		entry.Kind = RegistryEntryKindUIHost
	}
	return r.RegisterEntry(ctx, entry)
}

func (r *Registry) UnregisterEntry(ctx context.Context, entryID string) error {
	if err := r.repo.DeleteEntry(ctx, entryID); err != nil {
		return err
	}
	r.mu.Lock()
	delete(r.entries, entryID)
	r.mu.Unlock()
	return nil
}

func (r *Registry) UnregisterHost(ctx context.Context, hostClientID string) error {
	return r.UnregisterEntry(ctx, hostClientID)
}

func (r *Registry) GetEntry(ctx context.Context, entryID string) (*RuntimeEntry, error) {
	r.mu.RLock()
	entry, ok := r.entries[entryID]
	r.mu.RUnlock()
	if ok {
		return cloneRuntimeEntry(entry), nil
	}
	entry, err := r.repo.GetEntry(ctx, entryID)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.entries[entryID] = cloneRuntimeEntry(entry)
	r.mu.Unlock()
	return cloneRuntimeEntry(entry), nil
}

func (r *Registry) GetHost(ctx context.Context, hostClientID string) (*HostEntry, error) {
	entry, err := r.GetEntry(ctx, hostClientID)
	if err != nil {
		return nil, err
	}
	if entry.Kind != RegistryEntryKindUIHost {
		return nil, ErrHostNotFound
	}
	return entry, nil
}

func (r *Registry) ListEntriesByUser(ctx context.Context, userID runtimeidentity.UserID) ([]*RuntimeEntry, error) {
	entries, err := r.repo.ListEntriesByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	for _, e := range entries {
		r.entries[e.EntryID] = cloneRuntimeEntry(e)
	}
	r.mu.Unlock()
	return r.cloneEntries(entries), nil
}

func (r *Registry) ListEntriesByDevice(ctx context.Context, userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID) ([]*RuntimeEntry, error) {
	entries, err := r.repo.ListEntriesByDevice(ctx, userID, deviceID)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	for _, e := range entries {
		r.entries[e.EntryID] = cloneRuntimeEntry(e)
	}
	r.mu.Unlock()
	return r.cloneEntries(entries), nil
}

func (r *Registry) ListEntriesByRuntime(ctx context.Context, userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID) ([]*RuntimeEntry, error) {
	entries, err := r.repo.ListEntriesByRuntime(ctx, userID, deviceID, runtimeID)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	for _, e := range entries {
		r.entries[e.EntryID] = cloneRuntimeEntry(e)
	}
	r.mu.Unlock()
	return r.cloneEntries(entries), nil
}

func (r *Registry) ListReadyEntries(ctx context.Context, userID runtimeidentity.UserID) ([]*RuntimeEntry, error) {
	entries, err := r.repo.ListEntriesByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	for _, e := range entries {
		r.entries[e.EntryID] = cloneRuntimeEntry(e)
	}
	r.mu.Unlock()
	now := time.Now().UTC()
	var result []*RuntimeEntry
	for _, e := range entries {
		if e.IsReadyAt(now, r.heartbeatValidity) {
			result = append(result, cloneRuntimeEntry(e))
		}
	}
	return sortRuntimeEntries(result), nil
}

func (r *Registry) ListHostsByUser(ctx context.Context, userID runtimeidentity.UserID) ([]*HostEntry, error) {
	entries, err := r.ListEntriesByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	var result []*HostEntry
	for _, e := range entries {
		if e.Kind == RegistryEntryKindUIHost {
			result = append(result, e)
		}
	}
	return result, nil
}

func (r *Registry) ListHostsByUserString(ctx context.Context, userID string) ([]*HostEntry, error) {
	return r.ListHostsByUser(ctx, runtimeidentity.ParseUserID(userID))
}

func (r *Registry) ListReadyHosts(ctx context.Context, userID runtimeidentity.UserID, capability HostCapability) ([]*HostEntry, error) {
	hosts, err := r.ListHostsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	var result []*HostEntry
	for _, h := range hosts {
		if h.PresenceState == PresenceStateReady &&
			h.IsHeartbeatValidAt(now, r.heartbeatValidity) &&
			!h.IsExpiredAt(now) &&
			h.HasCapability(capability) {
			result = append(result, h)
		}
	}
	return result, nil
}

func (r *Registry) ListReadyHostsString(ctx context.Context, userID string, capability HostCapability) ([]*HostEntry, error) {
	return r.ListReadyHosts(ctx, runtimeidentity.ParseUserID(userID), capability)
}

func (r *Registry) UpdateEntryHeartbeat(ctx context.Context, entryID string) error {
	now := time.Now().UTC()
	if err := r.repo.UpdateEntryHeartbeat(ctx, entryID, now); err != nil {
		return err
	}
	r.mu.Lock()
	if h, ok := r.entries[entryID]; ok {
		h.LastHeartbeat = now
	}
	r.mu.Unlock()
	return nil
}

func (r *Registry) UpdateHeartbeat(ctx context.Context, hostClientID string) error {
	return r.UpdateEntryHeartbeat(ctx, hostClientID)
}

func (r *Registry) SetEntryDisconnected(ctx context.Context, entryID string) error {
	if err := r.repo.UpdateEntryState(ctx, entryID, PresenceStateDisconnected); err != nil {
		return err
	}
	r.mu.Lock()
	if h, ok := r.entries[entryID]; ok {
		h.PresenceState = PresenceStateDisconnected
	}
	r.mu.Unlock()
	return nil
}

func (r *Registry) SetDisconnected(ctx context.Context, hostClientID string) error {
	return r.SetEntryDisconnected(ctx, hostClientID)
}

func (r *Registry) FindTargetHost(ctx context.Context, userID runtimeidentity.UserID, capability HostCapability, platform runtimeidentity.Platform, windowID string) (*HostEntry, error) {
	hosts, err := r.ListHostsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	var bestMatch *HostEntry
	for _, h := range hosts {
		if h.PresenceState != PresenceStateReady {
			continue
		}
		if !h.IsHeartbeatValidAt(now, r.heartbeatValidity) {
			continue
		}
		if h.IsExpiredAt(now) {
			continue
		}
		if !h.HasCapability(capability) {
			continue
		}
		if platform != "" && h.Platform != platform {
			continue
		}
		if windowID != "" && h.WindowID != windowID {
			continue
		}
		if bestMatch == nil || bestMatch.LastHeartbeat.Before(h.LastHeartbeat) {
			bestMatch = h
		}
	}
	return bestMatch, nil
}

func (r *Registry) FindTargetHostString(ctx context.Context, userID string, capability HostCapability, platform string, windowID string) (*HostEntry, error) {
	return r.FindTargetHost(ctx, runtimeidentity.ParseUserID(userID), capability, runtimeidentity.ParsePlatform(platform), windowID)
}

func (r *Registry) FindRuntimeEntry(ctx context.Context, userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID) (*RuntimeEntry, error) {
	entries, err := r.repo.ListEntriesByRuntime(ctx, userID, deviceID, runtimeID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	var bestReady *RuntimeEntry
	var bestNotReady *RuntimeEntry
	for _, e := range entries {
		if e.IsReadyAt(now, r.heartbeatValidity) {
			if bestReady == nil || bestReady.LastHeartbeat.Before(e.LastHeartbeat) {
				bestReady = e
			}
		} else {
			if bestNotReady == nil || bestNotReady.LastHeartbeat.Before(e.LastHeartbeat) {
				bestNotReady = e
			}
		}
	}
	if bestReady != nil {
		return cloneRuntimeEntry(bestReady), nil
	}
	if bestNotReady != nil {
		return cloneRuntimeEntry(bestNotReady), nil
	}
	return nil, ErrRuntimePresenceNotFound
}

func (r *Registry) GetRuntimePresence(ctx context.Context, userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID) (RuntimePresence, error) {
	entries, err := r.repo.ListEntriesByRuntime(ctx, userID, deviceID, runtimeID)
	if err != nil {
		return RuntimePresence{}, err
	}
	if len(entries) == 0 {
		return RuntimePresence{}, ErrRuntimePresenceNotFound
	}
	return aggregateRuntimePresence(entries), nil
}

func (r *Registry) ListRuntimePresenceByDevice(ctx context.Context, userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID) ([]RuntimePresence, error) {
	entries, err := r.repo.ListEntriesByDevice(ctx, userID, deviceID)
	if err != nil {
		return nil, err
	}
	runtimeGroups := make(map[runtimeidentity.RuntimeID][]*RuntimeEntry)
	for _, e := range entries {
		if e.RuntimeID != "" {
			runtimeGroups[e.RuntimeID] = append(runtimeGroups[e.RuntimeID], e)
		}
	}
	result := make([]RuntimePresence, 0, len(runtimeGroups))
	for _, group := range runtimeGroups {
		result = append(result, aggregateRuntimePresence(group))
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].RuntimeID < result[j].RuntimeID
	})
	return result, nil
}

func (r *Registry) GetDevicePresence(ctx context.Context, userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID) (DevicePresence, error) {
	entries, err := r.repo.ListEntriesByDevice(ctx, userID, deviceID)
	if err != nil {
		return DevicePresence{}, err
	}
	if len(entries) == 0 {
		return DevicePresence{}, ErrDevicePresenceNotFound
	}
	return aggregateDevicePresence(entries), nil
}

func (r *Registry) ListDevicePresenceByUser(ctx context.Context, userID runtimeidentity.UserID) ([]DevicePresence, error) {
	entries, err := r.repo.ListEntriesByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	deviceGroups := make(map[runtimeidentity.DeviceID][]*RuntimeEntry)
	for _, e := range entries {
		if e.DeviceID != "" {
			deviceGroups[e.DeviceID] = append(deviceGroups[e.DeviceID], e)
		}
	}
	result := make([]DevicePresence, 0, len(deviceGroups))
	for _, group := range deviceGroups {
		result = append(result, aggregateDevicePresence(group))
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].DeviceID < result[j].DeviceID
	})
	return result, nil
}

func (r *Registry) HasReadyRuntime(ctx context.Context, userID runtimeidentity.UserID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID) bool {
	entries, err := r.repo.ListEntriesByRuntime(ctx, userID, deviceID, runtimeID)
	if err != nil {
		return false
	}
	now := time.Now().UTC()
	for _, e := range entries {
		if e.IsReadyAt(now, r.heartbeatValidity) {
			return true
		}
	}
	return false
}

func (r *Registry) HasReadyHost(ctx context.Context, userID runtimeidentity.UserID, capability HostCapability) bool {
	hosts, err := r.ListReadyHosts(ctx, userID, capability)
	if err != nil {
		return false
	}
	return len(hosts) > 0
}

func (r *Registry) HasReadyHostString(ctx context.Context, userID string, capability HostCapability) bool {
	return r.HasReadyHost(ctx, runtimeidentity.ParseUserID(userID), capability)
}

func (r *Registry) EntryCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entries)
}

func (r *Registry) Count() int {
	return r.EntryCount()
}

func (r *Registry) LoadFromStore(ctx context.Context) error {
	entries, err := r.repo.ListAllEntries(ctx)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.entries = make(map[string]*RuntimeEntry, len(entries))
	for _, e := range entries {
		e.NormalizeCompatibility()
		r.entries[e.EntryID] = cloneRuntimeEntry(e)
	}
	r.loaded = true
	r.mu.Unlock()
	return nil
}

func (r *Registry) CleanupExpired(ctx context.Context) error {
	expired, err := r.repo.ListExpiredEntries(ctx)
	if err != nil {
		return err
	}
	for _, h := range expired {
		if err := r.repo.UpdateEntryState(ctx, h.EntryID, PresenceStateExpired); err != nil {
			return err
		}
		r.mu.Lock()
		if entry, ok := r.entries[h.EntryID]; ok {
			entry.PresenceState = PresenceStateExpired
		}
		r.mu.Unlock()
	}
	return nil
}

func (r *Registry) SnapshotByUser(ctx context.Context, userID runtimeidentity.UserID) (PresenceSnapshot, error) {
	entries, err := r.repo.ListEntriesByUser(ctx, userID)
	if err != nil {
		return PresenceSnapshot{}, err
	}
	deviceGroups := make(map[runtimeidentity.DeviceID][]*RuntimeEntry)
	runtimeGroups := make(map[runtimeidentity.RuntimeID][]*RuntimeEntry)
	for _, e := range entries {
		if e.DeviceID != "" {
			deviceGroups[e.DeviceID] = append(deviceGroups[e.DeviceID], e)
		}
		if e.RuntimeID != "" {
			runtimeGroups[e.RuntimeID] = append(runtimeGroups[e.RuntimeID], e)
		}
	}
	devices := make([]DevicePresence, 0, len(deviceGroups))
	for _, group := range deviceGroups {
		devices = append(devices, aggregateDevicePresence(group))
	}
	sort.Slice(devices, func(i, j int) bool {
		return devices[i].DeviceID < devices[j].DeviceID
	})
	runtimes := make([]RuntimePresence, 0, len(runtimeGroups))
	for _, group := range runtimeGroups {
		runtimes = append(runtimes, aggregateRuntimePresence(group))
	}
	sort.Slice(runtimes, func(i, j int) bool {
		return runtimes[i].RuntimeID < runtimes[j].RuntimeID
	})
	return PresenceSnapshot{
		Devices:  devices,
		Runtimes: runtimes,
	}, nil
}

type RuntimeSessionBinding struct {
	UserID           runtimeidentity.UserID
	DeviceID         runtimeidentity.DeviceID
	RuntimeID        runtimeidentity.RuntimeID
	RuntimeSessionID runtimeidentity.RuntimeSessionID
	Platform         runtimeidentity.Platform

	ConnectionGeneration int64
	At                   time.Time
}

func (r *Registry) BindRuntimeSession(ctx context.Context, binding RuntimeSessionBinding) (*RuntimeEntry, error) {
	if binding.UserID == "" || binding.DeviceID == "" || binding.RuntimeID == "" {
		return nil, ErrInvalidRegistryEntry
	}
	if binding.RuntimeSessionID == "" {
		return nil, ErrInvalidRegistryEntry
	}
	if binding.ConnectionGeneration < 1 {
		return nil, ErrInvalidRegistryEntry
	}

	at := binding.At
	if at.IsZero() {
		at = time.Now().UTC()
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	entryID := RuntimeEntryID(binding.UserID, binding.DeviceID, binding.RuntimeID)

	entry, err := r.repo.GetEntry(ctx, entryID)
	if err != nil && !errors.Is(err, ErrRegistryEntryNotFound) {
		return nil, err
	}

	if errors.Is(err, ErrRegistryEntryNotFound) {
		now := at
		newEntry := &RuntimeEntry{
			EntryID:              entryID,
			Kind:                 RegistryEntryKindRuntime,
			UserID:               binding.UserID,
			DeviceID:             binding.DeviceID,
			RuntimeID:            binding.RuntimeID,
			Platform:             binding.Platform,
			PresenceState:        PresenceStateReady,
			RuntimeSessionID:     binding.RuntimeSessionID,
			ConnectionGeneration: binding.ConnectionGeneration,
			LastHeartbeat:        at,
			CreatedAt:            now,
			AuthenticatedAt:      now,
		}
		if err := r.repo.SaveEntry(ctx, newEntry); err != nil {
			return nil, err
		}
		r.entries[entryID] = cloneRuntimeEntry(newEntry)
		return cloneRuntimeEntry(newEntry), nil
	}

	if entry.Kind != RegistryEntryKindRuntime {
		return nil, nil
	}

	if entry.ConnectionGeneration > binding.ConnectionGeneration {
		return nil, ErrStaleRuntimeSessionBinding
	}
	if entry.ConnectionGeneration == binding.ConnectionGeneration && entry.RuntimeSessionID != binding.RuntimeSessionID {
		return nil, ErrRuntimeSessionBindingConflict
	}

	entry.RuntimeSessionID = binding.RuntimeSessionID
	entry.ConnectionGeneration = binding.ConnectionGeneration
	entry.Platform = binding.Platform
	entry.LastHeartbeat = at
	entry.PresenceState = PresenceStateReady

	if err := r.repo.SaveEntry(ctx, entry); err != nil {
		return nil, err
	}

	r.entries[entry.EntryID] = cloneRuntimeEntry(entry)
	return cloneRuntimeEntry(entry), nil
}

func (r *Registry) HeartbeatRuntimeSession(ctx context.Context, binding RuntimeSessionBinding) error {
	if binding.RuntimeSessionID == "" || binding.ConnectionGeneration < 1 {
		return ErrStaleRuntimeSessionBinding
	}

	at := binding.At
	if at.IsZero() {
		at = time.Now().UTC()
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	entryID := RuntimeEntryID(binding.UserID, binding.DeviceID, binding.RuntimeID)

	entry, ok := r.entries[entryID]
	if !ok {
		return ErrRuntimePresenceNotFound
	}

	if entry.Kind != RegistryEntryKindRuntime {
		return ErrRuntimePresenceNotFound
	}

	if entry.RuntimeSessionID != binding.RuntimeSessionID || entry.ConnectionGeneration != binding.ConnectionGeneration {
		return ErrStaleRuntimeSessionBinding
	}

	if err := r.repo.UpdateRuntimeSessionHeartbeat(ctx, entryID, binding.RuntimeSessionID, binding.ConnectionGeneration, at); err != nil {
		return err
	}

	entry.LastHeartbeat = at
	entry.PresenceState = PresenceStateReady
	return nil
}

func (r *Registry) DisconnectRuntimeSession(ctx context.Context, binding RuntimeSessionBinding) error {
	if binding.RuntimeSessionID == "" || binding.ConnectionGeneration < 1 {
		return ErrStaleRuntimeSessionBinding
	}

	at := binding.At
	if at.IsZero() {
		at = time.Now().UTC()
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	entryID := RuntimeEntryID(binding.UserID, binding.DeviceID, binding.RuntimeID)

	entry, ok := r.entries[entryID]
	if !ok {
		return ErrRuntimePresenceNotFound
	}

	if entry.Kind != RegistryEntryKindRuntime {
		return ErrRuntimePresenceNotFound
	}

	if entry.RuntimeSessionID != binding.RuntimeSessionID || entry.ConnectionGeneration != binding.ConnectionGeneration {
		return ErrStaleRuntimeSessionBinding
	}

	if entry.PresenceState == PresenceStateDisconnected {
		return nil
	}

	if err := r.repo.SetRuntimeSessionDisconnected(ctx, entryID, binding.RuntimeSessionID, binding.ConnectionGeneration, at); err != nil {
		return err
	}

	entry.PresenceState = PresenceStateDisconnected
	entry.LastHeartbeat = at
	return nil
}

func (r *Registry) MarkRuntimeEntriesDisconnectedOnStartup(ctx context.Context, at time.Time) error {
	if at.IsZero() {
		at = time.Now().UTC()
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.repo.MarkRuntimeEntriesDisconnected(ctx, at); err != nil {
		return err
	}

	for _, entry := range r.entries {
		if entry.Kind == RegistryEntryKindRuntime && entry.PresenceState == PresenceStateReady {
			entry.PresenceState = PresenceStateDisconnected
			entry.LastHeartbeat = at
		}
	}

	return nil
}

func (r *Registry) isHeartbeatValid(entry *RuntimeEntry, now time.Time) bool {
	return entry.IsHeartbeatValidAt(now, r.heartbeatValidity)
}

func (r *Registry) cloneEntries(entries []*RuntimeEntry) []*RuntimeEntry {
	result := make([]*RuntimeEntry, len(entries))
	for i, e := range entries {
		result[i] = cloneRuntimeEntry(e)
	}
	return result
}

func sortRuntimeEntries(entries []*RuntimeEntry) []*RuntimeEntry {
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.DeviceID != b.DeviceID {
			return a.DeviceID < b.DeviceID
		}
		if a.RuntimeID != b.RuntimeID {
			return a.RuntimeID < b.RuntimeID
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.EntryID < b.EntryID
	})
	return entries
}
