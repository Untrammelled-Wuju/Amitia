package deviceruntime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

const defaultSessionTTL = 24 * time.Hour

type SessionIDFactory func() runtimeidentity.RuntimeSessionID

func DefaultSessionIDFactory() runtimeidentity.RuntimeSessionID {
	return runtimeidentity.RuntimeSessionID("rtsess_" + uuid.NewString())
}

type AcquireRequest struct {
	Identity protocol.SessionIdentity

	Platform runtimeidentity.Platform

	RuntimeVersion         string
	RuntimeContractVersion string

	Capabilities []string

	Cursor protocol.SessionCursor

	Now time.Time
}

type AcquireResult struct {
	Session RuntimeSession
	Resume  protocol.ResumeDecision
}

type Service struct {
	store      SessionStore
	presence   protocol.SessionLifecyclePort
	idFactory  SessionIDFactory
	sessionTTL time.Duration
	events     SessionEventPublisher

	mu sync.Mutex
}

type ServiceOptions struct {
	PresencePort     protocol.SessionLifecyclePort
	SessionIDFactory SessionIDFactory
	SessionTTL       time.Duration
	Events           SessionEventPublisher
}

func NewService(store SessionStore, opts ServiceOptions) (*Service, error) {
	if store == nil {
		return nil, errors.New("deviceruntime: store is required")
	}
	s := &Service{
		store:      store,
		presence:   opts.PresencePort,
		idFactory:  opts.SessionIDFactory,
		sessionTTL: opts.SessionTTL,
		events:     opts.Events,
	}
	if s.presence == nil {
		s.presence = NoopPresencePort{}
	}
	if s.idFactory == nil {
		s.idFactory = DefaultSessionIDFactory
	}
	if s.sessionTTL <= 0 {
		s.sessionTTL = defaultSessionTTL
	}
	return s, nil
}

func (s *Service) normalizeTime(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now().UTC()
	}
	return now.UTC()
}

func (s *Service) txEnabled() (SessionUnitOfWork, bool) {
	if s.events == nil {
		return nil, false
	}
	uow, ok := s.store.(SessionUnitOfWork)
	return uow, ok
}

func (s *Service) Acquire(ctx context.Context, req AcquireRequest) (AcquireResult, error) {
	now := s.normalizeTime(req.Now)

	if req.Identity.UserID == "" || req.Identity.DeviceID == "" || req.Identity.RuntimeID == "" {
		return AcquireResult{}, ErrRuntimeSessionInvalid
	}

	caps := protocol.NormalizeCapabilities(req.Capabilities)
	capHash := protocol.ComputeCapabilitiesHash(req.Capabilities)

	s.mu.Lock()
	defer s.mu.Unlock()

	uow, txOn := s.txEnabled()

	existing, err := s.store.GetActiveByRuntime(ctx, req.Identity.UserID, req.Identity.DeviceID, req.Identity.RuntimeID)
	if err != nil && !errors.Is(err, ErrRuntimeSessionNotFound) {
		return AcquireResult{}, err
	}

	if err == nil && !existing.IsExpiredAt(now) {
		return s.reconnectExisting(ctx, existing, req, now, caps, capHash, uow, txOn)
	}

	if err == nil && existing.IsExpiredAt(now) {
		prev := existing
		if txOn {
			if closeErr := uow.WithinTx(ctx, func(tx SessionTx) error {
				if err := tx.Close(ctx, prev.ID, prev.ConnectionGeneration, "session_expired", now); err != nil {
					return err
				}
				prev.Revision++
				prev.Status = protocol.SessionStatusClosed
				prev.CloseReason = "session_expired"
				return s.events.PublishTx(ctx, tx.RawTx(), SessionDomainEvent{
					Type:       SessionEventExpired,
					Session:    prev,
					Reason:     "session_expired",
					OccurredAt: now,
				})
			}); closeErr != nil {
				return AcquireResult{}, closeErr
			}
		} else {
			if closeErr := s.store.Close(ctx, prev.ID, prev.ConnectionGeneration, "session_expired", now); closeErr != nil {
				return AcquireResult{}, closeErr
			}
		}
		if presenceErr := s.presence.SessionDisconnected(ctx, PresenceSnapshot{
			UserID:               prev.UserID,
			DeviceID:             prev.DeviceID,
			RuntimeID:            prev.RuntimeID,
			RuntimeSessionID:     prev.ID,
			Platform:             prev.Platform,
			ConnectionGeneration: prev.ConnectionGeneration,
			At:                   now,
		}, "session_expired"); presenceErr != nil {
			return AcquireResult{}, fmt.Errorf("%w: %v", ErrPresenceProjectionFailed, presenceErr)
		}
	}

	return s.createNewSession(ctx, req, now, caps, capHash, uow, txOn)
}

