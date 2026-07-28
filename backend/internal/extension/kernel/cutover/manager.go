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
	PhasePending          CutoverPhase = "pending"
	PhasePreCheck         CutoverPhase = "pre_check"
	PhaseSnapshot         CutoverPhase = "snapshot"
	PhaseFreezeOld        CutoverPhase = "freeze_old"
	PhaseRedirectAPI      CutoverPhase = "redirect_api"
	PhaseRedirectUI       CutoverPhase = "redirect_ui"
	PhaseRedirectModel    CutoverPhase = "redirect_model"
	PhaseRedirectElectron CutoverPhase = "redirect_electron"
	PhaseVerify           CutoverPhase = "verify"
	PhaseComplete         CutoverPhase = "complete"
	PhaseRolledBack       CutoverPhase = "rolled_back"
	PhaseFailed           CutoverPhase = "failed"
)

type CutoverState struct {
	Phase              CutoverPhase `json:"phase"`
	StartedAt          time.Time    `json:"startedAt"`
	CompletedAt        *time.Time   `json:"completedAt,omitempty"`
	PreCheckPassed     bool         `json:"preCheckPassed"`
	SnapshotID         string       `json:"snapshotId,omitempty"`
	FrozenOld          bool         `json:"frozenOld"`
	RedirectedAPI      bool         `json:"redirectedApi"`
	RedirectedUI       bool         `json:"redirectedUi"`
	RedirectedModel    bool         `json:"redirectedModel"`
	RedirectedElectron bool         `json:"redirectedElectron"`
	Verified           bool         `json:"verified"`
	Errors             []string     `json:"errors,omitempty"`
}

type CutoverManager struct {
	mu           sync.Mutex
	state        *CutoverState
	config       *CutoverConfig
	freezer      OldSystemFreezer
	redirector   Redirector
	zeroVerifier ZeroCallVerifier
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

func (m *CutoverManager) SetFreezer(freezer OldSystemFreezer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.freezer = freezer
}

func (m *CutoverManager) SetRedirector(redirector Redirector) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.redirector = redirector
}

func (m *CutoverManager) SetZeroCallVerifier(verifier ZeroCallVerifier) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.zeroVerifier = verifier
}

var (
	ErrPreCheckFailed = errors.New("cutover: pre-check failed")
	ErrSnapshotFailed = errors.New("cutover: snapshot failed")
	ErrFreezeFailed   = errors.New("cutover: freeze failed")
	ErrRedirectFailed = errors.New("cutover: redirect failed")
	ErrVerifyFailed   = errors.New("cutover: verify failed")
	ErrAlreadyCutOver = errors.New("cutover: already cut over")
	ErrCannotRollback = errors.New("cutover: cannot rollback after completion")
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
	if m.state.Phase == PhaseComplete {
		m.mu.Unlock()
		return ErrAlreadyCutOver
	}
	if !m.config.OldSystemFreezeEnabled {
		m.state.FrozenOld = true
		m.mu.Unlock()
		return nil
	}
	m.state.Phase = PhaseFreezeOld
	freezer := m.freezer
	m.mu.Unlock()

	if freezer != nil {
		freezeSteps := []struct {
			name string
			fn   func(ctx context.Context) error
		}{
			{"FreezePluginLifecycle", freezer.FreezePluginLifecycle},
			{"FreezeHookDispatch", freezer.FreezeHookDispatch},
			{"FreezeEventDispatch", freezer.FreezeEventDispatch},
			{"FreezeSchedule", freezer.FreezeSchedule},
			{"FreezeUIInjection", freezer.FreezeUIInjection},
			{"FreezeRuntime", freezer.FreezeRuntime},
			{"FreezeUpdate", freezer.FreezeUpdate},
			{"FreezeMigration", freezer.FreezeMigration},
			{"FreezeMCPReconnect", freezer.FreezeMCPReconnect},
			{"FreezeToolHandler", freezer.FreezeToolHandler},
		}
		for _, step := range freezeSteps {
			if err := step.fn(ctx); err != nil {
				m.mu.Lock()
				m.state.Phase = PhaseFailed
				m.state.Errors = append(m.state.Errors, fmt.Sprintf("freeze %s: %v", step.name, err))
				m.mu.Unlock()
				return fmt.Errorf("%w: %s: %v", ErrFreezeFailed, step.name, err)
			}
		}
	}

	m.mu.Lock()
	m.state.FrozenOld = true
	m.mu.Unlock()
	return nil
}

