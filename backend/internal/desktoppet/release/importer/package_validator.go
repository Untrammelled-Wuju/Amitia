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
	"path"
	"sort"
	"strings"

	"github.com/u-ai/backend/internal/desktoppet/security"
)

type DefaultPackageValidator struct {
	registry *security.PathRootRegistry
	repo     security.ImportStagingRepository
}

type PackageManifest struct {
	SchemaVersion  int    `json:"schemaVersion"`
	ManifestFormat string `json:"manifestFormat"`
	PetID          string `json:"petId"`
	ReleaseID      string `json:"releaseId"`
	Version        string `json:"version"`
	Name           string `json:"name"`

	Compatibility struct {
		MinRuntimeVersion string `json:"minRuntimeVersion"`
		RenderMode        string `json:"renderMode"`
	} `json:"compatibility"`

	Binding struct {
		Policy            string `json:"policy"`
		SourceCharacterID string `json:"sourceCharacterId"`
	} `json:"binding"`

	DefaultAction string `json:"defaultAction"`

	Actions []struct {
		Key        string `json:"key"`
		Name       string `json:"name"`
		Config     string `json:"config"`
		FrameCount int    `json:"frameCount"`
		FPS        int    `json:"fps"`
	} `json:"actions"`

	Integrity struct {
		Algorithm       string `json:"algorithm"`
		ManifestHash    string `json:"manifestHash"`
		ContentRootHash string `json:"contentRootHash"`
		FileCount       int    `json:"fileCount"`
		TotalBytes      int64  `json:"totalBytes"`
		Files           []struct {
			Path      string `json:"path"`
			SHA256    string `json:"sha256"`
			Bytes     int64  `json:"bytes"`
			MediaType string `json:"mediaType"`
			Role      string `json:"role"`
			ActionKey string `json:"actionKey"`
			FrameID   string `json:"frameId"`
		} `json:"files"`
	} `json:"integrity"`

	License struct {
		SPDX       string `json:"spdx"`
		NoticePath string `json:"noticePath"`
	} `json:"license"`
}

type inventoryEntry struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
	Hash string `json:"hash"`
}

func NewDefaultPackageValidator(
	registry *security.PathRootRegistry,
	repo security.ImportStagingRepository,
) *DefaultPackageValidator {
	return &DefaultPackageValidator{
		registry: registry,
		repo:     repo,
	}
}

func (
	v *DefaultPackageValidator,
) ValidatePackage(
	ctx context.Context,
	userID string,
	stagingID string,
) (
	*ImportValidationResult,
	error,
) {
	if v == nil ||
		v.registry == nil ||
		v.repo == nil {
		return nil, errors.New(
			"package validator dependencies missing",
		)
	}

	staging, err :=
		v.repo.GetForUser(
			ctx,
			stagingID,
			userID,
		)
	if err != nil {
		return nil, err
	}

	if staging.Status !=
		security.StagingStatusConsuming &&
		staging.Status !=
			security.StagingStatusReady {
		return nil, errors.New(
			"staging status is invalid",
		)
	}

	if staging.StorageKey == "" ||
		staging.InventoryJSON == "" ||
		staging.InventoryHash == "" {
		return nil, errors.New(
			"staging inventory is incomplete",
		)
	}

	sourcePath, err :=
		v.registry.Resolve(
			security.RootImportQuarantine,
			staging.StorageKey,
		)
	if err != nil {
		return nil, err
	}

	archive, err :=
		zip.OpenReader(sourcePath)
	if err != nil {
		return nil, err
	}
	defer archive.Close()

	manifestFile :=
		findZipFile(
			archive.File,
			"manifest.json",
		)
	if manifestFile == nil {
		return nil, errors.New(
			"manifest.json is missing",
		)
	}

	if manifestFile.UncompressedSize64 >
		4*1024*1024 {
		return nil, errors.New(
			"manifest.json is too large",
		)
	}

	reader, err :=
		manifestFile.Open()
	if err != nil {
		return nil, err
	}

	rawManifest, err :=
		io.ReadAll(
			io.LimitReader(
				reader,
				4*1024*1024+1,
			),
		)
	closeErr := reader.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if len(rawManifest) >
		4*1024*1024 {
		return nil, errors.New(
			"manifest.json is too large",
		)
	}

	var manifest PackageManifest
	if err := json.Unmarshal(
		rawManifest,
		&manifest,
	); err != nil {
		return nil, fmt.Errorf(
			"decode manifest: %w",
			err,
		)
	}

	if err :=
		validateManifestFields(
			&manifest,
		); err != nil {
		return nil, err
	}

	var inventory []inventoryEntry
	if err := json.Unmarshal(
		[]byte(staging.InventoryJSON),
		&inventory,
	); err != nil {
		return nil, fmt.Errorf(
			"decode inventory: %w",
			err,
		)
	}

	if err :=
		validateManifestIntegrity(
			&manifest,
			inventory,
		); err != nil {
		return nil, err
	}

	manifestHash :=
		sha256.Sum256(rawManifest)

	selectedActions :=
		make([]string, 0,
			len(manifest.Actions))

	for _, action :=
		range manifest.Actions {
		selectedActions =
			append(
				selectedActions,
				action.Key,
			)
	}

	sort.Strings(selectedActions)

	licenseDecision :=
		"unknown"
	if strings.TrimSpace(
		manifest.License.SPDX,
	) != "" {
		licenseDecision = "declared"
	}

	return &ImportValidationResult{
		IsValid: true,
		SourcePackageHash:
			staging.SourceContentHash,
		SourceManifestHash:
			hex.EncodeToString(
				manifestHash[:],
			),
		SourceSchemaVersion:
			manifest.SchemaVersion,
		Warnings: nil,
		BindingDecision:
			manifest.Binding.Policy,
		LicenseDecision:
			licenseDecision,
		RuntimeCompatibility:
			manifest.Compatibility.
				MinRuntimeVersion,
		SelectedActions:
			selectedActions,
		Manifest:
			&manifest,
	}, nil
}

