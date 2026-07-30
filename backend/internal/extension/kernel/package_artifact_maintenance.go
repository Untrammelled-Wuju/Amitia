package kernel

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

type PackageArtifactMaintenanceOperations interface {
	ExpirePackagePreviews(ctx context.Context, now time.Time) (int64, error)
	ReleaseExpiredArtifactReferences(ctx context.Context, now time.Time) (int64, error)
	VerifyDueArtifacts(ctx context.Context, before time.Time, limit int) (PackageArtifactVerificationResult, error)
	CollectGarbage(ctx context.Context, now time.Time, retention time.Duration, limit int) (PackageArtifactGCResult, error)
}

type PackageArtifactMaintenanceConfig struct {
	Interval               time.Duration
	VerificationAge        time.Duration
	InitialVerificationAge time.Duration
	Retention              time.Duration
	BatchSize              int
	Now                    func() time.Time
	OnError                func(error)
}

type PackageArtifactMaintenanceStatus struct {
	Running       bool
	RunCount      int64
	FailureCount  int64
	LastStartedAt time.Time
	LastEndedAt   time.Time
	LastError     string
}

type PackageArtifactMaintenance struct {
	operations PackageArtifactMaintenanceOperations
	config     PackageArtifactMaintenanceConfig

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
	status PackageArtifactMaintenanceStatus
}

type packageArtifactMaintenanceOperations struct {
	repository *PackageRepository
	lifecycle  *PackageArtifactLifecycle
}

func DefaultPackageArtifactMaintenanceConfig() PackageArtifactMaintenanceConfig {
	return PackageArtifactMaintenanceConfig{Interval: 15 * time.Minute, VerificationAge: 24 * time.Hour,
		InitialVerificationAge: 7 * 24 * time.Hour, Retention: 24 * time.Hour, BatchSize: 100,
		Now: func() time.Time { return time.Now().UTC() }, OnError: func(err error) { log.Printf("package artifact maintenance: %v", err) }}
}

func NewPackageArtifactMaintenance(operations PackageArtifactMaintenanceOperations, config PackageArtifactMaintenanceConfig) (*PackageArtifactMaintenance, error) {
	if operations == nil {
		return nil, fmt.Errorf("artifact maintenance operations required")
	}
	defaults := DefaultPackageArtifactMaintenanceConfig()
	if config.Interval <= 0 {
		config.Interval = defaults.Interval
	}
	if config.VerificationAge <= 0 {
		config.VerificationAge = defaults.VerificationAge
	}
	if config.InitialVerificationAge <= 0 {
		config.InitialVerificationAge = defaults.InitialVerificationAge
	}
	if config.Retention < 0 {
		config.Retention = defaults.Retention
	}
	if config.BatchSize <= 0 {
		config.BatchSize = defaults.BatchSize
	}
	if config.Now == nil {
		config.Now = defaults.Now
	}
	if config.OnError == nil {
		config.OnError = defaults.OnError
	}
	return &PackageArtifactMaintenance{operations: operations, config: config}, nil
}

func NewPackageArtifactMaintenanceForStore(repository *PackageRepository, store *PackageArtifactStore, config PackageArtifactMaintenanceConfig) (*PackageArtifactMaintenance, error) {
	if repository == nil || store == nil {
		return nil, fmt.Errorf("artifact maintenance store unavailable")
	}
	operations := &packageArtifactMaintenanceOperations{repository: repository, lifecycle: NewPackageArtifactLifecycle(repository, store)}
	return NewPackageArtifactMaintenance(operations, config)
}

func (o *packageArtifactMaintenanceOperations) ExpirePackagePreviews(ctx context.Context, now time.Time) (int64, error) {
	return o.repository.ExpirePackagePreviews(ctx, now)
}

func (o *packageArtifactMaintenanceOperations) ReleaseExpiredArtifactReferences(ctx context.Context, now time.Time) (int64, error) {
	return o.repository.ReleaseExpiredArtifactReferences(ctx, now)
}

func (o *packageArtifactMaintenanceOperations) VerifyDueArtifacts(ctx context.Context, before time.Time, limit int) (PackageArtifactVerificationResult, error) {
	return o.lifecycle.VerifyDueArtifacts(ctx, before, limit)
}

