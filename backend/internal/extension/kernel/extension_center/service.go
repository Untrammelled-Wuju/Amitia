package extension_center

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

type ExtensionStatus string

const (
	ExtensionStatusNotInstalled      ExtensionStatus = "not_installed"
	ExtensionStatusInstalling        ExtensionStatus = "installing"
	ExtensionStatusInstalledEnabled  ExtensionStatus = "installed_enabled"
	ExtensionStatusInstalledDisabled ExtensionStatus = "installed_disabled"
	ExtensionStatusPartial           ExtensionStatus = "partial"
	ExtensionStatusFailed            ExtensionStatus = "failed"
	ExtensionStatusNeedsPermission   ExtensionStatus = "needs_permission"
	ExtensionStatusMissingDependency ExtensionStatus = "missing_dependency"
	ExtensionStatusIncompatible      ExtensionStatus = "incompatible"
	ExtensionStatusQuarantined       ExtensionStatus = "quarantined"
	ExtensionStatusDev               ExtensionStatus = "dev"
	ExtensionStatusUpdating          ExtensionStatus = "updating"
)

type TrustLevel string

const (
	TrustLevelPlatform  TrustLevel = "platform"
	TrustLevelOfficial  TrustLevel = "official"
	TrustLevelVerified  TrustLevel = "verified"
	TrustLevelCommunity TrustLevel = "community"
	TrustLevelUntrusted TrustLevel = "untrusted"
)

type ContributionTag string

const (
	TagTools     ContributionTag = "tools"
	TagSkills    ContributionTag = "skills"
	TagWorkflows ContributionTag = "workflows"
	TagMCP       ContributionTag = "mcp"
	TagProviders ContributionTag = "providers"
	TagUI        ContributionTag = "ui"
	TagDesktop   ContributionTag = "desktop"
	TagTasks     ContributionTag = "tasks"
	TagHooks     ContributionTag = "hooks"
	TagEvents    ContributionTag = "events"
)

type ExtensionCard struct {
	ExtensionID           string            `json:"extensionId"`
	Publisher             string            `json:"publisher"`
	DisplayName           string            `json:"displayName"`
	Description           string            `json:"description"`
	Version               string            `json:"version"`
	Icon                  string            `json:"icon,omitempty"`
	Trust                 TrustLevel        `json:"trust"`
	Status                ExtensionStatus   `json:"status"`
	Enabled               bool              `json:"enabled"`
	UpdateAvailable       bool              `json:"updateAvailable"`
	ContributionTags      []ContributionTag `json:"contributionTags"`
	ProvidedCapabilities  []string          `json:"providedCapabilities,omitempty"`
	Platforms             []string          `json:"platforms"`
	FailureCount          int               `json:"failureCount"`
	PermissionRisk        string            `json:"permissionRisk,omitempty"`
	DevMode               bool              `json:"devMode"`
	UserModified          bool              `json:"userModified"`
	Source                string            `json:"source"`
	InstalledAt           *time.Time        `json:"installedAt,omitempty"`
	UpdatedAt             *time.Time        `json:"updatedAt,omitempty"`
}

type CenterFilter struct {
	Status     []ExtensionStatus
	Trust      []TrustLevel
	Tags       []ContributionTag
	Search     string
	Platform   string
	DevMode    *bool
	Enabled    *bool
	UpdateOnly bool
}

type CenterSortKey string

const (
	SortByName   CenterSortKey = "name"
	SortByRecent CenterSortKey = "recent"
	SortByTrust  CenterSortKey = "trust"
	SortByStatus CenterSortKey = "status"
)

type CenterView struct {
	Installed    []ExtensionCard `json:"installed"`
	Discover     []ExtensionCard `json:"discover"`
	LocalImports []ExtensionCard `json:"localImports"`
	Dev          []ExtensionCard `json:"dev"`
	Updates      []ExtensionCard `json:"updates"`
	NeedsAction  []ExtensionCard `json:"needsAction"`
	GeneratedAt  time.Time       `json:"generatedAt"`
}

type CardProvider interface {
	ListCards(ctx context.Context) ([]ExtensionCard, error)
}

type CenterService struct {
	mu       sync.RWMutex
	provider CardProvider
	cache    []ExtensionCard
	cacheAt  time.Time
	cacheTTL time.Duration
}

func NewCenterService(provider CardProvider) *CenterService {
	return &CenterService{
		provider: provider,
		cacheTTL: 5 * time.Second,
	}
}

var (
	ErrCardProviderUnavailable = errors.New("extension_center: card provider unavailable")
)

func (s *CenterService) GetView(ctx context.Context, filter CenterFilter, sortKey CenterSortKey) (*CenterView, error) {
	cards, err := s.getCards(ctx)
	if err != nil {
		return nil, err
	}
	filtered := applyFilter(cards, filter)
	sorted := applySort(filtered, sortKey)

	view := &CenterView{GeneratedAt: time.Now().UTC()}
	for _, card := range sorted {
		switch {
		case card.DevMode:
			view.Dev = append(view.Dev, card)
		case card.UpdateAvailable:
			view.Updates = append(view.Updates, card)
		case card.Status == ExtensionStatusNotInstalled:
			view.Discover = append(view.Discover, card)
		case card.Status == ExtensionStatusFailed || card.Status == ExtensionStatusNeedsPermission ||
			card.Status == ExtensionStatusMissingDependency || card.Status == ExtensionStatusIncompatible ||
			card.Status == ExtensionStatusQuarantined:
			view.NeedsAction = append(view.NeedsAction, card)
		case card.Status == ExtensionStatusInstalling || card.Status == ExtensionStatusUpdating:
			view.LocalImports = append(view.LocalImports, card)
		case card.Source == "local_import":
			view.LocalImports = append(view.LocalImports, card)
		default:
			view.Installed = append(view.Installed, card)
		}
	}
	return view, nil
}

