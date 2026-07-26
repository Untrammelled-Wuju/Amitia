package extension_page_host

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type PageID string
type PageSessionID string
type ExtensionID string
type ContributionID string

type PageScope string

const (
	ScopeGlobal       PageScope = "global"
	ScopeCharacter    PageScope = "character"
	ScopeConversation PageScope = "conversation"
)

type PageKind string

const (
	PageKindSchema PageKind = "schema_page"
	PageKindWeb    PageKind = "web_page"
)

type PageState string

const (
	PageStateResolving        PageState = "resolving"
	PageStatePermissionCheck  PageState = "permission_check"
	PageStateRuntimeStarting  PageState = "runtime_starting"
	PageStateLoading          PageState = "loading"
	PageStateReady            PageState = "ready"
	PageStateDegraded         PageState = "degraded"
	PageStateFailed           PageState = "failed"
	PageStateDisabled         PageState = "disabled"
	PageStateNotInstalled     PageState = "not_installed"
	PageStateIncompatible     PageState = "incompatible"
	PageStateSuspended        PageState = "suspended"
)

type PageStatePolicy string

const (
	StatePolicyEphemeral           PageStatePolicy = "ephemeral"
	StatePolicySession             PageStatePolicy = "session"
	StatePolicyPersistentPreferences PageStatePolicy = "persistent_preferences"
)

type DeepLinkPolicy struct {
	Allowed   bool     `json:"allowed"`
	Origins   []string `json:"origins,omitempty"`
	RequireConfirmation bool `json:"requireConfirmation"`
}

type LocalizedText struct {
	Default string            `json:"default"`
	Translations map[string]string `json:"translations,omitempty"`
}

type PageNavigationDefinition struct {
	Group      string `json:"group,omitempty"`
	Order      int    `json:"order"`
	ParentPage string `json:"parentPage,omitempty"`
	Hidden     bool   `json:"hidden"`
}

type PageParameterDefinition struct {
	Name      string          `json:"name"`
	Schema    json.RawMessage `json:"schema,omitempty"`
	Required  bool            `json:"required"`
	Sensitive bool            `json:"sensitive"`
}

type ExtensionPageSpec struct {
	PageID         PageID             `json:"pageId"`
	RouteKey       string             `json:"routeKey"`
	Title          LocalizedText      `json:"title"`
	Description    LocalizedText      `json:"description"`
	Icon           string             `json:"icon,omitempty"`
	Navigation     PageNavigationDefinition `json:"navigation"`
	EntryKind      PageKind           `json:"entryKind"`
	EntryPath      string             `json:"entryPath"`
	SchemaPath     string             `json:"schemaPath,omitempty"`
	Scope          PageScope          `json:"scope"`
	Permissions    []string           `json:"permissions,omitempty"`
	Parameters     []PageParameterDefinition `json:"parameters,omitempty"`
	DeepLinkPolicy DeepLinkPolicy     `json:"deepLinkPolicy"`
	StatePolicy    PageStatePolicy    `json:"statePolicy"`
}

type ExtensionPageDefinition struct {
	PageSpec         *ExtensionPageSpec
	contributionID   ContributionID
	extensionID      ExtensionID
	moduleID         string
	generation       int64
	contractVersion  int
	effective        bool
	enabled          bool
	runtimeReady     bool
	installedVersion string
	publisher        string
	trustLevel       string
}

type PageNavigationQuery struct {
	ExtensionID ExtensionID `json:"extensionId,omitempty"`
	Group       string      `json:"group,omitempty"`
	IncludeHidden bool      `json:"includeHidden"`
	ParentPage  string      `json:"parentPage,omitempty"`
}

type PageNavigationItem struct {
	PageID       PageID        `json:"pageId"`
	ExtensionID  ExtensionID   `json:"extensionId"`
	Title        string        `json:"title"`
	Description  string        `json:"description,omitempty"`
	Icon         string        `json:"icon,omitempty"`
	Group        string        `json:"group,omitempty"`
	Order        int           `json:"order"`
	ParentPage   string        `json:"parentPage,omitempty"`
	Hidden       bool          `json:"hidden"`
	Kind         PageKind      `json:"kind"`
	Effective    bool          `json:"effective"`
	Disabled     bool          `json:"disabled"`
}

type ExtensionPageSession struct {
	SessionID     PageSessionID    `json:"sessionId"`
	ContributionID ContributionID  `json:"contributionId"`
	ExtensionID   ExtensionID      `json:"extensionId"`
	ModuleID      string           `json:"moduleId"`
	PageID        PageID           `json:"pageId"`
	Generation    int64            `json:"generation"`
	ScopeSnapshot string           `json:"scopeSnapshot"`
	Contract      int              `json:"contract"`
	CreatedAt     time.Time        `json:"createdAt"`
	LastActiveAt  time.Time        `json:"lastActiveAt"`
	State         PageState        `json:"state"`
	mu            sync.Mutex
	subscriptions []string
	expired       bool
}

