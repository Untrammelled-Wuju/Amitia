package hook

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Pipeline struct {
	PointRegistry    HookPointRegistry
	ContribStore     ContributionStore
	RuntimeBridge    RuntimeBridge
	Permission       PermissionChecker
	Scope            ScopeChecker
	Dependency       DependencyChecker
	Trace            TraceRecorder
	Circuit          *CircuitBreaker
	DepthGuard       *DepthGuard
	Validator        *PatchValidator
	PlanCache        *PlanCache
	HostRevalidator  HostRevalidator
	MaxDepth         int
	PipelineTimeout  time.Duration
	chainBudgetMs    int64
}

func NewPipeline(
	pointRegistry HookPointRegistry,
	contribStore ContributionStore,
	runtimeBridge RuntimeBridge,
) *Pipeline {
	return &Pipeline{
		PointRegistry:   pointRegistry,
		ContribStore:    contribStore,
		RuntimeBridge:   runtimeBridge,
		Permission:      NopPermissionChecker{},
		Scope:           NopScopeChecker{},
		Dependency:      NopDependencyChecker{},
		Trace:           NopTraceRecorder{},
		Circuit:         NewCircuitBreaker(),
		DepthGuard:      NewDepthGuard(DefaultMaxDepth),
		Validator:       NewPatchValidator(),
		PlanCache:       NewPlanCache(),
		HostRevalidator: NopHostRevalidator{},
		MaxDepth:        DefaultMaxDepth,
		PipelineTimeout: 2000 * time.Millisecond,
		chainBudgetMs:   5000,
	}
}

func (p *Pipeline) WithPermission(checker PermissionChecker) *Pipeline {
	p.Permission = checker
	return p
}

func (p *Pipeline) WithScope(checker ScopeChecker) *Pipeline {
	p.Scope = checker
	return p
}

func (p *Pipeline) WithDependency(checker DependencyChecker) *Pipeline {
	p.Dependency = checker
	return p
}

func (p *Pipeline) WithTrace(recorder TraceRecorder) *Pipeline {
	p.Trace = recorder
	return p
}

func (p *Pipeline) WithHostRevalidator(r HostRevalidator) *Pipeline {
	p.HostRevalidator = r
	return p
}

func (p *Pipeline) WithPlanCache(c *PlanCache) *Pipeline {
	p.PlanCache = c
	return p
}

func (p *Pipeline) WithChainBudget(ms int64) *Pipeline {
	p.chainBudgetMs = ms
	return p
}

func (p *Pipeline) InvalidatePlan(hookPointID string) {
	if p.PlanCache != nil {
		p.PlanCache.Invalidate(hookPointID)
	}
}

func (p *Pipeline) RebuildPlan(ctx context.Context, hookPointID string) *CompiledHookPlan {
	point, err := p.PointRegistry.GetPoint(ctx, hookPointID)
	if err != nil {
		return nil
	}
	contribs, err := p.ContribStore.ListByHookPoint(ctx, hookPointID)
	if err != nil {
		return nil
	}
	circuitStates := p.collectCircuitStates(contribs)
	return p.PlanCache.BuildOrReplace(point, contribs, circuitStates)
}

func (p *Pipeline) collectCircuitStates(contribs []HookContributionDefinition) map[string]CircuitStats {
	states := make(map[string]CircuitStats)
	for _, c := range contribs {
		states[c.ContributionID] = p.Circuit.GetStats(c.ContributionID)
	}
	return states
}

type InvokeRequest struct {
	HookPointID string
	Payload     json.RawMessage
	Context     HookContextSnapshot
	Depth       int
	ParentStack []string
}