func (s *Service) reconnectExisting(
	ctx context.Context,
	existing RuntimeSession,
	req AcquireRequest,
	now time.Time,
	caps []string,
	capHash string,
	uow SessionUnitOfWork,
	txOn bool,
) (AcquireResult, error) {
	newGen := existing.ConnectionGeneration + 1

	clientCursor := req.Cursor
	mergedCursor := existing.ResumeCursor()

	if clientCursor.LastAppliedStateRevision > 0 && clientCursor.LastAppliedStateRevision <= existing.LastAppliedStateRevision {
		mergedCursor.LastAppliedStateRevision = max64(mergedCursor.LastAppliedStateRevision, clientCursor.LastAppliedStateRevision)
	} else if clientCursor.LastAppliedStateRevision > existing.LastAppliedStateRevision {
		return AcquireResult{
			Session: existing,
			Resume: protocol.ResumeDecision{
				Mode:   protocol.ResumeModeFull,
				Reason: "client_cursor_ahead",
			},
		}, nil
	}
	if clientCursor.LastProcessedCommandSequence > 0 && clientCursor.LastProcessedCommandSequence <= existing.LastProcessedCommandSequence {
		mergedCursor.LastProcessedCommandSequence = max64(mergedCursor.LastProcessedCommandSequence, clientCursor.LastProcessedCommandSequence)
	} else if clientCursor.LastProcessedCommandSequence > existing.LastProcessedCommandSequence {
		return AcquireResult{
			Session: existing,
			Resume: protocol.ResumeDecision{
				Mode:   protocol.ResumeModeFull,
				Reason: "client_cursor_ahead",
			},
		}, nil
	}
	if clientCursor.LastEventSequence > 0 && clientCursor.LastEventSequence <= existing.LastEventSequence {
		mergedCursor.LastEventSequence = max64(mergedCursor.LastEventSequence, clientCursor.LastEventSequence)
	} else if clientCursor.LastEventSequence > existing.LastEventSequence {
		return AcquireResult{
			Session: existing,
			Resume: protocol.ResumeDecision{
				Mode:   protocol.ResumeModeFull,
				Reason: "client_cursor_ahead",
			},
		}, nil
	}
	if clientCursor.ActualStateHash != "" {
		mergedCursor.ActualStateHash = clientCursor.ActualStateHash
	}

	updated := existing
	updated.ConnectionGeneration = newGen
	updated.Revision = existing.Revision + 1
	updated.RuntimeVersion = req.RuntimeVersion
	updated.RuntimeContractVersion = req.RuntimeContractVersion
	updated.Capabilities = caps
	updated.CapabilitiesHash = capHash
	updated.LastAppliedStateRevision = mergedCursor.LastAppliedStateRevision
	updated.LastProcessedCommandSequence = mergedCursor.LastProcessedCommandSequence
	updated.LastEventSequence = mergedCursor.LastEventSequence
	updated.ActualStateHash = mergedCursor.ActualStateHash
	updated.Status = protocol.SessionStatusSyncing
	updated.LastHeartbeatAt = now
	updated.ExpiresAt = now.Add(s.sessionTTL)
	updated.UpdatedAt = now

	if txOn {
		if err := uow.WithinTx(ctx, func(tx SessionTx) error {
			if err := tx.ReplaceForReconnect(ctx, existing.ConnectionGeneration, updated); err != nil {
				return err
			}
			return s.events.PublishTx(ctx, tx.RawTx(), SessionDomainEvent{
				Type:       SessionEventAcquired,
				Session:    updated,
				Reconnect:  true,
				OccurredAt: now,
			})
		}); err != nil {
			if errors.Is(err, ErrConnectionSuperseded) {
				return AcquireResult{}, err
			}
			return AcquireResult{}, err
		}
	} else {
		if err := s.store.ReplaceForReconnect(ctx, existing.ConnectionGeneration, updated); err != nil {
			if errors.Is(err, ErrConnectionSuperseded) {
				return AcquireResult{}, err
			}
			return AcquireResult{}, err
		}
	}

	return AcquireResult{
		Session: updated,
		Resume: protocol.ResumeDecision{
			Mode: protocol.ResumeModeResumeOrFull,
		},
	}, nil
}

