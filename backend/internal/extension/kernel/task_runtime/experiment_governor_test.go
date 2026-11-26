//go:build rsi_experiment

package task_runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

type fakeExperimentJob struct {
	mu            sync.Mutex
	id            string
	epoch         uint64
	effects       []string
	applied       []string
	pauseCalls    int
	resumeCalls   int
	checkpoints   []ExperimentCheckpoint
	checkpointErr error
	pauseErr      error
	inflight      []string
}

func (j *fakeExperimentJob) ID() string { return j.id }

func (j *fakeExperimentJob) Epoch() uint64 {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.epoch
}

func (j *fakeExperimentJob) Effects() []string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]string(nil), j.effects...)
}

func (j *fakeExperimentJob) Checkpoint(ctx context.Context) (ExperimentCheckpoint, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.checkpointErr != nil {
		return ExperimentCheckpoint{}, j.checkpointErr
	}
	return ExperimentCheckpoint{JobID: j.id, Epoch: j.epoch, Progress: len(j.applied), CompletedEffects: append([]string(nil), j.applied...), InFlightEffects: append([]string(nil), j.inflight...)}, nil
}

func (j *fakeExperimentJob) Pause(ctx context.Context, checkpoint ExperimentCheckpoint) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.pauseErr != nil {
		return j.pauseErr
	}
	j.pauseCalls++
	j.checkpoints = append(j.checkpoints, checkpoint)
	return nil
}

func (j *fakeExperimentJob) Resume(ctx context.Context, checkpoint ExperimentCheckpoint, pending []string) (int, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.resumeCalls++
	j.applied = append(j.applied, pending...)
	return len(pending), nil
}

func (j *fakeExperimentJob) appliedCount() int {
	j.mu.Lock()
	defer j.mu.Unlock()
	return len(j.applied)
}

func (j *fakeExperimentJob) lastCheckpoint() ExperimentCheckpoint {
	j.mu.Lock()
	defer j.mu.Unlock()
	if len(j.checkpoints) == 0 {
		return ExperimentCheckpoint{}
	}
	return j.checkpoints[len(j.checkpoints)-1]
}

func newExperimentJob(id string, effects ...string) *fakeExperimentJob {
	return &fakeExperimentJob{id: id, epoch: 1, effects: effects}
}

type experimentClock struct {
	mu sync.Mutex
	at time.Time
}

func (c *experimentClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *experimentClock) Advance(delta time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(delta)
}

func newExperimentGovernor(clock *experimentClock, maxActive int) *ExperimentGovernor {
	return NewExperimentGovernor(ExperimentGovernorConfig{IdleAfter: time.Minute, CheckpointTTL: 10 * time.Minute, MaxActive: maxActive}).WithClock(clock.Now)
}

