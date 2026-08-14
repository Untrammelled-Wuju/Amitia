package plugin_boundary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

var (
	ErrReconcilerClosed    = errors.New("plugin_boundary: reconciler closed")
	ErrInvalidContribution = errors.New("plugin_boundary: invalid contribution")
	ErrOwnershipMismatch   = errors.New("plugin_boundary: contribution does not belong to extension")
	ErrContributionExists  = errors.New("plugin_boundary: contribution already registered")
	ErrRevisionConflict    = errors.New("plugin_boundary: revision conflict: stale event ignored")
	ErrNotFound            = errors.New("plugin_boundary: contribution not found")
	ErrPermissionDenied    = errors.New("plugin_boundary: permission denied")
	ErrScopeDenied         = errors.New("plugin_boundary: scope denied")
	ErrValidationFailed    = errors.New("plugin_boundary: contribution validation failed")
	ErrUnavailable         = errors.New("plugin_boundary: contribution runtime unavailable")
	ErrDetachFailed        = errors.New("plugin_boundary: detach failed")
)

type LifecyclePhase string

const (
	PhaseInstalled    LifecyclePhase = "installed"
	PhaseEnabled      LifecyclePhase = "enabled"
	PhaseDisabled     LifecyclePhase = "disabled"
	PhaseUpdated      LifecyclePhase = "updated"
	PhaseUninstalled  LifecyclePhase = "uninstalled"
	PhaseReconcileAll LifecyclePhase = "reconcile_all"
)

type LifecycleEvent struct {
	Phase        LifecyclePhase
	ExtensionID  domain.ExtensionID
	Version      string
	OldVersion   string
	OperationID  string
	Timestamp    time.Time
	Contribution domain.ContributionDefinition
}

type ContributionReconciler interface {
	HandleEvent(ctx context.Context, evt LifecycleEvent) error
	ReconcileExtension(ctx context.Context, extID domain.ExtensionID) error
	DetachExtension(ctx context.Context, extID domain.ExtensionID) error
	View() ContributionRegistryView
	Get(ref ContributionRef) (ContributionRegistration, bool)
	IsExecutable(ref ContributionRef) bool
}

type KernelContributionSource interface {
	ListContributions(ctx context.Context, extID domain.ExtensionID) ([]domain.ContributionDefinition, error)
	GetInstallation(ctx context.Context, id domain.ExtensionID) (domain.ExtensionInstallation, error)
}

type ReconcilerOption func(*Reconciler)

func WithLogger(l *log.Logger) ReconcilerOption {
	return func(r *Reconciler) {
		if l != nil {
			r.logger = l
		}
	}
}

func WithClock(fn func() time.Time) ReconcilerOption {
	return func(r *Reconciler) {
		if fn != nil {
			r.now = fn
		}
	}
}

type Reconciler struct {
	mu       sync.RWMutex
	source   KernelContributionSource
	registry map[string]ContributionRegistration
	enabled  map[string]bool
	logger   *log.Logger
	now      func() time.Time

	adapters []ContributionAdapter
}

type ContributionAdapter interface {
	Kind() ContributionKind
	Register(ctx context.Context, ref ContributionRef, def map[string]any, rev int, enabled bool) (ContributionRegistration, error)
	Detach(ctx context.Context, reg ContributionRegistration) error
	Validate(ctx context.Context, ref ContributionRef, def map[string]any) error
}

func NewReconciler(source KernelContributionSource, adapters []ContributionAdapter, opts ...ReconcilerOption) *Reconciler {
	r := &Reconciler{
		source:   source,
		registry: make(map[string]ContributionRegistration),
		enabled:  make(map[string]bool),
		logger:   log.Default(),
		now:      func() time.Time { return time.Now().UTC() },
		adapters: adapters,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *Reconciler) AttachSource(source KernelContributionSource) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.source = source
}

