//go:build rsi_lifecycle

package enablement

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

var (
	ErrManifestInvalid   = errors.New("plugin manifest: invalid")
	ErrMetadataDuplicate = errors.New("plugin metadata: duplicate id")
	ErrInstallerState    = errors.New("plugin installer: unexpected lifecycle state")
)

type Manifest struct {
	ID          string
	Version     string
	Permissions []string
	Entry       string
}

func (m Manifest) Validate() error {
	if strings.TrimSpace(m.ID) == "" {
		return fmt.Errorf("%w: id is required", ErrManifestInvalid)
	}
	if strings.TrimSpace(m.Version) == "" {
		return fmt.Errorf("%w: version is required for %s", ErrManifestInvalid, m.ID)
	}
	return nil
}

type Metadata struct {
	ID          string
	Version     string
	Permissions []string
	Entry       string
}

type MetadataIndex struct {
	mu      sync.RWMutex
	entries map[string]Metadata
}

func NewMetadataIndex() *MetadataIndex {
	return &MetadataIndex{entries: map[string]Metadata{}}
}

func (i *MetadataIndex) Get(id string) (Metadata, bool) {
	if i == nil {
		return Metadata{}, false
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	entry, ok := i.entries[id]
	return entry, ok
}

func (i *MetadataIndex) Len() int {
	if i == nil {
		return 0
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	return len(i.entries)
}

func (i *MetadataIndex) set(entry Metadata) {
	if i == nil {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	i.entries[entry.ID] = entry
}

func (i *MetadataIndex) remove(id string) {
	if i == nil {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	delete(i.entries, id)
}

func (i *MetadataIndex) has(id string) bool {
	if i == nil {
		return false
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	_, ok := i.entries[id]
	return ok
}

func (i *MetadataIndex) entry(id string) (Metadata, bool) {
	if i == nil {
		return Metadata{}, false
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	entry, ok := i.entries[id]
	return entry, ok
}

func (i *MetadataIndex) IDs() []string {
	if i == nil {
		return nil
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	ids := make([]string, 0, len(i.entries))
	for id := range i.entries {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

type RuntimeHandle interface {
	ID() string
	Workers() int
}

type RuntimeSpawner interface {
	Spawn(ctx context.Context, metadata Metadata) (RuntimeHandle, error)
}

type ColdSnapshot struct {
	Plugins    int
	Processes  int
	Workers    int
	SpawnCalls int
}

type BatchReport struct {
	Installed int
	Skipped   int
}

type Installer struct {
	mu          sync.Mutex
	reqMu       sync.Mutex
	index       *MetadataIndex
	machines    map[string]*Machine
	handles     map[string]RuntimeHandle
	inflight    map[string]bool
	results     map[string]ExecuteResult
	activeCalls map[string]int
	spawner     RuntimeSpawner
	activity    func(string)
	spawns      int
}

func (i *Installer) WithActivity(activity func(string)) *Installer {
	i.activity = activity
	return i
}

func NewInstaller(spawner RuntimeSpawner) *Installer {
	return &Installer{
		index:       NewMetadataIndex(),
		machines:    map[string]*Machine{},
		handles:     map[string]RuntimeHandle{},
		inflight:    map[string]bool{},
		results:     map[string]ExecuteResult{},
		activeCalls: map[string]int{},
		spawner:     spawner,
	}
}

func (i *Installer) InstallBatch(ctx context.Context, manifests []Manifest) (BatchReport, error) {
	report := BatchReport{}
	if i == nil {
		return report, fmt.Errorf("%w: installer is nil", ErrManifestInvalid)
	}
	if err := ctx.Err(); err != nil {
		return report, err
	}
	staged := map[string]Metadata{}
	for _, manifest := range manifests {
		if err := manifest.Validate(); err != nil {
			return report, err
		}
		if _, exists := staged[manifest.ID]; exists {
			return report, fmt.Errorf("%w: %s appears twice in one batch", ErrMetadataDuplicate, manifest.ID)
		}
		staged[manifest.ID] = Metadata{ID: manifest.ID, Version: manifest.Version, Permissions: append([]string{}, manifest.Permissions...), Entry: manifest.Entry}
	}
	if err := ctx.Err(); err != nil {
		return report, err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	for id := range staged {
		if i.index.has(id) {
			report.Skipped++
			delete(staged, id)
		}
	}
	for id, entry := range staged {
		i.index.set(entry)
		machine, err := NewMachine(LifecycleInstalled)
		if err != nil {
			return report, err
		}
		i.machines[id] = machine
		report.Installed++
	}
	return report, nil
}

func (i *Installer) machineFor(id string) (*Machine, error) {
	machine, ok := i.machines[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s is not installed", ErrInstallerState, id)
	}
	return machine, nil
}

func (i *Installer) Enable(ctx context.Context, id string, epoch uint64) error {
	if i == nil {
		return fmt.Errorf("%w: installer is nil", ErrInstallerState)
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	machine, err := i.machineFor(id)
	if err != nil {
		return err
	}
	_, err = machine.Apply(ctx, EventEnable, epoch)
	return err
}

func (i *Installer) Activate(ctx context.Context, id string, epoch uint64) error {
	if i == nil {
		return fmt.Errorf("%w: installer is nil", ErrInstallerState)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if i.spawner == nil {
		return fmt.Errorf("%w: no runtime spawner", ErrInstallerState)
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	machine, err := i.machineFor(id)
	if err != nil {
		return err
	}
	if _, exists := i.handles[id]; exists {
		return nil
	}
	if machine.State() != LifecycleEnabled {
		return fmt.Errorf("%w: %s is %s, activation needs enabled", ErrInstallerState, id, machine.State())
	}
	metadata, ok := i.index.entry(id)
	if !ok {
		return fmt.Errorf("%w: %s metadata missing", ErrInstallerState, id)
	}
	if _, err := machine.Apply(ctx, EventActivate, epoch); err != nil {
		return err
	}
	handle, err := i.spawner.Spawn(ctx, metadata)
	if err != nil {
		return fmt.Errorf("spawn runtime for %s: %w", id, err)
	}
	i.handles[id] = handle
	i.spawns++
	return nil
}

func (i *Installer) Uninstall(ctx context.Context, id string, epoch uint64) error {
	if i == nil {
		return fmt.Errorf("%w: installer is nil", ErrInstallerState)
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	machine, err := i.machineFor(id)
	if err != nil {
		return err
	}
	if _, err := machine.Apply(ctx, EventUninstall, epoch); err != nil {
		return err
	}
	i.index.remove(id)
	delete(i.machines, id)
	delete(i.handles, id)
	return nil
}

func (i *Installer) State(id string) (LifecycleState, bool) {
	if i == nil {
		return "", false
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	machine, ok := i.machines[id]
	if !ok {
		return "", false
	}
	return machine.State(), true
}

func (i *Installer) Snapshot() ColdSnapshot {
	if i == nil {
		return ColdSnapshot{}
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	workers := 0
	for _, handle := range i.handles {
		workers += handle.Workers()
	}
	return ColdSnapshot{
		Plugins:    i.index.Len(),
		Processes:  len(i.handles),
		Workers:    workers,
		SpawnCalls: i.spawns,
	}
}