func (p *Pipeline) Invoke(ctx context.Context, req InvokeRequest) PipelineResult {
	start := time.Now()
	operationID := req.Context.OperationID
	if operationID == "" {
		operationID = uuid.NewString()
	}

	parentDepth := req.Depth
	if parentDepth < 0 {
		parentDepth = 0
	}
	if ctxDepth := DepthFromContext(ctx); ctxDepth > parentDepth {
		parentDepth = ctxDepth
	}

	result := PipelineResult{
		OperationID: operationID,
		HookPointID: req.HookPointID,
		Decision:    DecisionContinue,
		Depth:       parentDepth,
	}

	var chainBudget time.Duration
	if p.chainBudgetMs > 0 {
		chainBudget = time.Duration(p.chainBudgetMs) * time.Millisecond
	} else {
		chainBudget = p.PipelineTimeout
	}
	pipelineCtx, pipelineCancel := context.WithTimeout(ctx, chainBudget)
	defer pipelineCancel()

	point, err := p.PointRegistry.GetPoint(pipelineCtx, req.HookPointID)
	if err != nil {
		result.Aborted = true
		result.AbortReason = "hook point not found: " + err.Error()
		result.TotalDuration = time.Since(start).Milliseconds()
		return result
	}

	if int64(len(req.Payload)) > point.MaxPayloadBytes {
		result.Aborted = true
		result.AbortReason = fmt.Sprintf("payload %d bytes exceeds max %d", len(req.Payload), point.MaxPayloadBytes)
		result.TotalDuration = time.Since(start).Milliseconds()
		return result
	}

	plan := p.getOrBuildPlan(pipelineCtx, point)
	if plan == nil {
		result.Aborted = true
		result.AbortReason = "no eligible contributions or plan build failed"
		result.TotalDuration = time.Since(start).Milliseconds()
		return result
	}

	currentPayload := req.Payload
	writtenPaths := make(map[string]string)
	sequence := 0

	phaseOrder := []HookPhase{PhaseBefore, PhaseFilter, PhaseTransform, PhaseAfter, PhaseObserve}
	for _, phase := range phaseOrder {
		if result.Aborted {
			break
		}
		phaseContribs := plan.LookupByPhase(phase)
		for _, compiled := range phaseContribs {
			contribDef := p.resolveContribution(pipelineCtx, compiled.ContributionID)
			if contribDef == nil {
				continue
			}
			sequence++
			exec := p.executeContribCompiled(pipelineCtx, *contribDef, point, currentPayload, req.Context, parentDepth, req.ParentStack, sequence, writtenPaths)
			result.Executions = append(result.Executions, exec)

			if exec.Status == StatusDenied {
				result.Decision = DecisionDeny
				result.Aborted = true
				result.AbortReason = exec.Error
				break
			}

			if exec.Decision == DecisionReject {
				result.Decision = DecisionReject
				result.Aborted = true
				result.AbortReason = exec.Error
				break
			}

			if exec.Decision == DecisionReplace && exec.MutationCount > 0 {
				result.Transformed = true
			}

			if exec.Status == StatusFailed && point.ExecutionPolicy.StopOnFailure {
				if contribDef.EffectiveFailurePolicy(point).OnRuntimeError == FailureFailClosed {
					result.Aborted = true
					result.AbortReason = exec.Error
					break
				}
			}

			if phase == PhaseFilter && exec.Decision == DecisionDeny {
				result.Decision = DecisionDeny
				result.Aborted = true
				result.AbortReason = exec.Error
				break
			}
		}
		if result.Aborted {
			break
		}
	}

	if result.Transformed && p.HostRevalidator != nil && !result.Aborted {
		if revalErr := p.HostRevalidator.Revalidate(pipelineCtx, point, PhaseTransform, req.Payload, currentPayload); revalErr != nil {
			result.Aborted = true
			result.AbortReason = "post-hook revalidation failed: " + revalErr.Error()
		}
	}

	result.FinalPayload = currentPayload
	result.TotalDuration = time.Since(start).Milliseconds()
	p.Trace.RecordPipeline(ctx, result)
	return result
}

func (p *Pipeline) getOrBuildPlan(ctx context.Context, point HookPointDefinition) *CompiledHookPlan {
	if p.PlanCache == nil {
		contribs, err := p.ContribStore.ListByHookPoint(ctx, point.HookPointID)
		if err != nil {
			return nil
		}
		circuitStates := p.collectCircuitStates(contribs)
		tmpCache := NewPlanCache()
		return tmpCache.BuildOrReplace(point, contribs, circuitStates)
	}
	if cached, ok := p.PlanCache.Get(point.HookPointID); ok && !cached.IsStale(p.PlanCache.Generation()) {
		return cached
	}
	contribs, err := p.ContribStore.ListByHookPoint(ctx, point.HookPointID)
	if err != nil {
		return nil
	}
	circuitStates := p.collectCircuitStates(contribs)
	return p.PlanCache.BuildOrReplace(point, contribs, circuitStates)
}

func (p *Pipeline) resolveContribution(ctx context.Context, contributionID string) *HookContributionDefinition {
	c, err := p.ContribStore.Get(ctx, contributionID)
	if err != nil {
		return nil
	}
	return &c
}

