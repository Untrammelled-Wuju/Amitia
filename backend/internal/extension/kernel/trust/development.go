package trust

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"sync"
	"time"
)

type DevelopmentWorkspace struct {
	WorkspacePath string
	PublisherID   string
	KeyID         string
	PublicKey     []byte
	GrantedAt     time.Time
	ExpiresAt     *time.Time
	Revoked       bool
	RevokedAt     *time.Time
	RevokedReason string
}

func (w DevelopmentWorkspace) IsActive() bool {
	if w.Revoked {
		return false
	}
	if w.ExpiresAt != nil && time.Now().UTC().After(*w.ExpiresAt) {
		return false
	}
	return true
}

type DevelopmentTrustManager struct {
	mu         sync.RWMutex
	workspaces map[string]DevelopmentWorkspace
	userTrust  *UserTrustStore
	store      *PublisherStore
}

func NewDevelopmentTrustManager(store *PublisherStore, userTrust *UserTrustStore) *DevelopmentTrustManager {
	return &DevelopmentTrustManager{
		workspaces: make(map[string]DevelopmentWorkspace),
		userTrust:  userTrust,
		store:      store,
	}
}

type RegisterWorkspaceRequest struct {
	WorkspacePath string
	PublisherID   string
	KeyID         string
	PublicKey     []byte
	TTL           time.Duration
}

func (m *DevelopmentTrustManager) Register(ctx context.Context, req RegisterWorkspaceRequest) error {
	if req.WorkspacePath == "" {
		return errors.New("trust: workspace path required")
	}
	if req.PublisherID == "" {
		return errors.New("trust: publisher id required")
	}
	if req.KeyID == "" {
		return errors.New("trust: key id required")
	}
	if len(req.PublicKey) != ed25519.PublicKeySize {
		return errors.New("trust: invalid public key size")
	}

	now := time.Now().UTC()
	workspace := DevelopmentWorkspace{
		WorkspacePath: req.WorkspacePath,
		PublisherID:   req.PublisherID,
		KeyID:         req.KeyID,
		PublicKey:     req.PublicKey,
		GrantedAt:     now,
	}
	if req.TTL > 0 {
		expires := now.Add(req.TTL)
		workspace.ExpiresAt = &expires
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.workspaces[req.WorkspacePath] = workspace

	key := PublisherKey{
		KeyID:       req.KeyID,
		PublisherID: req.PublisherID,
		PublicKey:   req.PublicKey,
		Algorithm:   AlgorithmEd25519,
		State:       KeyStateActive,
		CreatedAt:   now,
	}
	if err := m.store.RegisterDevelopment(req.PublisherID, key); err != nil {
		return fmt.Errorf("trust: failed to register development publisher: %w", err)
	}

	decision := UserTrustDecision{
		DecisionID:    fmt.Sprintf("dev-%s-%d", req.WorkspacePath, now.UnixNano()),
		PublisherID:   req.PublisherID,
		KeyID:         req.KeyID,
		WorkspacePath: req.WorkspacePath,
		Scope:         TrustScopeWorkspace,
		GrantedLevel:  TrustLevelDevelopment,
		GrantedAt:     now,
		Reason:        "development workspace",
	}
	if workspace.ExpiresAt != nil {
		decision.ExpiresAt = workspace.ExpiresAt
	}
	if err := m.userTrust.Grant(decision); err != nil {
		return fmt.Errorf("trust: failed to grant user trust: %w", err)
	}

	return nil
}

func (m *DevelopmentTrustManager) Revoke(ctx context.Context, workspacePath string, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	workspace, ok := m.workspaces[workspacePath]
	if !ok {
		return fmt.Errorf("trust: workspace %s not found", workspacePath)
	}
	now := time.Now().UTC()
	workspace.Revoked = true
	workspace.RevokedAt = &now
	workspace.RevokedReason = reason
	m.workspaces[workspacePath] = workspace

	if err := m.userTrust.RevokeWorkspace(ctx, workspacePath); err != nil {
		return err
	}
	return nil
}

func (m *DevelopmentTrustManager) Get(workspacePath string) (*DevelopmentWorkspace, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	workspace, ok := m.workspaces[workspacePath]
	if !ok {
		return nil, fmt.Errorf("trust: workspace %s not found", workspacePath)
	}
	if !workspace.IsActive() {
		return nil, errors.New("trust: workspace not active")
	}
	w := workspace
	return &w, nil
}

func (m *DevelopmentTrustManager) List() []DevelopmentWorkspace {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]DevelopmentWorkspace, 0, len(m.workspaces))
	for _, workspace := range m.workspaces {
		if workspace.IsActive() {
			result = append(result, workspace)
		}
	}
	return result
}

func (m *DevelopmentTrustManager) Refresh(ctx context.Context, workspacePath string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	workspace, ok := m.workspaces[workspacePath]
	if !ok {
		return fmt.Errorf("trust: workspace %s not found", workspacePath)
	}
	if !workspace.IsActive() {
		return errors.New("trust: workspace not active")
	}
	now := time.Now().UTC()
	if ttl > 0 {
		expires := now.Add(ttl)
		workspace.ExpiresAt = &expires
	} else {
		workspace.ExpiresAt = nil
	}
	m.workspaces[workspacePath] = workspace
	return nil
}

func (m *DevelopmentTrustManager) IsDevelopmentWorkspace(workspacePath string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	workspace, ok := m.workspaces[workspacePath]
	if !ok {
		return false
	}
	return workspace.IsActive()
}

func (m *DevelopmentTrustManager) CannotPromoteToOfficial(workspacePath string) error {
	return errors.New("trust: development trust cannot be promoted to official; only builtin roots are official")
}
