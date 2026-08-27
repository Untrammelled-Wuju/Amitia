package runtime

import (
	"context"
	"fmt"
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

type RuntimeGenerationLeaseRevoker interface {
	RevokeRuntimeGenerationLeases(runtimeID string, generation int64, reason string)
}

// RuntimeGenerationNetworkCloser releases host-owned mediated network handles
// for an exited service generation before recovery is allowed to create a new
// process. Keeping this boundary in runtime avoids coupling recovery to the
// concrete Host API implementation.
type RuntimeGenerationNetworkCloser interface {
	CloseRuntimeGenerationNetwork(runtimeID, moduleID string, generation int64) int
}

type processExitBridgeImpl struct {
	mu            sync.Mutex
	supervisor    *trusted_service.ProcessSupervisor
	recovery      *recovery.RecoveryCoordinator
	topologyStore *TopologyStore
	registry      *Manager
	registered    bool
	leaseRevoker  RuntimeGenerationLeaseRevoker
	networkCloser RuntimeGenerationNetworkCloser
}

func (b *processExitBridgeImpl) SetRuntimeGenerationLeaseRevoker(revoker RuntimeGenerationLeaseRevoker) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.leaseRevoker = revoker
}

func (b *processExitBridgeImpl) SetRuntimeGenerationNetworkCloser(closer RuntimeGenerationNetworkCloser) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.networkCloser = closer
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

	runtimeID := domain.RuntimeInstanceID(event.RuntimeID)
	if runtimeID == "" {
		resolved, ok := b.resolveRuntimeID(event.ServiceID)
		if !ok {
			log.Printf("[process-exit-bridge] could not resolve runtime ID for service %s (ProcessInstanceID=%s)", event.ServiceID, event.ProcessInstanceID)
			return
		}
		runtimeID = resolved
	}

	generation := event.Generation
	if generation == 0 {
		var genErr error
		generation, genErr = b.registry.GetCurrentGeneration(runtimeID)
		if genErr != nil {
			log.Printf("[process-exit-bridge] could not determine generation for runtime %s: %v", runtimeID, genErr)
		}
	}

	// Network handles are host-owned resources and must be released for every
	// unexpected exit before any branch suppresses recovery. A crashed process
	// must never leave a game-side socket alive merely because the runtime was
	// already stopping, emergency-latched, disabled, or uninstalling.
	b.mu.Lock()
	networkCloser := b.networkCloser
	revoker := b.leaseRevoker
	b.mu.Unlock()
	if networkCloser != nil {
		// generation==0 deliberately means fail-safe cleanup for the runtime/module
		// when legacy supervisor metadata cannot recover the exact generation.
		networkCloser.CloseRuntimeGenerationNetwork(string(runtimeID), event.ModuleID, generation)
	}
	if revoker != nil && generation > 0 {
		revoker.RevokeRuntimeGenerationLeases(string(runtimeID), generation, "unexpected process exit")
	}

	state, err := b.registry.GetRuntimeState(runtimeID)
	if err == nil && (state == domain.RuntimeStateRestarting || state == domain.RuntimeStateStopping || state == domain.RuntimeStateStopped) {
		return
	}

	if b.registry.IsEmergencyLatched(runtimeID) {
		log.Printf("[process-exit-bridge] recovery suppressed: runtime %s is emergency-latched", runtimeID)
		return
	}
	intent, err := b.registry.GetLifecycleIntent(runtimeID)
	if err == nil && (intent == "emergency" || intent == "disable" || intent == "uninstall") {
		log.Printf("[process-exit-bridge] recovery suppressed: runtime %s lifecycle intent=%q", runtimeID, intent)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := recovery.RecoveryRequest{
		RuntimeID:      runtimeID,
		ServiceID:      event.ServiceID,
		FailureClass:   recovery.FailureProcessCrash,
		TriggeredBy:    "process_exit_bridge",
		IdempotencyKey: string(runtimeID) + ":" + event.ServiceID + ":" + fmt.Sprintf("%d", generation),
		MaxAttempts:    3,
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