func (o *packageArtifactMaintenanceOperations) CollectGarbage(ctx context.Context, now time.Time, retention time.Duration, limit int) (PackageArtifactGCResult, error) {
	return o.lifecycle.CollectGarbage(ctx, now, retention, limit)
}

func (m *PackageArtifactMaintenance) Start(ctx context.Context) error {
	if m == nil {
		return fmt.Errorf("artifact maintenance unavailable")
	}
	m.mu.Lock()
	if m.cancel != nil {
		m.mu.Unlock()
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.done = make(chan struct{})
	m.status.Running = true
	done := m.done
	m.mu.Unlock()
	if err := m.runInitial(runCtx); err != nil {
		m.observe(err)
	}
	go m.loop(runCtx, done)
	return nil
}

func (m *PackageArtifactMaintenance) Stop() {
	if m == nil {
		return
	}
	m.mu.Lock()
	cancel := m.cancel
	done := m.done
	if cancel == nil {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()
	cancel()
	<-done
}

func (m *PackageArtifactMaintenance) Tick(ctx context.Context) error {
	if m == nil || m.operations == nil {
		return fmt.Errorf("artifact maintenance unavailable")
	}
	now := m.config.Now().UTC()
	m.beginRun(now)
	var failures []error
	if _, err := m.operations.ExpirePackagePreviews(ctx, now); err != nil {
		failures = append(failures, fmt.Errorf("expire previews: %w", err))
	}
	if _, err := m.operations.ReleaseExpiredArtifactReferences(ctx, now); err != nil {
		failures = append(failures, fmt.Errorf("release references: %w", err))
	}
	if _, err := m.operations.VerifyDueArtifacts(ctx, now.Add(-m.config.VerificationAge), m.config.BatchSize); err != nil {
		failures = append(failures, fmt.Errorf("verify artifacts: %w", err))
	}
	if len(failures) == 0 {
		if _, err := m.operations.CollectGarbage(ctx, now, m.config.Retention, m.config.BatchSize); err != nil {
			failures = append(failures, fmt.Errorf("collect artifacts: %w", err))
		}
	}
	err := errors.Join(failures...)
	m.endRun(m.config.Now().UTC(), err)
	return err
}

func (m *PackageArtifactMaintenance) Status() PackageArtifactMaintenanceStatus {
	if m == nil {
		return PackageArtifactMaintenanceStatus{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

func (m *PackageArtifactMaintenance) loop(ctx context.Context, done chan struct{}) {
	defer close(done)
	defer func() {
		m.mu.Lock()
		m.status.Running = false
		if m.done == done {
			m.cancel = nil
			m.done = nil
		}
		m.mu.Unlock()
	}()
	ticker := time.NewTicker(m.config.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.Tick(ctx); err != nil {
				m.observe(err)
			}
		}
	}
}

func (m *PackageArtifactMaintenance) runInitial(ctx context.Context) error {
	now := m.config.Now().UTC()
	var failures []error
	if _, err := m.operations.ExpirePackagePreviews(ctx, now); err != nil {
		failures = append(failures, fmt.Errorf("initial expire previews: %w", err))
	}
	if _, err := m.operations.ReleaseExpiredArtifactReferences(ctx, now); err != nil {
		failures = append(failures, fmt.Errorf("initial release references: %w", err))
	}
	if _, err := m.operations.VerifyDueArtifacts(ctx, now.Add(-m.config.InitialVerificationAge), m.config.BatchSize); err != nil {
		failures = append(failures, fmt.Errorf("initial verify artifacts: %w", err))
	}
	return errors.Join(failures...)
}

func (m *PackageArtifactMaintenance) observe(err error) {
	if err != nil && m.config.OnError != nil {
		m.config.OnError(err)
	}
}

func (m *PackageArtifactMaintenance) beginRun(now time.Time) {
	m.mu.Lock()
	m.status.RunCount++
	m.status.LastStartedAt = now
	m.mu.Unlock()
}

func (m *PackageArtifactMaintenance) endRun(now time.Time, err error) {
	m.mu.Lock()
	m.status.LastEndedAt = now
	if err != nil {
		m.status.FailureCount++
		m.status.LastError = err.Error()
	} else {
		m.status.LastError = ""
	}
	m.mu.Unlock()
}