func (s *Service) createNewSession(
	ctx context.Context,
	req AcquireRequest,
	now time.Time,
	caps []string,
	capHash string,
	uow SessionUnitOfWork,
	txOn bool,
) (AcquireResult, error) {
	sessionID := s.idFactory()

	cursor := req.Cursor
	if cursor.LastAppliedStateRevision < 0 {
		cursor.LastAppliedStateRevision = 0
	}
	if cursor.LastProcessedCommandSequence < 0 {
		cursor.LastProcessedCommandSequence = 0
	}
	if cursor.LastEventSequence < 0 {
		cursor.LastEventSequence = 0
	}

	session := RuntimeSession{
		ID:                           sessionID,
		UserID:                       req.Identity.UserID,
		DeviceID:                     req.Identity.DeviceID,
		RuntimeID:                    req.Identity.RuntimeID,
		Platform:                     req.Platform,
		Status:                       protocol.SessionStatusRegistering,
		ConnectionGeneration:         1,
		Revision:                     1,
		RuntimeVersion:               req.RuntimeVersion,
		RuntimeContractVersion:       req.RuntimeContractVersion,
		Capabilities:                 caps,
		CapabilitiesHash:             capHash,
		LastAppliedStateRevision:     cursor.LastAppliedStateRevision,
		LastProcessedCommandSequence: cursor.LastProcessedCommandSequence,
		LastEventSequence:            cursor.LastEventSequence,
		ActualStateHash:              cursor.ActualStateHash,
		CreatedAt:                    now,
		UpdatedAt:                    now,
		LastHeartbeatAt:              now,
		ExpiresAt:                    now.Add(s.sessionTTL),
	}

	if txOn {
		if err := uow.WithinTx(ctx, func(tx SessionTx) error {
			if err := tx.Create(ctx, session); err != nil {
				return err
			}
			return s.events.PublishTx(ctx, tx.RawTx(), SessionDomainEvent{
				Type:       SessionEventAcquired,
				Session:    session,
				Reconnect:  false,
				OccurredAt: now,
			})
		}); err != nil {
			return AcquireResult{}, err
		}
	} else {
		if err := s.store.Create(ctx, session); err != nil {
			return AcquireResult{}, err
		}
	}

	return AcquireResult{
		Session: session,
		Resume: protocol.ResumeDecision{
			Mode: protocol.ResumeModeFresh,
		},
	}, nil
}

func (s *Service) MarkReady(
	ctx context.Context,
	sessionID runtimeidentity.RuntimeSessionID,
	generation int64,
	at time.Time,
) (RuntimeSession, error) {
	now := s.normalizeTime(at)

	s.mu.Lock()
	defer s.mu.Unlock()

	uow, txOn := s.txEnabled()

	session, err := s.store.Get(ctx, sessionID)
	if err != nil {
		return RuntimeSession{}, err
	}

	if session.ConnectionGeneration != generation {
		return RuntimeSession{}, ErrConnectionSuperseded
	}

	if !session.IsActive() {
		return RuntimeSession{}, ErrRuntimeSessionStale
	}

	session.Status = protocol.SessionStatusReady
	session.LastHeartbeatAt = now
	session.ExpiresAt = now.Add(s.sessionTTL)
	session.UpdatedAt = now
	session.Revision++

	if txOn {
		if err := uow.WithinTx(ctx, func(tx SessionTx) error {
			if err := tx.UpdateStatus(ctx, sessionID, generation, protocol.SessionStatusReady, now); err != nil {
				return err
			}
			return s.events.PublishTx(ctx, tx.RawTx(), SessionDomainEvent{
				Type:       SessionEventReady,
				Session:    session,
				OccurredAt: now,
			})
		}); err != nil {
			return RuntimeSession{}, err
		}
	} else {
		if err := s.store.UpdateStatus(ctx, sessionID, generation, protocol.SessionStatusReady, now); err != nil {
			return RuntimeSession{}, err
		}
	}

	presenceErr := s.presence.SessionReady(ctx, PresenceSnapshot{
		UserID:               session.UserID,
		DeviceID:             session.DeviceID,
		RuntimeID:            session.RuntimeID,
		RuntimeSessionID:     session.ID,
		Platform:             session.Platform,
		ConnectionGeneration: session.ConnectionGeneration,
		At:                   now,
	})
	if presenceErr != nil {
		return session, fmt.Errorf("%w: %v", ErrPresenceProjectionFailed, presenceErr)
	}

	return session, nil
}

func (s *Service) Heartbeat(
	ctx context.Context,
	sessionID runtimeidentity.RuntimeSessionID,
	generation int64,
	at time.Time,
) (RuntimeSession, error) {
	now := s.normalizeTime(at)

	s.mu.Lock()
	defer s.mu.Unlock()

	session, err := s.store.Get(ctx, sessionID)
	if err != nil {
		return RuntimeSession{}, err
	}

	if session.ConnectionGeneration != generation {
		return RuntimeSession{}, ErrConnectionSuperseded
	}

	if !session.IsActive() {
		return RuntimeSession{}, ErrRuntimeSessionStale
	}

	expiresAt := now.Add(s.sessionTTL)

	if err := s.store.UpdateHeartbeat(ctx, sessionID, generation, now, expiresAt); err != nil {
		return RuntimeSession{}, err
	}

	session.LastHeartbeatAt = now
	session.ExpiresAt = expiresAt
	session.UpdatedAt = now

	return session, nil
}