func (s *ExtensionPageSession) Touch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastActiveAt = time.Now().UTC()
}

func (s *ExtensionPageSession) IsExpired(maxIdle time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.expired {
		return true
	}
	if time.Since(s.LastActiveAt) > maxIdle {
		s.expired = true
		return true
	}
	return false
}

func (s *ExtensionPageSession) MarkExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expired = true
}

func (s *ExtensionPageSession) SetState(state PageState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.State = state
}

type PageRegistry interface {
	Register(ctx context.Context, def *ExtensionPageDefinition) error
	Resolve(ctx context.Context, extensionID ExtensionID, pageID PageID) (*ExtensionPageDefinition, error)
	ListNavigation(ctx context.Context, query PageNavigationQuery) ([]PageNavigationItem, error)
	ListByExtension(ctx context.Context, extensionID ExtensionID) ([]*ExtensionPageDefinition, error)
	Unregister(ctx context.Context, contributionID ContributionID) error
}

type pageRegistry struct {
	mu      sync.RWMutex
	pages   map[string]*ExtensionPageDefinition
	byExt   map[ExtensionID][]*ExtensionPageDefinition
}

func NewPageRegistry() PageRegistry {
	return &pageRegistry{
		pages: make(map[string]*ExtensionPageDefinition),
		byExt: make(map[ExtensionID][]*ExtensionPageDefinition),
	}
}

func keyFor(extensionID ExtensionID, pageID PageID) string {
	return fmt.Sprintf("%s#%s", extensionID, pageID)
}

func (r *pageRegistry) Register(ctx context.Context, def *ExtensionPageDefinition) error {
	if def == nil || def.extensionID == "" || def.PageSpec == nil || def.PageSpec.PageID == "" {
		return ErrInvalidPageDefinition
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	k := keyFor(def.extensionID, def.PageSpec.PageID)
	if _, exists := r.pages[k]; exists {
		return fmt.Errorf("%w: %s", ErrPageExists, k)
	}
	r.pages[k] = def
	r.byExt[def.extensionID] = append(r.byExt[def.extensionID], def)
	return nil
}

func (r *pageRegistry) Resolve(ctx context.Context, extensionID ExtensionID, pageID PageID) (*ExtensionPageDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	k := keyFor(extensionID, pageID)
	def, exists := r.pages[k]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrPageNotFound, k)
	}
	return def, nil
}

func (r *pageRegistry) ListNavigation(ctx context.Context, query PageNavigationQuery) ([]PageNavigationItem, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]PageNavigationItem, 0)
	for _, def := range r.pages {
		if query.ExtensionID != "" && def.extensionID != query.ExtensionID {
			continue
		}
		if !query.IncludeHidden && def.PageSpec.Navigation.Hidden {
			continue
		}
		if query.Group != "" && def.PageSpec.Navigation.Group != query.Group {
			continue
		}
		if query.ParentPage != "" && def.PageSpec.Navigation.ParentPage != query.ParentPage {
			continue
		}
		items = append(items, PageNavigationItem{
			PageID:      def.PageSpec.PageID,
			ExtensionID: def.extensionID,
			Title:       def.PageSpec.Title.Default,
			Description: def.PageSpec.Description.Default,
			Icon:        def.PageSpec.Icon,
			Group:       def.PageSpec.Navigation.Group,
			Order:       def.PageSpec.Navigation.Order,
			ParentPage:  def.PageSpec.Navigation.ParentPage,
			Hidden:      def.PageSpec.Navigation.Hidden,
			Kind:        def.PageSpec.EntryKind,
			Effective:   def.effective,
			Disabled:    !def.enabled,
		})
	}
	return items, nil
}

func (r *pageRegistry) ListByExtension(ctx context.Context, extensionID ExtensionID) ([]*ExtensionPageDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := r.byExt[extensionID]
	out := make([]*ExtensionPageDefinition, len(list))
	copy(out, list)
	return out, nil
}

func (r *pageRegistry) Unregister(ctx context.Context, contributionID ContributionID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for k, def := range r.pages {
		if def.contributionID == contributionID {
			delete(r.pages, k)
			list := r.byExt[def.extensionID]
			for i, d := range list {
				if d.contributionID == contributionID {
					r.byExt[def.extensionID] = append(list[:i], list[i+1:]...)
					break
				}
			}
		}
	}
	return nil
}

