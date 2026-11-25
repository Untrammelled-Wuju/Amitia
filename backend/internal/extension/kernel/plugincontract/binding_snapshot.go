package plugincontract

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	ErrBindingInvalid  = errors.New("plugincontract: binding invalid")
	ErrBindingNotFound = errors.New("plugincontract: binding not found")
	ErrSnapshotEmpty   = errors.New("plugincontract: no binding snapshot installed")
	ErrSnapshotRelease = errors.New("plugincontract: binding snapshot released")
)

type BindingScope string

const (
	BindingScopeDevice BindingScope = "device"
	BindingScopeSpace  BindingScope = "space"
	BindingScopeGlobal BindingScope = "global"
)

var BindingScopeOrder = []BindingScope{BindingScopeDevice, BindingScopeSpace, BindingScopeGlobal}

type BindingKey struct {
	CapabilityID string       `json:"capabilityId"`
	SpaceID      string       `json:"spaceId,omitempty"`
	DeviceID     string       `json:"deviceId,omitempty"`
	Scope        BindingScope `json:"scope"`
}

type Binding struct {
	Key                BindingKey `json:"key"`
	ProviderID         string     `json:"providerId"`
	ProviderInstanceID string     `json:"providerInstanceId"`
}

type BindingInvalidError struct {
	Field  string
	Value  string
	Reason string
}

func (e *BindingInvalidError) Error() string {
	return fmt.Sprintf("plugincontract: binding field %q=%q %s", e.Field, e.Value, e.Reason)
}

func (e *BindingInvalidError) Unwrap() error {
	return ErrBindingInvalid
}

func (b Binding) validate() error {
	if strings.TrimSpace(b.Key.CapabilityID) == "" {
		return &BindingInvalidError{Field: "capabilityId", Value: b.Key.CapabilityID, Reason: "must not be empty"}
	}
	if strings.TrimSpace(b.ProviderID) == "" {
		return &BindingInvalidError{Field: "providerId", Value: b.ProviderID, Reason: "must not be empty"}
	}
	if strings.TrimSpace(b.ProviderInstanceID) == "" {
		return &BindingInvalidError{Field: "providerInstanceId", Value: b.ProviderInstanceID, Reason: "must not be empty"}
	}
	switch b.Key.Scope {
	case BindingScopeDevice:
		if strings.TrimSpace(b.Key.SpaceID) == "" {
			return &BindingInvalidError{Field: "spaceId", Value: b.Key.SpaceID, Reason: "is required for device scope"}
		}
		if strings.TrimSpace(b.Key.DeviceID) == "" {
			return &BindingInvalidError{Field: "deviceId", Value: b.Key.DeviceID, Reason: "is required for device scope"}
		}
	case BindingScopeSpace:
		if strings.TrimSpace(b.Key.SpaceID) == "" {
			return &BindingInvalidError{Field: "spaceId", Value: b.Key.SpaceID, Reason: "is required for space scope"}
		}
		if b.Key.DeviceID != "" {
			return &BindingInvalidError{Field: "deviceId", Value: b.Key.DeviceID, Reason: "must be empty for space scope"}
		}
	case BindingScopeGlobal:
		if b.Key.SpaceID != "" || b.Key.DeviceID != "" {
			return &BindingInvalidError{Field: "spaceId/deviceId", Value: b.Key.SpaceID + "/" + b.Key.DeviceID, Reason: "must be empty for global scope"}
		}
	default:
		return &BindingInvalidError{Field: "scope", Value: string(b.Key.Scope), Reason: "must be one of device, space, global"}
	}
	return nil
}

type BindingSnapshot struct {
	revision uint64
	epoch    string
	digest   string
	bindings []Binding
	index    map[BindingKey]Binding
}

