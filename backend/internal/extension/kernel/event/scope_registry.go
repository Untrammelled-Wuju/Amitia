//go:build rsi_eventing

package event

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

var (
	ErrScopedSubscriptionInvalid  = errors.New("event: scoped subscription invalid")
	ErrScopedSubscriptionConflict = errors.New("event: scoped subscription conflict")
)

type ScopeFilter map[string]string

func (f ScopeFilter) Canonical() string {
	keys := make([]string, 0, len(f))
	for key := range f {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+f[key])
	}
	return strings.Join(parts, "&")
}

func (f ScopeFilter) Matches(fields map[string]string) bool {
	for key, want := range f {
		if fields[key] != want {
			return false
		}
	}
	return true
}

func (f ScopeFilter) validate() error {
	for key, value := range f {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("%w: empty filter field", ErrScopedSubscriptionInvalid)
		}
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%w: empty filter value for %s", ErrScopedSubscriptionInvalid, key)
		}
	}
	return nil
}

type ScopeSubscriptionKey struct {
	Topic string
	Scope string
	Owner string
}

func (k ScopeSubscriptionKey) normalized() ScopeSubscriptionKey {
	return ScopeSubscriptionKey{
		Topic: strings.TrimSpace(k.Topic),
		Scope: strings.TrimSpace(k.Scope),
		Owner: strings.TrimSpace(k.Owner),
	}
}

func (k ScopeSubscriptionKey) String() string {
	return k.Topic + "|" + k.Scope + "|" + k.Owner
}

func (k ScopeSubscriptionKey) validate() error {
	if strings.TrimSpace(k.Topic) == "" {
		return fmt.Errorf("%w: topic required", ErrScopedSubscriptionInvalid)
	}
	if strings.TrimSpace(k.Scope) == "" {
		return fmt.Errorf("%w: scope required", ErrScopedSubscriptionInvalid)
	}
	if strings.TrimSpace(k.Owner) == "" {
		return fmt.Errorf("%w: owner required", ErrScopedSubscriptionInvalid)
	}
	return nil
}

type ScopedEvent struct {
	ID     string
	Topic  string
	Scope  string
	Fields map[string]string
}

func (e ScopedEvent) validate() error {
	if strings.TrimSpace(e.Topic) == "" {
		return fmt.Errorf("%w: event topic required", ErrScopedSubscriptionInvalid)
	}
	if strings.TrimSpace(e.Scope) == "" {
		return fmt.Errorf("%w: event scope required", ErrScopedSubscriptionInvalid)
	}
	return nil
}

type ScopedHandler interface {
	HandleScopedEvent(ctx context.Context, event ScopedEvent) error
}

type ScopedHandlerFunc func(ctx context.Context, event ScopedEvent) error

func (f ScopedHandlerFunc) HandleScopedEvent(ctx context.Context, event ScopedEvent) error {
	return f(ctx, event)
}

type RuntimeActivator interface {
	EnsureActive(ctx context.Context, owner string) error
}

type ScopedRegistration struct {
	Topic   string
	Scope   string
	Owner   string
	Filter  ScopeFilter
	Wake    bool
	Handler ScopedHandler
}

func (r ScopedRegistration) key() ScopeSubscriptionKey {
	return ScopeSubscriptionKey{Topic: r.Topic, Scope: r.Scope, Owner: r.Owner}.normalized()
}

func (r ScopedRegistration) validate() error {
	if err := r.key().validate(); err != nil {
		return err
	}
	if r.Handler == nil {
		return fmt.Errorf("%w: handler required", ErrScopedSubscriptionInvalid)
	}
	return r.Filter.validate()
}

type ScopedSubscription struct {
	Key        ScopeSubscriptionKey
	Filter     string
	Wake       bool
	Generation int64
	Duplicates int
	Deliveries int
}

type scopedEntry struct {
	key        ScopeSubscriptionKey
	filter     ScopeFilter
	wake       bool
	handler    ScopedHandler
	generation int64
	duplicates int
	deliveries int
}

func (e *scopedEntry) view() ScopedSubscription {
	return ScopedSubscription{
		Key:        e.key,
		Filter:     e.filter.Canonical(),
		Wake:       e.wake,
		Generation: e.generation,
		Duplicates: e.duplicates,
		Deliveries: e.deliveries,
	}
}

