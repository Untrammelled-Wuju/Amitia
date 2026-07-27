package developer_console

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

type DiagnosticRepository struct {
	mu         sync.RWMutex
	extensions []ExtensionSummary
	invocations []InvocationRecord
	events     []EventRecord
	hooks      []HookRecord
	tasks      []TaskRecord
	sessions   []UISessionRecord
	storage    []StorageEntryRecord
	permissions []PermissionGrantRecord
	scopes     []ScopeRecord
	resources  []ResourceRecord
	lifecycle  []LifecycleEventRecord
	logs       []LogEntry
	performance []PerformanceRecord
	migration  []MigrationRecord
	compat     []CompatibilityRecord
	maxItems   int
}

func NewDiagnosticRepository(maxItems int) *DiagnosticRepository {
	if maxItems <= 0 {
		maxItems = 1000
	}
	return &DiagnosticRepository{maxItems: maxItems}
}

type InvocationRecord struct {
	ID           string                 `json:"id"`
	ExtensionID  string                 `json:"extensionId"`
	ModuleID     string                 `json:"moduleId"`
	ToolID       string                 `json:"toolId"`
	StartedAt    time.Time              `json:"startedAt"`
	CompletedAt  *time.Time             `json:"completedAt,omitempty"`
	Status       string                 `json:"status"`
	DurationMs   int64                  `json:"durationMs"`
	Input        map[string]interface{} `json:"input,omitempty"`
	Output       map[string]interface{} `json:"output,omitempty"`
	Error        string                 `json:"error,omitempty"`
	IdempotencyKey string               `json:"idempotencyKey,omitempty"`
	Trace        string                 `json:"trace,omitempty"`
}

type EventRecord struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Source     string                 `json:"source"`
	EmittedAt  time.Time              `json:"emittedAt"`
	Consumed   bool                   `json:"consumed"`
	Consumer   string                 `json:"consumer,omitempty"`
	Payload    map[string]interface{} `json:"payload,omitempty"`
}

type HookRecord struct {
	ID         string    `json:"id"`
	Pipeline   string    `json:"pipeline"`
	Stage      string    `json:"stage"`
	Phase      string    `json:"phase"`
	Extension  string    `json:"extension"`
	InvokedAt  time.Time `json:"invokedAt"`
	DurationMs int64     `json:"durationMs"`
	Vetoed     bool      `json:"vetoed"`
}

