package execution

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type IdempotencyState string

const (
	IdempotencyStateReserved      IdempotencyState = "reserved"
	IdempotencyStateIndeterminate IdempotencyState = "indeterminate"
	IdempotencyStateReleased      IdempotencyState = "released"
	IdempotencyStateDone          IdempotencyState = "done"
)

type RecordState string

type IdempotencyIdentity struct {
	ToolID         string
	Generation     int64
	UserID         string
	CharacterID    string
	ConversationID string
	Source         capability.InvocationSource
	CallerKey      string
}

type IdempotencyReservation struct {
	IdempotencyKey      string
	RequestFingerprint  string
	PreviousOwner       string
	OwnerInstanceID     string
	State               IdempotencyState
	SafeToReplay        bool
	PriorWorkResultJSON json.RawMessage
	WorkResultJSON      json.RawMessage
	NextOwner           string
	Reservation         json.RawMessage
	Cause               error
}

type IdempotencyRecord struct {
	IdempotencyKey     string
	RequestFingerprint string
	State              IdempotencyState
	WorkResultJSON     json.RawMessage
	OwnerInstanceID    string
	SafeToReplay       bool
	Revision           int64
	Reservation        json.RawMessage
	CreatedAt          time.Time
	UpdatedAt          time.Time
	ExpiresAt          time.Time
	ReleasedAt         *time.Time
}

type IdempotencyFlight struct {
	reserved  string
	done      chan struct{}
	createdAt time.Time
}

type IdempotencyStorage interface {
	Reserve(ctx context.Context, rec IdempotencyReservation) error
	Find(ctx context.Context, key string) (*IdempotencyRecord, error)
	Complete(ctx context.Context, key string, result json.RawMessage) (bool, error)
	MarkIndeterminate(ctx context.Context, key string) (bool, error)
	Release(ctx context.Context, key string) (bool, error)
	DeleteExpiredCAS(ctx context.Context, now time.Time) (int64, error)
}

type IdempotencyGuard struct {
	storage IdempotencyStorage
	nodeID  string

	mu      sync.Mutex
	flights map[string]*IdempotencyFlight

	hooks               IdempotencyHook
	now                 func() time.Time
	retentionWindow     time.Duration
	staleOwnerTTE       time.Duration
	cleanupIntervalNano int64
	takeoverGraceNano   int64

	cleanupStop chan struct{}
}

type IdempotencyHook interface {
	OnIdempotencyBegin(ctx context.Context, key string)
	OnIdempotencyCacheHit(ctx context.Context, key string)
	OnIdempotencyConflict(ctx context.Context, key string, prevID string, prevState IdempotencyState)
	OnIdempotencySingleFlightJoin(ctx context.Context, key string)
	OnIdempotencyComplete(ctx context.Context, key string, err error)
	OnIdempotencyIndeterminate(ctx context.Context, key string)
	OnIdempotencyReleased(ctx context.Context, key string)
}

var _ IdempotencyHook = (*NoopIdempotencyHook)(nil)

type NoopIdempotencyHook struct{}

func (NoopIdempotencyHook) OnIdempotencyBegin(ctx context.Context, key string)    {}
func (NoopIdempotencyHook) OnIdempotencyCacheHit(ctx context.Context, key string) {}
func (NoopIdempotencyHook) OnIdempotencyConflict(ctx context.Context, key string, prevID string, prevState IdempotencyState) {
}
func (NoopIdempotencyHook) OnIdempotencySingleFlightJoin(ctx context.Context, key string)    {}
func (NoopIdempotencyHook) OnIdempotencyComplete(ctx context.Context, key string, err error) {}
func (NoopIdempotencyHook) OnIdempotencyIndeterminate(ctx context.Context, key string)       {}
func (NoopIdempotencyHook) OnIdempotencyReleased(ctx context.Context, key string)            {}

var (
	ErrIdempotencyTakeoverForbidden = errors.New("idempotency: prior reservation present and takeover forbidden")
	ErrIdempotencyIndeterminate     = errors.New("idempotency: prior reservation indeterminate, caller must retry")
)

const (
	defaultRetentionWindow = 24 * time.Hour
	defaultOwnerTTE        = 5 * time.Minute
	defaultCleanupInterval = 1 * time.Minute
	defaultTakeoverGrace   = 30 * time.Second
)