func TestExperimentGovernorIdleReclaimsAfterTTL(t *testing.T) {
	clock := &experimentClock{at: time.Unix(1700000000, 0)}
	governor := newExperimentGovernor(clock, 8)
	for i := 0; i < 2; i++ {
		if err := governor.Register(context.Background(), newExperimentJob(fmt.Sprintf("exp-%d", i), "e1")); err != nil {
			t.Fatalf("register: %v", err)
		}
	}
	if report, err := governor.SweepIdle(context.Background()); err != nil || report.Paused != 0 {
		t.Fatalf("early sweep report=%+v err=%v", report, err)
	}
	clock.Advance(2 * time.Minute)
	report, err := governor.SweepIdle(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if report.Paused != 2 || governor.Stats().Active != 0 {
		t.Fatalf("report=%+v stats=%+v", report, governor.Stats())
	}
	fmt.Printf("m07-s06 idle paused=2 active=0\n")
}

func TestExperimentGovernorForegroundPausesAndResetsIdle(t *testing.T) {
	clock := &experimentClock{at: time.Unix(1700000000, 0)}
	governor := newExperimentGovernor(clock, 8)
	for i := 0; i < 2; i++ {
		if err := governor.Register(context.Background(), newExperimentJob(fmt.Sprintf("exp-%d", i), "e1")); err != nil {
			t.Fatalf("register: %v", err)
		}
	}
	clock.Advance(2 * time.Minute)
	report, err := governor.Foreground(context.Background())
	if err != nil {
		t.Fatalf("foreground: %v", err)
	}
	idle, err := governor.SweepIdle(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if report.Paused != 2 || idle.Paused != 0 {
		t.Fatalf("foreground=%+v idle=%+v", report, idle)
	}
	fmt.Printf("m07-s06 foreground paused=2 activity_blocks_idle=true\n")
}

func TestExperimentGovernorCheckpointIsStamped(t *testing.T) {
	clock := &experimentClock{at: time.Unix(1700000000, 0)}
	governor := newExperimentGovernor(clock, 8)
	job := newExperimentJob("exp-a", "e1", "e2")
	job.applied = []string{"e1"}
	if err := governor.Register(context.Background(), job); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := governor.Foreground(context.Background()); err != nil {
		t.Fatalf("foreground: %v", err)
	}
	checkpoint := job.lastCheckpoint()
	if checkpoint.JobID != "exp-a" || checkpoint.Epoch != 1 || checkpoint.Progress != 1 || len(checkpoint.CompletedEffects) != 1 || checkpoint.CreatedAt.IsZero() {
		t.Fatalf("checkpoint=%+v", checkpoint)
	}
	fmt.Printf("m07-s06 checkpoint progress=1 effects=1 stamped=true\n")
}

func TestExperimentGovernorResumeAppliesOnlyPending(t *testing.T) {
	clock := &experimentClock{at: time.Unix(1700000000, 0)}
	governor := newExperimentGovernor(clock, 8)
	job := newExperimentJob("exp-a", "e1", "e2", "e3", "e4")
	job.applied = []string{"e1", "e2"}
	if err := governor.Register(context.Background(), job); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := governor.Foreground(context.Background()); err != nil {
		t.Fatalf("foreground: %v", err)
	}
	report, err := governor.Resume(context.Background(), "exp-a")
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if report.Applied != 2 || report.Skipped != 2 || report.Total != 4 || job.appliedCount() != 4 {
		t.Fatalf("report=%+v applied=%d", report, job.appliedCount())
	}
	fmt.Printf("m07-s06 resume applied=2 skipped=2 total=4\n")
}

func TestExperimentGovernorResumeTwiceIsNoOp(t *testing.T) {
	clock := &experimentClock{at: time.Unix(1700000000, 0)}
	governor := newExperimentGovernor(clock, 8)
	job := newExperimentJob("exp-a", "e1", "e2", "e3")
	job.applied = []string{"e1"}
	if err := governor.Register(context.Background(), job); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := governor.Foreground(context.Background()); err != nil {
		t.Fatalf("foreground: %v", err)
	}
	if _, err := governor.Resume(context.Background(), "exp-a"); err != nil {
		t.Fatalf("resume: %v", err)
	}
	second, err := governor.Resume(context.Background(), "exp-a")
	if err != nil {
		t.Fatalf("second resume: %v", err)
	}
	if !second.AlreadyActive || job.appliedCount() != 3 || job.resumeCalls != 1 {
		t.Fatalf("report=%+v applied=%d calls=%d", second, job.appliedCount(), job.resumeCalls)
	}
	fmt.Printf("m07-s06 resume_twice applied=3 calls=1\n")
}

func TestExperimentGovernorRejectsStaleLease(t *testing.T) {
	clock := &experimentClock{at: time.Unix(1700000000, 0)}
	governor := newExperimentGovernor(clock, 8)
	job := newExperimentJob("exp-a", "e1")
	if err := governor.Register(context.Background(), job); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := governor.Foreground(context.Background()); err != nil {
		t.Fatalf("foreground: %v", err)
	}
	job.mu.Lock()
	job.epoch = 2
	job.mu.Unlock()
	_, err := governor.Resume(context.Background(), "exp-a")
	if !IsTaskErrorCode(err, ErrTaskResumeStaleGeneration) {
		t.Fatalf("stale err=%v", err)
	}
	if governor.Stats().Paused != 1 {
		t.Fatalf("stats=%+v", governor.Stats())
	}
	fmt.Printf("m07-s06 stale rejected=true\n")
}

func TestExperimentGovernorRejectsExpiredCheckpoint(t *testing.T) {
	clock := &experimentClock{at: time.Unix(1700000000, 0)}
	governor := newExperimentGovernor(clock, 8)
	if err := governor.Register(context.Background(), newExperimentJob("exp-a", "e1")); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := governor.Foreground(context.Background()); err != nil {
		t.Fatalf("foreground: %v", err)
	}
	clock.Advance(11 * time.Minute)
	_, err := governor.Resume(context.Background(), "exp-a")
	if !IsTaskErrorCode(err, ErrTaskCheckpointInvalid) {
		t.Fatalf("expired err=%v", err)
	}
	fmt.Printf("m07-s06 expired rejected=true\n")
}

func TestExperimentGovernorResumeActiveIsNoOp(t *testing.T) {
	clock := &experimentClock{at: time.Unix(1700000000, 0)}
	governor := newExperimentGovernor(clock, 8)
	job := newExperimentJob("exp-a", "e1")
	if err := governor.Register(context.Background(), job); err != nil {
		t.Fatalf("register: %v", err)
	}
	report, err := governor.Resume(context.Background(), "exp-a")
	if err != nil || !report.AlreadyActive || job.resumeCalls != 0 {
		t.Fatalf("report=%+v err=%v calls=%d", report, err, job.resumeCalls)
	}
	fmt.Printf("m07-s06 not_paused already_active=true\n")
}

func TestExperimentGovernorPauseErrorIsolation(t *testing.T) {
	clock := &experimentClock{at: time.Unix(1700000000, 0)}
	governor := newExperimentGovernor(clock, 8)
	sentinel := errors.New("checkpoint failed")
	bad := newExperimentJob("exp-bad", "e1")
	bad.checkpointErr = sentinel
	if err := governor.Register(context.Background(), bad); err != nil {
		t.Fatalf("register bad: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := governor.Register(context.Background(), newExperimentJob(fmt.Sprintf("exp-%d", i), "e1")); err != nil {
			t.Fatalf("register: %v", err)
		}
	}
	report, err := governor.Foreground(context.Background())
	if !errors.Is(err, sentinel) || report.Paused != 2 || report.Failed != 1 {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	if governor.Stats().Active != 1 || governor.Stats().Paused != 2 {
		t.Fatalf("stats=%+v", governor.Stats())
	}
	fmt.Printf("m07-s06 pause errors=1 paused=2\n")
}

func TestExperimentGovernorCapacityRejects(t *testing.T) {
	clock := &experimentClock{at: time.Unix(1700000000, 0)}
	governor := newExperimentGovernor(clock, 2)
	for i := 0; i < 2; i++ {
		if err := governor.Register(context.Background(), newExperimentJob(fmt.Sprintf("exp-%d", i), "e1")); err != nil {
			t.Fatalf("register: %v", err)
		}
	}
	err := governor.Register(context.Background(), newExperimentJob("exp-2", "e1"))
	if !errors.Is(err, ErrTaskProcessCapacity) {
		t.Fatalf("capacity err=%v", err)
	}
	if governor.Stats().Active != 2 {
		t.Fatalf("stats=%+v", governor.Stats())
	}
	fmt.Printf("m07-s06 capacity rejected=true active=2\n")
}

func TestExperimentGovernorLoadDeniesResume(t *testing.T) {
	clock := &experimentClock{at: time.Unix(1700000000, 0)}
	governor := newExperimentGovernor(clock, 8)
	for i := 0; i < 2; i++ {
		if err := governor.Register(context.Background(), newExperimentJob(fmt.Sprintf("exp-%d", i), "e1")); err != nil {
			t.Fatalf("register: %v", err)
		}
	}
	governor.SetLoad(ExperimentLoadHigh)
	if governor.Stats().Paused != 2 || governor.Stats().Active != 0 {
		t.Fatalf("stats=%+v", governor.Stats())
	}
	if _, err := governor.Resume(context.Background(), "exp-0"); !errors.Is(err, ErrExperimentLoadDenied) || !IsTaskErrorCode(err, ErrTaskResumeIncompatible) {
		t.Fatalf("load resume err=%v", err)
	}
	governor.SetLoad(ExperimentLoadNormal)
	if _, err := governor.Resume(context.Background(), "exp-0"); err != nil {
		t.Fatalf("normal resume: %v", err)
	}
	fmt.Printf("m07-s06 load paused=2 denied=true code=task_resume_incompatible\n")
}

func TestExperimentGovernorRejectsInFlightEffect(t *testing.T) {
	clock := &experimentClock{at: time.Unix(1700000000, 0)}
	governor := newExperimentGovernor(clock, 8)
	job := newExperimentJob("exp-a", "e1", "e2")
	job.applied = []string{"e1"}
	job.inflight = []string{"e2"}
	if err := governor.Register(context.Background(), job); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := governor.Foreground(context.Background()); err != nil {
		t.Fatalf("foreground: %v", err)
	}
	_, err := governor.Resume(context.Background(), "exp-a")
	if !IsTaskErrorCode(err, ErrTaskExecutionAttemptInvalid) || job.resumeCalls != 0 {
		t.Fatalf("inflight err=%v calls=%d", err, job.resumeCalls)
	}
	if governor.Stats().Paused != 1 {
		t.Fatalf("stats=%+v", governor.Stats())
	}
	fmt.Printf("m07-s06 inflight rejected=true calls=0\n")
}

func TestExperimentGovernorConcurrentPauseResume(t *testing.T) {
	clock := &experimentClock{at: time.Unix(1700000000, 0)}
	governor := newExperimentGovernor(clock, 16)
	jobs := make([]*fakeExperimentJob, 8)
	for i := range jobs {
		jobs[i] = newExperimentJob(fmt.Sprintf("exp-%d", i), "e1", "e2")
		if err := governor.Register(context.Background(), jobs[i]); err != nil {
			t.Fatalf("register: %v", err)
		}
	}
	var wait sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			if worker%2 == 0 {
				_, _ = governor.Foreground(context.Background())
				return
			}
			_, _ = governor.Resume(context.Background(), fmt.Sprintf("exp-%d", worker%8))
		}(worker)
	}
	wait.Wait()
	stats := governor.Stats()
	if stats.Active+stats.Paused != 8 {
		t.Fatalf("stats=%+v", stats)
	}
	for _, job := range jobs {
		if job.appliedCount() > len(job.effects) {
			t.Fatalf("job %s applied=%d effects=%d", job.id, job.appliedCount(), len(job.effects))
		}
	}
	fmt.Printf("m07-s06 concurrent jobs=8 consistent=true\n")
}
