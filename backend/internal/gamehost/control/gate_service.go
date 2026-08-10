package control

import (
	"context"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

const (
	OutputPermissionID = "gamehost.control.output"
)

type TopologyReader interface {
	HasRuntime(runtimeID domain.RuntimeInstanceID) bool
	GetPluginID(runtimeID domain.RuntimeInstanceID) (domain.PluginID, bool)
	ServiceBelongsToRuntime(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) bool
}

type EffectivePermissionChecker interface {
	CheckControlOutput(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, pluginID domain.PluginID) (PermissionCheckResult, error)
}

type ControlAuthoritySnapshotReader interface {
	GetSnapshot(ctx context.Context, runtimeID domain.RuntimeInstanceID) (ControlAuthoritySnapshot, error)
}

func (m *ControlAuthorityManager) GetSnapshot(ctx context.Context, runtimeID domain.RuntimeInstanceID) (ControlAuthoritySnapshot, error) {
	return m.Get(ctx, runtimeID)
}

type MetricsSink interface {
	RecordOutputDecision(runtimeID domain.RuntimeInstanceID, kind ControlOutputKind, reason OutputDecisionReason, allowed bool)
}

type NoopMetricsSink struct{}

func (NoopMetricsSink) RecordOutputDecision(runtimeID domain.RuntimeInstanceID, kind ControlOutputKind, reason OutputDecisionReason, allowed bool) {}

type PluginOutputGate struct {
	mu          sync.RWMutex
	closedFlags map[domain.RuntimeInstanceID]bool

	clock         Clock
	topology      TopologyReader
	runtimeReader RuntimeReader
	permChecker   EffectivePermissionChecker
	policyChecker HostPolicyChecker
	authority     ControlAuthoritySnapshotReader
	audit         AuthorityAuditSink
	metrics       MetricsSink

	perRuntime map[domain.RuntimeInstanceID]*sync.Mutex
	opMu       sync.Mutex
}

type PluginOutputGateOptions struct {
	Clock         Clock
	Topology      TopologyReader
	RuntimeReader RuntimeReader
	PermChecker   EffectivePermissionChecker
	PolicyChecker HostPolicyChecker
	Authority     ControlAuthoritySnapshotReader
	Audit         AuthorityAuditSink
	Metrics       MetricsSink
}

func NewPluginOutputGate(opts PluginOutputGateOptions) *PluginOutputGate {
	clock := opts.Clock
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	if opts.Metrics == nil {
		opts.Metrics = NoopMetricsSink{}
	}
	return &PluginOutputGate{
		closedFlags:   make(map[domain.RuntimeInstanceID]bool),
		clock:         clock,
		topology:      opts.Topology,
		runtimeReader: opts.RuntimeReader,
		permChecker:   opts.PermChecker,
		policyChecker: opts.PolicyChecker,
		authority:     opts.Authority,
		audit:         opts.Audit,
		metrics:       opts.Metrics,
		perRuntime:    make(map[domain.RuntimeInstanceID]*sync.Mutex),
	}
}

func (g *PluginOutputGate) getRuntimeLock(runtimeID domain.RuntimeInstanceID) *sync.Mutex {
	g.opMu.Lock()
	defer g.opMu.Unlock()
	lock, ok := g.perRuntime[runtimeID]
	if !ok {
		lock = &sync.Mutex{}
		g.perRuntime[runtimeID] = lock
	}
	return lock
}

type OutputCheckRequest struct {
	Intent  ControlOutputIntent
	Peer    TrustedPluginIdentity
	Actor   TransitionActor
	Payload []byte
}

func (g *PluginOutputGate) Check(ctx context.Context, req OutputCheckRequest) (OutputDecision, *OutputPermit) {
	now := g.clock()
	intent := req.Intent
	identity := req.Peer

	if err := ctx.Err(); err != nil {
		return OutputDenied(OutputDeniedInvalidPeer, 0, "", now), nil
	}

	if identity.Empty() {
		g.recordAudit(intent, identity, OutputDeniedInvalidPeer, false, now)
		return OutputDenied(OutputDeniedInvalidPeer, 0, "", now), nil
	}

	if intent.RuntimeID != identity.RuntimeID {
		g.recordAudit(intent, identity, OutputDeniedInvalidPeer, false, now)
		return OutputDenied(OutputDeniedInvalidPeer, 0, "", now), nil
	}

	if identity.ServiceID != "" && intent.ServiceID != identity.ServiceID {
		g.recordAudit(intent, identity, OutputDeniedInvalidPeer, false, now)
		return OutputDenied(OutputDeniedInvalidPeer, 0, "", now), nil
	}

	if intent.AuthorityEpoch == 0 {
		g.recordAudit(intent, identity, OutputDeniedStaleEpoch, false, now)
		return OutputDenied(OutputDeniedStaleEpoch, 0, "", now), nil
	}

	g.mu.RLock()
	_, gateClosed := g.closedFlags[intent.RuntimeID]
	g.mu.RUnlock()
	if gateClosed {
		g.recordAudit(intent, identity, OutputDeniedGateClosed, false, now)
		return OutputDenied(OutputDeniedGateClosed, 0, "", now), nil
	}

	lock := g.getRuntimeLock(intent.RuntimeID)
	lock.Lock()
	defer lock.Unlock()

	if g.topology != nil {
		if !g.topology.HasRuntime(intent.RuntimeID) {
			g.recordAudit(intent, identity, OutputDeniedRuntimeNotFound, false, now)
			return OutputDenied(OutputDeniedRuntimeNotFound, 0, "", now), nil
		}
		runtimePluginID, ok := g.topology.GetPluginID(intent.RuntimeID)
		if !ok || runtimePluginID != identity.PluginID {
			g.recordAudit(intent, identity, OutputDeniedInvalidPeer, false, now)
			return OutputDenied(OutputDeniedInvalidPeer, 0, "", now), nil
		}
		if intent.ServiceID != "" && !g.topology.ServiceBelongsToRuntime(intent.RuntimeID, intent.ServiceID) {
			g.recordAudit(intent, identity, OutputDeniedServiceNotFound, false, now)
			return OutputDenied(OutputDeniedServiceNotFound, 0, "", now), nil
		}
	}

	if g.runtimeReader != nil {
		isActive, _ := g.runtimeReader.IsRuntimeActive(ctx, intent.RuntimeID)
		if !isActive {
			g.recordAudit(intent, identity, OutputDeniedNotEligible, false, now)
			return OutputDenied(OutputDeniedNotEligible, 0, "", now), nil
		}
		isStopping, _ := g.runtimeReader.IsRuntimeStopping(ctx, intent.RuntimeID)
		if isStopping {
			g.recordAudit(intent, identity, OutputDeniedNotEligible, false, now)
			return OutputDenied(OutputDeniedNotEligible, 0, "", now), nil
		}
		isReady, _ := g.runtimeReader.IsRuntimeReady(ctx, intent.RuntimeID)
		if !isReady {
			g.recordAudit(intent, identity, OutputDeniedNotReady, false, now)
			return OutputDenied(OutputDeniedNotReady, 0, "", now), nil
		}
	}

	var currentEpoch uint64
	var currentMode domain.ControlMode
	var authorityExists bool

	if g.authority != nil {
		snap, err := g.authority.GetSnapshot(ctx, intent.RuntimeID)
		if err != nil {
			g.recordAudit(intent, identity, OutputDeniedRuntimeNotFound, false, now)
			return OutputDenied(OutputDeniedRuntimeNotFound, 0, "", now), nil
		}
		currentEpoch = snap.Epoch
		currentMode = snap.Mode
		authorityExists = true
	}

	if authorityExists {
		if !isAuthorityModeAllowedForOutput(currentMode) {
			g.recordAudit(intent, identity, OutputDeniedAuthorityMode, false, now)
			return OutputDenied(OutputDeniedAuthorityMode, currentEpoch, currentMode, now), nil
		}

		if intent.AuthorityEpoch != currentEpoch {
			g.recordAudit(intent, identity, OutputDeniedStaleEpoch, false, now)
			return OutputDenied(OutputDeniedStaleEpoch, currentEpoch, currentMode, now), nil
		}
	}

	if g.permChecker != nil {
		permResult, err := g.permChecker.CheckControlOutput(ctx, intent.RuntimeID, intent.ServiceID, identity.PluginID)
		if err != nil || !permResult.Allowed {
			g.recordAudit(intent, identity, OutputDeniedPermission, false, now)
			return OutputDenied(OutputDeniedPermission, currentEpoch, currentMode, now), nil
		}
	}

	if g.policyChecker != nil {
		policyResult, err := g.policyChecker.AllowPluginControl(ctx, intent.RuntimeID, currentMode)
		if err != nil || !policyResult.Allowed {
			g.recordAudit(intent, identity, OutputDeniedHostPolicy, false, now)
			return OutputDenied(OutputDeniedHostPolicy, currentEpoch, currentMode, now), nil
		}
	}

	permit := NewOutputPermit(
		intent.RuntimeID,
		intent.ServiceID,
		identity.PluginID,
		currentEpoch,
		intent.Kind,
		currentMode,
		DefaultPermitTTL,
		now,
	)

	g.recordAudit(intent, identity, "", true, now)
	return OutputAllowed(currentEpoch, currentMode, now), &permit
}

func (g *PluginOutputGate) AuthorizeAndDispatch(ctx context.Context, req OutputCheckRequest, sink ControlEffectSink) (OutputDecision, error) {
	decision, permit := g.Check(ctx, req)
	if decision.Deny() {
		return decision, g.mapDecisionToError(decision)
	}

	currentEpoch, _ := g.currentEpoch(ctx, req.Intent.RuntimeID)
	if permit == nil || !permit.IsCurrent(currentEpoch) {
		denied := OutputDenied(OutputDeniedStaleEpoch, currentEpoch, decision.EvaluatedMode, g.clock())
		return denied, g.mapDecisionToError(denied)
	}

	if sink != nil {
		err := sink.ExecuteAuthorized(ctx, req.Intent.RuntimeID, req.Intent.ServiceID, req.Peer.PluginID, *permit, req.Payload)
		return decision, err
	}

	return decision, nil
}

func (g *PluginOutputGate) CloseRuntimeOutputs(runtimeID domain.RuntimeInstanceID) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.closedFlags[runtimeID] = true
}

