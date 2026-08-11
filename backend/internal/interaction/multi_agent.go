package interaction

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/decision"
)

type CoordinationID string

type MultiAgentPolicy struct {
	MaxWorkers            int
	MaxAssignments        int
	MaxDepth              int
	DefaultCompletionPlan CoordinationCompletionPolicy
	DefaultStrategy       CoordinationStrategy
}

func DefaultMultiAgentPolicy() MultiAgentPolicy {
	return MultiAgentPolicy{
		MaxWorkers:            4,
		MaxAssignments:        8,
		MaxDepth:              2,
		DefaultCompletionPlan: CoordinationRequireAll,
		DefaultStrategy:       CoordinationParallel,
	}
}

type CoordinationStrategy string

const (
	CoordinationParallel   CoordinationStrategy = "parallel"
	CoordinationSequential CoordinationStrategy = "sequential"
)

type CoordinationCompletionPolicy string

const (
	CoordinationRequireAll   CoordinationCompletionPolicy = "require_all"
	CoordinationBestEffort   CoordinationCompletionPolicy = "best_effort"
	CoordinationFirstSuccess CoordinationCompletionPolicy = "first_success"
)

type CoordinationStatus string

const (
	CoordinationPlanning    CoordinationStatus = "planning"
	CoordinationRunning     CoordinationStatus = "running"
	CoordinationWaiting     CoordinationStatus = "waiting"
	CoordinationAggregating CoordinationStatus = "aggregating"
	CoordinationSucceeded   CoordinationStatus = "succeeded"
	CoordinationFailed      CoordinationStatus = "failed"
	CoordinationCancelled   CoordinationStatus = "cancelled"
	CoordinationPaused      CoordinationStatus = "paused"
)

func (s CoordinationStatus) IsTerminal() bool {
	switch s {
	case CoordinationSucceeded, CoordinationFailed, CoordinationCancelled:
		return true
	}
	return false
}

type AgentAssignmentStatus string

const (
	AssignmentPending   AgentAssignmentStatus = "pending"
	AssignmentRunning   AgentAssignmentStatus = "running"
	AssignmentWaiting   AgentAssignmentStatus = "waiting"
	AssignmentSucceeded AgentAssignmentStatus = "succeeded"
	AssignmentFailed    AgentAssignmentStatus = "failed"
	AssignmentCancelled AgentAssignmentStatus = "cancelled"
	AssignmentPaused    AgentAssignmentStatus = "paused"
)

func (s AgentAssignmentStatus) IsTerminal() bool {
	switch s {
	case AssignmentSucceeded, AssignmentFailed, AssignmentCancelled:
		return true
	}
	return false
}

type AgentWorkerRef struct {
	WorkerID    string `json:"workerId"`
	CharacterID string `json:"characterId"`
	Role        string `json:"role"`
}

type AgentAssignment struct {
	ID                 string                `json:"id"`
	CoordinationID     string                `json:"coordinationId"`
	ParentGoalID       string                `json:"parentGoalId"`
	ParentGoalRevision int64                 `json:"parentGoalRevision"`
	WorkerRef          AgentWorkerRef        `json:"workerRef"`
	Objective          string                `json:"objective"`
	Constraints        []string              `json:"constraints"`
	ExpectedOutcome    string                `json:"expectedOutcome"`
	Dependencies       []string              `json:"dependencies"`
	Status             AgentAssignmentStatus `json:"status"`
	ChildInteractionID string                `json:"childInteractionId,omitempty"`
	ChildGoalID        string                `json:"childGoalId,omitempty"`
	Error              string                `json:"error,omitempty"`
	CreatedAt          time.Time             `json:"createdAt"`
	UpdatedAt          time.Time             `json:"updatedAt"`
}

type AgentAssignmentResult struct {
	AssignmentID       string                `json:"assignmentId"`
	ChildInteractionID string                `json:"childInteractionId,omitempty"`
	ChildGoalID        string                `json:"childGoalId,omitempty"`
	Status             AgentAssignmentStatus `json:"status"`
	ObservationRefs    []string              `json:"observationRefs,omitempty"`
	Summary            string                `json:"summary"`
	ArtifactRefs       []string              `json:"artifactRefs,omitempty"`
	Error              *AssignmentError      `json:"error,omitempty"`
}

