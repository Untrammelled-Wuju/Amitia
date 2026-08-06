// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdranthealth

import (
	"context"
	"sync"
	"time"
)

type ProcessGuard interface {
	IsStarted() bool
	IsExited() bool
	PID() int
}

type ReadyWaiter struct {
	mu        sync.Mutex
	target    Target
	policy    Policy
	prober    Prober
	backoff   *Backoff
	guard     ProcessGuard
	stopCh    chan struct{}
	stopped   bool
	onReady   func(Snapshot)
	onFailure func(Snapshot, error)
}

func NewReadyWaiter(target Target, policy Policy, guard ProcessGuard) *ReadyWaiter {
	return &ReadyWaiter{
		target:  target,
		policy:  policy,
		prober:  NewProber(),
		backoff: NewBackoffFromPolicy(policy),
		guard:   guard,
		stopCh:  make(chan struct{}),
	}
}

func (w *ReadyWaiter) SetOnReady(fn func(Snapshot)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onReady = fn
}

func (w *ReadyWaiter) SetOnFailure(fn func(Snapshot, error)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onFailure = fn
}

func (w *ReadyWaiter) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.stopped {
		close(w.stopCh)
		w.stopped = true
	}
}

func (w *ReadyWaiter) Stopped() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.stopped
}

func (w *ReadyWaiter) Wait(ctx context.Context) (Snapshot, error) {
	if err := w.target.Validate(); err != nil {
		return Snapshot{}, err
	}
	if err := w.policy.Validate(); err != nil {
		return Snapshot{}, err
	}
	if w.guard == nil {
		return Snapshot{}, ErrGuardRequired
	}

	deadline := time.Now().Add(w.policy.StartupTimeout)
	snapshot := NewSnapshot(StateProcessStarted, w.target)

	if !w.guard.IsStarted() {
		return snapshot.WithState(StateProcessNotStarted), ErrProcessNotStarted
	}

	for {
		select {
		case <-ctx.Done():
			return snapshot, ctx.Err()
		case <-w.stopCh:
			return snapshot, ErrWaiterStopped
		default:
		}

		if w.guard.IsExited() {
			return snapshot.WithState(StateProcessNotStarted), ErrProcessExited
		}

		identityResult := w.prober.Probe(ctx, w.target.IdentityProbe())
		identityResult = identityResult.WithTimestamp(time.Now())

		if identityResult.IsOK() {
			identity, err := ParseIdentity(identityResult.Body)
			if err == nil {
				identity.Confirmed = true
				identity.ConfirmedAt = time.Now()
				if verr := identity.Validate(); verr == nil {
					snapshot.Identity = identity
					snapshot = snapshot.WithState(StateIdentityConfirmed)
				}
			}
		}

		readyResult := w.prober.Probe(ctx, w.target.ReadyProbe())
		readyResult = readyResult.WithTimestamp(time.Now())

		if readyResult.IsOK() {
			if !snapshot.Identity.Confirmed && !w.policy.AllowFallbackLive {
				identityResult := w.prober.Probe(ctx, w.target.IdentityProbe())
				identityResult = identityResult.WithTimestamp(time.Now())
				if identityResult.IsOK() {
					identity, err := ParseIdentity(identityResult.Body)
					if err == nil {
						identity.Confirmed = true
						identity.ConfirmedAt = time.Now()
						if verr := identity.Validate(); verr == nil {
							snapshot.Identity = identity
							snapshot = snapshot.WithState(StateIdentityConfirmed)
						}
					}
				}
			}
			snapshot = snapshot.WithLastResult(readyResult)
			snapshot = snapshot.WithState(StateReady)
			w.fireReady(snapshot)
			return snapshot, nil
		}

		liveResult := w.prober.Probe(ctx, w.target.LiveProbe())
		liveResult = liveResult.WithTimestamp(time.Now())

		if liveResult.IsOK() {
			snapshot = snapshot.WithState(StateLive)
		}

		snapshot = snapshot.WithLastResult(readyResult)

		now := time.Now()
		if now.After(deadline) {
			w.fireFailure(snapshot, ErrStartupTimeout)
			return snapshot, ErrStartupTimeout
		}

		delay := w.backoff.Next()
		if rem := time.Until(deadline); rem < delay {
			delay = rem
		}

		select {
		case <-ctx.Done():
			return snapshot, ctx.Err()
		case <-w.stopCh:
			return snapshot, ErrWaiterStopped
		case <-time.After(delay):
		}
	}
}

func (w *ReadyWaiter) fireReady(snapshot Snapshot) {
	w.mu.Lock()
	fn := w.onReady
	w.mu.Unlock()
	if fn != nil {
		fn(snapshot)
	}
}

func (w *ReadyWaiter) fireFailure(snapshot Snapshot, err error) {
	w.mu.Lock()
	fn := w.onFailure
	w.mu.Unlock()
	if fn != nil {
		fn(snapshot, err)
	}
}
