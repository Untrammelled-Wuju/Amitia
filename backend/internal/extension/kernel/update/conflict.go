package update

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

type UserAssetType string

const (
	UserAssetForkWorkflow  UserAssetType = "fork_workflow"
	UserAssetModifiedSkill UserAssetType = "modified_skill"
	UserAssetUserMCP       UserAssetType = "user_mcp"
	UserAssetUserStorage   UserAssetType = "user_storage"
	UserAssetUserSecret    UserAssetType = "user_secret"
	UserAssetUISetting     UserAssetType = "ui_setting"
)

type UserAsset struct {
	AssetID     string
	ExtensionID string
	AssetType   UserAssetType
	ResourceID  string
	Owner       string
	Hash        string
	ModifiedAt  time.Time
}

type AssetConflict struct {
	ConflictID    string
	ExtensionID   string
	AssetType     UserAssetType
	ResourceID    string
	OldHash       string
	NewHash       string
	ConflictType  string
	Resolution    ConflictResolution
	UserAssetHash string
}

type ConflictResolution string

const (
	ConflictKeepUser     ConflictResolution = "keep_user"
	ConflictUseNew       ConflictResolution = "use_new"
	ConflictMerge        ConflictResolution = "merge"
	ConflictBlock        ConflictResolution = "block"
	ConflictRequiresUser ConflictResolution = "requires_user_decision"
)

type ConflictRegistry struct {
	mu        sync.RWMutex
	conflicts map[string]AssetConflict
	assets    map[string][]UserAsset
}

func NewConflictRegistry() *ConflictRegistry {
	return &ConflictRegistry{
		conflicts: make(map[string]AssetConflict),
		assets:    make(map[string][]UserAsset),
	}
}

func (r *ConflictRegistry) RegisterAsset(asset UserAsset) error {
	if asset.AssetID == "" {
		return errors.New("update: asset id required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.assets[asset.ExtensionID] = append(r.assets[asset.ExtensionID], asset)
	return nil
}

func (r *ConflictRegistry) DetectConflicts(extensionID string, newAssets []UserAsset) []AssetConflict {
	r.mu.RLock()
	oldAssets := r.assets[extensionID]
	r.mu.RUnlock()

	oldByResource := make(map[string]UserAsset)
	for _, a := range oldAssets {
		oldByResource[a.ResourceID] = a
	}

	var conflicts []AssetConflict
	for _, newAsset := range newAssets {
		if old, ok := oldByResource[newAsset.ResourceID]; ok {
			if old.Hash != newAsset.Hash {
				conflict := AssetConflict{
					ConflictID:    fmt.Sprintf("conflict-%s-%s", extensionID, newAsset.ResourceID),
					ExtensionID:   extensionID,
					AssetType:     newAsset.AssetType,
					ResourceID:    newAsset.ResourceID,
					OldHash:       old.Hash,
					NewHash:       newAsset.Hash,
					ConflictType:  "hash_mismatch",
					Resolution:    ConflictRequiresUser,
					UserAssetHash: old.Hash,
				}
				conflicts = append(conflicts, conflict)
			}
		}
	}

	r.mu.Lock()
	for _, c := range conflicts {
		r.conflicts[c.ConflictID] = c
	}
	r.mu.Unlock()

	return conflicts
}

func (r *ConflictRegistry) ResolveConflict(conflictID string, resolution ConflictResolution) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.conflicts[conflictID]
	if !ok {
		return fmt.Errorf("update: conflict %s not found", conflictID)
	}
	c.Resolution = resolution
	r.conflicts[conflictID] = c
	return nil
}

func (r *ConflictRegistry) ListConflicts(extensionID string) []AssetConflict {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []AssetConflict
	for _, c := range r.conflicts {
		if c.ExtensionID == extensionID {
			result = append(result, c)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ConflictID < result[j].ConflictID
	})
	return result
}

func (r *ConflictRegistry) HasUnresolvedConflicts(extensionID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, c := range r.conflicts {
		if c.ExtensionID == extensionID && c.Resolution == ConflictRequiresUser {
			return true
		}
	}
	return false
}

func (r *ConflictRegistry) ClearAsset(extensionID, resourceID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	assets := r.assets[extensionID]
	for i, a := range assets {
		if a.ResourceID == resourceID {
			r.assets[extensionID] = append(assets[:i], assets[i+1:]...)
			return
		}
	}
}