type SessionManager struct {
	mu       sync.RWMutex
	sessions map[PageSessionID]*ExtensionPageSession
	maxIdle  time.Duration
	maxAge   time.Duration
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[PageSessionID]*ExtensionPageSession),
		maxIdle:  30 * time.Minute,
		maxAge:   24 * time.Hour,
	}
}

type CreateSessionRequest struct {
	ContributionID ContributionID
	ExtensionID    ExtensionID
	ModuleID       string
	PageID         PageID
	Generation     int64
	ScopeSnapshot  string
	Contract       int
}

func (m *SessionManager) Create(req CreateSessionRequest) (*ExtensionPageSession, error) {
	if req.ExtensionID == "" || req.PageID == "" || req.ContributionID == "" {
		return nil, ErrInvalidSessionRequest
	}
	now := time.Now().UTC()
	session := &ExtensionPageSession{
		SessionID:     PageSessionID(newSessionID()),
		ContributionID: req.ContributionID,
		ExtensionID:   req.ExtensionID,
		ModuleID:      req.ModuleID,
		PageID:        req.PageID,
		Generation:    req.Generation,
		ScopeSnapshot: req.ScopeSnapshot,
		Contract:      req.Contract,
		CreatedAt:     now,
		LastActiveAt:  now,
		State:         PageStateResolving,
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[session.SessionID] = session
	return session, nil
}

func (m *SessionManager) Get(id PageSessionID) (*ExtensionPageSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, exists := m.sessions[id]
	if !exists {
		return nil, ErrSessionNotFound
	}
	if session.IsExpired(m.maxIdle) {
		return nil, ErrSessionExpired
	}
	return session, nil
}

func (m *SessionManager) Touch(id PageSessionID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, exists := m.sessions[id]
	if !exists {
		return ErrSessionNotFound
	}
	session.Touch()
	return nil
}

func (m *SessionManager) Destroy(id PageSessionID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, exists := m.sessions[id]
	if !exists {
		return nil
	}
	session.MarkExpired()
	delete(m.sessions, id)
	return nil
}

func (m *SessionManager) DestroyByExtension(extensionID ExtensionID) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for id, session := range m.sessions {
		if session.ExtensionID == extensionID {
			session.MarkExpired()
			delete(m.sessions, id)
			count++
		}
	}
	return count
}

func (m *SessionManager) DestroyByContribution(contributionID ContributionID) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for id, session := range m.sessions {
		if session.ContributionID == contributionID {
			session.MarkExpired()
			delete(m.sessions, id)
			count++
		}
	}
	return count
}

func (m *SessionManager) List() []*ExtensionPageSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*ExtensionPageSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		if !s.IsExpired(m.maxIdle) {
			out = append(out, s)
		}
	}
	return out
}

func (m *SessionManager) CleanupExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for id, session := range m.sessions {
		if session.IsExpired(m.maxIdle) || time.Since(session.CreatedAt) > m.maxAge {
			session.MarkExpired()
			delete(m.sessions, id)
			count++
		}
	}
	return count
}

type PageHost struct {
	registry  PageRegistry
	sessions  *SessionManager
	mu        sync.RWMutex
	resolver  RouteResolver
	validator PageValidator
}

type RouteResolver interface {
	ResolveRoute(ctx context.Context, extensionID ExtensionID, pageID PageID, params map[string]string) (*ResolveResult, error)
}

type PageValidator interface {
	ValidateAccess(ctx context.Context, def *ExtensionPageDefinition, scopeSnapshot string) ([]string, error)
	ValidateParams(ctx context.Context, def *ExtensionPageDefinition, params map[string]string) error
}

type ResolveResult struct {
	Definition *ExtensionPageDefinition
	Session    *ExtensionPageSession
	Missing    []string
	Reason     string
}

func NewPageHost(registry PageRegistry, sessions *SessionManager) *PageHost {
	return &PageHost{
		registry: registry,
		sessions: sessions,
	}
}

func (h *PageHost) SetRouteResolver(r RouteResolver) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.resolver = r
}

func (h *PageHost) SetValidator(v PageValidator) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.validator = v
}

type OpenPageRequest struct {
	ExtensionID    ExtensionID       `json:"extensionId"`
	PageID         PageID            `json:"pageId"`
	Params         map[string]string `json:"params,omitempty"`
	ScopeSnapshot  string            `json:"scopeSnapshot,omitempty"`
	DeepLinkOrigin string            `json:"deepLinkOrigin,omitempty"`
}

type OpenPageResult struct {
	SessionID    PageSessionID `json:"sessionId"`
	State        PageState     `json:"state"`
	Definition   *ExtensionPageSpec `json:"definition"`
	MissingPerms []string      `json:"missingPermissions,omitempty"`
	Reason       string        `json:"reason,omitempty"`
}