func (m *CutoverManager) Redirect(ctx context.Context) error {
	m.mu.Lock()
	if !m.state.FrozenOld {
		m.mu.Unlock()
		return ErrFreezeFailed
	}
	redirector := m.redirector
	config := m.config
	m.mu.Unlock()

	redirectSteps := []struct {
		name    string
		phase   CutoverPhase
		enabled bool
		fn      func(ctx context.Context) error
	}{
		{"RedirectAPI", PhaseRedirectAPI, config.APIShouldRedirect, nil},
		{"RedirectUI", PhaseRedirectUI, config.UIShouldRedirect, nil},
		{"RedirectModel", PhaseRedirectModel, config.ModelShouldRedirect, nil},
		{"RedirectElectron", PhaseRedirectElectron, config.ElectronShouldRedirect, nil},
	}
	if redirector != nil {
		redirectSteps[0].fn = redirector.RedirectAPI
		redirectSteps[1].fn = redirector.RedirectUI
		redirectSteps[2].fn = redirector.RedirectModel
		redirectSteps[3].fn = redirector.RedirectElectron
	}

	for _, step := range redirectSteps {
		m.mu.Lock()
		m.state.Phase = step.phase
		m.mu.Unlock()
		if !step.enabled {
			continue
		}
		if step.fn != nil {
			if err := step.fn(ctx); err != nil {
				m.mu.Lock()
				m.state.Phase = PhaseFailed
				m.state.Errors = append(m.state.Errors, fmt.Sprintf("redirect %s: %v", step.name, err))
				m.mu.Unlock()
				return fmt.Errorf("%w: %s: %v", ErrRedirectFailed, step.name, err)
			}
		}
		m.mu.Lock()
		switch step.name {
		case "RedirectAPI":
			m.state.RedirectedAPI = true
		case "RedirectUI":
			m.state.RedirectedUI = true
		case "RedirectModel":
			m.state.RedirectedModel = true
		case "RedirectElectron":
			m.state.RedirectedElectron = true
		}
		m.mu.Unlock()
	}
	return nil
}

func (m *CutoverManager) Verify(ctx context.Context, verifier Verifier) error {
	m.mu.Lock()
	if !m.state.RedirectedAPI || !m.state.RedirectedUI {
		m.mu.Unlock()
		return ErrRedirectFailed
	}
	m.state.Phase = PhaseVerify
	zeroVerifier := m.zeroVerifier
	m.mu.Unlock()

	if verifier != nil {
		if err := verifier.Verify(ctx); err != nil {
			m.mu.Lock()
			m.state.Phase = PhaseFailed
			m.state.Errors = append(m.state.Errors, err.Error())
			m.mu.Unlock()
			return fmt.Errorf("%w: %v", ErrVerifyFailed, err)
		}
	}

	if zeroVerifier != nil {
		report, err := zeroVerifier.VerifyZeroCalls(ctx)
		if err != nil {
			m.mu.Lock()
			m.state.Phase = PhaseFailed
			m.state.Errors = append(m.state.Errors, fmt.Sprintf("zero-call verify: %v", err))
			m.mu.Unlock()
			return fmt.Errorf("%w: zero-call verify: %v", ErrVerifyFailed, err)
		}
		if !report.AllZero {
			detail := fmt.Sprintf("zero-call check failed: plugin=%d hook=%d event=%d schedule=%d ui=%d runtime=%d update=%d migration=%d",
				report.PluginLifecycleCalls, report.HookCalls, report.EventCalls,
				report.ScheduleCalls, report.UICalls, report.RuntimeCalls,
				report.UpdateCalls, report.MigrationCalls)
			m.mu.Lock()
			m.state.Phase = PhaseFailed
			m.state.Errors = append(m.state.Errors, detail)
			for _, d := range report.Details {
				m.state.Errors = append(m.state.Errors, d)
			}
			m.mu.Unlock()
			return fmt.Errorf("%w: old system still has active calls", ErrVerifyFailed)
		}
	}

	m.mu.Lock()
	m.state.Verified = true
	m.state.Phase = PhaseComplete
	completed := time.Now().UTC()
	m.state.CompletedAt = &completed
	m.mu.Unlock()
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
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Reason string `json:"reason,omitempty"`
}

type Verifier interface {
	Verify(ctx context.Context) error
}

