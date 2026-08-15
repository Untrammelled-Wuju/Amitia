package runtime

import (
	"context"
	"fmt"
	"sync"

	"github.com/u-ai/backend/internal/gamehost/contracts"
	"github.com/u-ai/backend/log"
)

const (
	AMITIA_SECRET_LEASE_SESSION = "AMITIA_SECRET_LEASE_SESSION"
)

type EpochState struct {
	Epoch       uint64
	Generation  uint64
	ControlMode string
}

type FrameState struct {
	LastFrameSequence uint64
	LastFrameKey      string
	PendingFrames     int
}

func (e EpochState) IsFresh() bool {
	return e.Epoch == 0
}

func (e EpochState) IsCompatible(generation uint64) bool {
	return e.Generation == generation
}

type PetRuntimeLifecycle struct {
	service         *Service
	secretLeases    map[string]*contracts.RuntimeSecretLeaseSession
	epochStates     map[string]EpochState
	frameStates     map[string]FrameState
	mu              sync.RWMutex
	startupManifest map[string][]SecretLeaseManifestEntry
}

type SecretLeaseManifestEntry struct {
	Ref      string
	Purpose  string
	Required bool
}

func NewPetRuntimeLifecycle(service *Service) *PetRuntimeLifecycle {
	return &PetRuntimeLifecycle{
		service:         service,
		secretLeases:    make(map[string]*contracts.RuntimeSecretLeaseSession),
		epochStates:     make(map[string]EpochState),
		frameStates:     make(map[string]FrameState),
		startupManifest: make(map[string][]SecretLeaseManifestEntry),
	}
}

func (l *PetRuntimeLifecycle) RegisterStartupSecretManifest(runtimeID string, entries []SecretLeaseManifestEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.startupManifest[runtimeID] = entries
}

func (l *PetRuntimeLifecycle) GetStartupSecretManifest(runtimeID string) []SecretLeaseManifestEntry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.startupManifest[runtimeID]
}

func (l *PetRuntimeLifecycle) StoreLeaseSession(runtimeID string, session *contracts.RuntimeSecretLeaseSession) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if session != nil {
		l.secretLeases[runtimeID] = session
	}
}

func (l *PetRuntimeLifecycle) GetLeaseSession(runtimeID string) (*contracts.RuntimeSecretLeaseSession, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	sess, ok := l.secretLeases[runtimeID]
	return sess, ok
}

func (l *PetRuntimeLifecycle) RevokeLeaseSession(runtimeID string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.secretLeases, runtimeID)
}

func (l *PetRuntimeLifecycle) BuildLeaseEnv(runtimeID string) map[string]string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	env := make(map[string]string)
	if sess, ok := l.secretLeases[runtimeID]; ok && sess != nil {
		env[AMITIA_SECRET_LEASE_SESSION] = sess.SessionID
	}
	return env
}

func (l *PetRuntimeLifecycle) UpdateEpochState(runtimeID string, state EpochState) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.epochStates[runtimeID] = state
}

func (l *PetRuntimeLifecycle) GetEpochState(runtimeID string) (EpochState, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	s, ok := l.epochStates[runtimeID]
	return s, ok
}

func (l *PetRuntimeLifecycle) UpdateFrameState(runtimeID string, state FrameState) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.frameStates[runtimeID] = state
}

func (l *PetRuntimeLifecycle) GetFrameState(runtimeID string) (FrameState, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	s, ok := l.frameStates[runtimeID]
	return s, ok
}

func (l *PetRuntimeLifecycle) Start(ctx context.Context) error {
	if l.service == nil {
		return fmt.Errorf("pet runtime lifecycle: service is nil")
	}
	return l.service.Start(ctx)
}

func (l *PetRuntimeLifecycle) Close(ctx context.Context) error {
	if l.service == nil {
		return nil
	}
	l.mu.Lock()
	l.secretLeases = make(map[string]*contracts.RuntimeSecretLeaseSession)
	l.mu.Unlock()
	return l.service.Close(ctx)
}

func (l *PetRuntimeLifecycle) Service() *Service {
	return l.service
}

func (l *PetRuntimeLifecycle) InitEpoch(runtimeID string, generation uint64) EpochState {
	state := EpochState{
		Epoch:      1,
		Generation: generation,
	}
	l.UpdateEpochState(runtimeID, state)
	log.Logger.Infof("pet runtime lifecycle: epoch initialized runtimeID=%s generation=%d", runtimeID, generation)
	return state
}

func (l *PetRuntimeLifecycle) AdvanceEpoch(runtimeID string) (EpochState, error) {
	state, ok := l.GetEpochState(runtimeID)
	if !ok {
		return EpochState{}, fmt.Errorf("pet runtime lifecycle: no epoch state for runtime %s", runtimeID)
	}
	state.Epoch++
	l.UpdateEpochState(runtimeID, state)
	return state, nil
}