func (s *CenterService) ListInstalled(ctx context.Context) ([]ExtensionCard, error) {
	cards, err := s.getCards(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ExtensionCard, 0)
	for _, card := range cards {
		if card.Status == ExtensionStatusInstalledEnabled || card.Status == ExtensionStatusInstalledDisabled {
			out = append(out, card)
		}
	}
	return out, nil
}

func (s *CenterService) ListDiscoverable(ctx context.Context) ([]ExtensionCard, error) {
	cards, err := s.getCards(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ExtensionCard, 0)
	for _, card := range cards {
		if card.Status == ExtensionStatusNotInstalled && card.Source != "dev" {
			out = append(out, card)
		}
	}
	return out, nil
}

func (s *CenterService) ListUpdates(ctx context.Context) ([]ExtensionCard, error) {
	cards, err := s.getCards(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ExtensionCard, 0)
	for _, card := range cards {
		if card.UpdateAvailable {
			out = append(out, card)
		}
	}
	return out, nil
}

func (s *CenterService) ListNeedsAction(ctx context.Context) ([]ExtensionCard, error) {
	cards, err := s.getCards(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ExtensionCard, 0)
	for _, card := range cards {
		switch card.Status {
		case ExtensionStatusFailed, ExtensionStatusNeedsPermission,
			ExtensionStatusMissingDependency, ExtensionStatusIncompatible,
			ExtensionStatusQuarantined:
			out = append(out, card)
		}
	}
	return out, nil
}

func (s *CenterService) InvalidateCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = nil
	s.cacheAt = time.Time{}
}

func (s *CenterService) getCards(ctx context.Context) ([]ExtensionCard, error) {
	s.mu.RLock()
	if s.cache != nil && time.Since(s.cacheAt) < s.cacheTTL {
		out := make([]ExtensionCard, len(s.cache))
		copy(out, s.cache)
		s.mu.RUnlock()
		return out, nil
	}
	s.mu.RUnlock()
	if s.provider == nil {
		return nil, ErrCardProviderUnavailable
	}
	cards, err := s.provider.ListCards(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.cache = cards
	s.cacheAt = time.Now()
	s.mu.Unlock()
	out := make([]ExtensionCard, len(cards))
	copy(out, cards)
	return out, nil
}

func applyFilter(cards []ExtensionCard, filter CenterFilter) []ExtensionCard {
	out := make([]ExtensionCard, 0)
	for _, card := range cards {
		if len(filter.Status) > 0 && !containsStatus(filter.Status, card.Status) {
			continue
		}
		if len(filter.Trust) > 0 && !containsTrust(filter.Trust, card.Trust) {
			continue
		}
		if len(filter.Tags) > 0 && !containsAnyTag(filter.Tags, card.ContributionTags) {
			continue
		}
		if filter.Platform != "" && !containsString(card.Platforms, filter.Platform) {
			continue
		}
		if filter.DevMode != nil && card.DevMode != *filter.DevMode {
			continue
		}
		if filter.Enabled != nil && card.Enabled != *filter.Enabled {
			continue
		}
		if filter.UpdateOnly && !card.UpdateAvailable {
			continue
		}
		if filter.Search != "" && !matchesSearch(card, filter.Search) {
			continue
		}
		out = append(out, card)
	}
	return out
}

func applySort(cards []ExtensionCard, key CenterSortKey) []ExtensionCard {
	out := make([]ExtensionCard, len(cards))
	copy(out, cards)
	switch key {
	case SortByName:
		sort.SliceStable(out, func(i, j int) bool { return out[i].DisplayName < out[j].DisplayName })
	case SortByRecent:
		sort.SliceStable(out, func(i, j int) bool {
			ai, aj := out[i].UpdatedAt, out[j].UpdatedAt
			if ai == nil {
				return false
			}
			if aj == nil {
				return true
			}
			return ai.After(*aj)
		})
	case SortByTrust:
		sort.SliceStable(out, func(i, j int) bool { return trustRank(out[i].Trust) > trustRank(out[j].Trust) })
	case SortByStatus:
		sort.SliceStable(out, func(i, j int) bool { return string(out[i].Status) < string(out[j].Status) })
	}
	return out
}

func trustRank(t TrustLevel) int {
	switch t {
	case TrustLevelPlatform:
		return 5
	case TrustLevelOfficial:
		return 4
	case TrustLevelVerified:
		return 3
	case TrustLevelCommunity:
		return 2
	case TrustLevelUntrusted:
		return 1
	}
	return 0
}

func containsStatus(list []ExtensionStatus, v ExtensionStatus) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

func containsTrust(list []TrustLevel, v TrustLevel) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

func containsString(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

func containsAnyTag(filter []ContributionTag, tags []ContributionTag) bool {
	tagSet := make(map[ContributionTag]bool, len(tags))
	for _, t := range tags {
		tagSet[t] = true
	}
	for _, f := range filter {
		if tagSet[f] {
			return true
		}
	}
	return false
}

func matchesSearch(card ExtensionCard, search string) bool {
	needle := search
	haystacks := []string{
		card.ExtensionID,
		card.Publisher,
		card.DisplayName,
		card.Description,
	}
	for _, h := range haystacks {
		if containsSubstring(h, needle) {
			return true
		}
	}
	return false
}

func containsSubstring(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	if len(s) < len(sub) {
		return false
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

var _ = fmt.Sprintf