type TaskRecord struct {
	ID         string    `json:"id"`
	TaskID     string    `json:"taskId"`
	Extension  string    `json:"extension"`
	StartedAt  time.Time `json:"startedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	Status     string    `json:"status"`
	Progress   float64   `json:"progress"`
	Attempt    int       `json:"attempt"`
}

type UISessionRecord struct {
	ID         string    `json:"id"`
	Extension  string    `json:"extension"`
	Contribution string  `json:"contribution"`
	StartedAt  time.Time `json:"startedAt"`
	LastActive time.Time `json:"lastActive"`
	Origin     string    `json:"origin"`
	CSPViolations int    `json:"cspViolations"`
}

type StorageEntryRecord struct {
	Namespace string    `json:"namespace"`
	Key       string    `json:"key"`
	Version   int       `json:"version"`
	Scope     string    `json:"scope"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type PermissionGrantRecord struct {
	Permission string    `json:"permission"`
	Extension  string    `json:"extension"`
	Scope      string    `json:"scope"`
	Granted    bool      `json:"granted"`
	GrantedAt  time.Time `json:"grantedAt"`
	Reason     string    `json:"reason,omitempty"`
}

type ScopeRecord struct {
	Scope          string `json:"scope"`
	CharacterID    string `json:"characterId,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
	UserID         string `json:"userId,omitempty"`
	Active         bool   `json:"active"`
}

type ResourceRecord struct {
	Handle    string    `json:"handle"`
	Kind      string    `json:"kind"`
	Extension string    `json:"extension"`
	CreatedAt time.Time `json:"createdAt"`
	Size      int64     `json:"size"`
}

type LifecycleEventRecord struct {
	Extension string    `json:"extension"`
	Stage     string    `json:"stage"`
	At        time.Time `json:"at"`
	Success   bool      `json:"success"`
	Reason    string    `json:"reason,omitempty"`
}

type LogEntry struct {
	Extension string                 `json:"extension"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	At        time.Time              `json:"at"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

type PerformanceRecord struct {
	Extension string  `json:"extension"`
	Metric    string  `json:"metric"`
	Value     float64 `json:"value"`
	At        time.Time `json:"at"`
}

type MigrationRecord struct {
	Stage     string    `json:"stage"`
	Status    string    `json:"status"`
	At        time.Time `json:"at"`
	Details   string    `json:"details,omitempty"`
}

type CompatibilityRecord struct {
	Extension string `json:"extension"`
	Required  string `json:"required"`
	Host      string `json:"host"`
	OK        bool   `json:"ok"`
	Reason    string `json:"reason,omitempty"`
}

var (
	ErrNotFound = errors.New("developer_console: record not found")
)

func (r *DiagnosticRepository) RecordInvocation(rec InvocationRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.invocations = append(r.invocations, rec)
	if len(r.invocations) > r.maxItems {
		r.invocations = r.invocations[len(r.invocations)-r.maxItems:]
	}
}

func (r *DiagnosticRepository) RecordEvent(rec EventRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, rec)
	if len(r.events) > r.maxItems {
		r.events = r.events[len(r.events)-r.maxItems:]
	}
}

func (r *DiagnosticRepository) RecordHook(rec HookRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hooks = append(r.hooks, rec)
	if len(r.hooks) > r.maxItems {
		r.hooks = r.hooks[len(r.hooks)-r.maxItems:]
	}
}

func (r *DiagnosticRepository) RecordTask(rec TaskRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks = append(r.tasks, rec)
	if len(r.tasks) > r.maxItems {
		r.tasks = r.tasks[len(r.tasks)-r.maxItems:]
	}
}

func (r *DiagnosticRepository) RecordUISession(rec UISessionRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions = append(r.sessions, rec)
	if len(r.sessions) > r.maxItems {
		r.sessions = r.sessions[len(r.sessions)-r.maxItems:]
	}
}

func (r *DiagnosticRepository) RecordLog(rec LogEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = append(r.logs, rec)
	if len(r.logs) > r.maxItems {
		r.logs = r.logs[len(r.logs)-r.maxItems:]
	}
}

func (r *DiagnosticRepository) RecordLifecycle(rec LifecycleEventRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lifecycle = append(r.lifecycle, rec)
	if len(r.lifecycle) > r.maxItems {
		r.lifecycle = r.lifecycle[len(r.lifecycle)-r.maxItems:]
	}
}

func (r *DiagnosticRepository) RecordPerformance(rec PerformanceRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.performance = append(r.performance, rec)
	if len(r.performance) > r.maxItems {
		r.performance = r.performance[len(r.performance)-r.maxItems:]
	}
}

func (r *DiagnosticRepository) RecordMigration(rec MigrationRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.migration = append(r.migration, rec)
	if len(r.migration) > r.maxItems {
		r.migration = r.migration[len(r.migration)-r.maxItems:]
	}
}

func (r *DiagnosticRepository) RecordCompatibility(rec CompatibilityRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.compat = append(r.compat, rec)
	if len(r.compat) > r.maxItems {
		r.compat = r.compat[len(r.compat)-r.maxItems:]
	}
}

func (r *DiagnosticRepository) ListInvocations(ctx context.Context, filter ConsoleFilters) ([]InvocationRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]InvocationRecord, 0)
	for _, rec := range r.invocations {
		if filter.ExtensionID != "" && rec.ExtensionID != filter.ExtensionID {
			continue
		}
		if filter.StartTime != nil && rec.StartedAt.Before(*filter.StartTime) {
			continue
		}
		if filter.EndTime != nil && rec.StartedAt.After(*filter.EndTime) {
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

func (r *DiagnosticRepository) ListEvents(ctx context.Context, filter ConsoleFilters) ([]EventRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]EventRecord, 0)
	for _, rec := range r.events {
		if filter.StartTime != nil && rec.EmittedAt.Before(*filter.StartTime) {
			continue
		}
		if filter.EndTime != nil && rec.EmittedAt.After(*filter.EndTime) {
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

func (r *DiagnosticRepository) ListHooks(ctx context.Context, filter ConsoleFilters) ([]HookRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]HookRecord, 0)
	for _, rec := range r.hooks {
		if filter.ExtensionID != "" && rec.Extension != filter.ExtensionID {
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

func (r *DiagnosticRepository) ListTasks(ctx context.Context, filter ConsoleFilters) ([]TaskRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]TaskRecord, 0)
	for _, rec := range r.tasks {
		if filter.ExtensionID != "" && rec.Extension != filter.ExtensionID {
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

func (r *DiagnosticRepository) ListUISessions(ctx context.Context, filter ConsoleFilters) ([]UISessionRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]UISessionRecord, 0)
	for _, rec := range r.sessions {
		if filter.ExtensionID != "" && rec.Extension != filter.ExtensionID {
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

func (r *DiagnosticRepository) ListStorage(ctx context.Context, filter ConsoleFilters) ([]StorageEntryRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]StorageEntryRecord, 0)
	for _, rec := range r.storage {
		if filter.ExtensionID != "" && !startsWith(rec.Namespace, filter.ExtensionID) {
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

func (r *DiagnosticRepository) ListPermissions(ctx context.Context, filter ConsoleFilters) ([]PermissionGrantRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]PermissionGrantRecord, 0)
	for _, rec := range r.permissions {
		if filter.ExtensionID != "" && rec.Extension != filter.ExtensionID {
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

func (r *DiagnosticRepository) ListScopes(ctx context.Context) ([]ScopeRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ScopeRecord, len(r.scopes))
	copy(out, r.scopes)
	return out, nil
}

func (r *DiagnosticRepository) ListResources(ctx context.Context, filter ConsoleFilters) ([]ResourceRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ResourceRecord, 0)
	for _, rec := range r.resources {
		if filter.ExtensionID != "" && rec.Extension != filter.ExtensionID {
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

func (r *DiagnosticRepository) ListLifecycle(ctx context.Context, filter ConsoleFilters) ([]LifecycleEventRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]LifecycleEventRecord, 0)
	for _, rec := range r.lifecycle {
		if filter.ExtensionID != "" && rec.Extension != filter.ExtensionID {
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

func (r *DiagnosticRepository) ListLogs(ctx context.Context, filter ConsoleFilters) ([]LogEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]LogEntry, 0)
	for _, rec := range r.logs {
		if filter.ExtensionID != "" && rec.Extension != filter.ExtensionID {
			continue
		}
		if filter.Severity != "" && rec.Level != filter.Severity {
			continue
		}
		if filter.StartTime != nil && rec.At.Before(*filter.StartTime) {
			continue
		}
		if filter.EndTime != nil && rec.At.After(*filter.EndTime) {
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

func (r *DiagnosticRepository) ListPerformance(ctx context.Context, filter ConsoleFilters) ([]PerformanceRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]PerformanceRecord, 0)
	for _, rec := range r.performance {
		if filter.ExtensionID != "" && rec.Extension != filter.ExtensionID {
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

func (r *DiagnosticRepository) ListMigration(ctx context.Context) ([]MigrationRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]MigrationRecord, len(r.migration))
	copy(out, r.migration)
	return out, nil
}

func (r *DiagnosticRepository) ListCompatibility(ctx context.Context) ([]CompatibilityRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]CompatibilityRecord, len(r.compat))
	copy(out, r.compat)
	return out, nil
}

type DiagnosticExport struct {
	GeneratedAt   time.Time               `json:"generatedAt"`
	Invocations   []InvocationRecord      `json:"invocations"`
	Events        []EventRecord           `json:"events"`
	Hooks         []HookRecord            `json:"hooks"`
	Tasks         []TaskRecord            `json:"tasks"`
	UISessions    []UISessionRecord       `json:"uiSessions"`
	Storage       []StorageEntryRecord    `json:"storage"`
	Permissions   []PermissionGrantRecord `json:"permissions"`
	Scopes        []ScopeRecord           `json:"scopes"`
	Resources     []ResourceRecord        `json:"resources"`
	Lifecycle     []LifecycleEventRecord  `json:"lifecycle"`
	Logs          []LogEntry              `json:"logs"`
	Performance   []PerformanceRecord     `json:"performance"`
	Migration     []MigrationRecord       `json:"migration"`
	Compatibility []CompatibilityRecord   `json:"compatibility"`
}

func (r *DiagnosticRepository) ExportDiagnostics(ctx context.Context, filter ConsoleFilters) (*DiagnosticExport, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	export := &DiagnosticExport{GeneratedAt: time.Now().UTC()}

	export.Invocations = r.filterInvocationsLocked(filter)
	export.Events = r.filterEventsLocked(filter)
	export.Hooks = r.filterHooksLocked(filter)
	export.Tasks = r.filterTasksLocked(filter)
	export.UISessions = r.filterUISessionsLocked(filter)
	export.Storage = r.filterStorageLocked(filter)
	export.Permissions = r.filterPermissionsLocked(filter)
	export.Scopes = append([]ScopeRecord(nil), r.scopes...)
	export.Resources = r.filterResourcesLocked(filter)
	export.Lifecycle = r.filterLifecycleLocked(filter)
	export.Logs = r.filterLogsLocked(filter)
	export.Performance = r.filterPerformanceLocked(filter)
	export.Migration = append([]MigrationRecord(nil), r.migration...)
	export.Compatibility = append([]CompatibilityRecord(nil), r.compat...)

	return export, nil
}

func (r *DiagnosticRepository) filterInvocationsLocked(filter ConsoleFilters) []InvocationRecord {
	out := make([]InvocationRecord, 0)
	for _, rec := range r.invocations {
		if filter.ExtensionID != "" && rec.ExtensionID != filter.ExtensionID {
			continue
		}
		if filter.StartTime != nil && rec.StartedAt.Before(*filter.StartTime) {
			continue
		}
		if filter.EndTime != nil && rec.StartedAt.After(*filter.EndTime) {
			continue
		}
		out = append(out, rec)
	}
	return out
}

func (r *DiagnosticRepository) filterEventsLocked(filter ConsoleFilters) []EventRecord {
	out := make([]EventRecord, 0)
	for _, rec := range r.events {
		if filter.StartTime != nil && rec.EmittedAt.Before(*filter.StartTime) {
			continue
		}
		if filter.EndTime != nil && rec.EmittedAt.After(*filter.EndTime) {
			continue
		}
		out = append(out, rec)
	}
	return out
}

func (r *DiagnosticRepository) filterHooksLocked(filter ConsoleFilters) []HookRecord {
	out := make([]HookRecord, 0)
	for _, rec := range r.hooks {
		if filter.ExtensionID != "" && rec.Extension != filter.ExtensionID {
			continue
		}
		out = append(out, rec)
	}
	return out
}

func (r *DiagnosticRepository) filterTasksLocked(filter ConsoleFilters) []TaskRecord {
	out := make([]TaskRecord, 0)
	for _, rec := range r.tasks {
		if filter.ExtensionID != "" && rec.Extension != filter.ExtensionID {
			continue
		}
		out = append(out, rec)
	}
	return out
}

func (r *DiagnosticRepository) filterUISessionsLocked(filter ConsoleFilters) []UISessionRecord {
	out := make([]UISessionRecord, 0)
	for _, rec := range r.sessions {
		if filter.ExtensionID != "" && rec.Extension != filter.ExtensionID {
			continue
		}
		out = append(out, rec)
	}
	return out
}

func (r *DiagnosticRepository) filterStorageLocked(filter ConsoleFilters) []StorageEntryRecord {
	out := make([]StorageEntryRecord, 0)
	for _, rec := range r.storage {
		if filter.ExtensionID != "" && !startsWith(rec.Namespace, filter.ExtensionID) {
			continue
		}
		out = append(out, rec)
	}
	return out
}

func (r *DiagnosticRepository) filterPermissionsLocked(filter ConsoleFilters) []PermissionGrantRecord {
	out := make([]PermissionGrantRecord, 0)
	for _, rec := range r.permissions {
		if filter.ExtensionID != "" && rec.Extension != filter.ExtensionID {
			continue
		}
		out = append(out, rec)
	}
	return out
}

func (r *DiagnosticRepository) filterResourcesLocked(filter ConsoleFilters) []ResourceRecord {
	out := make([]ResourceRecord, 0)
	for _, rec := range r.resources {
		if filter.ExtensionID != "" && rec.Extension != filter.ExtensionID {
			continue
		}
		out = append(out, rec)
	}
	return out
}

func (r *DiagnosticRepository) filterLifecycleLocked(filter ConsoleFilters) []LifecycleEventRecord {
	out := make([]LifecycleEventRecord, 0)
	for _, rec := range r.lifecycle {
		if filter.ExtensionID != "" && rec.Extension != filter.ExtensionID {
			continue
		}
		out = append(out, rec)
	}
	return out
}

func (r *DiagnosticRepository) filterLogsLocked(filter ConsoleFilters) []LogEntry {
	out := make([]LogEntry, 0)
	for _, rec := range r.logs {
		if filter.ExtensionID != "" && rec.Extension != filter.ExtensionID {
			continue
		}
		if filter.Severity != "" && rec.Level != filter.Severity {
			continue
		}
		if filter.StartTime != nil && rec.At.Before(*filter.StartTime) {
			continue
		}
		if filter.EndTime != nil && rec.At.After(*filter.EndTime) {
			continue
		}
		out = append(out, rec)
	}
	return out
}

func (r *DiagnosticRepository) filterPerformanceLocked(filter ConsoleFilters) []PerformanceRecord {
	out := make([]PerformanceRecord, 0)
	for _, rec := range r.performance {
		if filter.ExtensionID != "" && rec.Extension != filter.ExtensionID {
			continue
		}
		out = append(out, rec)
	}
	return out
}

func (r *DiagnosticRepository) JSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func startsWith(s, prefix string) bool {
	if len(prefix) > len(s) {
		return false
	}
	return s[:len(prefix)] == prefix
}

var _ = fmt.Sprintf
