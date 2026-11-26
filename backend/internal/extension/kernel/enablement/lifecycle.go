//go:build rsi_lifecycle

package enablement

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type LifecycleState string

const (
	LifecycleAbsent    LifecycleState = "absent"
	LifecycleInstalled LifecycleState = "installed"
	LifecycleEnabled   LifecycleState = "enabled"
	LifecycleActivated LifecycleState = "activated"
	LifecycleWarm      LifecycleState = "warm"
	LifecycleRunning   LifecycleState = "running"
	LifecycleSuspended LifecycleState = "suspended"
)

type LifecycleEvent string

const (
	EventInstall    LifecycleEvent = "install"
	EventEnable     LifecycleEvent = "enable"
	EventActivate   LifecycleEvent = "activate"
	EventWarm       LifecycleEvent = "warm"
	EventStart      LifecycleEvent = "start"
	EventDrain      LifecycleEvent = "drain"
	EventSuspend    LifecycleEvent = "suspend"
	EventResume     LifecycleEvent = "resume"
	EventDeactivate LifecycleEvent = "deactivate"
	EventDisable    LifecycleEvent = "disable"
	EventUninstall  LifecycleEvent = "uninstall"
)

var (
	ErrLifecycleInvalid    = errors.New("plugin lifecycle: invalid state or event")
	ErrLifecycleTransition = errors.New("plugin lifecycle: illegal transition")
	ErrLifecycleStale      = errors.New("plugin lifecycle: stale or replayed event epoch")
)

type Transition struct {
	From    LifecycleState
	To      LifecycleState
	Event   LifecycleEvent
	Epoch   uint64
	Changed bool
	At      time.Time
}

type lifecycleEdge struct {
	event LifecycleEvent
	to    LifecycleState
}

var lifecycleGraph = map[LifecycleState][]lifecycleEdge{
	LifecycleAbsent: {
		{EventInstall, LifecycleInstalled},
	},
	LifecycleInstalled: {
		{EventEnable, LifecycleEnabled},
		{EventUninstall, LifecycleAbsent},
	},
	LifecycleEnabled: {
		{EventActivate, LifecycleActivated},
		{EventDisable, LifecycleInstalled},
		{EventUninstall, LifecycleAbsent},
	},
	LifecycleActivated: {
		{EventWarm, LifecycleWarm},
		{EventSuspend, LifecycleSuspended},
		{EventDeactivate, LifecycleEnabled},
		{EventUninstall, LifecycleAbsent},
	},
	LifecycleWarm: {
		{EventStart, LifecycleRunning},
		{EventSuspend, LifecycleSuspended},
		{EventDeactivate, LifecycleActivated},
		{EventUninstall, LifecycleAbsent},
	},
	LifecycleRunning: {
		{EventDrain, LifecycleWarm},
		{EventSuspend, LifecycleSuspended},
	},
	LifecycleSuspended: {
		{EventResume, LifecycleActivated},
		{EventDeactivate, LifecycleEnabled},
		{EventDisable, LifecycleInstalled},
		{EventUninstall, LifecycleAbsent},
	},
}

func AllLifecycleStates() []LifecycleState {
	return []LifecycleState{LifecycleAbsent, LifecycleInstalled, LifecycleEnabled, LifecycleActivated, LifecycleWarm, LifecycleRunning, LifecycleSuspended}
}

func AllLifecycleEvents() []LifecycleEvent {
	return []LifecycleEvent{EventInstall, EventEnable, EventActivate, EventWarm, EventStart, EventDrain, EventSuspend, EventResume, EventDeactivate, EventDisable, EventUninstall}
}

func (s LifecycleState) Valid() bool {
	for _, candidate := range AllLifecycleStates() {
		if candidate == s {
			return true
		}
	}
	return false
}

func (e LifecycleEvent) Valid() bool {
	for _, candidate := range AllLifecycleEvents() {
		if candidate == e {
			return true
		}
	}
	return false
}