func (p *Pipeline) executeContribCompiled(
	ctx context.Context,
	contrib HookContributionDefinition,
	point HookPointDefinition,
	currentPayload json.RawMessage,
	hookCtx HookContextSnapshot,
	parentDepth int,
	parentStack []string,
	sequence int,
	writtenPaths map[string]string,
) HookExecution {
	exec := HookExecution{
		ContributionID: contrib.ContributionID,
		ExtensionID:    contrib.ExtensionID,
		Phase:          contrib.Phase,
		Sequence:       sequence,
		StartedAt:      time.Now().UTC().Format(time.RFC3339),
	}
	startTime := time.Now()

	inputHash := HashPayload(currentPayload)
	exec.InputHash = inputHash

	permOK, permReason := p.Permission.Check(ctx, contrib.ExtensionID, contrib.ToPermissionRequirements(), hookCtx.InvocationID)
	if !permOK {
		exec.Status = StatusDenied
		exec.ErrorCode = string(ErrCodePermissionDenied)
		exec.Error = "permission denied: " + permReason
		exec.DurationMs = time.Since(startTime).Milliseconds()
		p.handleFailure(ctx, contrib, point, ErrCodePermissionDenied, fmt.Errorf("permission denied: %s", permReason))
		p.Trace.RecordInvocation(ctx, exec, inputHash, "")
		return exec
	}

	scopeOK, scopeReason := p.Scope.Check(ctx, contrib.ToScopeEvaluationRequest())
	if !scopeOK {
		exec.Status = StatusDenied
		exec.ErrorCode = string(ErrCodeScopeDenied)
		exec.Error = "scope denied: " + scopeReason
		exec.DurationMs = time.Since(startTime).Milliseconds()
		p.handleFailure(ctx, contrib, point, ErrCodeScopeDenied, fmt.Errorf("scope denied: %s", scopeReason))
		p.Trace.RecordInvocation(ctx, exec, inputHash, "")
		return exec
	}

	depOK, depReason := p.Dependency.Check(ctx, contrib.DependencyRequirements)
	if !depOK {
		exec.Status = StatusSkipped
		exec.ErrorCode = string(ErrCodeDependencyUnavailable)
		exec.Error = "dependency unavailable: " + depReason
		exec.DurationMs = time.Since(startTime).Milliseconds()
		p.Trace.RecordInvocation(ctx, exec, inputHash, "")
		return exec
	}

	newDepth, err := p.DepthGuard.CheckAndEnter(hookCtx.InvocationID, contrib.ContributionID, contrib.HookPointID, parentDepth)
	if err != nil {
		exec.Status = StatusFailed
		if err == ErrRecursion {
			exec.ErrorCode = string(ErrCodeHookRecursionDetected)
		} else {
			exec.ErrorCode = string(ErrCodeHookDepthExceeded)
		}
		exec.Error = err.Error()
		exec.DurationMs = time.Since(startTime).Milliseconds()
		p.Trace.RecordInvocation(ctx, exec, inputHash, "")
		return exec
	}
	enteredDepth := newDepth
	_ = enteredDepth
	defer p.DepthGuard.Exit(hookCtx.InvocationID, contrib.ContributionID)

	if !p.RuntimeBridge.IsReady(ctx, contrib) {
		exec.Status = StatusSkipped
		exec.ErrorCode = string(ErrCodeRuntimeNotReady)
		exec.Error = "runtime not ready"
		exec.DurationMs = time.Since(startTime).Milliseconds()
		p.handleFailure(ctx, contrib, point, ErrCodeRuntimeNotReady, fmt.Errorf("runtime not ready"))
		p.Trace.RecordInvocation(ctx, exec, inputHash, "")
		return exec
	}

	nestedCtx := ContextWithDepth(ctx, newDepth)

	invocationInput := HookInvocationInput{
		HookPointID:     contrib.HookPointID,
		ContractVersion: contrib.ContractVersion,
		Payload:         currentPayload,
		Context:         hookCtx,
	}

	result, err := p.RuntimeBridge.Invoke(nestedCtx, contrib, invocationInput)
	if err != nil {
		exec.ErrorCode = classifyError(err)
		exec.Error = err.Error()
		exec.Status = p.failureStatus(exec.ErrorCode)
		exec.DurationMs = time.Since(startTime).Milliseconds()
		p.handleFailure(ctx, contrib, point, HookErrorCode(exec.ErrorCode), err)
		resultHash := HashResult(result)
		exec.ResultHash = resultHash
		p.Trace.RecordInvocation(ctx, exec, inputHash, resultHash)
		return exec
	}

	resultHash := HashResult(result)
	exec.ResultHash = resultHash

	if !result.Decision.AllowedForPhase(contrib.Phase) {
		exec.Status = StatusFailed
		exec.ErrorCode = string(ErrCodeInvalidDecision)
		exec.Error = fmt.Sprintf("decision %s not allowed for phase %s", result.Decision, contrib.Phase)
		exec.DurationMs = time.Since(startTime).Milliseconds()
		p.handleFailure(ctx, contrib, point, ErrCodeInvalidDecision, fmt.Errorf("invalid decision"))
		p.Trace.RecordInvocation(ctx, exec, inputHash, resultHash)
		return exec
	}

	exec.Decision = result.Decision

	if result.Decision == DecisionDeny {
		exec.Status = StatusDenied
		exec.DurationMs = time.Since(startTime).Milliseconds()
		p.Trace.RecordInvocation(ctx, exec, inputHash, resultHash)
		return exec
	}

	if result.Decision == DecisionReject {
		exec.Status = StatusFailed
		exec.ErrorCode = string(ErrCodeHookResultInvalid)
		if msg, ok := result.Metadata["reason"].(string); ok {
			exec.Error = msg
		} else {
			exec.Error = "rejected"
		}
		exec.DurationMs = time.Since(startTime).Milliseconds()
		p.Trace.RecordInvocation(ctx, exec, inputHash, resultHash)
		return exec
	}

	if len(result.Patch) > 0 {
		vctx := &ValidationContext{
			Point:        point,
			Contrib:      contrib,
			CurrentObj:   nil,
			WrittenPaths: writtenPaths,
		}
		validationErrs := p.Validator.Validate(result, vctx)
		if len(validationErrs) > 0 {
			exec.Status = StatusFailed
			exec.ErrorCode = string(ErrCodeHookResultInvalid)
			exec.Error = validationErrs[0].Error()
			exec.DurationMs = time.Since(startTime).Milliseconds()
			p.handleFailure(ctx, contrib, point, ErrCodeHookResultInvalid, fmt.Errorf("validation failed: %s", validationErrs[0].Error()))
			p.Trace.RecordInvocation(ctx, exec, inputHash, resultHash)
			return exec
		}

		var currentObj map[string]any
		if err := json.Unmarshal(currentPayload, &currentObj); err != nil {
			exec.Status = StatusFailed
			exec.ErrorCode = string(ErrCodeHookResultInvalid)
			exec.Error = "unmarshal payload: " + err.Error()
			exec.DurationMs = time.Since(startTime).Milliseconds()
			p.Trace.RecordInvocation(ctx, exec, inputHash, resultHash)
			return exec
		}

		beforeHash := HashPayload(currentPayload)
		updated, err := ApplyPatch(currentObj, result.Patch)
		if err != nil {
			exec.Status = StatusFailed
			exec.ErrorCode = string(ErrCodeHookResultInvalid)
			exec.Error = "apply patch: " + err.Error()
			exec.DurationMs = time.Since(startTime).Milliseconds()
			p.Trace.RecordInvocation(ctx, exec, inputHash, resultHash)
			return exec
		}

		newPayload, err := json.Marshal(updated)
		if err != nil {
			exec.Status = StatusFailed
			exec.ErrorCode = string(ErrCodeHookResultInvalid)
			exec.Error = "marshal updated: " + err.Error()
			exec.DurationMs = time.Since(startTime).Milliseconds()
			p.Trace.RecordInvocation(ctx, exec, inputHash, resultHash)
			return exec
		}

		afterHash := HashPayload(newPayload)
		for _, op := range result.Patch {
			p.Trace.RecordMutation(ctx, hookCtx.InvocationID, op, beforeHash, afterHash, true, false)
		}
		currentPayload = newPayload
		exec.MutationCount = len(result.Patch)
	}

	p.Circuit.RecordSuccess(contrib.ContributionID)
	exec.Status = StatusSuccess
	exec.DurationMs = time.Since(startTime).Milliseconds()
	p.Trace.RecordInvocation(ctx, exec, inputHash, resultHash)
	return exec
}

