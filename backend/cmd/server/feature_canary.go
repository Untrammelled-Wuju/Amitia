// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/featuregate"
	"github.com/u-ai/backend/internal/system"
	"github.com/u-ai/backend/log"
)

const (
	featureCanaryProviderID   = "featuregate.canary.provider"
	featureCanaryCapabilityID = "featuregate.canary"
	featureCanarySubscriberID = "featuregate-canary"
	featureCanaryTickInterval = 5 * time.Second
)

type featureCanaryRuntime struct {
	mu              sync.RWMutex
	registeredAt    string
	subscribedAt    string
	workerStartedAt string
	workerTicks     atomic.Int64
	events          atomic.Int64
}

var featureCanary = &featureCanaryRuntime{}

func applyFeatureCanary(ctx context.Context, services *AppServices) error {
	outcome, err := featuregate.Apply(ctx, featuregate.NameRuntimeCanary, featuregate.Hooks{
		Provider:   func() error { return registerFeatureCanaryProvider(services) },
		Subscriber: func() error { return subscribeFeatureCanary(ctx) },
		Worker:     runFeatureCanaryWorker,
	})
	if err != nil {
		return err
	}
	if !outcome.Enabled {
		featureCanary.reset()
		log.Info("[FeatureGate] runtime-canary disabled: provider/worker/subscriber 均未注册")
		return nil
	}
	log.Info("[FeatureGate] runtime-canary enabled: registered=", outcome.Registered, " duplicate=", outcome.Duplicate)
	return nil
}

func registerFeatureCanaryProvider(services *AppServices) error {
	if services == nil || services.KernelContainer == nil || services.KernelContainer.ProviderLifecycle == nil {
		return fmt.Errorf("注册 featuregate canary provider: capability provider lifecycle 不可用")
	}
	if err := services.KernelContainer.ProviderLifecycle.RegisterProvider(capability.CapabilityProviderDefinition{
		ID:           capability.ProviderID(featureCanaryProviderID),
		CapabilityID: capability.CapabilityID(featureCanaryCapabilityID),
		Kind:         capability.ProviderKindBuiltin,
		Placement:    capability.ProviderPlacementCore,
		Runtime: capability.RuntimeBinding{
			RuntimeType: capability.RuntimeTypeBuiltin,
			HandlerName: featureCanaryCapabilityID,
		},
		Metadata: map[string]any{"probe": "featuregate", "inert": true},
	}); err != nil {
		return fmt.Errorf("注册 featuregate canary provider: %w", err)
	}
	featureCanary.markRegistered()
	return nil
}

func subscribeFeatureCanary(ctx context.Context) error {
	bus := system.GetMessageEventBus()
	subscriber := bus.Subscribe(featureCanarySubscriberID, nil)
	go func() {
		<-ctx.Done()
		bus.Unsubscribe(featureCanarySubscriberID)
	}()
	go func() {
		for range subscriber.Events {
			featureCanary.events.Add(1)
		}
	}()
	featureCanary.markSubscribed()
	return nil
}

func runFeatureCanaryWorker(ctx context.Context) {
	featureCanary.markWorkerStarted()
	ticker := time.NewTicker(featureCanaryTickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			featureCanary.workerTicks.Add(1)
		}
	}
}

func (r *featureCanaryRuntime) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.registeredAt = ""
	r.subscribedAt = ""
	r.workerStartedAt = ""
	r.workerTicks.Store(0)
	r.events.Store(0)
}

func (r *featureCanaryRuntime) markRegistered() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.registeredAt = time.Now().UTC().Format(time.RFC3339Nano)
}

func (r *featureCanaryRuntime) markSubscribed() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.subscribedAt = time.Now().UTC().Format(time.RFC3339Nano)
}

func (r *featureCanaryRuntime) markWorkerStarted() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.workerStartedAt = time.Now().UTC().Format(time.RFC3339Nano)
}

func (r *featureCanaryRuntime) snapshot() map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return map[string]any{
		"providerId":      featureCanaryProviderID,
		"capabilityId":    featureCanaryCapabilityID,
		"subscriberId":    featureCanarySubscriberID,
		"registeredAt":    r.registeredAt,
		"subscribedAt":    r.subscribedAt,
		"workerStartedAt": r.workerStartedAt,
		"workerTicks":     r.workerTicks.Load(),
		"eventsSeen":      r.events.Load(),
	}
}