func NewBindingSnapshot(epoch string, revision uint64, bindings []Binding) (*BindingSnapshot, error) {
	if strings.TrimSpace(epoch) == "" {
		return nil, &BindingInvalidError{Field: "epoch", Value: epoch, Reason: "must not be empty"}
	}
	if revision == 0 {
		return nil, &BindingInvalidError{Field: "revision", Value: "0", Reason: "must be at least 1"}
	}
	copied := make([]Binding, len(bindings))
	copy(copied, bindings)
	index := make(map[BindingKey]Binding, len(copied))
	for _, binding := range copied {
		if err := binding.validate(); err != nil {
			return nil, err
		}
		if _, exists := index[binding.Key]; exists {
			return nil, &BindingInvalidError{Field: "bindingKey", Value: bindingKeyString(binding.Key), Reason: "is declared twice"}
		}
		index[binding.Key] = binding
	}
	sort.Slice(copied, func(left, right int) bool {
		return bindingKeyString(copied[left].Key) < bindingKeyString(copied[right].Key)
	})
	return &BindingSnapshot{
		revision: revision,
		epoch:    strings.TrimSpace(epoch),
		digest:   bindingDigest(copied),
		bindings: copied,
		index:    index,
	}, nil
}

func bindingKeyString(key BindingKey) string {
	return strings.Join([]string{key.CapabilityID, key.SpaceID, key.DeviceID, string(key.Scope)}, "|")
}

func bindingDigest(bindings []Binding) string {
	lines := make([]string, len(bindings))
	for index, binding := range bindings {
		lines[index] = strings.Join([]string{
			binding.Key.CapabilityID,
			binding.Key.SpaceID,
			binding.Key.DeviceID,
			string(binding.Key.Scope),
			binding.ProviderID,
			binding.ProviderInstanceID,
		}, "|")
	}
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}

func (s *BindingSnapshot) Revision() uint64 { return s.revision }

func (s *BindingSnapshot) Epoch() string { return s.epoch }

func (s *BindingSnapshot) Digest() string { return s.digest }

func (s *BindingSnapshot) Len() int { return len(s.bindings) }

func (s *BindingSnapshot) Bindings() []Binding {
	copied := make([]Binding, len(s.bindings))
	copy(copied, s.bindings)
	return copied
}

func (s *BindingSnapshot) Lookup(key BindingKey) (Binding, bool) {
	binding, ok := s.index[key]
	return binding, ok
}

type BindingRequest struct {
	CapabilityID string `json:"capabilityId"`
	SpaceID      string `json:"spaceId"`
	DeviceID     string `json:"deviceId"`
}

type BindingResolution struct {
	Binding  Binding
	Revision uint64
	Epoch    string
	Scope    BindingScope
	Trail    []string
}

type BindingNotFoundError struct {
	CapabilityID string
	SpaceID      string
	DeviceID     string
	Revision     uint64
	Epoch        string
	Tried        []BindingScope
	Missing      []string
}

func (e *BindingNotFoundError) Error() string {
	detail := fmt.Sprintf("plugincontract: binding for capability %q not found at revision %d epoch %q (space %q device %q; tried %s", e.CapabilityID, e.Revision, e.Epoch, e.SpaceID, e.DeviceID, joinScopes(e.Tried))
	if len(e.Missing) > 0 {
		detail += fmt.Sprintf("; missing %s", strings.Join(e.Missing, ", "))
	}
	return detail + ")"
}

func (e *BindingNotFoundError) Unwrap() error { return ErrBindingNotFound }

func joinScopes(scopes []BindingScope) string {
	parts := make([]string, len(scopes))
	for index, scope := range scopes {
		parts[index] = string(scope)
	}
	return strings.Join(parts, ", ")
}

func (s *BindingSnapshot) Resolve(request BindingRequest) (BindingResolution, error) {
	capabilityID := strings.TrimSpace(request.CapabilityID)
	if capabilityID == "" {
		return BindingResolution{}, &BindingInvalidError{Field: "capabilityId", Value: request.CapabilityID, Reason: "must not be empty"}
	}
	trail := make([]string, 0, len(BindingScopeOrder))
	missing := make([]string, 0, 2)
	if strings.TrimSpace(request.SpaceID) == "" {
		missing = append(missing, "spaceId")
	}
	if strings.TrimSpace(request.DeviceID) == "" {
		missing = append(missing, "deviceId")
	}
	for _, scope := range BindingScopeOrder {
		key := BindingKey{CapabilityID: capabilityID, Scope: scope}
		switch scope {
		case BindingScopeDevice:
			if strings.TrimSpace(request.SpaceID) == "" || strings.TrimSpace(request.DeviceID) == "" {
				trail = append(trail, "device=skip")
				continue
			}
			key.SpaceID, key.DeviceID = request.SpaceID, request.DeviceID
		case BindingScopeSpace:
			if strings.TrimSpace(request.SpaceID) == "" {
				trail = append(trail, "space=skip")
				continue
			}
			key.SpaceID = request.SpaceID
		case BindingScopeGlobal:
		}
		if binding, ok := s.index[key]; ok {
			trail = append(trail, string(scope)+"=hit")
			return BindingResolution{
				Binding:  binding,
				Revision: s.revision,
				Epoch:    s.epoch,
				Scope:    scope,
				Trail:    trail,
			}, nil
		}
		trail = append(trail, string(scope)+"=miss")
	}
	return BindingResolution{}, &BindingNotFoundError{
		CapabilityID: capabilityID,
		SpaceID:      request.SpaceID,
		DeviceID:     request.DeviceID,
		Revision:     s.revision,
		Epoch:        s.epoch,
		Tried:        append([]BindingScope(nil), BindingScopeOrder...),
		Missing:      missing,
	}
}