func NewIdempotencyGuard(storage IdempotencyStorage) *IdempotencyGuard {
	g := &IdempotencyGuard{
		storage:             storage,
		nodeID:              uuid.NewString(),
		flights:             make(map[string]*IdempotencyFlight),
		hooks:               NoopIdempotencyHook{},
		now:                 func() time.Time { return time.Now().UTC() },
		retentionWindow:     defaultRetentionWindow,
		staleOwnerTTE:       defaultOwnerTTE,
		cleanupIntervalNano: int64(defaultCleanupInterval),
		takeoverGraceNano:   int64(defaultTakeoverGrace),
		cleanupStop:         make(chan struct{}),
	}
	go g.cleanupLoop()
	return g
}

func (g *IdempotencyGuard) SetHooks(h IdempotencyHook) {
	if h == nil {
		h = NoopIdempotencyHook{}
	}
	g.hooks = h
}

func (g *IdempotencyGuard) key(identity IdempotencyIdentity) string {
	return BuildIdempotencyKeySHA(identity)
}

func (g *IdempotencyGuard) Begin(ctx context.Context, identity IdempotencyIdentity, fingerprint string) (IdempotencyReservation, bool, error) {
	key := g.key(identity)
	g.hooks.OnIdempotencyBegin(ctx, key)

	g.mu.Lock()
	if f, ok := g.flights[key]; ok {
		g.mu.Unlock()
		g.hooks.OnIdempotencySingleFlightJoin(ctx, key)
		<-f.done
		rec, findErr := g.storage.Find(ctx, key)
		if findErr != nil || rec == nil {
			return IdempotencyReservation{IdempotencyKey: key, Cause: findErr}, false, findErr
		}
		if rec.State == IdempotencyStateDone {
			return IdempotencyReservation{
				IdempotencyKey:      key,
				OwnerInstanceID:     rec.OwnerInstanceID,
				State:               IdempotencyStateDone,
				PriorWorkResultJSON: rec.WorkResultJSON,
				NextOwner:           rec.OwnerInstanceID,
			}, true, nil
		}
		return IdempotencyReservation{IdempotencyKey: key, Cause: ErrIdempotencyIndeterminate}, false, ErrIdempotencyIndeterminate
	}
	g.mu.Unlock()

	rec, findErr := g.storage.Find(ctx, key)
	now := g.now()

	if findErr != nil && !errors.Is(findErr, sql.ErrNoRows) {
		return IdempotencyReservation{IdempotencyKey: key, Cause: findErr}, false, findErr
	}

	if rec != nil && !rec.ExpiresAt.IsZero() && rec.ExpiresAt.After(now) {
		switch rec.State {
		case IdempotencyStateDone:
			g.hooks.OnIdempotencyCacheHit(ctx, key)
			return IdempotencyReservation{
				IdempotencyKey:      key,
				OwnerInstanceID:     rec.OwnerInstanceID,
				State:               IdempotencyStateDone,
				PriorWorkResultJSON: rec.WorkResultJSON,
				NextOwner:           rec.OwnerInstanceID,
				Reservation:         rec.Reservation,
			}, true, nil
		case IdempotencyStateIndeterminate:
			g.hooks.OnIdempotencyConflict(ctx, key, rec.OwnerInstanceID, rec.State)
			return IdempotencyReservation{
				IdempotencyKey:  key,
				PreviousOwner:   rec.OwnerInstanceID,
				OwnerInstanceID: rec.OwnerInstanceID,
				State:           IdempotencyStateIndeterminate,
				Cause:           ErrIdempotencyIndeterminate,
			}, false, ErrIdempotencyIndeterminate
		case IdempotencyStateReserved:
			if rec.OwnerInstanceID == g.nodeID {
				g.mu.Lock()
				g.flights[key] = &IdempotencyFlight{reserved: key, done: make(chan struct{}), createdAt: now}
				g.mu.Unlock()
				return IdempotencyReservation{
					IdempotencyKey:  key,
					OwnerInstanceID: g.nodeID,
					State:           IdempotencyStateReserved,
					Reservation:     rec.Reservation,
				}, false, nil
			}
			g.hooks.OnIdempotencyConflict(ctx, key, rec.OwnerInstanceID, rec.State)
			if rec.SafeToReplay {
				elapsed := now.Sub(rec.UpdatedAt)
				if g.canTakeoverLocked(rec, elapsed) {
					if _, delErr := g.storage.Release(ctx, key); delErr != nil {
						return IdempotencyReservation{IdempotencyKey: key, Cause: delErr}, false, delErr
					}
				} else {
					return IdempotencyReservation{
						IdempotencyKey:  key,
						PreviousOwner:   rec.OwnerInstanceID,
						OwnerInstanceID: rec.OwnerInstanceID,
						State:           IdempotencyStateReserved,
						Cause:           ErrIdempotencyTakeoverForbidden,
					}, false, ErrIdempotencyTakeoverForbidden
				}
			} else {
				return IdempotencyReservation{
					IdempotencyKey:  key,
					PreviousOwner:   rec.OwnerInstanceID,
					OwnerInstanceID: rec.OwnerInstanceID,
					State:           IdempotencyStateReserved,
					Cause:           ErrIdempotencyTakeoverForbidden,
				}, false, ErrIdempotencyTakeoverForbidden
			}
		}
	}

	newRec := IdempotencyReservation{
		IdempotencyKey:     key,
		RequestFingerprint: fingerprint,
		OwnerInstanceID:    g.nodeID,
		State:              IdempotencyStateReserved,
		SafeToReplay:       true,
	}
	resBytes, _ := json.Marshal(newRec)
	newRec.Reservation = resBytes

	if err := g.storage.Reserve(ctx, newRec); err != nil {
		return IdempotencyReservation{IdempotencyKey: key, Cause: err}, false, err
	}

	g.mu.Lock()
	g.flights[key] = &IdempotencyFlight{reserved: key, done: make(chan struct{}), createdAt: now}
	g.mu.Unlock()

	return IdempotencyReservation{
		IdempotencyKey:  key,
		OwnerInstanceID: g.nodeID,
		State:           IdempotencyStateReserved,
		Reservation:     resBytes,
	}, false, nil
}

