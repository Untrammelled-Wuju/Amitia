// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package qdranthealth

import (
	"context"
	"sync"
	"time"
)

type ProcessController interface {
	Stop(ctx context.Context) error
	ReleaseLease(ctx context.Context) error
}

type Coordinator struct {
	mu         sync.Mutex
	target     Target
	policy     Policy
	guard      ProcessGuard
	controller ProcessController
	waiter     *ReadyWaiter
	check      *HealthCheck
	snapshot   Snapshot
	started    bool
}

func NewCoordinator(target Target, policy Policy, guard ProcessGuard, controller ProcessController) *Coordinator {
	c := &Coordinator{
		target:     target,
		policy:     policy,
		guard:      guard,
		controller: controller,
	}
	c.waiter = NewReadyWaiter(target, policy, guard)
	c.check = NewHealthCheck(target, policy)
	return c
}

func (c *Coordinator) WaitReady(ctx context.Context) (Snapshot, error) {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return c.snapshot, nil
	}
	c.mu.Unlock()

	snapshot, err := c.waiter.Wait(ctx)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.snapshot = snapshot

	if err != nil {
		if c.controller != nil {
			_ = c.controller.Stop(ctx)
			_ = c.controller.ReleaseLease(ctx)
		}
		return snapshot, err
	}

	c.started = true
	return snapshot, nil
}

func (c *Coordinator) HealthCheck(ctx context.Context) Snapshot {
	return c.check.Check(ctx)
}

func (c *Coordinator) CheckLive(ctx context.Context) Snapshot {
	return c.check.CheckLive(ctx)
}

func (c *Coordinator) CheckReady(ctx context.Context) Snapshot {
	return c.check.CheckReady(ctx)
}

func (c *Coordinator) CheckIdentity(ctx context.Context) (Identity, error) {
	return c.check.CheckIdentity(ctx)
}

func (c *Coordinator) Snapshot() Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.snapshot.Clone()
}

func (c *Coordinator) State() State {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.snapshot.State
}

func (c *Coordinator) IsReady() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.snapshot.State.IsReady()
}

func (c *Coordinator) Target() Target {
	return c.target
}

func (c *Coordinator) Stop() {
	c.waiter.Stop()
}

func (c *Coordinator) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.started = false
	c.check.Reset()
	c.snapshot = NewSnapshot(StateProcessStarted, c.target)
}

type ReadyCallback func(Snapshot) error

func WaitWithTimeout(target Target, policy Policy, guard ProcessGuard, timeout time.Duration) (Snapshot, error) {
	if timeout <= 0 {
		timeout = policy.StartupTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	waiter := NewReadyWaiter(target, policy, guard)
	return waiter.Wait(ctx)
}
