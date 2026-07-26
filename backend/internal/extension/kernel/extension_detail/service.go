package extension_detail

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

type ExtensionDetail struct {
	ExtensionID       string                 `json:"extensionId"`
	Publisher         string                 `json:"publisher"`
	DisplayName       string                 `json:"displayName"`
	Description       string                 `json:"description"`
	Version           string                 `json:"version"`
	Icon              string                 `json:"icon,omitempty"`
	Trust             string                 `json:"trust"`
	Source            string                 `json:"source"`
	License           string                 `json:"license,omitempty"`
	Homepage          string                 `json:"homepage,omitempty"`
	Repository        string                 `json:"repository,omitempty"`
	InstalledAt       *time.Time             `json:"installedAt,omitempty"`
	UpdatedAt         *time.Time             `json:"updatedAt,omitempty"`
	Enabled           bool                   `json:"enabled"`
	EffectiveState    string                 `json:"effectiveState"`
	FailureCount      int                    `json:"failureCount"`
	UpdateAvailable   bool                   `json:"updateAvailable"`
	Platforms         []string               `json:"platforms"`
	PermissionRisk    string                 `json:"permissionRisk,omitempty"`
	DevMode           bool                   `json:"devMode"`
	UserModified      bool                   `json:"userModified"`
	Modules           []ModuleDetail         `json:"modules"`
	Contributions     []ContributionDetail   `json:"contributions"`
	Permissions       []PermissionDetail     `json:"permissions"`
	Scopes            []ScopeDetail          `json:"scopes"`
	Runtimes          []RuntimeDetail        `json:"runtimes"`
	Storage           []StorageNamespaceDetail `json:"storage"`
	Resources         []ResourceDetail       `json:"resources"`
	UIContributions   []UIContributionDetail `json:"uiContributions"`
	Versions          []VersionDetail        `json:"versions"`
	RecentLogs        []LogEntry             `json:"recentLogs"`
	RecentInvocations []InvocationDetail     `json:"recentInvocations"`
	LifecycleEvents   []LifecycleDetail      `json:"lifecycleEvents"`
	Actions           []ActionDescriptor     `json:"actions"`
}

type ModuleDetail struct {
	ModuleID       string    `json:"moduleId"`
	Kind           string    `json:"kind"`
	Entry          string    `json:"entry"`
	Runtime        string    `json:"runtime,omitempty"`
	DisplayName    string    `json:"displayName,omitempty"`
	Description    string    `json:"description,omitempty"`
	Enabled        bool      `json:"enabled"`
	Status         string    `json:"status"`
	LastActiveAt   *time.Time `json:"lastActiveAt,omitempty"`
}

type ContributionDetail struct {
	ContributionID string `json:"contributionId"`
	ModuleID       string `json:"moduleId"`
	Kind           string `json:"kind"`
	Title          string `json:"title"`
	Description    string `json:"description,omitempty"`
	Enabled        bool   `json:"enabled"`
	Exposure       string `json:"exposure,omitempty"`
	RiskLevel      string `json:"riskLevel,omitempty"`
	Deprecated     bool   `json:"deprecated"`
}

type PermissionDetail struct {
	Permission string `json:"permission"`
	Required   bool   `json:"required"`
	Granted    bool   `json:"granted"`
	Reason     string `json:"reason,omitempty"`
	Scope      string `json:"scope,omitempty"`
}