func (p *Pipeline) handleFailure(ctx context.Context, contrib HookContributionDefinition, point HookPointDefinition, code HookErrorCode, err error) {
	if ShouldCountCircuitFailure(code) {
		p.Circuit.RecordFailure(contrib.ContributionID, code)
	}

	fctx := FailureContext{
		Err:      err,
		ErrCode:  code,
		Policy:   contrib.EffectiveFailurePolicy(point),
		Point:    point,
		Contrib:  contrib,
		Phase:    contrib.Phase,
		IsFilter: contrib.Phase == PhaseFilter,
	}
	outcome := ProcessFailure(fctx)

	if outcome.DisableContrib {
		_ = p.ContribStore.SetEnabled(ctx, contrib.ContributionID, false)
	}
}

func classifyError(err error) string {
	if err == nil {
		return ""
	}
	if he, ok := err.(*HookError); ok {
		return string(he.Code)
	}
	if err == context.DeadlineExceeded {
		return string(ErrCodeHookTimeout)
	}
	if err == context.Canceled {
		return string(ErrCodeHookCancelled)
	}
	return string(ErrCodeHookRuntimeError)
}

func (p *Pipeline) failureStatus(code string) string {
	switch HookErrorCode(code) {
	case ErrCodeHookTimeout:
		return StatusTimeout
	case ErrCodeHookCancelled:
		return StatusCancelled
	case ErrCodeCircuitOpen:
		return StatusCircuitOpen
	case ErrCodePermissionDenied, ErrCodeScopeDenied:
		return StatusDenied
	default:
		return StatusFailed
	}
}
