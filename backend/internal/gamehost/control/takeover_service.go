package control

import (
	"context"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

const (
	DefaultReleaseTarget   domain.ControlMode = domain.ControlModeObserveOnly
	MaxTakeoverRetry       int                = 3
	TakeoverRetryBaseDelay time.Duration      = 5 * time.Millisecond
)

type TakeoverService struct {
	manager       *ControlAuthorityManager
	contextStore  *TakeoverContextStore
	runtimeReader RuntimeReader
	permChecker   PermissionChecker
	policyChecker HostPolicyChecker
	clock         Clock
	audit         AuthorityAuditSink

	opMu       sync.Mutex
	perRuntime map[domain.RuntimeInstanceID]*sync.Mutex
}

type TakeoverServiceOptions struct {
	Manager       *ControlAuthorityManager
	ContextStore  *TakeoverContextStore
	RuntimeReader RuntimeReader
	PermChecker   PermissionChecker
	PolicyChecker HostPolicyChecker
	Clock         Clock
	Audit         AuthorityAuditSink
}

func NewTakeoverService(opts TakeoverServiceOptions) *TakeoverService {
	manager := opts.Manager
	var store *TakeoverContextStore
	if opts.ContextStore != nil {
		store = opts.ContextStore
	} else {
		store = NewTakeoverContextStore()
	}
	rtReader := opts.RuntimeReader
	pChecker := opts.PermChecker
	hChecker := opts.PolicyChecker
	clock := opts.Clock
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	audit := opts.Audit
	if audit == nil {
		audit = NewInMemoryAuthorityAuditSink()
	}
	return &TakeoverService{
		manager:       manager,
		contextStore:  store,
		runtimeReader: rtReader,
		permChecker:   pChecker,
		policyChecker: hChecker,
		clock:         clock,
		audit:         audit,
		perRuntime:    make(map[domain.RuntimeInstanceID]*sync.Mutex),
	}
}

func (s *TakeoverService) getOpLock(runtimeID domain.RuntimeInstanceID) *sync.Mutex {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	lock, ok := s.perRuntime[runtimeID]
	if !ok {
		lock = &sync.Mutex{}
		s.perRuntime[runtimeID] = lock
	}
	return lock
}

type TakeoverRequest struct {
	RuntimeID     domain.RuntimeInstanceID
	PluginID      domain.PluginID
	Actor         TransitionActor
	ExpectedEpoch *uint64
}

type ReleaseRequest struct {
	RuntimeID     domain.RuntimeInstanceID
	PluginID      domain.PluginID
	TargetMode    domain.ControlMode
	Actor         TransitionActor
	ExpectedEpoch uint64
	UseExpected   bool
}

type TakeoverResult struct {
	PreviousMode  domain.ControlMode
	NewMode       domain.ControlMode
	PreviousEpoch uint64
	NewEpoch      uint64
	Snapshot      ControlAuthoritySnapshot
}

type ReleaseResult struct {
	PreviousMode  domain.ControlMode
	NewMode       domain.ControlMode
	PreviousEpoch uint64
	NewEpoch      uint64
	Snapshot      ControlAuthoritySnapshot
}

func (s *TakeoverService) Takeover(ctx context.Context, req TakeoverRequest) (TakeoverResult, error) {
	if err := ctx.Err(); err != nil {
		return TakeoverResult{}, err
	}
	if req.RuntimeID == "" {
		return TakeoverResult{}, &AuthorityError{
			Code:    domain.ErrInvalidArgument,
			Message: "runtime id must not be empty",
		}
	}
	if req.Actor == "" {
		return TakeoverResult{}, &AuthorityError{
			Code:    domain.ErrInvalidArgument,
			Message: "actor must not be empty",
		}
	}

	lock := s.getOpLock(req.RuntimeID)
	lock.Lock()
	defer lock.Unlock()

	current, err := s.manager.Get(ctx, req.RuntimeID)
	if err != nil {
		return TakeoverResult{}, err
	}

	previousMode := current.Mode
	previousEpoch := current.Epoch

	if previousMode == domain.ControlModeUserControl {
		return TakeoverResult{
			PreviousMode:  previousMode,
			NewMode:       previousMode,
			PreviousEpoch: previousEpoch,
			NewEpoch:      previousEpoch,
			Snapshot:      current,
		}, nil
	}

	if req.ExpectedEpoch != nil && *req.ExpectedEpoch != previousEpoch {
		s.recordTakeoverAudit(req, previousMode, previousMode, previousEpoch, previousEpoch, AuditResultDenied, "stale epoch")
		return TakeoverResult{}, errStaleEpoch(req.RuntimeID, *req.ExpectedEpoch, previousEpoch)
	}

	isActive, err := s.runtimeReader.IsRuntimeActive(ctx, req.RuntimeID)
	if err != nil {
		return TakeoverResult{}, err
	}
	if !isActive {
		s.recordTakeoverAudit(req, previousMode, previousMode, previousEpoch, previousEpoch, AuditResultDenied, "runtime not active")
		return TakeoverResult{}, &AuthorityError{
			Code:    domain.ErrRuntimeUnavailable,
			Message: "runtime not active: " + string(req.RuntimeID),
		}
	}

	currentSnap, err := s.manager.Get(ctx, req.RuntimeID)
	if err != nil {
		return TakeoverResult{}, err
	}
	currentEpoch := currentSnap.Epoch
	newEpoch := currentEpoch + 1

	snap, err := s.manager.Transition(ctx, req.RuntimeID, TransitionRequest{
		Target:        domain.ControlModeUserControl,
		Actor:         req.Actor,
		Reason:        ReasonUserRequest,
		ExpectedEpoch: currentEpoch,
		UseExpected:   true,
	})
	if err != nil {
		s.recordTakeoverAudit(req, previousMode, previousMode, previousEpoch, previousEpoch, AuditResultError, err.Error())
		return TakeoverResult{}, err
	}

	s.contextStore.Record(req.RuntimeID, currentSnap.Mode, newEpoch, s.clock())
	s.recordTakeoverAudit(req, currentSnap.Mode, domain.ControlModeUserControl, currentEpoch, newEpoch, AuditResultSuccess, "")

	return TakeoverResult{
		PreviousMode:  currentSnap.Mode,
		NewMode:       domain.ControlModeUserControl,
		PreviousEpoch: currentEpoch,
		NewEpoch:      newEpoch,
		Snapshot:      snap,
	}, nil
}

func (s *TakeoverService) Release(ctx context.Context, req ReleaseRequest) (ReleaseResult, error) {
	if err := ctx.Err(); err != nil {
		return ReleaseResult{}, err
	}
	if req.RuntimeID == "" {
		return ReleaseResult{}, &AuthorityError{
			Code:    domain.ErrInvalidArgument,
			Message: "runtime id must not be empty",
		}
	}
	if req.Actor == "" {
		return ReleaseResult{}, &AuthorityError{
			Code:    domain.ErrInvalidArgument,
			Message: "actor must not be empty",
		}
	}
	if req.TargetMode == "" {
		req.TargetMode = DefaultReleaseTarget
	}
	if req.TargetMode == domain.ControlModeUserControl {
		return ReleaseResult{}, &AuthorityError{
			Code:    domain.ErrInvalidArgument,
			Message: "release target cannot be user",
		}
	}
	if !IsValidControlMode(req.TargetMode) {
		return ReleaseResult{}, errInvalidControlMode(req.TargetMode)
	}

	lock := s.getOpLock(req.RuntimeID)
	lock.Lock()
	defer lock.Unlock()

	current, err := s.manager.Get(ctx, req.RuntimeID)
	if err != nil {
		return ReleaseResult{}, err
	}

	if current.Mode != domain.ControlModeUserControl {
		return ReleaseResult{}, &AuthorityError{
			Code:    domain.ErrInvalidState,
			Message: "current mode is not user, cannot release",
		}
	}

	previousEpoch := current.Epoch

	if req.UseExpected && req.ExpectedEpoch != previousEpoch {
		s.recordReleaseAudit(req, current.Mode, req.TargetMode, previousEpoch, previousEpoch, AuditResultDenied, "stale epoch")
		return ReleaseResult{}, errStaleEpoch(req.RuntimeID, req.ExpectedEpoch, previousEpoch)
	}

	isStopping, err := s.runtimeReader.IsRuntimeStopping(ctx, req.RuntimeID)
	if err != nil {
		return ReleaseResult{}, err
	}
	if isStopping {
		s.recordReleaseAudit(req, current.Mode, req.TargetMode, previousEpoch, previousEpoch, AuditResultDenied, "runtime stopping")
		return ReleaseResult{}, &AuthorityError{
			Code:    domain.ErrInvalidState,
			Message: "cannot release while runtime is stopping",
		}
	}

	if req.TargetMode != domain.ControlModeObserveOnly &&
		req.TargetMode != domain.ControlModeSuspended {

		isReady, err := s.runtimeReader.IsRuntimeReady(ctx, req.RuntimeID)
		if err != nil {
			return ReleaseResult{}, err
		}
		if !isReady {
			s.recordReleaseAudit(req, current.Mode, req.TargetMode, previousEpoch, previousEpoch, AuditResultDenied, "runtime not ready")
			return ReleaseResult{}, &AuthorityError{
				Code:    domain.ErrRuntimeUnavailable,
				Message: "runtime not ready for plugin control",
			}
		}

		permResult, err := s.permChecker.CanPluginControl(ctx, req.RuntimeID, req.PluginID, req.TargetMode)
		if err != nil {
			return ReleaseResult{}, err
		}
		if !permResult.Allowed {
			s.recordReleaseAudit(req, current.Mode, req.TargetMode, previousEpoch, previousEpoch, AuditResultDenied, permResult.Reason)
			return ReleaseResult{}, &AuthorityError{
				Code:    domain.ErrPermissionDenied,
				Message: "permission denied for release target " + string(req.TargetMode) + ": " + permResult.Reason,
			}
		}

		policyResult, err := s.policyChecker.AllowPluginControl(ctx, req.RuntimeID, req.TargetMode)
		if err != nil {
			return ReleaseResult{}, err
		}
		if !policyResult.Allowed {
			s.recordReleaseAudit(req, current.Mode, req.TargetMode, previousEpoch, previousEpoch, AuditResultDenied, policyResult.Reason)
			return ReleaseResult{}, &AuthorityError{
				Code:    domain.ErrPermissionDenied,
				Message: "host policy denied release target " + string(req.TargetMode) + ": " + policyResult.Reason,
			}
		}
	}

	newEpoch := previousEpoch + 1

	var expectedEpochForTransition uint64
	if !req.UseExpected {
		expectedEpochForTransition = previousEpoch
	} else {
		expectedEpochForTransition = req.ExpectedEpoch
	}

	snap, err := s.manager.Transition(ctx, req.RuntimeID, TransitionRequest{
		Target:        req.TargetMode,
		Actor:         req.Actor,
		Reason:        ReasonUserRequest,
		ExpectedEpoch: expectedEpochForTransition,
		UseExpected:   true,
	})
	if err != nil {
		s.recordReleaseAudit(req, current.Mode, req.TargetMode, previousEpoch, previousEpoch, AuditResultError, err.Error())
		return ReleaseResult{}, err
	}

	s.contextStore.Remove(req.RuntimeID)
	s.recordReleaseAudit(req, domain.ControlModeUserControl, req.TargetMode, previousEpoch, newEpoch, AuditResultSuccess, "")

	return ReleaseResult{
		PreviousMode:  domain.ControlModeUserControl,
		NewMode:       req.TargetMode,
		PreviousEpoch: previousEpoch,
		NewEpoch:      newEpoch,
		Snapshot:      snap,
	}, nil
}

func (s *TakeoverService) TakeoverContext(runtimeID domain.RuntimeInstanceID) (*TakeoverContext, bool) {
	return s.contextStore.Get(runtimeID)
}

func (s *TakeoverService) recordTakeoverAudit(
	req TakeoverRequest,
	previousMode domain.ControlMode,
	newMode domain.ControlMode,
	previousEpoch uint64,
	newEpoch uint64,
	result TransitionAuditResult,
	errMsg string,
) {
	s.audit.RecordTransition(AuthorityAuditEvent{
		RuntimeID:     req.RuntimeID,
		PluginID:      req.PluginID,
		PreviousMode:  previousMode,
		NewMode:       newMode,
		PreviousEpoch: previousEpoch,
		NewEpoch:      newEpoch,
		Actor:         req.Actor,
		Reason:        ReasonUserRequest,
		Result:        result,
		Error:         errMsg,
		Timestamp:     s.clock(),
	})
}

func (s *TakeoverService) recordReleaseAudit(
	req ReleaseRequest,
	previousMode domain.ControlMode,
	newMode domain.ControlMode,
	previousEpoch uint64,
	newEpoch uint64,
	result TransitionAuditResult,
	errMsg string,
) {
	s.audit.RecordTransition(AuthorityAuditEvent{
		RuntimeID:     req.RuntimeID,
		PluginID:      req.PluginID,
		PreviousMode:  previousMode,
		NewMode:       newMode,
		PreviousEpoch: previousEpoch,
		NewEpoch:      newEpoch,
		Actor:         req.Actor,
		Reason:        ReasonUserRequest,
		Result:        result,
		Error:         errMsg,
		Timestamp:     s.clock(),
	})
}