func (g *PluginOutputGate) ReopenRuntimeOutputs(runtimeID domain.RuntimeInstanceID) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.closedFlags, runtimeID)
}

func (g *PluginOutputGate) IsRuntimeClosed(runtimeID domain.RuntimeInstanceID) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.closedFlags[runtimeID]
}

func (g *PluginOutputGate) currentEpoch(ctx context.Context, runtimeID domain.RuntimeInstanceID) (uint64, domain.ControlMode) {
	if g.authority == nil {
		return 0, ""
	}
	snap, err := g.authority.GetSnapshot(ctx, runtimeID)
	if err != nil {
		return 0, ""
	}
	return snap.Epoch, snap.Mode
}

func (g *PluginOutputGate) mapDecisionToError(decision OutputDecision) *AuthorityError {
	switch decision.Reason {
	case OutputDeniedInvalidPeer:
		return errOutputInvalidPeer(TrustedPluginIdentity{})
	case OutputDeniedRuntimeNotFound:
		return errOutputRuntimeNotFound("")
	case OutputDeniedServiceNotFound:
		return errOutputServiceNotFound("", "")
	case OutputDeniedNotEligible:
		return errOutputRuntimeNotEligible("", "")
	case OutputDeniedNotReady:
		return errOutputRuntimeNotReady("")
	case OutputDeniedPermission:
		return errOutputPermissionDenied("")
	case OutputDeniedAuthorityMode:
		return errOutputAuthorityModeDenied("")
	case OutputDeniedStaleEpoch:
		return errOutputStaleEpoch(0, 0)
	case OutputDeniedHostPolicy:
		return errOutputHostPolicyDenied("")
	case OutputDeniedGateClosed:
		return errOutputGateClosed("")
	default:
		return &AuthorityError{
			Code:    domain.ErrPermissionDenied,
			Message: "control output denied: " + string(decision.Reason),
		}
	}
}