func (h *PageHost) OpenPage(ctx context.Context, req OpenPageRequest) (*OpenPageResult, error) {
	if req.ExtensionID == "" || req.PageID == "" {
		return nil, ErrInvalidRequest
	}
	def, err := h.registry.Resolve(ctx, req.ExtensionID, req.PageID)
	if err != nil {
		return &OpenPageResult{
			State:  PageStateNotInstalled,
			Reason: "extension page not found",
		}, nil
	}
	if !def.enabled {
		return &OpenPageResult{
			State:  PageStateDisabled,
			Reason: "extension disabled",
		}, nil
	}
	if !def.effective {
		return &OpenPageResult{
			State:  PageStateIncompatible,
			Reason: "contribution not effective",
		}, nil
	}
	h.mu.RLock()
	validator := h.validator
	h.mu.RUnlock()
	missing := []string{}
	if validator != nil {
		missing, err = validator.ValidateAccess(ctx, def, req.ScopeSnapshot)
		if err != nil {
			return &OpenPageResult{
				State:  PageStateFailed,
				Reason: err.Error(),
			}, nil
		}
		if err := validator.ValidateParams(ctx, def, req.Params); err != nil {
			return &OpenPageResult{
				State:  PageStateFailed,
				Reason: err.Error(),
			}, nil
		}
	}
	session, err := h.sessions.Create(CreateSessionRequest{
		ContributionID: def.contributionID,
		ExtensionID:    def.extensionID,
		ModuleID:       def.moduleID,
		PageID:         def.PageSpec.PageID,
		Generation:     def.generation,
		ScopeSnapshot:  req.ScopeSnapshot,
		Contract:       def.contractVersion,
	})
	if err != nil {
		return nil, err
	}
	if !def.runtimeReady {
		session.SetState(PageStateRuntimeStarting)
	} else if len(missing) > 0 {
		session.SetState(PageStatePermissionCheck)
	} else {
		session.SetState(PageStateLoading)
	}
	return &OpenPageResult{
		SessionID:    session.SessionID,
		State:        session.State,
		Definition:   def.PageSpec,
		MissingPerms: missing,
	}, nil
}

func (h *PageHost) ClosePage(ctx context.Context, sessionID PageSessionID) error {
	return h.sessions.Destroy(sessionID)
}

func (h *PageHost) HandleExtensionDisabled(ctx context.Context, extensionID ExtensionID) int {
	return h.sessions.DestroyByExtension(extensionID)
}

func (h *PageHost) HandleExtensionUpdated(ctx context.Context, extensionID ExtensionID, newGeneration int64) int {
	return h.sessions.DestroyByExtension(extensionID)
}

func (h *PageHost) HandleExtensionUninstalled(ctx context.Context, extensionID ExtensionID) (int, error) {
	count := h.sessions.DestroyByExtension(extensionID)
	_ = h.registry.Unregister(ctx, "")
	return count, nil
}

type DeepLink struct {
	ExtensionID ExtensionID
	PageID      PageID
	Params      map[string]string
	Origin      string
}

func ParseDeepLink(raw string) (*DeepLink, error) {
	if !strings.HasPrefix(raw, "amitia://extension/") {
		return nil, ErrInvalidDeepLink
	}
	body := strings.TrimPrefix(raw, "amitia://extension/")
	parts := strings.SplitN(body, "?", 2)
	pathParts := strings.SplitN(parts[0], "/page/", 2)
	if len(pathParts) != 2 {
		return nil, ErrInvalidDeepLink
	}
	extID := ExtensionID(pathParts[0])
	pageID := PageID(pathParts[1])
	if extID == "" || pageID == "" {
		return nil, ErrInvalidDeepLink
	}
	params := map[string]string{}
	if len(parts) > 1 {
		for _, kv := range strings.Split(parts[1], "&") {
			eq := strings.Index(kv, "=")
			if eq < 0 {
				continue
			}
			params[kv[:eq]] = kv[eq+1:]
		}
	}
	return &DeepLink{
		ExtensionID: extID,
		PageID:      pageID,
		Params:      params,
	}, nil
}

func newSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "pgs_" + hex.EncodeToString(b)
}

var (
	ErrInvalidPageDefinition = errors.New("page_host: invalid page definition")
	ErrPageExists            = errors.New("page_host: page exists")
	ErrPageNotFound          = errors.New("page_host: page not found")
	ErrInvalidSessionRequest = errors.New("page_host: invalid session request")
	ErrSessionNotFound       = errors.New("page_host: session not found")
	ErrSessionExpired        = errors.New("page_host: session expired")
	ErrInvalidRequest        = errors.New("page_host: invalid request")
	ErrInvalidDeepLink       = errors.New("page_host: invalid deep link")
)
