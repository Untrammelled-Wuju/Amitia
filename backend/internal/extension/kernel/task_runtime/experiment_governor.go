//go:build rsi_experiment

package task_runtime

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

var ErrExperimentLoadDenied = errors.New("task_runtime: experiment resume denied under load")

type ExperimentLoadDeniedError struct {
	Load ExperimentLoad
}

func (e *ExperimentLoadDeniedError) Error() string {
	return fmt.Sprintf("%v: load=%s", ErrExperimentLoadDenied, e.Load)
}

func (e *ExperimentLoadDeniedError) Is(target error) bool {
	return target == ErrExperimentLoadDenied
}

func (e *ExperimentLoadDeniedError) Unwrap() error {
	return WrapTaskError(ErrTaskResumeIncompatible, "experiment resume denied under load", ErrExperimentLoadDenied)
}

type ExperimentLoad int

const (
	ExperimentLoadIdle ExperimentLoad = iota
	ExperimentLoadNormal
	ExperimentLoadHigh
	ExperimentLoadCritical
)

func (l ExperimentLoad) String() string {
	switch l {
	case ExperimentLoadIdle:
		return "idle"
	case ExperimentLoadNormal:
		return "normal"
	case ExperimentLoadHigh:
		return "high"
	case ExperimentLoadCritical:
		return "critical"
	default:
		return "unknown"
	}
}

type ExperimentCheckpoint struct {
	JobID            string
	Epoch            uint64
	Progress         int
	CompletedEffects []string
	InFlightEffects  []string
	CreatedAt        time.Time
}

type ExperimentJob interface {
	ID() string
	Epoch() uint64
	Effects() []string
	Checkpoint(ctx context.Context) (ExperimentCheckpoint, error)
	Pause(ctx context.Context, checkpoint ExperimentCheckpoint) error
	Resume(ctx context.Context, checkpoint ExperimentCheckpoint, pending []string) (int, error)
}

type ExperimentGovernorConfig struct {
	IdleAfter       time.Duration
	CheckpointTTL   time.Duration
	MaxActive       int
}

func (c ExperimentGovernorConfig) withDefaults() ExperimentGovernorConfig {
	if c.IdleAfter <= 0 {
		c.IdleAfter = 5 * time.Minute
	}
	if c.CheckpointTTL <= 0 {
		c.CheckpointTTL = 30 * time.Minute
	}
	if c.MaxActive <= 0 {
		c.MaxActive = 8
	}
	return c
}

type experimentJobState struct {
	job        ExperimentJob
	status     string
	checkpoint ExperimentCheckpoint
	pausedAt   time.Time
}

const (
	experimentActive   = "active"
	experimentPaused   = "paused"
	experimentPausing  = "pausing"
	experimentResuming = "resuming"
)

type ExperimentGovernor struct {
	mu           sync.Mutex
	config       ExperimentGovernorConfig
	jobs         map[string]*experimentJobState
	order        []string
	lastActivity time.Time
	load         ExperimentLoad
	now          func() time.Time
	pausedTotal  int
}

func NewExperimentGovernor(config ExperimentGovernorConfig) *ExperimentGovernor {
	governor := &ExperimentGovernor{
		config: config.withDefaults(),
		jobs:   map[string]*experimentJobState{},
		now:    time.Now,
	}
	governor.lastActivity = governor.now()
	return governor
}

func (g *ExperimentGovernor) WithClock(now func() time.Time) *ExperimentGovernor {
	g.mu.Lock()
	defer g.mu.Unlock()
	if now != nil {
		g.now = now
		g.lastActivity = now()
	}
	return g
}

func (g *ExperimentGovernor) Register(ctx context.Context, job ExperimentJob) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if job == nil || job.ID() == "" {
		return NewTaskError(ErrTaskDefinitionInvalid, "experiment job required")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.jobs[job.ID()]; exists {
		return NewTaskError(ErrTaskAlreadyExists, "experiment "+job.ID())
	}
	active := 0
	for _, state := range g.jobs {
		if state.status == experimentActive {
			active++
		}
	}
	if active >= g.config.MaxActive {
		return ErrTaskProcessCapacity
	}
	g.jobs[job.ID()] = &experimentJobState{job: job, status: experimentActive}
	g.order = append(g.order, job.ID())
	return nil
}

type ExperimentPauseReport struct {
	Paused   int
	Failed   int
	Released int
}

func (g *ExperimentGovernor) Foreground(ctx context.Context) (ExperimentPauseReport, error) {
	if err := ctx.Err(); err != nil {
		return ExperimentPauseReport{}, err
	}
	g.mu.Lock()
	g.lastActivity = g.now()
	ids := g.activeIDsLocked()
	g.mu.Unlock()
	return g.pauseAll(ctx, ids)
}

func (g *ExperimentGovernor) SetLoad(level ExperimentLoad) {
	g.mu.Lock()
	g.load = level
	ids := []string{}
	if level >= ExperimentLoadHigh {
		ids = g.activeIDsLocked()
	}
	g.mu.Unlock()
	if level >= ExperimentLoadHigh && len(ids) > 0 {
		_, _ = g.pauseAll(context.Background(), ids)
	}
}

func (g *ExperimentGovernor) SweepIdle(ctx context.Context) (ExperimentPauseReport, error) {
	if err := ctx.Err(); err != nil {
		return ExperimentPauseReport{}, err
	}
	g.mu.Lock()
	idleFor := g.now().Sub(g.lastActivity)
	if idleFor < g.config.IdleAfter {
		g.mu.Unlock()
		return ExperimentPauseReport{}, nil
	}
	ids := g.activeIDsLocked()
	g.mu.Unlock()
	return g.pauseAll(ctx, ids)
}