func findZipFile(
	files []*zip.File,
	target string,
) *zip.File {
	for _, file := range files {
		clean := path.Clean(
			strings.ReplaceAll(
				file.Name,
				`\`,
				"/",
			),
		)
		if clean == target {
			return file
		}
	}
	return nil
}

func validateManifestFields(
	manifest *PackageManifest,
) error {
	if manifest.SchemaVersion != 2 {
		return errors.New(
			"unsupported package schema version",
		)
	}

	if manifest.ManifestFormat !=
		"amitia-desktop-pet" {
		return errors.New(
			"invalid manifest format",
		)
	}

	if strings.TrimSpace(
		manifest.Name,
	) == "" ||
		strings.TrimSpace(
			manifest.DefaultAction,
		) == "" ||
		len(manifest.Actions) == 0 {
		return errors.New(
			"manifest required fields missing",
		)
	}

	if manifest.Compatibility.
		RenderMode != "sprite" {
		return errors.New(
			"unsupported render mode",
		)
	}

	actionKeys :=
		map[string]struct{}{}

	for _, action :=
		range manifest.Actions {
		key := strings.TrimSpace(
			action.Key,
		)
		if key == "" ||
			action.Config == "" ||
			action.FrameCount <= 0 ||
			action.FPS <= 0 ||
			action.FPS > 120 {
			return errors.New(
				"invalid action definition",
			)
		}

		if _, exists :=
			actionKeys[key];
			exists {
			return errors.New(
				"duplicate action key",
			)
		}

		actionKeys[key] =
			struct{}{}
	}

	if _, exists := actionKeys[manifest.DefaultAction]; !exists {
		return errors.New(
			"default action is missing",
		)
	}

	if manifest.Integrity.Algorithm !=
		"amitia-package-sha256-v2" {
		return errors.New(
			"unsupported integrity algorithm",
		)
	}

	return nil
}

func validateManifestIntegrity(
	manifest *PackageManifest,
	inventory []inventoryEntry,
) error {
	inventoryMap :=
		make(map[string]inventoryEntry)

	for _, entry :=
		range inventory {
		inventoryMap[entry.Path] =
			entry
	}

	if manifest.Integrity.FileCount !=
		len(manifest.Integrity.Files) {
		return errors.New(
			"manifest fileCount mismatch",
		)
	}

	var total int64

	for _, file :=
		range manifest.Integrity.Files {
		entry, ok :=
			inventoryMap[file.Path]
		if !ok {
			return fmt.Errorf(
				"manifest file missing: %s",
				file.Path,
			)
		}

		if !strings.EqualFold(
			entry.Hash,
			file.SHA256,
		) ||
			entry.Size != file.Bytes {
			return fmt.Errorf(
				"manifest file hash mismatch: %s",
				file.Path,
			)
		}

		total += file.Bytes
	}

	if total !=
		manifest.Integrity.TotalBytes {
		return errors.New(
			"manifest totalBytes mismatch",
		)
	}

	return nil
}
