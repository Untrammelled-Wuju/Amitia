package developer_console

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

type ConsoleSessionID string

type ConsoleSession struct {
	SessionID   ConsoleSessionID
	WorkspaceID string
	StartedAt   time.Time
	ExpiresAt   time.Time
	Filters     ConsoleFilters
	Subscribed  []string
}

type ConsoleFilters struct {
	ExtensionID    string
	ModuleID       string
	Severity       string
	Stage          string
	Search         string
	StartTime      *time.Time
	EndTime        *time.Time
}

type ConsoleOverview struct {
	GeneratedAt         time.Time           `json:"generatedAt"`
	Extensions          int                 `json:"extensions"`
	Modules             int                 `json:"modules"`
	Contributions       int                 `json:"contributions"`
	Runtimes            int                 `json:"runtimes"`
	ActiveInvocations   int                 `json:"activeInvocations"`
	HostAPICalls        int                 `json:"hostApiCalls"`
	EventsLast5Min      int                 `json:"eventsLast5Min"`
	HookInvocations     int                 `json:"hookInvocations"`
	ActiveTasks         int                 `json:"activeTasks"`
	ActiveUISessions    int                 `json:"activeUiSessions"`
	StorageEntries      int                 `json:"storageEntries"`
	PermissionGrants    int                 `json:"permissionGrants"`
	ActiveScopes        int                 `json:"activeScopes"`
	Resources           int                 `json:"resources"`
	Errors              int                 `json:"errors"`
	Warnings            int                 `json:"warnings"`
	LifecycleEvents     int                 `json:"lifecycleEvents"`
	DevWorkspaces       int                 `json:"devWorkspaces"`
	CompatibilityIssues int                 `json:"compatibilityIssues"`
	TopExtensions       []ExtensionSummary  `json:"topExtensions"`
	ToolFacadeCounters  map[string]int64    `json:"toolFacadeCounters"`
	LegacyCallCounters  map[string]int64    `json:"legacyCallCounters"`
}

type ExtensionSummary struct {
	ExtensionID    string `json:"extensionId"`
	Publisher      string `json:"publisher"`
	Version        string `json:"version"`
	ModuleCount    int    `json:"moduleCount"`
	Enabled        bool   `json:"enabled"`
	Status         string `json:"status"`
	ErrorCount     int    `json:"errorCount"`
	InvocationCount int   `json:"invocationCount"`
}

type ConsoleService struct {
	mu          sync.RWMutex
	sessions    map[ConsoleSessionID]*ConsoleSession
	streams     map[ConsoleSessionID]chan ConsoleStreamEvent
	aggregators *ConsoleAggregators
}

type ConsoleAggregators struct {
	ExtensionProvider  ExtensionSummaryProvider
	InvocationProvider InvocationSummaryProvider
	EventProvider      EventSummaryProvider
	StorageProvider    StorageSummaryProvider
	PermissionProvider PermissionSummaryProvider
	LifecycleProvider  LifecycleSummaryProvider
	ToolFacadeProvider ToolFacadeSummaryProvider
	LegacyCallProvider ToolFacadeSummaryProvider
}

type ExtensionSummaryProvider interface {
	List(ctx context.Context) ([]ExtensionSummary, error)
}

type InvocationSummaryProvider interface {
	Active(ctx context.Context) (int, error)
	Recent(ctx context.Context, since time.Time) (int, error)
}

type EventSummaryProvider interface {
	Recent(ctx context.Context, since time.Time) (int, error)
}

type StorageSummaryProvider interface {
	EntryCount(ctx context.Context) (int, error)
}

type PermissionSummaryProvider interface {
	Grants(ctx context.Context) (int, error)
}

type LifecycleSummaryProvider interface {
	Events(ctx context.Context, since time.Time) (int, error)
}

type ToolFacadeSummaryProvider interface {
	Snapshot() map[string]int64
}

func NewConsoleService(aggregators *ConsoleAggregators) *ConsoleService {
	return &ConsoleService{
		sessions:    make(map[ConsoleSessionID]*ConsoleSession),
		streams:     make(map[ConsoleSessionID]chan ConsoleStreamEvent),
		aggregators: aggregators,
	}
}

func (s *ConsoleService) SetToolFacadeProvider(provider ToolFacadeSummaryProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.aggregators == nil {
		s.aggregators = &ConsoleAggregators{}
	}
	s.aggregators.ToolFacadeProvider = provider
}

func (s *ConsoleService) SetLegacyCallProvider(provider ToolFacadeSummaryProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.aggregators == nil {
		s.aggregators = &ConsoleAggregators{}
	}
	s.aggregators.LegacyCallProvider = provider
}

var (
	ErrSessionClosed = errors.New("developer_console: session closed")
)

func (s *ConsoleService) OpenSession(ctx context.Context, workspaceID string, ttl time.Duration) (*ConsoleSession, error) {
	if ttl <= 0 {
		ttl = time.Hour
	}
	now := time.Now().UTC()
	sess := &ConsoleSession{
		SessionID:   ConsoleSessionID(fmt.Sprintf("console-%d", now.UnixNano())),
		WorkspaceID: workspaceID,
		StartedAt:   now,
		ExpiresAt:   now.Add(ttl),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.SessionID] = sess
	s.streams[sess.SessionID] = make(chan ConsoleStreamEvent, 32)
	return sess, nil
}