func (g *ExperimentGovernor) activeIDsLocked() []string {
	ids := make([]string, 0, len(g.order))
	for _, id := range g.order {
		if state, ok := g.jobs[id]; ok && state.status == experimentActive {
			ids = append(ids, id)
		}
	}
	return ids
}

func (g *ExperimentGovernor) pauseAll(ctx context.Context, ids []string) (ExperimentPauseReport, error) {
	report := ExperimentPauseReport{}
	var failures []error
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		if err := g.pauseOne(ctx, id); err != nil {
			report.Failed++
			failures = append(failures, fmt.Errorf("pause %s: %w", id, err))
			continue
		}
		report.Paused++
		report.Released++
	}
	g.mu.Lock()
	g.pausedTotal += report.Paused
	g.mu.Unlock()
	return report, errors.Join(failures...)
}

func (g *ExperimentGovernor) pauseOne(ctx context.Context, id string) error {
	g.mu.Lock()
	state, ok := g.jobs[id]
	if !ok || state.status != experimentActive {
		g.mu.Unlock()
		return nil
	}
	job := state.job
	state.status = experimentPausing
	g.mu.Unlock()
	checkpoint, err := job.Checkpoint(ctx)
	if err != nil {
		g.abortPause(id)
		return err
	}
	if checkpoint.JobID == "" {
		checkpoint.JobID = job.ID()
	}
	if checkpoint.Epoch == 0 {
		checkpoint.Epoch = job.Epoch()
	}
	if checkpoint.CreatedAt.IsZero() {
		checkpoint.CreatedAt = g.now()
	}
	if err := job.Pause(ctx, checkpoint); err != nil {
		g.abortPause(id)
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	state = g.jobs[id]
	if state.status == experimentPausing {
		state.status = experimentPaused
		state.checkpoint = checkpoint
		state.pausedAt = g.now()
	}
	return nil
}

func (g *ExperimentGovernor) abortPause(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if state, ok := g.jobs[id]; ok && state.status == experimentPausing {
		state.status = experimentActive
	}
}

type ExperimentResumeReport struct {
	Applied       int
	Skipped       int
	Total         int
	AlreadyActive bool
}

func (g *ExperimentGovernor) Resume(ctx context.Context, id string) (ExperimentResumeReport, error) {
	if err := ctx.Err(); err != nil {
		return ExperimentResumeReport{}, err
	}
	g.mu.Lock()
	state, ok := g.jobs[id]
	if !ok {
		g.mu.Unlock()
		return ExperimentResumeReport{}, NewTaskError(ErrTaskNotFound, "experiment "+id)
	}
	if state.status != experimentPaused {
		g.mu.Unlock()
		return ExperimentResumeReport{AlreadyActive: true}, nil
	}
	if g.load >= ExperimentLoadHigh {
		g.mu.Unlock()
		return ExperimentResumeReport{}, &ExperimentLoadDeniedError{Load: g.load}
	}
	checkpoint := state.checkpoint
	job := state.job
	state.status = experimentResuming
	g.mu.Unlock()
	if checkpoint.JobID != job.ID() {
		g.abortResume(id)
		return ExperimentResumeReport{}, NewTaskError(ErrTaskCheckpointInvalid, "checkpoint job mismatch")
	}
	if checkpoint.Epoch != job.Epoch() {
		g.abortResume(id)
		return ExperimentResumeReport{}, NewTaskError(ErrTaskResumeStaleGeneration, fmt.Sprintf("checkpoint epoch=%d job epoch=%d", checkpoint.Epoch, job.Epoch()))
	}
	if g.config.CheckpointTTL > 0 && g.now().Sub(checkpoint.CreatedAt) > g.config.CheckpointTTL {
		g.abortResume(id)
		return ExperimentResumeReport{}, NewTaskError(ErrTaskCheckpointInvalid, "checkpoint expired")
	}
	completed := map[string]bool{}
	for _, effect := range checkpoint.CompletedEffects {
		completed[effect] = true
	}
	inFlight := map[string]bool{}
	for _, effect := range checkpoint.InFlightEffects {
		inFlight[effect] = true
	}
	for effect := range inFlight {
		if !completed[effect] {
			g.abortResume(id)
			return ExperimentResumeReport{}, NewTaskError(ErrTaskExecutionAttemptInvalid, "effect started but unconfirmed: "+effect)
		}
	}
	pending := make([]string, 0, len(job.Effects()))
	skipped := 0
	seen := map[string]bool{}
	for _, effect := range job.Effects() {
		if seen[effect] {
			continue
		}
		seen[effect] = true
		if completed[effect] {
			skipped++
			continue
		}
		pending = append(pending, effect)
	}
	sort.Strings(pending)
	applied, err := job.Resume(ctx, checkpoint, pending)
	if err != nil {
		g.abortResume(id)
		return ExperimentResumeReport{}, err
	}
	g.mu.Lock()
	state = g.jobs[id]
	state.status = experimentActive
	g.mu.Unlock()
	return ExperimentResumeReport{Applied: applied, Skipped: skipped, Total: applied + skipped}, nil
}

func (g *ExperimentGovernor) abortResume(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if state, ok := g.jobs[id]; ok && state.status == experimentResuming {
		state.status = experimentPaused
	}
}

type ExperimentGovernorStats struct {
	Active      int
	Paused      int
	Load        ExperimentLoad
	PausedTotal int
}

func (g *ExperimentGovernor) Stats() ExperimentGovernorStats {
	g.mu.Lock()
	defer g.mu.Unlock()
	stats := ExperimentGovernorStats{Load: g.load, PausedTotal: g.pausedTotal}
	for _, state := range g.jobs {
		if state.status == experimentActive {
			stats.Active++
		} else {
			stats.Paused++
		}
	}
	return stats
}
