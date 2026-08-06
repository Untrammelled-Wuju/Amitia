// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdranthealth

import (
	"context"
	"sync"
	"time"
)

type HealthCheck struct {
	mu         sync.RWMutex
	target     Target
	policy     Policy
	prober     Prober
	snapshot   Snapshot
	listeners  []func(Snapshot)
}

func NewHealthCheck(target Target, policy Policy) *HealthCheck {
	return &HealthCheck{
		target:    target,
		policy:    policy,
		prober:    NewProber(),
		snapshot:  NewSnapshot(StateProcessStarted, target),
		listeners: make([]func(Snapshot), 0),
	}
}

func (h *HealthCheck) Target() Target {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.target
}

func (h *HealthCheck) Snapshot() Snapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.snapshot.Clone()
}

func (h *HealthCheck) AddListener(fn func(Snapshot)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.listeners = append(h.listeners, fn)
}

func (h *HealthCheck) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.snapshot = h.snapshot.Reset()
}

func (h *HealthCheck) Check(ctx context.Context) Snapshot {
	h.mu.Lock()
	defer h.mu.Unlock()

	liveResult := h.prober.Probe(ctx, h.target.LiveProbe())
	liveResult = liveResult.WithTimestamp(time.Now())

	readyResult := h.prober.Probe(ctx, h.target.ReadyProbe())
	readyResult = readyResult.WithTimestamp(time.Now())

	if readyResult.IsOK() {
		h.snapshot = h.snapshot.WithLastResult(readyResult)
		h.snapshot = h.snapshot.WithState(StateReady)
		h.snapshot.LastError = nil
	} else if liveResult.IsOK() {
		h.snapshot = h.snapshot.WithLastResult(liveResult)
		h.snapshot = h.snapshot.WithState(StateLive)
	} else {
		h.snapshot = h.snapshot.WithLastResult(liveResult)
		h.snapshot.LastError = liveResult.Err
	}

	h.notifyListeners(h.snapshot)
	return h.snapshot.Clone()
}

func (h *HealthCheck) CheckLive(ctx context.Context) Snapshot {
	h.mu.Lock()
	defer h.mu.Unlock()

	result := h.prober.Probe(ctx, h.target.LiveProbe())
	result = result.WithTimestamp(time.Now())

	h.snapshot = h.snapshot.WithLastResult(result)
	if result.IsOK() {
		if !h.snapshot.State.IsReady() {
			h.snapshot = h.snapshot.WithState(StateLive)
		}
	}
	h.notifyListeners(h.snapshot)
	return h.snapshot.Clone()
}

func (h *HealthCheck) CheckReady(ctx context.Context) Snapshot {
	h.mu.Lock()
	defer h.mu.Unlock()

	result := h.prober.Probe(ctx, h.target.ReadyProbe())
	result = result.WithTimestamp(time.Now())

	h.snapshot = h.snapshot.WithLastResult(result)
	if result.IsOK() {
		h.snapshot = h.snapshot.WithState(StateReady)
		h.snapshot.LastError = nil
	} else {
		h.snapshot.LastError = result.Err
	}
	h.notifyListeners(h.snapshot)
	return h.snapshot.Clone()
}

func (h *HealthCheck) CheckIdentity(ctx context.Context) (Identity, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	result := h.prober.Probe(ctx, h.target.IdentityProbe())
	result = result.WithTimestamp(time.Now())

	if !result.IsOK() {
		return Identity{}, result.Err
	}

	identity, err := ParseIdentity(result.Body)
	if err != nil {
		return Identity{}, err
	}

	identity.Confirmed = true
	identity.ConfirmedAt = time.Now()

	if verr := identity.Validate(); verr != nil {
		return identity, verr
	}

	h.snapshot.Identity = identity
	h.snapshot = h.snapshot.WithState(StateIdentityConfirmed)
	h.notifyListeners(h.snapshot)
	return identity, nil
}

func (h *HealthCheck) IsReady() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.snapshot.State.IsReady()
}

func (h *HealthCheck) IsLive() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.snapshot.State.IsLive()
}

func (h *HealthCheck) State() State {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.snapshot.State
}

func (h *HealthCheck) notifyListeners(snapshot Snapshot) {
	for _, fn := range h.listeners {
		fn(snapshot)
	}
}