func (s *ConsoleService) CloseSession(id ConsoleSessionID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch, ok := s.streams[id]
	if ok {
		close(ch)
		delete(s.streams, id)
	}
	delete(s.sessions, id)
	return nil
}

func (s *ConsoleService) UpdateFilters(id ConsoleSessionID, filters ConsoleFilters) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok {
		return fmt.Errorf("%w: %s", ErrSessionClosed, id)
	}
	sess.Filters = filters
	return nil
}

func (s *ConsoleService) Subscribe(id ConsoleSessionID, channels []string) (<-chan ConsoleStreamEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrSessionClosed, id)
	}
	sess.Subscribed = channels
	ch, ok := s.streams[id]
	if !ok {
		ch = make(chan ConsoleStreamEvent, 32)
		s.streams[id] = ch
	}
	return ch, nil
}

func (s *ConsoleService) Emit(event ConsoleStreamEvent) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for id, ch := range s.streams {
		sess := s.sessions[id]
		if sess == nil {
			continue
		}
		if !matchChannels(event.Channels, sess.Subscribed) {
			continue
		}
		if !matchFilters(event, sess.Filters) {
			continue
		}
		select {
		case ch <- event:
		default:
		}
	}
}

func (s *ConsoleService) BuildOverview(ctx context.Context) (*ConsoleOverview, error) {
	overview := &ConsoleOverview{GeneratedAt: time.Now().UTC()}
	if s.aggregators == nil {
		return overview, nil
	}
	if s.aggregators.ExtensionProvider != nil {
		if exts, err := s.aggregators.ExtensionProvider.List(ctx); err == nil {
			overview.Extensions = len(exts)
			overview.TopExtensions = exts
			moduleCount := 0
			invocationTotal := 0
			for _, e := range exts {
				moduleCount += e.ModuleCount
				if e.ErrorCount > 0 {
					overview.Errors += e.ErrorCount
				}
				invocationTotal += e.InvocationCount
			}
			overview.Modules = moduleCount
			overview.HostAPICalls += invocationTotal
		}
	}
	if s.aggregators.InvocationProvider != nil {
		if active, err := s.aggregators.InvocationProvider.Active(ctx); err == nil {
			overview.ActiveInvocations = active
		}
		if recent, err := s.aggregators.InvocationProvider.Recent(ctx, time.Now().Add(-5*time.Minute)); err == nil {
			overview.HostAPICalls = recent
		}
	}
	if s.aggregators.EventProvider != nil {
		if recent, err := s.aggregators.EventProvider.Recent(ctx, time.Now().Add(-5*time.Minute)); err == nil {
			overview.EventsLast5Min = recent
		}
	}
	if s.aggregators.StorageProvider != nil {
		if count, err := s.aggregators.StorageProvider.EntryCount(ctx); err == nil {
			overview.StorageEntries = count
		}
	}
	if s.aggregators.PermissionProvider != nil {
		if grants, err := s.aggregators.PermissionProvider.Grants(ctx); err == nil {
			overview.PermissionGrants = grants
		}
	}
	if s.aggregators.LifecycleProvider != nil {
		if events, err := s.aggregators.LifecycleProvider.Events(ctx, time.Now().Add(-24*time.Hour)); err == nil {
			overview.LifecycleEvents = events
		}
	}
	if s.aggregators.ToolFacadeProvider != nil {
		overview.ToolFacadeCounters = s.aggregators.ToolFacadeProvider.Snapshot()
	}
	if s.aggregators.LegacyCallProvider != nil {
		overview.LegacyCallCounters = s.aggregators.LegacyCallProvider.Snapshot()
	}
	return overview, nil
}

type ConsoleStreamEvent struct {
	Timestamp time.Time              `json:"timestamp"`
	Channels  []string               `json:"channels"`
	Severity  string                 `json:"severity"`
	Stage     string                 `json:"stage"`
	Extension string                 `json:"extension,omitempty"`
	Module    string                 `json:"module,omitempty"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

func matchChannels(event, subscribed []string) bool {
	if len(subscribed) == 0 {
		return true
	}
	for _, sub := range subscribed {
		for _, ev := range event {
			if sub == ev {
				return true
			}
		}
	}
	return false
}

func matchFilters(event ConsoleStreamEvent, filters ConsoleFilters) bool {
	if filters.Severity != "" && event.Severity != filters.Severity {
		return false
	}
	if filters.Stage != "" && event.Stage != filters.Stage {
		return false
	}
	if filters.ExtensionID != "" && event.Extension != filters.ExtensionID {
		return false
	}
	if filters.ModuleID != "" && event.Module != filters.ModuleID {
		return false
	}
	if filters.Search != "" {
		if !contains(event.Message, filters.Search) {
			return false
		}
	}
	if filters.StartTime != nil && event.Timestamp.Before(*filters.StartTime) {
		return false
	}
	if filters.EndTime != nil && event.Timestamp.After(*filters.EndTime) {
		return false
	}
	return true
}

func contains(s, sub string) bool {
	if sub == "" {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func (s *ConsoleService) MarshalOverview(o *ConsoleOverview) ([]byte, error) {
	return json.Marshal(o)
}
