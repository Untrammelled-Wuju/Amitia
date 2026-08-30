package importer

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/u-ai/backend/internal/desktoppet/packageformat"
	"github.com/u-ai/backend/internal/desktoppet/security"
)

type ImportInventoryEntry struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	Hash string `json:"hash"`
}

type ImportInspector struct {
	registry *security.PathRootRegistry
	repo     security.ImportStagingRepository
}

func NewImportInspector(
	registry *security.PathRootRegistry,
	repo security.ImportStagingRepository,
) *ImportInspector {
	return &ImportInspector{
		registry: registry,
		repo:     repo,
	}
}

func (i *ImportInspector) InspectAndMarkReady(
	ctx context.Context,
	staging *security.ImportStaging,
) error {
	if staging == nil {
		return errors.New("staging is nil")
	}

	filePath, err := i.registry.Resolve(
		security.RootImportQuarantine,
		staging.StorageKey,
	)
	if err != nil {
		return err
	}

	archive, err := zip.OpenReader(filePath)
	if err != nil {
		return err
	}
	defer archive.Close()

	if len(archive.File) == 0 || len(archive.File) > 2000 {
		return errors.New("archive entry count invalid")
	}

	inventory := make([]ImportInventoryEntry, 0, len(archive.File))
	var total int64
	seen := map[string]struct{}{}

	for _, entry := range archive.File {
		entryName := entry.Name
		if entry.FileInfo().IsDir() {
			entryName = strings.TrimSuffix(entryName, "/")
		}
		clean, pathErr := packageformat.NormalizePackagePath(entryName)
		if pathErr != nil {
			return fmt.Errorf("archive path is not Package V2 canonical: %s: %w", entry.Name, pathErr)
		}

		if clean == "manifest.json" {
			continue
		}

		key := packageformat.CaseFoldPath(clean)
		if _, exists := seen[key]; exists {
			return errors.New("archive path collision detected")
		}
		seen[key] = struct{}{}

		if entry.FileInfo().Mode()&os.ModeSymlink != 0 {
			return errors.New("archive symlink is not allowed")
		}

		if entry.UncompressedSize64 > 64*1024*1024 {
			return errors.New("archive entry too large")
		}

		total += int64(entry.UncompressedSize64)
		if total > 512*1024*1024 {
			return errors.New("archive total size too large")
		}

		if entry.FileInfo().IsDir() {
			continue
		}

		reader, err := entry.Open()
		if err != nil {
			return err
		}

		hash := sha256.New()
		_, copyErr := io.Copy(hash, io.LimitReader(reader, int64(entry.UncompressedSize64)+1))
		closeErr := reader.Close()

		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}

		inventory = append(inventory, ImportInventoryEntry{
			Path: clean,
			Size: int64(entry.UncompressedSize64),
			Hash: hex.EncodeToString(hash.Sum(nil)),
		})
	}

	sort.Slice(inventory, func(a, b int) bool {
		return inventory[a].Path < inventory[b].Path
	})

	raw, err := json.Marshal(inventory)
	if err != nil {
		return err
	}

	inventoryHash := sha256.Sum256(raw)
	updated, err := i.repo.UpdateInventory(
		ctx,
		staging.ID,
		staging.OwnerUserID,
		string(raw),
		hex.EncodeToString(inventoryHash[:]),
	)
	if err != nil {
		return err
	}
	if !updated {
		return errors.New("staging state changed during inspection")
	}

	return nil
}