type retainedBindingSnapshot struct {
	snapshot *BindingSnapshot
	refs     int
}

type BindingSnapshotStore struct {
	mu       sync.Mutex
	epoch    string
	next     uint64
	current  *BindingSnapshot
	retained map[uint64]*retainedBindingSnapshot
}

func NewBindingSnapshotStore(epoch string) (*BindingSnapshotStore, error) {
	if strings.TrimSpace(epoch) == "" {
		return nil, &BindingInvalidError{Field: "epoch", Value: epoch, Reason: "must not be empty"}
	}
	return &BindingSnapshotStore{
		epoch:    strings.TrimSpace(epoch),
		retained: make(map[uint64]*retainedBindingSnapshot),
	}, nil
}

func (s *BindingSnapshotStore) Epoch() string { return s.epoch }

func (s *BindingSnapshotStore) Install(bindings []Binding) (*BindingSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	revision := s.next + 1
	snapshot, err := NewBindingSnapshot(s.epoch, revision, bindings)
	if err != nil {
		return nil, err
	}
	s.current = snapshot
	s.next = revision
	return snapshot, nil
}

func (s *BindingSnapshotStore) Current() *BindingSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current
}

func (s *BindingSnapshotStore) Acquire() (*BindingHandle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == nil {
		return nil, ErrSnapshotEmpty
	}
	snapshot := s.current
	entry, ok := s.retained[snapshot.Revision()]
	if !ok {
		entry = &retainedBindingSnapshot{snapshot: snapshot}
		s.retained[snapshot.Revision()] = entry
	}
	entry.refs++
	return &BindingHandle{snapshot: snapshot, store: s}, nil
}

func (s *BindingSnapshotStore) release(revision uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.retained[revision]
	if !ok {
		return
	}
	entry.refs--
	if entry.refs > 0 {
		return
	}
	if s.current != nil && s.current.Revision() == revision {
		entry.refs = 0
		return
	}
	delete(s.retained, revision)
}

func (s *BindingSnapshotStore) RetainedRevisions() []uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	revisions := make([]uint64, 0, len(s.retained))
	for revision, entry := range s.retained {
		if entry.refs > 0 {
			revisions = append(revisions, revision)
		}
	}
	sort.Slice(revisions, func(left, right int) bool { return revisions[left] < revisions[right] })
	return revisions
}

func (s *BindingSnapshotStore) RetainedCount() int {
	return len(s.RetainedRevisions())
}

type BindingHandle struct {
	snapshot *BindingSnapshot
	store    *BindingSnapshotStore
	once     sync.Once
	released atomic.Bool
}

func (h *BindingHandle) Snapshot() *BindingSnapshot {
	if h == nil {
		return nil
	}
	return h.snapshot
}

func (h *BindingHandle) Revision() uint64 {
	if h == nil || h.snapshot == nil {
		return 0
	}
	return h.snapshot.Revision()
}

func (h *BindingHandle) Resolve(request BindingRequest) (BindingResolution, error) {
	if h == nil || h.snapshot == nil {
		return BindingResolution{}, ErrSnapshotEmpty
	}
	if h.released.Load() {
		return BindingResolution{}, ErrSnapshotRelease
	}
	return h.snapshot.Resolve(request)
}

func (h *BindingHandle) Release() {
	if h == nil || h.store == nil {
		return
	}
	h.once.Do(func() {
		h.released.Store(true)
		h.store.release(h.snapshot.Revision())
	})
}