func (r *Reconciler) HandleEvent(ctx context.Context, evt LifecycleEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if evt.Contribution.ExtensionID != "" && evt.Contribution.ExtensionID != evt.ExtensionID {
		return ErrOwnershipMismatch
	}

	switch evt.Phase {
	case PhaseInstalled:
		return r.handleInstalledLocked(ctx, evt)
	case PhaseEnabled:
		return r.handleEnabledLocked(evt)
	case PhaseDisabled:
		return r.handleDisabledLocked(ctx, evt)
	case PhaseUpdated:
		return r.handleReconcileExtensionLocked(ctx, evt.ExtensionID)
	case PhaseUninstalled:
		return r.handleExtensionUninstallLocked(ctx, evt.ExtensionID)
	case PhaseReconcileAll:
		return r.handleReconcileExtensionLocked(ctx, evt.ExtensionID)
	default:
		return fmt.Errorf("plugin_boundary: unknown lifecycle phase: %s", evt.Phase)
	}
}

func (r *Reconciler) handleInstalledLocked(ctx context.Context, evt LifecycleEvent) error {
	ref := ContributionRefFromDefinition(evt.Contribution)
	if _, exists := r.registry[ref.Key()]; exists {
		existing := r.registry[ref.Key()]
		if existing.Revision >= revisionOf(evt.Contribution) {
			r.log("install: skip existing %s rev %d >= %d", ref.Key(), existing.Revision, revisionOf(evt.Contribution))
			return nil
		}
	}
	reg, err := r.contribToRegistrationLocked(ctx, evt.Contribution, false)
	if err != nil {
		return err
	}
	r.registry[ref.Key()] = reg
	r.enabled[ref.Key()] = false
	r.log("install: registered %s kind=%s rev=%d", ref.Key(), reg.Kind, reg.Revision)
	return nil
}

func (r *Reconciler) handleEnabledLocked(evt LifecycleEvent) error {
	if evt.Contribution.ID != "" {
		ref := ContributionRefFromDefinition(evt.Contribution)
		reg, ok := r.registry[ref.Key()]
		if !ok {
			return fmt.Errorf("%w: %s", ErrNotFound, ref.Key())
		}
		r.enabled[ref.Key()] = true
		reg.Status = ContributionStatusActive
		reg.UpdatedAt = r.now().Format(time.RFC3339Nano)
		r.registry[ref.Key()] = reg
		r.log("enable: contribution %s active", ref.Key())
		return nil
	}
	for key, reg := range r.registry {
		if reg.Ref.ExtensionID != evt.ExtensionID {
			continue
		}
		r.enabled[key] = true
		reg.Status = ContributionStatusActive
		reg.UpdatedAt = r.now().Format(time.RFC3339Nano)
		r.registry[key] = reg
		r.log("enable: contribution %s active", key)
	}
	return nil
}