type ScopeDetail struct {
	Scope          string `json:"scope"`
	CharacterID    string `json:"characterId,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
	UserID         string `json:"userId,omitempty"`
	Active         bool   `json:"active"`
}

type RuntimeDetail struct {
	RuntimeID  string     `json:"runtimeId"`
	Kind       string     `json:"kind"`
	Status     string     `json:"status"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`
	Pid        int        `json:"pid,omitempty"`
	Generation int        `json:"generation"`
}

type StorageNamespaceDetail struct {
	Namespace string `json:"namespace"`
	Entries   int    `json:"entries"`
	QuotaUsed int64  `json:"quotaUsed"`
}

type ResourceDetail struct {
	Handle    string    `json:"handle"`
	Kind      string    `json:"kind"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"createdAt"`
}

type UIContributionDetail struct {
	ContributionID string `json:"contributionId"`
	SlotID         string `json:"slotId"`
	Kind           string `json:"kind"`
	Title          string `json:"title"`
	Order          int    `json:"order"`
	Sandbox        string `json:"sandbox,omitempty"`
}

type VersionDetail struct {
	Version     string     `json:"version"`
	ReleasedAt  *time.Time `json:"releasedAt,omitempty"`
	Changelog   string     `json:"changelog,omitempty"`
	Current     bool       `json:"current"`
}

type LogEntry struct {
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	At        time.Time              `json:"at"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

type InvocationDetail struct {
	InvocationID string    `json:"invocationId"`
	ToolID       string    `json:"toolId"`
	StartedAt    time.Time `json:"startedAt"`
	DurationMs   int64     `json:"durationMs"`
	Status       string    `json:"status"`
}

type LifecycleDetail struct {
	Stage     string    `json:"stage"`
	At        time.Time `json:"at"`
	Success   bool      `json:"success"`
	Reason    string    `json:"reason,omitempty"`
}

type ActionDescriptor struct {
	ActionID  string `json:"actionId"`
	Title     string `json:"title"`
	Kind      string `json:"kind"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

type DetailProvider interface {
	GetExtension(ctx context.Context, extensionID string) (*ExtensionDetail, error)
}

type DetailService struct {
	mu       sync.RWMutex
	provider DetailProvider
	cache    map[string]*ExtensionDetail
	cacheAt  map[string]time.Time
	cacheTTL time.Duration
}

func NewDetailService(provider DetailProvider) *DetailService {
	return &DetailService{
		provider: provider,
		cache:    make(map[string]*ExtensionDetail),
		cacheAt:  make(map[string]time.Time),
		cacheTTL: 3 * time.Second,
	}
}

var (
	ErrDetailNotFound = errors.New("extension_detail: extension not found")
)

func (s *DetailService) GetDetail(ctx context.Context, extensionID string) (*ExtensionDetail, error) {
	if extensionID == "" {
		return nil, ErrDetailNotFound
	}
	s.mu.RLock()
	cached, ok := s.cache[extensionID]
	cachedAt := s.cacheAt[extensionID]
	s.mu.RUnlock()
	if ok && time.Since(cachedAt) < s.cacheTTL {
		return cached, nil
	}
	if s.provider == nil {
		return nil, ErrDetailNotFound
	}
	detail, err := s.provider.GetExtension(ctx, extensionID)
	if err != nil {
		return nil, err
	}
	if detail == nil {
		return nil, fmt.Errorf("%w: %s", ErrDetailNotFound, extensionID)
	}
	sortDetail(detail)
	s.mu.Lock()
	s.cache[extensionID] = detail
	s.cacheAt[extensionID] = time.Now()
	s.mu.Unlock()
	return detail, nil
}

func (s *DetailService) Invalidate(extensionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cache, extensionID)
	delete(s.cacheAt, extensionID)
}

func (s *DetailService) ListActions(ctx context.Context, extensionID string) ([]ActionDescriptor, error) {
	detail, err := s.GetDetail(ctx, extensionID)
	if err != nil {
		return nil, err
	}
	return detail.Actions, nil
}

func sortDetail(detail *ExtensionDetail) {
	sort.SliceStable(detail.Modules, func(i, j int) bool {
		return detail.Modules[i].ModuleID < detail.Modules[j].ModuleID
	})
	sort.SliceStable(detail.Contributions, func(i, j int) bool {
		if detail.Contributions[i].ModuleID != detail.Contributions[j].ModuleID {
			return detail.Contributions[i].ModuleID < detail.Contributions[j].ModuleID
		}
		return detail.Contributions[i].ContributionID < detail.Contributions[j].ContributionID
	})
	sort.SliceStable(detail.Permissions, func(i, j int) bool {
		return detail.Permissions[i].Permission < detail.Permissions[j].Permission
	})
}
