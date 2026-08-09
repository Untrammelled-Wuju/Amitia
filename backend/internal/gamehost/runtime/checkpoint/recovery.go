package checkpoint

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/storage"
)

type RecoveryStatus string

const (
	RecoveryStatusClean    RecoveryStatus = "clean"
	RecoveryStatusUnclean  RecoveryStatus = "unclean"
	RecoveryStatusStale    RecoveryStatus = "stale"
	RecoveryStatusOrphaned RecoveryStatus = "orphaned"
	RecoveryStatusInvalid  RecoveryStatus = "invalid"
	RecoveryStatusMissing  RecoveryStatus = "missing"
)

type StoredRuntimeInfo struct {
	RuntimeID          domain.RuntimeInstanceID
	PluginID           domain.PluginID
	RecoveryStatus     RecoveryStatus
	RuntimeState       domain.RuntimeState
	CleanShutdown      bool
	ExtensionID        string
	PluginVersion      string
	DescriptorRevision string
}

type RecoveryClassifier struct {
	dir      storage.DirectoryManager
	store    CheckpointStore
	resolver DescriptorResolver
}

func NewRecoveryClassifier(dir storage.DirectoryManager, store CheckpointStore, resolver DescriptorResolver) (*RecoveryClassifier, error) {
	if dir == nil {
		return nil, &CheckpointError{Op: "new_classifier", Kind: ErrInvalidSchema, ID: "", Cause: errorString("directory manager must not be nil")}
	}
	if store == nil {
		return nil, &CheckpointError{Op: "new_classifier", Kind: ErrInvalidSchema, ID: "", Cause: errorString("store must not be nil")}
	}
	return &RecoveryClassifier{
		dir:      dir,
		store:    store,
		resolver: resolver,
	}, nil
}

func (c *RecoveryClassifier) ListStoredRuntimeIDs(ctx context.Context) ([]domain.RuntimeInstanceID, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	runtimesDir, err := c.runtimesDirectory()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(runtimesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []domain.RuntimeInstanceID{}, nil
		}
		return nil, newError("list_stored", ErrCorrupt, "", err)
	}

	var result []domain.RuntimeInstanceID
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirName := entry.Name()
		if !strings.HasPrefix(dirName, "run-") {
			continue
		}
		runtimeID, ok := c.resolveRuntimeIDFromDir(ctx, dirName)
		if !ok {
			continue
		}
		result = append(result, runtimeID)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i] < result[j]
	})
	return result, nil
}

func (c *RecoveryClassifier) runtimesDirectory() (string, error) {
	paths, err := c.dir.ResolveRuntimePaths("__scan__")
	if err != nil {
		return "", err
	}
	return filepath.Dir(paths.Root), nil
}

func (c *RecoveryClassifier) resolveRuntimeIDFromDir(ctx context.Context, dirName string) (domain.RuntimeInstanceID, bool) {
	dir, err := c.runtimesDirectory()
	if err != nil {
		return "", false
	}
	metaPath := filepath.Join(dir, dirName, metadataFileName)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return "", false
	}
	var md RuntimeMetadata
	if err := json.Unmarshal(data, &md); err != nil {
		return "", false
	}
	if md.RuntimeID == "" {
		return "", false
	}
	return md.RuntimeID, true
}

func (c *RecoveryClassifier) Classify(ctx context.Context, runtimeID domain.RuntimeInstanceID) (StoredRuntimeInfo, error) {
	var info StoredRuntimeInfo
	info.RuntimeID = runtimeID

	metadata, err := c.store.LoadMetadata(ctx, runtimeID)
	if err != nil {
		if isErrKind(err, ErrNotFound) {
			info.RecoveryStatus = RecoveryStatusMissing
			return info, nil
		}
		if isErrKind(err, ErrCorruptMetadata) {
			info.RecoveryStatus = RecoveryStatusInvalid
			return info, err
		}
		return info, err
	}

	info.PluginID = metadata.PluginID
	info.ExtensionID = metadata.ExtensionID
	info.PluginVersion = metadata.PluginVersion
	info.DescriptorRevision = metadata.DescriptorRevision
	info.RuntimeState = domain.RuntimeStateCreated
	info.CleanShutdown = false

	checkpoint, err := c.store.LoadCheckpoint(ctx, runtimeID)
	if err != nil {
		if isErrKind(err, ErrNotFound) {
			info.RecoveryStatus = RecoveryStatusInvalid
			return info, nil
		}
		if isErrKind(err, ErrCorrupt) {
			info.RecoveryStatus = RecoveryStatusInvalid
			return info, err
		}
		return info, err
	}

	if checkpoint.RuntimeID != metadata.RuntimeID || checkpoint.PluginID != metadata.PluginID {
		info.RecoveryStatus = RecoveryStatusInvalid
		return info, errRuntimeIDMismatch("classify", string(runtimeID), errIDMismatch(string(checkpoint.RuntimeID), string(metadata.RuntimeID)))
	}

	info.RuntimeState = checkpoint.RuntimeState
	info.CleanShutdown = checkpoint.CleanShutdown
	info.DescriptorRevision = checkpoint.DescriptorRevision

	checkpointOK := true

	if checkpoint.DescriptorRevision != "" && c.resolver != nil {
		if descriptor, ok := c.resolver.Resolve(metadata.PluginID); ok {
			expectedRev := ComputeDescriptorRevision(descriptor)
			if checkpoint.DescriptorRevision != expectedRev {
				info.RecoveryStatus = RecoveryStatusStale
				checkpointOK = false
			}
		}
	}

	if info.RecoveryStatus == "" && checkpointOK {
		if c.resolver != nil {
			if _, ok := c.resolver.Resolve(metadata.PluginID); !ok {
				info.RecoveryStatus = RecoveryStatusOrphaned
				return info, nil
			}
		}

		if checkpoint.CleanShutdown {
			info.RecoveryStatus = RecoveryStatusClean
		} else if domain.IsActiveRuntimeState(checkpoint.RuntimeState) {
			info.RecoveryStatus = RecoveryStatusUnclean
		} else {
			info.RecoveryStatus = RecoveryStatusClean
		}
	}

	return info, nil
}

func (c *RecoveryClassifier) ListStoredRuntimes(ctx context.Context) ([]StoredRuntimeInfo, error) {
	ids, err := c.ListStoredRuntimeIDs(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]StoredRuntimeInfo, 0, len(ids))
	for _, id := range ids {
		info, _ := c.Classify(ctx, id)
		result = append(result, info)
	}
	return result, nil
}
