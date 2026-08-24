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
// extension generation to deploy declared artifacts beneath one canonical
// directory. Plugins cannot create these grants through the plugin RPC surface.
type TargetRootGrant struct {
	ExtensionID string    `json:"extensionId"`
	TargetRoot  string    `json:"targetRoot"`
	Generation  string    `json:"generation"`
	GrantedAt   time.Time `json:"grantedAt"`
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

// AuthorizeTargetRoot is intentionally a management-side operation. It binds
// the exact canonical directory to the currently installed generation so an
// updated plugin cannot inherit arbitrary filesystem access without approval.
func (m *ArtifactManager) AuthorizeTargetRoot(ctx context.Context, extensionID, targetRoot string) (TargetRootGrant, error) {
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
		ExtensionID: extensionID,
		TargetRoot:  root,
		Generation:  generation.GenerationID,
		GrantedAt:   time.Now().UTC(),
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