func (g *PluginOutputGate) recordDecision(intent ControlOutputIntent, peer TrustedPluginIdentity, reason OutputDecisionReason, allowed bool, now time.Time) {
	if g.audit != nil {
		if _, ok := g.audit.(NoopAuthorityAuditSink); !ok {
			result := AuditResultSuccess
			if !allowed {
				result = AuditResultDenied
			}
			g.audit.RecordTransition(AuthorityAuditEvent{
				RuntimeID:     intent.RuntimeID,
				PluginID:      peer.PluginID,
				PreviousMode:  "",
				NewMode:       "",
				PreviousEpoch: intent.AuthorityEpoch,
				NewEpoch:      intent.AuthorityEpoch,
				Actor:         ActorPlugin,
				Reason:        ReasonPluginRequest,
				Result:        result,
				Error:         string(reason),
				Timestamp:     now,
			})
		}
	}
	g.metrics.RecordOutputDecision(intent.RuntimeID, intent.Kind, reason, allowed)
}

func (g *PluginOutputGate) recordAudit(intent ControlOutputIntent, peer TrustedPluginIdentity, reason OutputDecisionReason, allowed bool, now time.Time) {
	g.recordDecision(intent, peer, reason, allowed, now)
}

func isAuthorityModeAllowedForOutput(mode domain.ControlMode) bool {
	switch mode {
	case domain.ControlModePluginControl,
		domain.ControlModeSharedControl,
		domain.ControlModeAssist:
		return true
	case domain.ControlModeObserveOnly,
		domain.ControlModeUserControl,
		domain.ControlModeSuspended:
		return false
	default:
		return false
	}
}