func (r *Reconciler) handleDisabledLocked(ctx context.Context, evt LifecycleEvent) error {
	var firstErr error
	if evt.Contribution.ID != "" {
		ref := ContributionRefFromDefinition(evt.Contribution)
		reg, ok := r.registry[ref.Key()]
		if !ok {
			return nil
		}
		if err := r.detachSingleLocked(ctx, reg.Ref); err != nil {
			r.log("disable: detach %s: %v", ref.Key(), err)
			firstErr = err
		}
		r.enabled[ref.Key()] = false
		reg.Status = ContributionStatusDisabled
		reg.UpdatedAt = r.now().Format(time.RFC3339Nano)
		r.registry[ref.Key()] = reg
		r.log("disable: contribution %s disabled", ref.Key())
		return firstErr
	}
	for key, reg := range r.registry {
		if reg.Ref.ExtensionID != evt.ExtensionID {
			continue
		}
		if err := r.detachSingleLocked(ctx, reg.Ref); err != nil {
			r.log("disable: detach %s: %v", key, err)
			reg.Status = ContributionStatusInvalid
			reg.ErrorMessage = err.Error()
			r.registry[key] = reg
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		r.enabled[key] = false
		reg.Status = ContributionStatusDisabled
		reg.UpdatedAt = r.now().Format(time.RFC3339Nano)
		r.registry[key] = reg
		r.log("disable: contribution %s disabled", key)
	}
	return firstErr
}

func (r *Reconciler) handleReconcileExtensionLocked(ctx context.Context, extID domain.ExtensionID) error {
	if r.source == nil {
		return nil
	}
	contribs, err := r.source.ListContributions(ctx, extID)
	if err != nil {
		return fmt.Errorf("reconcile: list contributions: %w", err)
	}

	desired := make(map[string]domain.ContributionDefinition)
	for _, c := range contribs {
		if c.Kind != domain.ContributionKindDesktopPetPlugin {
			continue
		}
		ref := ContributionRefFromDefinition(c)
		desired[ref.Key()] = c
	}

	var firstErr error
	for key, existing := range r.registry {
		if existing.Ref.ExtensionID != extID {
			continue
		}
		if _, ok := desired[key]; !ok {
			if detachErr := r.detachSingleLocked(ctx, existing.Ref); detachErr != nil {
				r.log("reconcile: detach removed %s: %v", key, detachErr)
				if firstErr == nil {
					firstErr = detachErr
				}
			}
			delete(r.registry, key)
			delete(r.enabled, key)
			r.log("reconcile: removed %s", key)
		}
	}

	for key, c := range desired {
		newReg, convErr := r.contribToRegistrationLocked(ctx, c, r.enabled[key])
		if convErr != nil {
			r.log("reconcile: convert %s: %v", key, convErr)
			if firstErr == nil {
				firstErr = convErr
			}
			continue
		}
		if existing, ok := r.registry[key]; ok {
			if existing.Revision == newReg.Revision && existing.Status == newReg.Status {
				continue
			}
		}
		r.registry[key] = newReg
		if _, ok := r.enabled[key]; !ok {
			r.enabled[key] = false
		}
		r.log("reconcile: upserted %s rev=%d", key, newReg.Revision)
	}

	return firstErr
}

func (r *Reconciler) handleExtensionUninstallLocked(ctx context.Context, extID domain.ExtensionID) error {
	var firstErr error
	for key, reg := range r.registry {
		if reg.Ref.ExtensionID != extID {
			continue
		}
		if err := r.detachSingleLocked(ctx, reg.Ref); err != nil {
			r.log("uninstall: detach %s: %v", key, err)
			reg.Status = ContributionStatusInvalid
			reg.ErrorMessage = err.Error()
			r.registry[key] = reg
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		reg.Status = ContributionStatusDetached
		reg.UpdatedAt = r.now().Format(time.RFC3339Nano)
		r.registry[key] = reg
		delete(r.enabled, key)
		r.log("uninstall: detached %s", key)
	}
	return firstErr
}

func (r *Reconciler) detachSingleLocked(ctx context.Context, ref ContributionRef) error {
	reg, ok := r.registry[ref.Key()]
	if !ok {
		return nil
	}
	if reg.Status == ContributionStatusDetached || reg.Status == ContributionStatusInvalid {
		return nil
	}
	adapter := r.adapterForKind(reg.Kind)
	if adapter != nil {
		if err := adapter.Detach(ctx, reg); err != nil {
			return fmt.Errorf("detach %s: %w", ref.Key(), err)
		}
	}
	return nil
}

func (r *Reconciler) adapterForKind(kind ContributionKind) ContributionAdapter {
	for _, a := range r.adapters {
		if a.Kind() == kind {
			return a
		}
	}
	return nil
}

func (r *Reconciler) contribToRegistrationLocked(ctx context.Context, c domain.ContributionDefinition, enabled bool) (ContributionRegistration, error) {
	ref := ContributionRefFromDefinition(c)
	kind := inferContributionKind(c)

	status := ContributionStatusRegistered
	if enabled {
		status = ContributionStatusActive
	}

	reg := ContributionRegistration{
		Ref:          ref,
		Kind:         kind,
		Revision:     revisionOf(c),
		Status:       status,
		Definition:   cloneMap(c.Definition),
		RegisteredAt: r.now().Format(time.RFC3339Nano),
		UpdatedAt:    r.now().Format(time.RFC3339Nano),
	}

	adapter := r.adapterForKind(kind)
	if adapter == nil {
		reg.Status = ContributionStatusInvalid
		reg.ErrorMessage = fmt.Sprintf("no adapter for kind: %s", kind)
		return reg, nil
	}

	if err := adapter.Validate(ctx, ref, c.Definition); err != nil {
		reg.Status = ContributionStatusInvalid
		reg.ErrorMessage = err.Error()
		return reg, nil
	}

	filled, err := adapter.Register(ctx, ref, c.Definition, revisionOf(c), enabled)
	if err != nil {
		return ContributionRegistration{}, err
	}
	filled.Ref = ref
	filled.Kind = kind
	if !enabled && filled.Status == ContributionStatusActive {
		filled.Status = ContributionStatusRegistered
	}
	return filled, nil
}

func inferContributionKind(c domain.ContributionDefinition) ContributionKind {
	if v, ok := c.Definition["contributionKind"].(string); ok {
		return ParseContributionKind(v)
	}
	switch c.Kind {
	case domain.ContributionKindDesktopPetPlugin:
		return KindAction
	default:
		return KindUnknown
	}
}

func revisionOf(c domain.ContributionDefinition) int {
	if v, ok := c.Definition["revision"].(float64); ok {
		return int(v)
	}
	if v, ok := c.Metadata["revision"].(float64); ok {
		return int(v)
	}
	_ = json.Marshal
	return 1
}

func (r *Reconciler) ReconcileExtension(ctx context.Context, extID domain.ExtensionID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.handleReconcileExtensionLocked(ctx, extID)
}

func (r *Reconciler) DetachExtension(ctx context.Context, extID domain.ExtensionID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.handleExtensionUninstallLocked(ctx, extID)
}

func (r *Reconciler) View() ContributionRegistryView {
	r.mu.RLock()
	defer r.mu.RUnlock()
	regs := make([]ContributionRegistration, 0, len(r.registry))
	for _, reg := range r.registry {
		enabled := r.enabled[reg.Ref.Key()] && reg.Status != ContributionStatusDisabled && reg.Status != ContributionStatusDetached
		regCopy := reg
		regCopy.Status = computeVisibleStatus(regCopy.Status, enabled)
		regs = append(regs, regCopy)
	}
	return NewContributionRegistryView(regs)
}

func (r *Reconciler) Get(ref ContributionRef) (ContributionRegistration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	reg, ok := r.registry[ref.Key()]
	if !ok {
		return ContributionRegistration{}, false
	}
	reg.Status = computeVisibleStatus(reg.Status, r.enabled[ref.Key()])
	return reg, true
}

func (r *Reconciler) IsExecutable(ref ContributionRef) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	reg, ok := r.registry[ref.Key()]
	if !ok {
		return false
	}
	if reg.Status == ContributionStatusInvalid {
		return false
	}
	return r.enabled[ref.Key()]
}

func computeVisibleStatus(base ContributionStatus, enabled bool) ContributionStatus {
	switch base {
	case ContributionStatusDisabled:
		if enabled {
			return ContributionStatusActive
		}
		return ContributionStatusDisabled
	case ContributionStatusRegistered:
		if enabled {
			return ContributionStatusActive
		}
		return ContributionStatusRegistered
	default:
		return base
	}
}

func (r *Reconciler) log(fmtStr string, args ...any) {
	if r.logger != nil {
		r.logger.Printf("[desktop-pet-boundary] "+fmtStr, args...)
	}
}

func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
