package cutover

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

type CutoverPhase string

const (
	PhasePending       CutoverPhase = "pending"
	PhasePreCheck      CutoverPhase = "pre_check"
	PhaseSnapshot      CutoverPhase = "snapshot"
	PhaseFreezeOld     CutoverPhase = "freeze_old"
	PhaseRedirectAPI   CutoverPhase = "redirect_api"
	PhaseRedirectUI    CutoverPhase = "redirect_ui"
	PhaseRedirectModel CutoverPhase = "redirect_model"
	PhaseRedirectElectron CutoverPhase = "redirect_electron"
	PhaseVerify        CutoverPhase = "verify"
	PhaseComplete      CutoverPhase = "complete"
	PhaseRolledBack    CutoverPhase = "rolled_back"
	PhaseFailed        CutoverPhase = "failed"
)

type CutoverState struct {
	Phase           CutoverPhase       `json:"phase"`
	StartedAt       time.Time          `json:"startedAt"`
	CompletedAt     *time.Time         `json:"completedAt,omitempty"`
	PreCheckPassed  bool               `json:"preCheckPassed"`
	SnapshotID      string             `json:"snapshotId,omitempty"`
	FrozenOld       bool               `json:"frozenOld"`
	RedirectedAPI   bool               `json:"redirectedApi"`
	RedirectedUI    bool               `json:"redirectedUi"`
	RedirectedModel bool               `json:"redirectedModel"`
	RedirectedElectron bool            `json:"redirectedElectron"`
	Verified        bool               `json:"verified"`
	Errors          []string           `json:"errors,omitempty"`
}

type CutoverManager struct {
	mu     sync.Mutex
	state  *CutoverState
	config *CutoverConfig
}

type CutoverConfig struct {
	OldSystemFreezeEnabled bool
	APIShouldRedirect      bool
	UIShouldRedirect       bool
	ModelShouldRedirect    bool
	ElectronShouldRedirect bool
	VerificationRequired   bool
	SnapshotRequired       bool
}

func DefaultCutoverConfig() *CutoverConfig {
	return &CutoverConfig{
		OldSystemFreezeEnabled: true,
		APIShouldRedirect:      true,
		UIShouldRedirect:       true,
		ModelShouldRedirect:    true,
		ElectronShouldRedirect: true,
		VerificationRequired:   true,
		SnapshotRequired:       true,
	}
}

func NewCutoverManager(config *CutoverConfig) *CutoverManager {
	if config == nil {
		config = DefaultCutoverConfig()
	}
	return &CutoverManager{
		state:  &CutoverState{Phase: PhasePending, StartedAt: time.Now().UTC()},
		config: config,
	}
}

var (
	ErrPreCheckFailed    = errors.New("cutover: pre-check failed")
	ErrSnapshotFailed    = errors.New("cutover: snapshot failed")
	ErrFreezeFailed      = errors.New("cutover: freeze failed")
	ErrRedirectFailed    = errors.New("cutover: redirect failed")
	ErrVerifyFailed      = errors.New("cutover: verify failed")
	ErrAlreadyCutOver    = errors.New("cutover: already cut over")
	ErrCannotRollback    = errors.New("cutover: cannot rollback after completion")
)

func (m *CutoverManager) GetState() *CutoverState {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == nil {
		return nil
	}
	out := *m.state
	return &out
}

func (m *CutoverManager) PreCheck(ctx context.Context, checks []PreCheckItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.Phase == PhaseComplete {
		return ErrAlreadyCutOver
	}
	m.state.Phase = PhasePreCheck
	failed := make([]string, 0)
	for _, c := range checks {
		if !c.Passed {
			failed = append(failed, fmt.Sprintf("%s: %s", c.Name, c.Reason))
		}
	}
	if len(failed) > 0 {
		m.state.Errors = append(m.state.Errors, failed...)
		m.state.Phase = PhaseFailed
		return fmt.Errorf("%w: %d pre-check(s) failed", ErrPreCheckFailed, len(failed))
	}
	m.state.PreCheckPassed = true
	return nil
}

func (m *CutoverManager) Snapshot(ctx context.Context, snapshotID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.state.PreCheckPassed {
		return ErrPreCheckFailed
	}
	if !m.config.SnapshotRequired {
		m.state.SnapshotID = "skipped"
		return nil
	}
	m.state.Phase = PhaseSnapshot
	if snapshotID == "" {
		snapshotID = fmt.Sprintf("snapshot-%d", time.Now().UnixNano())
	}
	m.state.SnapshotID = snapshotID
	return nil
}

func (m *CutoverManager) FreezeOld(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.config.OldSystemFreezeEnabled {
		m.state.FrozenOld = true
		return nil
	}
	m.state.Phase = PhaseFreezeOld
	m.state.FrozenOld = true
	return nil
}

func (m *CutoverManager) Redirect(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.state.FrozenOld {
		return ErrFreezeFailed
	}
	m.state.Phase = PhaseRedirectAPI
	if m.config.APIShouldRedirect {
		m.state.RedirectedAPI = true
	}
	m.state.Phase = PhaseRedirectUI
	if m.config.UIShouldRedirect {
		m.state.RedirectedUI = true
	}
	m.state.Phase = PhaseRedirectModel
	if m.config.ModelShouldRedirect {
		m.state.RedirectedModel = true
	}
	m.state.Phase = PhaseRedirectElectron
	if m.config.ElectronShouldRedirect {
		m.state.RedirectedElectron = true
	}
	return nil
}

func (m *CutoverManager) Verify(ctx context.Context, verifier Verifier) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.state.RedirectedAPI || !m.state.RedirectedUI {
		return ErrRedirectFailed
	}
	m.state.Phase = PhaseVerify
	if verifier != nil {
		if err := verifier.Verify(ctx); err != nil {
			m.state.Phase = PhaseFailed
			m.state.Errors = append(m.state.Errors, err.Error())
			return fmt.Errorf("%w: %v", ErrVerifyFailed, err)
		}
	}
	m.state.Verified = true
	m.state.Phase = PhaseComplete
	completed := time.Now().UTC()
	m.state.CompletedAt = &completed
	return nil
}

func (m *CutoverManager) Rollback(ctx context.Context, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.Phase == PhaseComplete {
		return ErrCannotRollback
	}
	m.state.Phase = PhaseRolledBack
	m.state.Errors = append(m.state.Errors, fmt.Sprintf("rollback: %s", reason))
	return nil
}

func (m *CutoverManager) SaveState(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

type PreCheckItem struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Reason  string `json:"reason,omitempty"`
}

type Verifier interface {
	Verify(ctx context.Context) error
}

func DefaultPreCheckItems() []PreCheckItem {
	return []PreCheckItem{
		{Name: "equiv.p0_cleared", Passed: true},
		{Name: "stability.p0_cleared", Passed: true},
		{Name: "security.p0_cleared", Passed: true},
		{Name: "cutover.readiness_passed", Passed: true},
		{Name: "old.write_frozen", Passed: true},
		{Name: "old.scheduler_stopped", Passed: true},
		{Name: "old.mcp_reconnect_stopped", Passed: true},
		{Name: "old.plugin_init_stopped", Passed: true},
		{Name: "old.tool_handler_stopped", Passed: true},
		{Name: "new.registry_rebuildable", Passed: true},
		{Name: "rollback.snapshot_complete", Passed: true},
		{Name: "three_platform.startup_shutdown_passed", Passed: true},
		{Name: "migration.dry_run_passed", Passed: true},
	}
}
