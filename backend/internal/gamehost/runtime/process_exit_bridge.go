package runtime

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/trusted_service"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/recovery"
)

type ProcessExitBridge interface {
	RegisterObserver()
	UnregisterObserver()
}

type processExitBridgeImpl struct {
	mu              sync.Mutex
	supervisor      *trusted_service.ProcessSupervisor
	recovery        *recovery.RecoveryCoordinator
	topologyStore   *TopologyStore
	registry        *Manager
	registered      bool
}

func NewProcessExitBridge(
	supervisor *trusted_service.ProcessSupervisor,
	recoveryCoordinator *recovery.RecoveryCoordinator,
	topologyStore *TopologyStore,
	registry *Manager,
) ProcessExitBridge {
	return &processExitBridgeImpl{
		supervisor:    supervisor,
		recovery:      recoveryCoordinator,
		topologyStore: topologyStore,
		registry:      registry,
	}
}

func (b *processExitBridgeImpl) RegisterObserver() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.registered {
		return
	}
	b.supervisor.RegisterProcessExitObserver(b)
	b.registered = true
}

func (b *processExitBridgeImpl) UnregisterObserver() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.registered {
		return
	}
	b.supervisor.UnregisterProcessExitObserver(b)
	b.registered = false
}

func (b *processExitBridgeImpl) resolveRuntimeID(serviceID string) (domain.RuntimeInstanceID, bool) {
	b.topologyStore.mu.RLock()
	defer b.topologyStore.mu.RUnlock()
	for runtimeID, svcMap := range b.topologyStore.definitionIDs {
		if _, ok := svcMap[domain.ServiceID(serviceID)]; ok {
			return runtimeID, true
		}
	}
	return "", false
}

func (b *processExitBridgeImpl) OnProcessExit(event trusted_service.ProcessExitEvent) {
	if event.Expected {
		return
	}

	runtimeID, ok := b.resolveRuntimeID(event.ServiceID)
	if !ok {
		log.Printf("[process-exit-bridge] could not resolve runtime ID for service %s", event.ServiceID)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := recovery.RecoveryRequest{
		RuntimeID:    runtimeID,
		FailureClass: recovery.FailureProcessCrash,
		TriggeredBy:  "process_exit_bridge",
		MaxAttempts:  3,
	}

	resp, err := b.recovery.ExecuteRecovery(ctx, req)
	if err != nil {
		log.Printf("[process-exit-bridge] recovery failed for runtime %s: %v", runtimeID, err)
		return
	}

	if resp.Success {
		log.Printf("[process-exit-bridge] recovery completed for runtime %s, stage=%s", runtimeID, resp.Stage)
	} else {
		log.Printf("[process-exit-bridge] recovery unsuccessful for runtime %s, stage=%s, err=%v", runtimeID, resp.Stage, resp.Error)
	}
}
