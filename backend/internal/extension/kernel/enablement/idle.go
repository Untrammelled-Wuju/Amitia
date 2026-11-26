//go:build rsi_lifecycle

package enablement

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"
)

var ErrIdleInvalid = errors.New("plugin idle: invalid policy")

type SupervisorDrainer interface {
	Drain(ctx context.Context, instanceID string, timeout time.Duration) error
}

type InstanceResolver func(pluginID string) (string, bool)

type IdlePolicy struct {
	TTL          time.Duration
	DrainTimeout time.Duration
}

func (p IdlePolicy) Validate() error {
	if p.TTL <= 0 {
		return fmt.Errorf("%w: ttl=%s", ErrIdleInvalid, p.TTL)
	}
	if p.DrainTimeout < 0 {
		return fmt.Errorf("%w: drain timeout=%s", ErrIdleInvalid, p.DrainTimeout)
	}
	return nil
}

type IdleReport struct {
	Released []string
	Skipped  []string
}

type IdleReaper struct {
	installer *Installer
	policy    IdlePolicy
	drainer   SupervisorDrainer
	resolve   InstanceResolver
	now       func() time.Time
	hotCheck  func(string) bool
	mu        sync.Mutex
	last      map[string]time.Time
}

func NewIdleReaper(installer *Installer, policy IdlePolicy, drainer SupervisorDrainer, resolve InstanceResolver) (*IdleReaper, error) {
	if installer == nil {
		return nil, fmt.Errorf("%w: installer is nil", ErrIdleInvalid)
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if policy.DrainTimeout == 0 {
		policy.DrainTimeout = 5 * time.Second
	}
	return &IdleReaper{
		installer: installer,
		policy:    policy,
		drainer:   drainer,
		resolve:   resolve,
		now:       time.Now,
		last:      map[string]time.Time{},
	}, nil
}

func NewIdleReaperWithClock(installer *Installer, policy IdlePolicy, drainer SupervisorDrainer, resolve InstanceResolver, now func() time.Time) (*IdleReaper, error) {
	reaper, err := NewIdleReaper(installer, policy, drainer, resolve)
	if err != nil {
		return nil, err
	}
	if now != nil {
		reaper.now = now
	}
	return reaper, nil
}

func (r *IdleReaper) Observe(pluginID string) {
	if r == nil || pluginID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.last[pluginID] = r.now().UTC()
}

func (r *IdleReaper) LastUsed(pluginID string) (time.Time, bool) {
	if r == nil {
		return time.Time{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	at, ok := r.last[pluginID]
	return at, ok
}

type idleCandidate struct {
	id    string
	state LifecycleState
}

func (r *IdleReaper) Sweep(ctx context.Context) (IdleReport, error) {
	report := IdleReport{Released: []string{}, Skipped: []string{}}
	if r == nil {
		return report, fmt.Errorf("%w: reaper is nil", ErrIdleInvalid)
	}
	if err := ctx.Err(); err != nil {
		return report, err
	}
	now := r.now().UTC()
	r.installer.mu.Lock()
	candidates := make([]idleCandidate, 0, len(r.installer.machines))
	for id, machine := range r.installer.machines {
		state := machine.State()
		if state != LifecycleActivated && state != LifecycleWarm {
			report.Skipped = append(report.Skipped, id)
			continue
		}
		candidates = append(candidates, idleCandidate{id: id, state: state})
	}
	r.installer.mu.Unlock()
	sort.Slice(candidates, func(left, right int) bool { return candidates[left].id < candidates[right].id })

	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		r.mu.Lock()
		lastUsed, seen := r.last[candidate.id]
		r.mu.Unlock()
		if !seen {
			report.Skipped = append(report.Skipped, candidate.id)
			continue
		}
		if now.Sub(lastUsed) < r.policy.TTL {
			report.Skipped = append(report.Skipped, candidate.id)
			continue
		}
		if r.hotCheck != nil && r.hotCheck(candidate.id) {
			report.Skipped = append(report.Skipped, candidate.id)
			continue
		}
		if r.drainer != nil && r.resolve != nil {
			instanceID, ok := r.resolve(candidate.id)
			if !ok || instanceID == "" {
				return report, fmt.Errorf("%w: %s has no runtime instance", ErrIdleInvalid, candidate.id)
			}
			if err := r.drainer.Drain(ctx, instanceID, r.policy.DrainTimeout); err != nil {
				return report, fmt.Errorf("%w: drain %s: %v", ErrIdleInvalid, candidate.id, err)
			}
		}
		handle, err := r.installer.releaseIdle(candidate.id, candidate.state)
		if err != nil {
			return report, err
		}
		if closer, ok := handle.(io.Closer); ok {
			_ = closer.Close()
		}
		r.mu.Lock()
		delete(r.last, candidate.id)
		r.mu.Unlock()
		report.Released = append(report.Released, candidate.id)
	}
	sort.Strings(report.Skipped)
	return report, nil
}

func (i *Installer) releaseIdle(pluginID string, state LifecycleState) (RuntimeHandle, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	machine, ok := i.machines[pluginID]
	if !ok {
		return nil, fmt.Errorf("%w: %s is not installed", ErrInstallerState, pluginID)
	}
	current := machine.State()
	if current != state {
		return nil, fmt.Errorf("%w: %s changed to %s during idle release", ErrInstallerState, pluginID, current)
	}
	if current == LifecycleWarm {
		if _, err := machine.Apply(context.Background(), EventDeactivate, machine.Epoch()+1); err != nil {
			return nil, err
		}
	}
	if _, err := machine.Apply(context.Background(), EventDeactivate, machine.Epoch()+1); err != nil {
		return nil, err
	}
	handle := i.handles[pluginID]
	delete(i.handles, pluginID)
	return handle, nil
}