type AssignmentError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type WorkerRunRequest struct {
	CoordinationID       string `json:"coordinationId"`
	AssignmentID         string `json:"assignmentId"`
	ParentInteractionID  string `json:"parentInteractionId"`
	ParentGoalID         string `json:"parentGoalId"`
	ParentGoalRevision   int64  `json:"parentGoalRevision"`
	CharacterID          string `json:"characterId"`
	ConversationID       string `json:"conversationId"`
	Source               string `json:"source"`
	UserID               string `json:"userId"`
	Objective            string `json:"objective"`
	ExpectedOutcome      string `json:"expectedOutcome"`
	CoordinationDepth    int    `json:"coordinationDepth"`
	InternalSourceMarker string `json:"-"`
}

type AgentWorkerRunner interface {
	StartWorker(ctx context.Context, req WorkerRunRequest) (childInteractionID string, err error)
}

type CoordinatorClock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

type MultiAgentCoordinator struct {
	tracker     InteractionTracker
	goals       *decision.GoalRegistry
	recovery    *RecoveryDescriptorService
	starter     AgentWorkerRunner
	pauseSvc    *PauseResumeService
	policy      MultiAgentPolicy
	clock       CoordinatorClock

	mu            sync.Mutex
	coordinations map[string]*activeCoordination
}

type activeCoordination struct {
	id              CoordinationID
	parentGoalID    string
	parentGoalRev   int64
	parentInteractionID string
	status          CoordinationStatus
	strategy        CoordinationStrategy
	completionPlan  CoordinationCompletionPolicy
	depth           int
	assignments     []*AgentAssignment
	deadlineAt      *time.Time
	createdAt       time.Time
	updatedAt       time.Time
}

func NewMultiAgentCoordinator(
	tracker InteractionTracker,
	goals *decision.GoalRegistry,
	recovery *RecoveryDescriptorService,
	starter AgentWorkerRunner,
	pauseSvc *PauseResumeService,
	policy MultiAgentPolicy,
) *MultiAgentCoordinator {
	if policy.MaxWorkers <= 0 {
		policy = DefaultMultiAgentPolicy()
	}
	return &MultiAgentCoordinator{
		tracker:       tracker,
		goals:         goals,
		recovery:      recovery,
		starter:       starter,
		pauseSvc:      pauseSvc,
		policy:        policy,
		clock:         realClock{},
		coordinations: make(map[string]*activeCoordination),
	}
}

type StartCoordinationRequest struct {
	ParentInteractionID string
	ParentGoalID        string
	ParentGoalRevision  int64
	WorkerRefs          []AgentWorkerRef
	Objectives          []AssignmentObjective
	Strategy            CoordinationStrategy
	CompletionPlan      CoordinationCompletionPolicy
	DeadlineAt          *time.Time
	Depth               int
}

type AssignmentObjective struct {
	WorkerIndex    int
	Objective      string
	ExpectedOutcome string
	Dependencies   []string
	Constraints    []string
}

type StartCoordinationResult struct {
	CoordinationID CoordinationID
	AssignmentIDs  []string
}