func (g *IdempotencyGuard) canTakeoverLocked(rec *IdempotencyRecord, elapsed time.Duration) bool {
	if rec.OwnerInstanceID == g.nodeID {
		return true
	}
	if elapsed < g.staleOwnerTTE {
		return false
	}
	return elapsed >= time.Duration(g.takeoverGraceNano)
}

func (g *IdempotencyGuard) Complete(ctx context.Context, res IdempotencyReservation, workResult *capability.UnifiedToolResult) error {
	key := res.IdempotencyKey
	var resultJSON json.RawMessage
	if workResult != nil {
		data, err := json.Marshal(workResult)
		if err != nil {
			return err
		}
		resultJSON = data
	}

	var opErr error
	ok, err := g.storage.Complete(ctx, key, resultJSON)
	if err != nil {
		opErr = err
	} else if !ok {
		opErr = errors.New("idempotency: complete no-op (reservation released)")
	}

	g.signalFlight(key)
	g.hooks.OnIdempotencyComplete(ctx, key, opErr)
	return opErr
}

func (g *IdempotencyGuard) MarkIndeterminate(ctx context.Context, key string) (bool, error) {
	ok, err := g.storage.MarkIndeterminate(ctx, key)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, errors.New("idempotency: markIndeterminate no-op")
	}
	g.signalFlight(key)
	g.hooks.OnIdempotencyIndeterminate(ctx, key)
	return true, nil
}

func (g *IdempotencyGuard) Release(ctx context.Context, key string) (bool, error) {
	ok, err := g.storage.Release(ctx, key)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	g.signalFlight(key)
	g.hooks.OnIdempotencyReleased(ctx, key)
	return true, nil
}

func (g *IdempotencyGuard) signalFlight(key string) {
	g.mu.Lock()
	f, ok := g.flights[key]
	if ok {
		delete(g.flights, key)
	}
	g.mu.Unlock()
	if ok {
		close(f.done)
	}
}

func (g *IdempotencyGuard) InstanceID() string {
	return g.nodeID
}

func (g *IdempotencyGuard) stopCleanup() {
	select {
	case <-g.cleanupStop:
	default:
		close(g.cleanupStop)
	}
}

func (g *IdempotencyGuard) cleanupLoop() {
	t := time.NewTicker(defaultCleanupInterval)
	defer t.Stop()
	for {
		select {
		case <-g.cleanupStop:
			return
		case <-t.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_, _ = g.storage.DeleteExpiredCAS(ctx, g.now())
			cancel()
		}
	}
}