func (s *Service) Close(
	ctx context.Context,
	sessionID runtimeidentity.RuntimeSessionID,
	generation int64,
	reason string,
	at time.Time,
) error {
	now := s.normalizeTime(at)

	s.mu.Lock()
	defer s.mu.Unlock()

	uow, txOn := s.txEnabled()

	session, err := s.store.Get(ctx, sessionID)
	if err != nil {
		return err
	}

	if session.ConnectionGeneration != generation {
		return ErrConnectionSuperseded
	}

	if session.Status.IsTerminal() {
		return nil
	}

	if txOn {
		if err := uow.WithinTx(ctx, func(tx SessionTx) error {
			if err := tx.Close(ctx, sessionID, generation, reason, now); err != nil {
				return err
			}
			session.Status = protocol.SessionStatusClosed
			session.CloseReason = reason
			session.Revision++
			closedAt := now
			session.ClosedAt = &closedAt
			session.UpdatedAt = now
			return s.events.PublishTx(ctx, tx.RawTx(), SessionDomainEvent{
				Type:       SessionEventClosed,
				Session:    session,
				Reason:     reason,
				OccurredAt: now,
			})
		}); err != nil {
			return err
		}
	} else {
		if err := s.store.Close(ctx, sessionID, generation, reason, now); err != nil {
			return err
		}
	}

	presenceErr := s.presence.SessionDisconnected(ctx, PresenceSnapshot{
		UserID:               session.UserID,
		DeviceID:             session.DeviceID,
		RuntimeID:            session.RuntimeID,
		RuntimeSessionID:     session.ID,
		Platform:             session.Platform,
		ConnectionGeneration: session.ConnectionGeneration,
		At:                   now,
	}, reason)
	if presenceErr != nil {
		return fmt.Errorf("%w: %v", ErrPresenceProjectionFailed, presenceErr)
	}

	return nil
}

func (s *Service) UpdateCursor(
	ctx context.Context,
	sessionID runtimeidentity.RuntimeSessionID,
	generation int64,
	cursor protocol.SessionCursor,
	at time.Time,
) error {
	now := s.normalizeTime(at)

	s.mu.Lock()
	defer s.mu.Unlock()

	session, err := s.store.Get(ctx, sessionID)
	if err != nil {
		return err
	}

	if session.ConnectionGeneration != generation {
		return ErrConnectionSuperseded
	}

	if !session.IsActive() {
		return ErrRuntimeSessionStale
	}

	if cursor.LastAppliedStateRevision < session.LastAppliedStateRevision {
		return ErrRuntimeCursorStale
	}
	if cursor.LastProcessedCommandSequence < session.LastProcessedCommandSequence {
		return ErrRuntimeCursorStale
	}
	if cursor.LastEventSequence < session.LastEventSequence {
		return ErrRuntimeCursorStale
	}

	if err := s.store.UpdateCursor(ctx, sessionID, generation, cursor, now); err != nil {
		return err
	}

	return nil
}

func (s *Service) GetSession(
	ctx context.Context,
	sessionID runtimeidentity.RuntimeSessionID,
) (RuntimeSession, error) {
	return s.store.Get(ctx, sessionID)
}

func (s *Service) GetActiveSession(
	ctx context.Context,
	userID runtimeidentity.UserID,
	deviceID runtimeidentity.DeviceID,
	runtimeID runtimeidentity.RuntimeID,
) (RuntimeSession, error) {
	return s.store.GetActiveByRuntime(ctx, userID, deviceID, runtimeID)
}

func (s *Service) RecoverStartup(ctx context.Context, at time.Time) error {
	now := s.normalizeTime(at)
	return s.store.CloseActiveOnStartup(ctx, now, "core_restart")
}

func (s *Service) CleanupExpiredSessions(ctx context.Context, at time.Time) error {
	now := s.normalizeTime(at)
	s.mu.Lock()
	defer s.mu.Unlock()

	sessions, err := s.store.ListActive(ctx)
	if err != nil {
		return err
	}

	for _, session := range sessions {
		if session.IsExpiredAt(now) {
			if err := s.store.Close(ctx, session.ID, session.ConnectionGeneration, "session_expired", now); err != nil {
				return err
			}
		}
	}

	return nil
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
