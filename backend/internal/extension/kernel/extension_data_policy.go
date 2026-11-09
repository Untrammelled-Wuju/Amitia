package kernel

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type extensionDataPolicy struct {
	Backup    bool
	Export    bool
	Uninstall string
}

type extensionStateSnapshot struct {
	Path string
	Hash string
}

func extensionDataPolicyFromDefinition(definition domain.ExtensionDefinition) extensionDataPolicy {
	policy := extensionDataPolicy{Uninstall: "retain"}
	if definition.Metadata == nil {
		return policy
	}
	if value, ok := definition.Metadata["data.backup"].(bool); ok {
		policy.Backup = value
	}
	if value, ok := definition.Metadata["data.export"].(bool); ok {
		policy.Export = value
	}
	if value, ok := definition.Metadata["data.uninstall"].(string); ok {
		switch strings.TrimSpace(value) {
		case "retain", "delete", "export":
			policy.Uninstall = strings.TrimSpace(value)
		}
	}
	return policy
}

func (r *Runtime) writeExtensionStateSnapshot(ctx context.Context, extensionID, operationID, category string) (extensionStateSnapshot, error) {
	if r.container == nil || r.container.ExtensionStateStore == nil {
		return extensionStateSnapshot{}, nil
	}
	payload, err := r.container.ExtensionStateStore.Export(ctx, extensionID)
	if err != nil {
		return extensionStateSnapshot{}, err
	}
	targetDir := filepath.Join(r.container.ExtRoot, "extension-data", safeDirectoryName(extensionID), category)
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		return extensionStateSnapshot{}, fmt.Errorf("kernel: create extension data snapshot directory: %w", err)
	}
	targetPath := filepath.Join(targetDir, safeDirectoryName(operationID)+".json")
	tempPath := targetPath + ".tmp"
	if err := os.WriteFile(tempPath, payload, 0o600); err != nil {
		return extensionStateSnapshot{}, fmt.Errorf("kernel: write extension data snapshot: %w", err)
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		_ = os.Remove(tempPath)
		return extensionStateSnapshot{}, fmt.Errorf("kernel: commit extension data snapshot: %w", err)
	}
	sum := sha256.Sum256(payload)
	return extensionStateSnapshot{Path: targetPath, Hash: fmt.Sprintf("%x", sum)}, nil
}
