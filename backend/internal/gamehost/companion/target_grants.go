package artifact

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// TargetRootGrant records an explicit user/admin authorization for one
// extension to deploy declared artifacts beneath one canonical directory.
// Generation records the package generation most recently validated by the host;
// trusted host lifecycle code may carry the same exact root forward across a
// confirmed package update, but plugins cannot create, widen, or migrate grants.
type TargetRootGrant struct {
	ExtensionID          string    `json:"extensionId"`
	TargetRoot           string    `json:"targetRoot"`
	Generation           string    `json:"generation"`
	CompatibilityVersion string    `json:"compatibilityVersion,omitempty"`
	GrantedAt            time.Time `json:"grantedAt"`
}

func targetGrantKey(extensionID, targetRoot string) string {
	return strings.TrimSpace(extensionID) + "\x00" + filepath.Clean(targetRoot)
}

func (m *ArtifactManager) loadTargetGrants() error {
	data, err := os.ReadFile(m.targetGrantPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var grants []TargetRootGrant
	if err := json.Unmarshal(data, &grants); err != nil {
		return fmt.Errorf("artifact: decode target root grants: %w", err)
	}
	for _, grant := range grants {
		if grant.ExtensionID == "" || grant.TargetRoot == "" || grant.Generation == "" {
			continue
		}
		m.targetGrants[targetGrantKey(grant.ExtensionID, grant.TargetRoot)] = grant
	}
	return nil
}

func (m *ArtifactManager) saveTargetGrantsLocked() error {
	grants := make([]TargetRootGrant, 0, len(m.targetGrants))
	for _, grant := range m.targetGrants {
		grants = append(grants, grant)
	}
	sort.Slice(grants, func(i, j int) bool {
		return targetGrantKey(grants[i].ExtensionID, grants[i].TargetRoot) < targetGrantKey(grants[j].ExtensionID, grants[j].TargetRoot)
	})
	data, err := json.MarshalIndent(grants, "", "  ")
	if err != nil {
		return err
	}
	tmp := m.targetGrantPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return replaceStateFile(tmp, m.targetGrantPath)
}

// AuthorizeTargetRoot is intentionally a management-side operation. It grants
// the extension access to one exact canonical directory. The current package
// generation is recorded for audit/staleness checks; only host lifecycle code
// may carry this same root forward after a confirmed extension update.
func (m *ArtifactManager) AuthorizeTargetRoot(ctx context.Context, extensionID, targetRoot string) (TargetRootGrant, error) {
	return m.AuthorizeTargetRootForCompatibility(ctx, extensionID, targetRoot, "")
}

func (m *ArtifactManager) AuthorizeTargetRootForCompatibility(ctx context.Context, extensionID, targetRoot, compatibilityVersion string) (TargetRootGrant, error) {
	if m == nil {
		return TargetRootGrant{}, fmt.Errorf("artifact: manager unavailable")
	}
	extensionID = strings.TrimSpace(extensionID)
	if extensionID == "" {
		return TargetRootGrant{}, fmt.Errorf("artifact: extension id is required")
	}
	root, err := canonicalTargetRoot(targetRoot)
	if err != nil {
		return TargetRootGrant{}, err
	}
	generation, err := m.generations.ResolveInstalledGeneration(ctx, extensionID)
	if err != nil {
		return TargetRootGrant{}, fmt.Errorf("artifact: resolve installed generation: %w", err)
	}
	if strings.TrimSpace(generation.GenerationID) == "" {
		return TargetRootGrant{}, fmt.Errorf("artifact: installed generation id is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	grant := TargetRootGrant{
		ExtensionID:          extensionID,
		TargetRoot:           root,
		Generation:           generation.GenerationID,
		CompatibilityVersion: strings.TrimSpace(compatibilityVersion),
		GrantedAt:            time.Now().UTC(),
	}
	key := targetGrantKey(extensionID, root)
	previous, hadPrevious := m.targetGrants[key]
	m.targetGrants[key] = grant
	if err := m.saveTargetGrantsLocked(); err != nil {
		if hadPrevious {
			m.targetGrants[key] = previous
		} else {
			delete(m.targetGrants, key)
		}
		return TargetRootGrant{}, err
	}
	return grant, nil
}

func (m *ArtifactManager) RevokeTargetRoot(ctx context.Context, extensionID, targetRoot string) error {
	_ = ctx
	if m == nil {
		return fmt.Errorf("artifact: manager unavailable")
	}
	extensionID = strings.TrimSpace(extensionID)
	root, err := canonicalTargetRoot(targetRoot)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	key := targetGrantKey(extensionID, root)
	previous, ok := m.targetGrants[key]
	if !ok {
		return nil
	}
	delete(m.targetGrants, key)
	if err := m.saveTargetGrantsLocked(); err != nil {
		m.targetGrants[key] = previous
		return err
	}
	return nil
}

// RevokeAllTargetRoots removes every persisted filesystem authorization owned
// by one extension. This is a host-lifecycle operation used only after a
// confirmed uninstall. Keeping these grants after uninstall would allow a
// later package that reuses the same extension ID to inherit filesystem access
// that the user granted to a different installation generation.
func (m *ArtifactManager) RevokeAllTargetRoots(ctx context.Context, extensionID string) error {
	_ = ctx
	if m == nil {
		return fmt.Errorf("artifact: manager unavailable")
	}
	extensionID = strings.TrimSpace(extensionID)
	if extensionID == "" {
		return fmt.Errorf("artifact: extension id is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	previous := make(map[string]TargetRootGrant)
	for key, grant := range m.targetGrants {
		if grant.ExtensionID != extensionID {
			continue
		}
		previous[key] = grant
		delete(m.targetGrants, key)
	}
	if len(previous) == 0 {
		return nil
	}
	if err := m.saveTargetGrantsLocked(); err != nil {
		for key, grant := range previous {
			m.targetGrants[key] = grant
		}
		return fmt.Errorf("artifact: persist target root grant cleanup: %w", err)
	}
	return nil
}

// RefreshAuthorizedTargetRootsForCurrentGeneration is a host-lifecycle-only
// operation invoked only after the extension kernel has completed a confirmed
// package update. It preserves durable user/admin consent for the exact same
// canonical roots across that trusted update while
// rebinding the audit generation. It never creates a new root and is deliberately
// not exposed through the plugin RPC surface.
func (m *ArtifactManager) RefreshAuthorizedTargetRootsForCurrentGeneration(ctx context.Context, extensionID string) error {
	if m == nil {
		return fmt.Errorf("artifact: manager unavailable")
	}
	extensionID = strings.TrimSpace(extensionID)
	if extensionID == "" {
		return fmt.Errorf("artifact: extension id is required")
	}
	generation, err := m.generations.ResolveInstalledGeneration(ctx, extensionID)
	if err != nil {
		return fmt.Errorf("artifact: resolve installed generation: %w", err)
	}
	currentGeneration := strings.TrimSpace(generation.GenerationID)
	if currentGeneration == "" {
		return fmt.Errorf("artifact: installed generation id is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	changed := false
	previous := make(map[string]TargetRootGrant)
	for key, grant := range m.targetGrants {
		if grant.ExtensionID != extensionID || grant.Generation == currentGeneration {
			continue
		}
		previous[key] = grant
		grant.Generation = currentGeneration
		m.targetGrants[key] = grant
		changed = true
	}
	if !changed {
		return nil
	}
	if err := m.saveTargetGrantsLocked(); err != nil {
		for key, grant := range previous {
			m.targetGrants[key] = grant
		}
		return fmt.Errorf("artifact: persist refreshed target root grants: %w", err)
	}
	return nil
}

func (m *ArtifactManager) ListAuthorizedTargetRoots(ctx context.Context, extensionID string) ([]TargetRootGrant, error) {
	if m == nil {
		return nil, fmt.Errorf("artifact: manager unavailable")
	}
	extensionID = strings.TrimSpace(extensionID)
	generation, err := m.generations.ResolveInstalledGeneration(ctx, extensionID)
	if err != nil {
		return nil, fmt.Errorf("artifact: resolve installed generation: %w", err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]TargetRootGrant, 0)
	for _, grant := range m.targetGrants {
		if grant.ExtensionID == extensionID && grant.Generation == generation.GenerationID {
			out = append(out, grant)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TargetRoot < out[j].TargetRoot })
	return out, nil
}

func (m *ArtifactManager) RequireAuthorizedTargetRoot(ctx context.Context, extensionID, targetRoot string) (string, error) {
	if m == nil {
		return "", fmt.Errorf("artifact: manager unavailable")
	}
	root, err := canonicalTargetRoot(targetRoot)
	if err != nil {
		return "", err
	}
	generation, err := m.generations.ResolveInstalledGeneration(ctx, strings.TrimSpace(extensionID))
	if err != nil {
		return "", fmt.Errorf("artifact: resolve installed generation: %w", err)
	}
	m.mu.Lock()
	grant, ok := m.targetGrants[targetGrantKey(extensionID, root)]
	m.mu.Unlock()
	if !ok || grant.Generation != generation.GenerationID {
		return "", fmt.Errorf("artifact: target root is not authorized for the current extension generation")
	}
	return root, nil
}