type ScopedDeliveryReport struct {
	Matched          int
	Delivered        int
	ActivationFailed int
	HandlerFailed    int
	Wakes            int
	Skipped          int
}

type ScopedRegistry struct {
	mu        sync.RWMutex
	entries   map[ScopeSubscriptionKey]*scopedEntry
	epochs    map[ScopeSubscriptionKey]int64
	activator RuntimeActivator
}

func NewScopedRegistry() *ScopedRegistry {
	return &ScopedRegistry{
		entries: map[ScopeSubscriptionKey]*scopedEntry{},
		epochs:  map[ScopeSubscriptionKey]int64{},
	}
}

func (r *ScopedRegistry) WithActivator(activator RuntimeActivator) *ScopedRegistry {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.activator = activator
	return r
}

func (r *ScopedRegistry) Subscribe(ctx context.Context, registration ScopedRegistration) (ScopedSubscription, bool, error) {
	if err := ctx.Err(); err != nil {
		return ScopedSubscription{}, false, err
	}
	if err := registration.validate(); err != nil {
		return ScopedSubscription{}, false, err
	}
	key := registration.key()
	filter := registration.Filter.Canonical()
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.entries[key]; ok {
		if existing.filter.Canonical() != filter {
			return existing.view(), false, fmt.Errorf("%w: %s filter %q != %q", ErrScopedSubscriptionConflict, key.String(), existing.filter.Canonical(), filter)
		}
		existing.duplicates++
		return existing.view(), false, nil
	}
	generation := r.epochs[key] + 1
	r.epochs[key] = generation
	entry := &scopedEntry{
		key:        key,
		filter:     registration.Filter,
		wake:       registration.Wake,
		handler:    registration.Handler,
		generation: generation,
	}
	r.entries[key] = entry
	return entry.view(), true, nil
}

func (r *ScopedRegistry) Unsubscribe(ctx context.Context, key ScopeSubscriptionKey) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	normalized := key.normalized()
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.entries[normalized]; !ok {
		return false, nil
	}
	delete(r.entries, normalized)
	return true, nil
}

func (r *ScopedRegistry) Dispatch(ctx context.Context, event ScopedEvent) (ScopedDeliveryReport, error) {
	report := ScopedDeliveryReport{}
	if err := ctx.Err(); err != nil {
		return report, err
	}
	if err := event.validate(); err != nil {
		return report, err
	}
	r.mu.RLock()
	matches := make([]*scopedEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		if entry.key.Topic != strings.TrimSpace(event.Topic) {
			continue
		}
		if entry.key.Scope != "*" && entry.key.Scope != strings.TrimSpace(event.Scope) {
			continue
		}
		if !entry.filter.Matches(event.Fields) {
			continue
		}
		matches = append(matches, entry)
	}
	activator := r.activator
	report.Matched = len(matches)
	r.mu.RUnlock()
	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].key.String() < matches[j].key.String()
	})
	var failures []error
	for _, entry := range matches {
		if entry.wake && activator != nil {
			report.Wakes++
			if err := activator.EnsureActive(ctx, entry.key.Owner); err != nil {
				report.ActivationFailed++
				failures = append(failures, fmt.Errorf("activate %s: %w", entry.key.String(), err))
				continue
			}
		}
		if err := entry.handler.HandleScopedEvent(ctx, event); err != nil {
			report.HandlerFailed++
			failures = append(failures, fmt.Errorf("deliver %s: %w", entry.key.String(), err))
			continue
		}
		report.Delivered++
		r.mu.Lock()
		entry.deliveries++
		r.mu.Unlock()
	}
	report.Skipped = report.Matched - report.Delivered
	return report, errors.Join(failures...)
}

func (r *ScopedRegistry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entries)
}

func (r *ScopedRegistry) Get(key ScopeSubscriptionKey) (ScopedSubscription, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.entries[key.normalized()]
	if !ok {
		return ScopedSubscription{}, false
	}
	return entry.view(), true
}

func (r *ScopedRegistry) Snapshot() []ScopedSubscription {
	r.mu.RLock()
	defer r.mu.RUnlock()
	views := make([]ScopedSubscription, 0, len(r.entries))
	for _, entry := range r.entries {
		views = append(views, entry.view())
	}
	sort.SliceStable(views, func(i, j int) bool {
		return views[i].Key.String() < views[j].Key.String()
	})
	return views
}