func CanTransition(from LifecycleState, event LifecycleEvent) (LifecycleState, bool) {
	for _, edge := range lifecycleGraph[from] {
		if edge.event == event {
			return edge.to, true
		}
	}
	return "", false
}

func LegalEvents(from LifecycleState) []LifecycleEvent {
	events := make([]LifecycleEvent, 0, len(lifecycleGraph[from]))
	for _, edge := range lifecycleGraph[from] {
		events = append(events, edge.event)
	}
	return events
}

func MinHops(from, to LifecycleState) (int, bool) {
	if !from.Valid() || !to.Valid() {
		return 0, false
	}
	if from == to {
		return 0, true
	}
	distance := map[LifecycleState]int{from: 0}
	queue := []LifecycleState{from}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, edge := range lifecycleGraph[current] {
			if _, seen := distance[edge.to]; seen {
				continue
			}
			distance[edge.to] = distance[current] + 1
			if edge.to == to {
				return distance[edge.to], true
			}
			queue = append(queue, edge.to)
		}
	}
	return 0, false
}

func (s LifecycleState) Installation() (InstallationState, error) {
	switch s {
	case LifecycleAbsent:
		return InstallationStateNotInstalled, nil
	case LifecycleInstalled, LifecycleEnabled, LifecycleActivated, LifecycleWarm, LifecycleRunning, LifecycleSuspended:
		return InstallationStateInstalled, nil
	default:
		return "", fmt.Errorf("%w: state=%q", ErrLifecycleInvalid, s)
	}
}

func (s LifecycleState) Enablement() (EnablementState, error) {
	switch s {
	case LifecycleAbsent, LifecycleInstalled:
		return EnablementDisabled, nil
	case LifecycleEnabled, LifecycleActivated, LifecycleWarm, LifecycleRunning, LifecycleSuspended:
		return EnablementEnabled, nil
	default:
		return "", fmt.Errorf("%w: state=%q", ErrLifecycleInvalid, s)
	}
}

func (s LifecycleState) Desired() (DesiredRuntimeState, error) {
	switch s {
	case LifecycleAbsent, LifecycleInstalled, LifecycleEnabled, LifecycleActivated:
		return DesiredRuntimeStopped, nil
	case LifecycleWarm, LifecycleRunning:
		return DesiredRuntimeStarted, nil
	case LifecycleSuspended:
		return DesiredRuntimePaused, nil
	default:
		return "", fmt.Errorf("%w: state=%q", ErrLifecycleInvalid, s)
	}
}

func (s LifecycleState) Actual() (ActualRuntimeState, error) {
	switch s {
	case LifecycleAbsent, LifecycleInstalled, LifecycleEnabled, LifecycleActivated:
		return ActualRuntimeStopped, nil
	case LifecycleWarm:
		return ActualRuntimeReady, nil
	case LifecycleRunning:
		return ActualRuntimeRunning, nil
	case LifecycleSuspended:
		return ActualRuntimeStopped, nil
	default:
		return "", fmt.Errorf("%w: state=%q", ErrLifecycleInvalid, s)
	}
}