type OldSystemFreezer interface {
	FreezePluginLifecycle(ctx context.Context) error
	FreezeHookDispatch(ctx context.Context) error
	FreezeEventDispatch(ctx context.Context) error
	FreezeSchedule(ctx context.Context) error
	FreezeUIInjection(ctx context.Context) error
	FreezeRuntime(ctx context.Context) error
	FreezeUpdate(ctx context.Context) error
	FreezeMigration(ctx context.Context) error
	FreezeMCPReconnect(ctx context.Context) error
	FreezeToolHandler(ctx context.Context) error
}

type Redirector interface {
	RedirectAPI(ctx context.Context) error
	RedirectUI(ctx context.Context) error
	RedirectModel(ctx context.Context) error
	RedirectElectron(ctx context.Context) error
}

type ZeroCallVerifier interface {
	VerifyZeroCalls(ctx context.Context) (ZeroCallReport, error)
}

type ZeroCallReport struct {
	PluginLifecycleCalls int
	HookCalls            int
	EventCalls           int
	ScheduleCalls        int
	UICalls              int
	RuntimeCalls         int
	UpdateCalls          int
	MigrationCalls       int
	AllZero              bool
	Details              []string
}

type DefaultOldSystemFreezer struct {
	Logger func(format string, args ...any)
}

func (f *DefaultOldSystemFreezer) log(action string) {
	if f.Logger != nil {
		f.Logger("freeze old system: %s (no-op, old system already removed)", action)
	}
}

func (f *DefaultOldSystemFreezer) FreezePluginLifecycle(ctx context.Context) error {
	f.log("plugin-lifecycle")
	return nil
}

func (f *DefaultOldSystemFreezer) FreezeHookDispatch(ctx context.Context) error {
	f.log("hook-dispatch")
	return nil
}

func (f *DefaultOldSystemFreezer) FreezeEventDispatch(ctx context.Context) error {
	f.log("event-dispatch")
	return nil
}

func (f *DefaultOldSystemFreezer) FreezeSchedule(ctx context.Context) error {
	f.log("schedule")
	return nil
}

func (f *DefaultOldSystemFreezer) FreezeUIInjection(ctx context.Context) error {
	f.log("ui-injection")
	return nil
}

func (f *DefaultOldSystemFreezer) FreezeRuntime(ctx context.Context) error {
	f.log("runtime")
	return nil
}

func (f *DefaultOldSystemFreezer) FreezeUpdate(ctx context.Context) error {
	f.log("update")
	return nil
}

func (f *DefaultOldSystemFreezer) FreezeMigration(ctx context.Context) error {
	f.log("migration")
	return nil
}

func (f *DefaultOldSystemFreezer) FreezeMCPReconnect(ctx context.Context) error {
	f.log("mcp-reconnect")
	return nil
}

func (f *DefaultOldSystemFreezer) FreezeToolHandler(ctx context.Context) error {
	f.log("tool-handler")
	return nil
}

func DefaultPreCheckItems() []PreCheckItem {
	return []PreCheckItem{
		{Name: "equiv.p0_cleared", Passed: false},
		{Name: "stability.p0_cleared", Passed: false},
		{Name: "security.p0_cleared", Passed: false},
		{Name: "cutover.readiness_passed", Passed: false},
		{Name: "old.write_frozen", Passed: false},
		{Name: "old.scheduler_stopped", Passed: false},
		{Name: "old.mcp_reconnect_stopped", Passed: false},
		{Name: "old.plugin_init_stopped", Passed: false},
		{Name: "old.tool_handler_stopped", Passed: false},
		{Name: "new.registry_rebuildable", Passed: false},
		{Name: "rollback.snapshot_complete", Passed: false},
		{Name: "three_platform.startup_shutdown_passed", Passed: false},
		{Name: "migration.dry_run_passed", Passed: false},
	}
}

type PreCheckProvider interface {
	Check(ctx context.Context, name string) (passed bool, reason string, err error)
}

func (m *CutoverManager) RunPreCheck(ctx context.Context, provider PreCheckProvider) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.Phase == PhaseComplete {
		return ErrAlreadyCutOver
	}
	m.state.Phase = PhasePreCheck
	if provider == nil {
		m.state.Errors = append(m.state.Errors, "pre-check provider is nil")
		m.state.Phase = PhaseFailed
		return fmt.Errorf("%w: pre-check provider is nil", ErrPreCheckFailed)
	}
	items := DefaultPreCheckItems()
	failed := make([]string, 0)
	for _, item := range items {
		passed, reason, err := provider.Check(ctx, item.Name)
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: check error: %v", item.Name, err))
			continue
		}
		if !passed {
			if reason == "" {
				reason = "not passed"
			}
			failed = append(failed, fmt.Sprintf("%s: %s", item.Name, reason))
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
