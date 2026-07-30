// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package runtime

import (
	"context"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/contracts"
	"github.com/u-ai/backend/log"
)

type Service struct {
	config     *DesktopPetRuntimeConfig
	auth       *Auth
	registry   *RuntimeRegistry
	pending    *PendingTracker
	store      CommandStore
	state      StateStore
	dispatcher *Dispatcher
	snapshot   *SnapshotBuilder
	reconciler *Reconciler
	handler    *Handler
	metrics    *Metrics
	notifier   *RuntimeNotifierAdapter

	startOnce sync.Once
	stopOnce  sync.Once
	cancel    context.CancelFunc
	ctx       context.Context
}

func NewService(
	config *DesktopPetRuntimeConfig,
	store CommandStore,
	state StateStore,
	snapshot *SnapshotBuilder,
	eventSink RuntimeEventSink,
) *Service {
	config.EnsureToken()

	auth := NewAuth(config)
	registry := NewRuntimeRegistry()
	pending := NewPendingTracker()
	dispatcher := NewDispatcher(config, registry, pending, store)
	reconciler := NewReconciler(config, registry, pending, store, state, snapshot)
	metrics := NewMetrics()
	notifier := NewRuntimeNotifierAdapter(dispatcher, registry)

	svc := &Service{
		config:     config,
		auth:       auth,
		registry:   registry,
		pending:    pending,
		store:      store,
		state:      state,
		dispatcher: dispatcher,
		snapshot:   snapshot,
		reconciler: reconciler,
		handler:    NewHandler(config, auth, registry, pending, state, eventSink),
		metrics:    metrics,
		notifier:   notifier,
	}

	svc.handler.SetCallbacks(
		svc.handler.HandleRegister,
		svc.handler.HandleResult,
		svc.handler.HandleEvent,
		svc.handler.HandleHeartbeat,
	)

	return svc
}

func (s *Service) Start(ctx context.Context) error {
	var err error
	s.startOnce.Do(func() {
		s.ctx, s.cancel = context.WithCancel(ctx)
		log.Logger.Infof("runtime service: started path=%s backendInstanceID=%s", s.config.Path, s.config.BackendInstanceID)
		go s.reconciler.RunPeriodic(s.ctx, 10*time.Second)
		go s.runDispatchLoop(s.ctx)
	})
	return err
}

func (s *Service) Close(ctx context.Context) error {
	s.stopOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		s.pending.FailAll(
			contracts.ResultCancelled,
			ErrCodeBackendShuttingDown,
			"backend shutting down",
			ErrBackendShuttingDown,
		)
		s.registry.CloseAll(1001, "backend shutting down")
		log.Logger.Info("runtime service: closed")
	})
	return nil
}

func (s *Service) Dispatch(ctx context.Context, req CommandRequest) (DispatchReceipt, error) {
	return s.dispatcher.Dispatch(ctx, req)
}

func (s *Service) Reconcile(ctx context.Context, runtimeID string) error {
	conn := s.registry.GetByRuntime(runtimeID)
	if conn == nil {
		return NewRuntimeError(ErrCodeRuntimeOffline, "runtime not connected", ErrRuntimeOffline)
	}
	return s.reconciler.Reconcile(ctx, conn)
}

func (s *Service) GetRuntimeStatus(ctx context.Context, userID, runtimeID string) (*StatusView, error) {
	conn := s.registry.GetByRuntime(runtimeID)
	if conn == nil {
		return &StatusView{
			RuntimeID: runtimeID,
			Sync: &SyncView{Status: "offline", PendingCommands: 0},
		}, nil
	}

	view := &StatusView{
		RuntimeID: runtimeID,
		Connection: &ConnectionView{
			State:           string(conn.State()),
			SessionID:       conn.SessionID(),
			LastHeartbeatAt: conn.LastHeartbeat(),
			Capabilities:    conn.Capabilities().ToList(),
		},
	}

	pendingCount := s.pending.Count()
	view.Sync = &SyncView{
		Status:          string(conn.State()),
		PendingCommands: pendingCount,
	}

	states, _ := s.state.ListActualStatesByRuntime(runtimeID)
	if len(states) > 0 {
		st := states[0]
		view.Actual = &ActualView{
			InstallationID:  st.InstallationID,
			Revision:        st.DesiredRevision,
			Visible:         st.Visible != 0,
			CurrentActionKey: st.CurrentActionKey,
			Stale:           st.Health == "offline" || st.Health == "stale",
		}
		if st.ObservedAt != "" {
			if t, err := time.Parse(runtimeTimeFormat, st.ObservedAt); err == nil {
				view.Actual.ObservedAt = t
			}
		}
	}

	return view, nil
}

func (s *Service) ListRuntimeStatuses(ctx context.Context, userID string) ([]StatusView, error) {
	conns := s.registry.ListAll()
	views := make([]StatusView, 0, len(conns))
	for _, conn := range conns {
		view := StatusView{
			RuntimeID: conn.RuntimeID(),
			Connection: &ConnectionView{
				State:           string(conn.State()),
				SessionID:       conn.SessionID(),
				LastHeartbeatAt: conn.LastHeartbeat(),
				Capabilities:    conn.Capabilities().ToList(),
			},
		}
		views = append(views, view)
	}
	if len(views) == 0 {
		views = []StatusView{}
	}
	return views, nil
}

func (s *Service) GetMetrics() MetricsSnapshot {
	return s.metrics.Snapshot()
}

func (s *Service) Notifier() *RuntimeNotifierAdapter {
	return s.notifier
}

func (s *Service) Handler() *Handler {
	return s.handler
}

func (s *Service) Config() *DesktopPetRuntimeConfig {
	return s.config
}

func (s *Service) Registry() *RuntimeRegistry {
	return s.registry
}

func (s *Service) State() StateStore {
	return s.state
}

func (s *Service) Dispatcher() *Dispatcher {
	return s.dispatcher
}

func (s *Service) runDispatchLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count := s.dispatcher.ScanAndDispatch(ctx)
			if count > 0 {
				log.Logger.Debugf("runtime service: dispatched %d pending commands", count)
			}
			s.cleanupOldCommands()
		}
	}
}

func (s *Service) cleanupOldCommands() {
	cutoff := time.Now().Add(-time.Duration(s.config.CommandRetentionHours) * time.Hour).Format(runtimeTimeFormat)
	if n, err := s.store.DeleteCompletedBefore(cutoff); err != nil {
		log.Logger.Errorf("runtime service: cleanup old commands failed err=%v", err)
	} else if n > 0 {
		log.Logger.Infof("runtime service: cleaned up %d old commands", n)
	}
}