func DeriveLifecycle(installation InstallationState, mode EnablementState, desired DesiredRuntimeState, actual ActualRuntimeState) (LifecycleState, error) {
	if installation == InstallationStateNotInstalled {
		if mode == EnablementEnabled || actual == ActualRuntimeRunning || actual == ActualRuntimeReady || desired == DesiredRuntimePaused {
			return "", fmt.Errorf("%w: not installed but enabled=%q desired=%q actual=%q", ErrLifecycleTransition, mode, desired, actual)
		}
		return LifecycleAbsent, nil
	}
	if installation != InstallationStateInstalled {
		return "", fmt.Errorf("%w: installation=%q is transitional", ErrLifecycleInvalid, installation)
	}
	if mode == EnablementDisabled {
		if actual == ActualRuntimeRunning || actual == ActualRuntimeReady || desired == DesiredRuntimeStarted || desired == DesiredRuntimePaused {
			return "", fmt.Errorf("%w: disabled but desired=%q actual=%q", ErrLifecycleTransition, desired, actual)
		}
		return LifecycleInstalled, nil
	}
	if mode != EnablementEnabled {
		return "", fmt.Errorf("%w: enablement=%q is not a stable state", ErrLifecycleInvalid, mode)
	}
	if desired == DesiredRuntimePaused {
		if actual != ActualRuntimeStopped {
			return "", fmt.Errorf("%w: suspended but actual=%q", ErrLifecycleTransition, actual)
		}
		return LifecycleSuspended, nil
	}
	switch actual {
	case ActualRuntimeStopped:
		return LifecycleEnabled, nil
	case ActualRuntimeReady:
		return LifecycleWarm, nil
	case ActualRuntimeRunning:
		return LifecycleRunning, nil
	default:
		return "", fmt.Errorf("%w: actual=%q is not a stable state", ErrLifecycleInvalid, actual)
	}
}

type Machine struct {
	mu      sync.Mutex
	state   LifecycleState
	epoch   uint64
	history []Transition
	applied map[uint64]Transition
	now     func() time.Time
}

func NewMachine(initial LifecycleState) (*Machine, error) {
	return NewMachineWithClock(initial, time.Now)
}

func NewMachineWithClock(initial LifecycleState, now func() time.Time) (*Machine, error) {
	if !initial.Valid() {
		return nil, fmt.Errorf("%w: state=%q", ErrLifecycleInvalid, initial)
	}
	if now == nil {
		now = time.Now
	}
	return &Machine{state: initial, applied: map[uint64]Transition{}, now: now}, nil
}

func (m *Machine) State() LifecycleState {
	if m == nil {
		return ""
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

func (m *Machine) Epoch() uint64 {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.epoch
}

func (m *Machine) History() []Transition {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Transition{}, m.history...)
}

func (m *Machine) Apply(ctx context.Context, event LifecycleEvent, epoch uint64) (Transition, error) {
	if m == nil {
		return Transition{}, fmt.Errorf("%w: machine is nil", ErrLifecycleInvalid)
	}
	if err := ctx.Err(); err != nil {
		return Transition{}, err
	}
	if !event.Valid() {
		return Transition{}, fmt.Errorf("%w: event=%q", ErrLifecycleInvalid, event)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now().UTC()
	if previous, ok := m.applied[epoch]; ok {
		if previous.Event != event {
			return Transition{From: m.state, To: m.state, Event: event, Epoch: m.epoch, Changed: false, At: now}, fmt.Errorf("%w: epoch=%d already applied event=%s", ErrLifecycleStale, epoch, previous.Event)
		}
		return Transition{From: previous.From, To: previous.To, Event: event, Epoch: epoch, Changed: false, At: now}, nil
	}
	if epoch <= m.epoch {
		return Transition{From: m.state, To: m.state, Event: event, Epoch: m.epoch, Changed: false, At: now}, fmt.Errorf("%w: epoch=%d current=%d", ErrLifecycleStale, epoch, m.epoch)
	}
	next, ok := CanTransition(m.state, event)
	if !ok {
		return Transition{From: m.state, To: m.state, Event: event, Epoch: m.epoch, Changed: false, At: now}, fmt.Errorf("%w: %s + %s", ErrLifecycleTransition, m.state, event)
	}
	transition := Transition{From: m.state, To: next, Event: event, Epoch: epoch, Changed: true, At: now}
	m.state = next
	m.epoch = epoch
	m.history = append(m.history, transition)
	if len(m.history) > 128 {
		m.history = append(m.history[:0], m.history[len(m.history)-128:]...)
	}
	m.applied[epoch] = transition
	for appliedEpoch := range m.applied {
		if appliedEpoch+128 < m.epoch {
			delete(m.applied, appliedEpoch)
		}
	}
	return transition, nil
}