func (c *MultiAgentCoordinator) Start(ctx context.Context, req StartCoordinationRequest) (*StartCoordinationResult, error) {
	if len(req.Objectives) == 0 {
		return nil, fmt.Errorf("multi_agent: no objectives provided")
	}
	if len(req.Objectives) > c.policy.MaxAssignments {
		return nil, fmt.Errorf("multi_agent_assignment_limit_exceeded: %d > %d", len(req.Objectives), c.policy.MaxAssignments)
	}
	if req.Depth > c.policy.MaxDepth {
		return nil, fmt.Errorf("multi_agent_depth_exceeded: %d > %d", req.Depth, c.policy.MaxDepth)
	}

	parentGoal, ok := c.goals.Get(req.ParentGoalID)
	if !ok {
		return nil, fmt.Errorf("multi_agent: parent goal not found: %s", req.ParentGoalID)
	}
	if parentGoal.Revision != req.ParentGoalRevision {
		return nil, fmt.Errorf("multi_agent: parent goal revision mismatch: expected %d got %d", req.ParentGoalRevision, parentGoal.Revision)
	}

	strategy := req.Strategy
	if strategy == "" {
		strategy = c.policy.DefaultStrategy
	}
	plan := req.CompletionPlan
	if plan == "" {
		plan = c.policy.DefaultCompletionPlan
	}

	coordID := CoordinationID(uuid.NewString())
	now := c.clock.Now()
	ac := &activeCoordination{
		id:                  coordID,
		parentGoalID:        req.ParentGoalID,
		parentGoalRev:       req.ParentGoalRevision,
		parentInteractionID: req.ParentInteractionID,
		status:              CoordinationPlanning,
		strategy:            strategy,
		completionPlan:      plan,
		depth:               req.Depth,
		deadlineAt:          req.DeadlineAt,
		assignments:         make([]*AgentAssignment, 0, len(req.Objectives)),
		createdAt:           now,
		updatedAt:           now,
	}

	multiRef := &MultiAgentRecoveryRef{
		CoordinationID:     string(coordID),
		ParentGoalID:        req.ParentGoalID,
		ParentGoalRevision:  req.ParentGoalRevision,
		Status:             string(CoordinationPlanning),
		AssignmentRefs:     make([]AssignmentRecoveryRef, 0, len(req.Objectives)),
	}
	multiRef.DeriveAssignmentRefs = func() []AssignmentRecoveryRef {
		refs := make([]AssignmentRecoveryRef, 0, len(ac.assignments))
		for _, a := range ac.assignments {
			refs = append(refs, AssignmentRecoveryRef{
				AssignmentID:       a.ID,
				WorkerID:           a.WorkerRef.WorkerID,
				ChildInteractionID: a.ChildInteractionID,
				ChildGoalID:        a.ChildGoalID,
				Status:             string(a.Status),
			})
		}
		return refs
	}

	assignmentIDs := make([]string, 0, len(req.Objectives))
	for i, obj := range req.Objectives {
		if obj.WorkerIndex >= len(req.WorkerRefs) || obj.WorkerIndex < 0 {
			return nil, fmt.Errorf("multi_agent: objective %d references missing worker index %d", i, obj.WorkerIndex)
		}
		wref := req.WorkerRefs[obj.WorkerIndex]
		a := &AgentAssignment{
			ID:                 uuid.NewString(),
			CoordinationID:     string(coordID),
			ParentGoalID:       req.ParentGoalID,
			ParentGoalRevision: req.ParentGoalRevision,
			WorkerRef:          wref,
			Objective:          truncateObjective(obj.Objective),
			Constraints:        obj.Constraints,
			ExpectedOutcome:    truncateOutcome(obj.ExpectedOutcome),
			Dependencies:       obj.Dependencies,
			Status:             AssignmentPending,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		ac.assignments = append(ac.assignments, a)
		multiRef.AssignmentRefs = append(multiRef.AssignmentRefs, AssignmentRecoveryRef{
			AssignmentID: a.ID,
			WorkerID:     wref.WorkerID,
			Status:       string(AssignmentPending),
		})
		assignmentIDs = append(assignmentIDs, a.ID)
	}

	if req.ParentInteractionID != "" {
		if err := c.attachRecoveryDescriptor(ctx, req.ParentInteractionID, multiRef); err != nil {
			return nil, fmt.Errorf("multi_agent: persist coordination descriptor: %w", err)
		}
	}

	c.mu.Lock()
	c.coordinations[string(coordID)] = ac
	c.mu.Unlock()

	ac.status = CoordinationRunning
	multiRef.Status = string(CoordinationRunning)

	if err := c.launchReadyAssignments(ctx, ac); err != nil {
		return nil, fmt.Errorf("multi_agent: launch initial assignments: %w", err)
	}

	return &StartCoordinationResult{
		CoordinationID: coordID,
		AssignmentIDs:  assignmentIDs,
	}, nil
}

func (c *MultiAgentCoordinator) attachRecoveryDescriptor(ctx context.Context, parentInteractionID string, multiRef *MultiAgentRecoveryRef) error {
	record, ok, err := c.tracker.Get(ctx, parentInteractionID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("multi_agent: parent interaction not found: %s", parentInteractionID)
	}
	var updated RecoveryDescriptor
	if record.RecoveryDescriptor != nil {
		updated = *record.RecoveryDescriptor
	} else {
		updated = RecoveryDescriptor{
			SchemaVersion: RecoveryDescriptorSchemaVersion,
			Requirement:   RecoveryBestEffort,
			Interaction:   RecoveryInteractionRef{InteractionID: parentInteractionID},
			State:         RecoveryDescriptorActive,
			CreatedAt:     c.clock.Now(),
			UpdatedAt:     c.clock.Now(),
		}
	}
	updated.MultiAgent = multiRef
	updated.UpdatedAt = c.clock.Now()
	updated.ComputeFingerprint()
	result, err := c.tracker.UpdateMetadata(ctx, parentInteractionID, InteractionMetadataUpdate{
		RecoveryDescriptor: &updated,
	})
	if err != nil {
		return err
	}
	_ = result
	return nil
}

func (c *MultiAgentCoordinator) launchReadyAssignments(ctx context.Context, ac *activeCoordination) error {
	if ac.strategy == CoordinationSequential {
		hasRunning := false
		for _, a := range ac.assignments {
			if a.Status == AssignmentRunning {
				hasRunning = true
				break
			}
		}
		if hasRunning {
			return nil
		}
		for _, a := range ac.assignments {
			if a.Status == AssignmentPending && c.dependenciesSatisfied(ac, a) {
				return c.startOneAssignment(ctx, ac, a)
			}
		}
		return nil
	}

	activeCount := 0
	for _, a := range ac.assignments {
		if a.Status == AssignmentRunning {
			activeCount++
		}
	}

	for _, a := range ac.assignments {
		if activeCount >= c.policy.MaxWorkers {
			break
		}
		if a.Status == AssignmentPending && c.dependenciesSatisfied(ac, a) {
			if err := c.startOneAssignment(ctx, ac, a); err != nil {
				return err
			}
			activeCount++
		}
	}
	return nil
}

func (c *MultiAgentCoordinator) dependenciesSatisfied(ac *activeCoordination, a *AgentAssignment) bool {
	if len(a.Dependencies) == 0 {
		return true
	}
	for _, depID := range a.Dependencies {
		depAssigned := false
		for _, other := range ac.assignments {
			if other.ID != depID {
				continue
			}
			depAssigned = true
			if other.Status != AssignmentSucceeded {
				return false
			}
		}
		if !depAssigned {
			return false
		}
	}
	return true
}

func (c *MultiAgentCoordinator) startOneAssignment(ctx context.Context, ac *activeCoordination, a *AgentAssignment) error {
	if c.starter == nil {
		return fmt.Errorf("multi_agent: no worker starter configured")
	}
	now := c.clock.Now()
	a.Status = AssignmentRunning
	a.UpdatedAt = now

	addr := WorkerRunRequest{
		CoordinationID:      string(ac.id),
		AssignmentID:        a.ID,
		ParentInteractionID: ac.parentInteractionID,
		ParentGoalID:        ac.parentGoalID,
		ParentGoalRevision:  ac.parentGoalRev,
		CharacterID:         a.WorkerRef.CharacterID,
		Source:              "multi_agent",
		Objective:           a.Objective,
		ExpectedOutcome:     a.ExpectedOutcome,
		CoordinationDepth:   ac.depth + 1,
	}

	childID, err := c.starter.StartWorker(ctx, addr)
	if err != nil {
		a.Status = AssignmentFailed
		a.Error = err.Error()
		a.UpdatedAt = c.clock.Now()
		return err
	}
	a.ChildInteractionID = childID
	a.UpdatedAt = c.clock.Now()
	_ = now
	return nil
}

func (c *MultiAgentCoordinator) OnAssignmentTerminal(ctx context.Context, assignmentID string, result AgentAssignmentResult) error {
	c.mu.Lock()

	var ac *activeCoordination
	var target *AgentAssignment
	for _, candidate := range c.coordinations {
		for _, a := range candidate.assignments {
			if a.ID == assignmentID {
				ac = candidate
				target = a
				break
			}
		}
		if ac != nil {
			break
		}
	}
	if ac == nil || target == nil {
		c.mu.Unlock()
		return fmt.Errorf("multi_agent: unknown assignment %s", assignmentID)
	}

	if target.Status.IsTerminal() {
		c.mu.Unlock()
		return nil
	}
	now := c.clock.Now()
	target.Status = result.Status
	target.UpdatedAt = now
	if result.Error != nil {
		target.Error = result.Error.Code + ": " + result.Error.Message
	}

	allTerminal := true
	anyFailed := false
	anySucceeded := false
	anyCancelled := false
	for _, a := range ac.assignments {
		if !a.Status.IsTerminal() {
			allTerminal = false
		}
		switch a.Status {
		case AssignmentFailed:
			anyFailed = true
		case AssignmentSucceeded:
			anySucceeded = true
		case AssignmentCancelled:
			anyCancelled = true
		}
	}

	switch {
	case ac.completionPlan == CoordinationFirstSuccess && anySucceeded:
		ac.status = CoordinationAggregating
	case ac.completionPlan == CoordinationBestEffort && (anyFailed && !anySucceeded):
		ac.status = CoordinationAggregating
	case allTerminal:
		ac.status = CoordinationAggregating
	case ac.completionPlan == CoordinationRequireAll && anyFailed:
		pendingLeft := false
		for _, a := range ac.assignments {
			if a.Status == AssignmentPending || a.Status == AssignmentRunning || a.Status == AssignmentWaiting {
				pendingLeft = true
			}
		}
		if !pendingLeft {
			ac.status = CoordinationAggregating
		}
	case ac.deadlineAt != nil && !c.clock.Now().Before(*ac.deadlineAt):
		ac.status = CoordinationAggregating
	}

	shouldAggregate := ac.status == CoordinationAggregating

	if !allTerminal && !shouldAggregate {
		c.mu.Unlock()
		_ = c.launchReadyAssignments(ctx, ac)
		c.mu.Lock()
	}

	ac.updatedAt = now
	_ = anySucceeded
	_ = anyCancelled

	if shouldAggregate {
		c.mu.Unlock()
		_ = c.aggregate(ctx, ac)
		return nil
	}

	c.mu.Unlock()
	return nil
}

func (c *MultiAgentCoordinator) aggregate(ctx context.Context, ac *activeCoordination) error {
	if ac.status == CoordinationSucceeded || ac.status == CoordinationFailed || ac.status == CoordinationCancelled {
		return nil
	}
	now := c.clock.Now()

	succeeded := 0
	failed := 0
	cancelled := 0

	sort.Slice(ac.assignments, func(i, j int) bool {
		return ac.assignments[i].ID < ac.assignments[j].ID
	})

	for _, a := range ac.assignments {
		switch a.Status {
		case AssignmentSucceeded:
			succeeded++
		case AssignmentFailed:
			failed++
		case AssignmentCancelled:
			cancelled++
		}
	}

	allDone := true
	for _, a := range ac.assignments {
		if a.Status == AssignmentSucceeded && a.ChildInteractionID != "" {
			record, ok, err := c.tracker.Get(ctx, a.ChildInteractionID)
			if err == nil && ok && record != nil {
				_ = record
			}
		}
		if !a.Status.IsTerminal() {
			allDone = false
		}
	}
	_ = allDone

	aggObsID := BuildAggregateObservationID(string(ac.id), ac.parentGoalRev)

	aggObs := &decision.Observation{
		Version:        decision.ObservationVersionV1,
		ID:             aggObsID,
		InteractionID:  ac.parentInteractionID,
		GoalIDs:        []string{ac.parentGoalID},
		GoalRefs: []decision.GoalRef{
			{ID: ac.parentGoalID, Revision: ac.parentGoalRev},
		},
		Kind:           decision.ObservationKindCoordinationResult,
		TargetKind:     decision.ObservationTargetCoordination,
		Outcome:        c.deriveCoordinationOutcome(ac),
		TaskRunID:      string(ac.id),
		ObservedAt:     now,
	}

	_ = aggObs

	coordOutcome := c.deriveCoordinationOutcome(ac)
	switch coordOutcome {
	case decision.ObservationOutcomeSucceeded:
		ac.status = CoordinationSucceeded
	default:
		ac.status = CoordinationFailed
	}
	ac.updatedAt = now
	_ = cancelled

	if ac.parentInteractionID != "" {
		meta := map[string]any{
			"multi_agent_coordination_id":    string(ac.id),
			"multi_agent_status":             string(ac.status),
			"multi_agent_succeeded_workers":  succeeded,
			"multi_agent_failed_workers":     failed,
			"multi_agent_cancelled_workers":  cancelled,
			"multi_agent_aggregate_obs_id":   aggObsID,
		}
		_ = meta
	}

	return nil
}

func (c *MultiAgentCoordinator) deriveCoordinationOutcome(ac *activeCoordination) decision.ObservationOutcome {
	if ac == nil {
		return decision.ObservationOutcomeFailed
	}
	if ac.completionPlan == CoordinationBestEffort {
		for _, a := range ac.assignments {
			if a.Status == AssignmentSucceeded {
				return decision.ObservationOutcomeSucceeded
			}
		}
		return decision.ObservationOutcomeFailed
	}
	if ac.completionPlan == CoordinationFirstSuccess {
		for _, a := range ac.assignments {
			if a.Status == AssignmentSucceeded {
				return decision.ObservationOutcomeSucceeded
			}
		}
		return decision.ObservationOutcomeFailed
	}
	allOK := len(ac.assignments) > 0
	anySucceeded := false
	for _, a := range ac.assignments {
		if a.Status == AssignmentSucceeded {
			anySucceeded = true
		}
		if a.Status != AssignmentSucceeded {
			allOK = false
		}
	}
	if allOK {
		return decision.ObservationOutcomeSucceeded
	}
	if anySucceeded {
		return decision.ObservationOutcomeFailed
	}
	return decision.ObservationOutcomeFailed
}

func (c *MultiAgentCoordinator) Reconcile(ctx context.Context, coordinationID string) error {
	c.mu.Lock()
	ac, ok := c.coordinations[coordinationID]
	c.mu.Unlock()
	if !ok {
		return nil
	}
	if ac.status.IsTerminal() {
		return nil
	}

	now := c.clock.Now()
	changed := false
	for _, a := range ac.assignments {
		if a.Status == AssignmentRunning && a.ChildInteractionID != "" {
			record, found, err := c.tracker.Get(ctx, a.ChildInteractionID)
			if err != nil || !found {
				continue
			}
			if record.IsTerminal() && !a.Status.IsTerminal() {
				changed = true
			}
			_ = record
			_ = now
		}
	}

	if changed {
		if allTerminal(ac) {
			ac.status = CoordinationAggregating
			_ = c.aggregate(ctx, ac)
		} else {
			_ = c.launchReadyAssignments(ctx, ac)
		}
	}
	return nil
}

func allTerminal(ac *activeCoordination) bool {
	for _, a := range ac.assignments {
		if !a.Status.IsTerminal() {
			return false
		}
	}
	return true
}

func (c *MultiAgentCoordinator) ReconcilePendingCoordinations(ctx context.Context) error {
	return c.tracker.Range(ctx, func(record *InteractionRecord) bool {
		if record.RecoveryDescriptor == nil || record.RecoveryDescriptor.MultiAgent == nil {
			return true
		}
		multiRef := record.RecoveryDescriptor.MultiAgent
		if CoordinationStatus(multiRef.Status).IsTerminal() {
			return true
		}
		_ = c.Reconcile(ctx, multiRef.CoordinationID)
		return true
	})
}

func (c *MultiAgentCoordinator) Cancel(ctx context.Context, coordinationID string) error {
	c.mu.Lock()
	ac, ok := c.coordinations[coordinationID]
	c.mu.Unlock()
	if !ok {
		return nil
	}
	if ac.status.IsTerminal() {
		return nil
	}
	now := c.clock.Now()
	ac.status = CoordinationCancelled
	ac.updatedAt = now

	for _, a := range ac.assignments {
		if a.Status.IsTerminal() {
			continue
		}
		if a.ChildInteractionID != "" {
			_ = c.tracker.RequestCancel(ctx, a.ChildInteractionID, "coordination_cancelled")
		}
		if !a.Status.IsTerminal() {
			a.Status = AssignmentCancelled
			a.UpdatedAt = now
		}
	}
	return nil
}

func (c *MultiAgentCoordinator) Pause(ctx context.Context, coordinationID string) error {
	c.mu.Lock()
	ac, ok := c.coordinations[coordinationID]
	c.mu.Unlock()
	if !ok {
		return nil
	}
	if ac.status == CoordinationPaused {
		return nil
	}
	if ac.status.IsTerminal() {
		return nil
	}

	allPaused := true
	anyPaused := false
	anyRunning := false
	now := c.clock.Now()
	for _, a := range ac.assignments {
		if a.Status == AssignmentRunning && a.ChildInteractionID != "" {
			anyRunning = true
			if c.pauseSvc != nil {
				if err := c.pauseSvc.Pause(ctx, a.ChildInteractionID, "coordination_pause"); err != nil {
					allPaused = false
				} else {
					a.Status = AssignmentPaused
					a.UpdatedAt = now
					anyPaused = true
				}
			}
		} else if a.Status == AssignmentSucceeded || a.Status == AssignmentFailed || a.Status == AssignmentCancelled {
			// already terminal
		} else if a.Status == AssignmentPaused {
			anyPaused = true
		}
	}
	_ = anyRunning

	if anyPaused && allPaused {
		ac.status = CoordinationPaused
		ac.updatedAt = now
	} else if !allPaused {
		ac.status = CoordinationRunning
		ac.updatedAt = now
	}
	return nil
}

func (c *MultiAgentCoordinator) Resume(ctx context.Context, coordinationID string) error {
	c.mu.Lock()
	ac, ok := c.coordinations[coordinationID]
	c.mu.Unlock()
	if !ok {
		return nil
	}
	if ac.status != CoordinationPaused {
		return nil
	}
	now := c.clock.Now()
	ac.status = CoordinationRunning
	ac.updatedAt = now

	if parentGoal, goalOK := c.goals.Get(ac.parentGoalID); goalOK {
		if parentGoal.Revision != ac.parentGoalRev {
			ac.status = CoordinationCancelled
			for _, a := range ac.assignments {
				if !a.Status.IsTerminal() {
					a.Status = AssignmentCancelled
					a.UpdatedAt = now
				}
			}
			return nil
		}
	}

	for _, a := range ac.assignments {
		if a.Status == AssignmentPaused && a.ChildInteractionID != "" && c.pauseSvc != nil {
			if err := c.pauseSvc.Resume(ctx, a.ChildInteractionID); err == nil {
				a.Status = AssignmentRunning
				a.UpdatedAt = now
			}
		}
	}

	_ = c.launchReadyAssignments(ctx, ac)
	return nil
}

func (c *MultiAgentCoordinator) DetectConflicts(ac *activeCoordination) []CoordinationConflict {
	if ac == nil {
		return nil
	}
	conflicts := make([]CoordinationConflict, 0)
	seen := make(map[string]bool)

	for _, a := range ac.assignments {
		if a.Status == AssignmentSucceeded && a.ParentGoalRevision != 0 {
			if current, ok := c.goals.Get(a.ParentGoalID); ok {
				if current.Revision != a.ParentGoalRevision && current.Revision != ac.parentGoalRev {
					key := a.ID + ":stale"
					if !seen[key] {
						seen[key] = true
						conflicts = append(conflicts, CoordinationConflict{
							Kind:          ConflictStaleResult,
							AssignmentID:  a.ID,
							Detail:        fmt.Sprintf("goal revision changed from %d to %d", a.ParentGoalRevision, current.Revision),
						})
					}
				}
			}
		}
	}

	for i := 0; i < len(ac.assignments); i++ {
		for j := i + 1; j < len(ac.assignments); j++ {
			a := ac.assignments[i]
			b := ac.assignments[j]
			if a.ID == b.ID {
				continue
			}
			if a.ID > b.ID {
				a, b = b, a
			}
			if a.WorkerRef.WorkerID == b.WorkerRef.WorkerID && a.ChildGoalID != "" && a.ChildGoalID == b.ChildGoalID {
				key := a.ID + ":" + b.ID + ":dup"
				if !seen[key] {
					seen[key] = true
					conflicts = append(conflicts, CoordinationConflict{
						Kind:         ConflictDuplicateAssignment,
						AssignmentID: a.ID,
						Detail:       fmt.Sprintf("duplicate assignment %s / %s sharing goal %s", a.ID, b.ID, a.ChildGoalID),
					})
				}
			}
			if a.WorkerRef.WorkerID == b.WorkerRef.WorkerID && objectiveFingerprint(a.Objective) == objectiveFingerprint(b.Objective) && a.ChildGoalID == "" && b.ChildGoalID == "" {
				key := a.ID + ":" + b.ID + ":obj"
				if !seen[key] {
					seen[key] = true
					conflicts = append(conflicts, CoordinationConflict{
						Kind:         ConflictDuplicateAssignment,
						AssignmentID: a.ID,
						Detail:       fmt.Sprintf("potential duplicate worker+objective for %s / %s", a.ID, b.ID),
					})
				}
			}
		}
	}

	sort.Slice(conflicts, func(i, j int) bool {
		if string(conflicts[i].Kind) != string(conflicts[j].Kind) {
			return string(conflicts[i].Kind) < string(conflicts[j].Kind)
		}
		return conflicts[i].AssignmentID < conflicts[j].AssignmentID
	})

	return conflicts
}

type CoordinationConflict struct {
	Kind         CoordinationConflictKind `json:"kind"`
	AssignmentID string                   `json:"assignmentId"`
	Detail       string                   `json:"detail"`
}

type CoordinationConflictKind string

const (
	ConflictStaleResult        CoordinationConflictKind = "stale_result"
	ConflictGoalRevision       CoordinationConflictKind = "goal_revision"
	ConflictDuplicateAssignment CoordinationConflictKind = "duplicate_assignment"
	ConflictResource           CoordinationConflictKind = "resource"
	ConflictResultContradiction CoordinationConflictKind = "result_contradiction"
)

func ctxBackground() context.Context {
	return context.Background()
}

func BuildAggregateObservationID(coordinationID string, parentGoalRevision int64) string {
	h := sha256.New()
	h.Write([]byte("coordination"))
	h.Write([]byte{0x00})
	h.Write([]byte(coordinationID))
	h.Write([]byte{0x00})
	h.Write([]byte(fmt.Sprintf("%d", parentGoalRevision)))
	h.Write([]byte(":aggregate"))
	return "obs-cord:" + hex.EncodeToString(h.Sum(nil))[:32]
}

func BuildCoordinationTriggerID(coordinationID string) string {
	h := sha256.New()
	h.Write([]byte("multi-agent-coordination"))
	h.Write([]byte{0x00})
	h.Write([]byte(coordinationID))
	return "coordtrigger:" + hex.EncodeToString(h.Sum(nil))[:32]
}

func objectiveFingerprint(obj string) string {
	h := sha256.New()
	h.Write([]byte(obj))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func truncateObjective(obj string) string {
	if len(obj) > 8192 {
		return obj[:8192]
	}
	return obj
}

func truncateOutcome(out string) string {
	if out == "" {
		return ""
	}
	if len(out) > 4096 {
		return out[:4096]
	}
	return out
}

type unifiedEntryWorkerRunner struct {
	entry *UnifiedEntry
}

func NewUnifiedEntryWorkerRunner(entry *UnifiedEntry) AgentWorkerRunner {
	return &unifiedEntryWorkerRunner{entry: entry}
}

func (r *unifiedEntryWorkerRunner) StartWorker(ctx context.Context, req WorkerRunRequest) (string, error) {
	res, err := r.entry.Handle(ctx, &UnifiedEntryRequest{
		Channel:        "web",
		Message:        req.Objective,
		UserID:         req.UserID,
		CharacterID:    req.CharacterID,
		ConversationID: req.ConversationID,
		Source:         req.Source,
		RequestID:      "ma-" + req.AssignmentID,
		IsInternal:     true,
	})
	if err != nil {
		return "", err
	}
	return res.InteractionID, nil
}
