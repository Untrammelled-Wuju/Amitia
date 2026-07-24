// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package processing

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const ManifestSchemaVersion = 1

type ManifestCanvas struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type ManifestAction struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Config string `json:"config"`
}

type ManifestCapabilities struct {
	HasTransparentBackground bool `json:"hasTransparentBackground"`
	SupportsFrameSequence    bool `json:"supportsFrameSequence"`
}

type Manifest struct {
	SchemaVersion     int                  `json:"schemaVersion"`
	PackageID         string               `json:"packageId"`
	Name              string               `json:"name"`
	CharacterID       string               `json:"characterId"`
	GenerationTaskID  string               `json:"generationTaskId"`
	ProcessingVersion int                  `json:"processingVersion"`
	CreatedAt         string               `json:"createdAt"`
	Canvas            ManifestCanvas       `json:"canvas"`
	DefaultAction     string               `json:"defaultAction"`
	Preview           string               `json:"preview"`
	Actions           []ManifestAction     `json:"actions"`
	Capabilities      ManifestCapabilities `json:"capabilities"`
}

type ManifestBuilder struct {
	dataDir string
}

func NewManifestBuilder(dataDir string) *ManifestBuilder {
	return &ManifestBuilder{dataDir: dataDir}
}

func BuildManifest(packageID, name, characterID, generationTaskID string, processingVersion int, canvasWidth, canvasHeight int, defaultAction string, actions []ManifestAction) *Manifest {
	return &Manifest{
		SchemaVersion:     ManifestSchemaVersion,
		PackageID:         packageID,
		Name:              name,
		CharacterID:       characterID,
		GenerationTaskID:  generationTaskID,
		ProcessingVersion: processingVersion,
		CreatedAt:         time.Now().UTC().Format(time.RFC3339),
		Canvas: ManifestCanvas{
			Width:  canvasWidth,
			Height: canvasHeight,
		},
		DefaultAction: defaultAction,
		Preview:       "preview.png",
		Actions:       actions,
		Capabilities: ManifestCapabilities{
			HasTransparentBackground: true,
			SupportsFrameSequence:    true,
		},
	}
}

func BuildManifestAction(actionKey, actionName string) ManifestAction {
	return ManifestAction{
		Key:    actionKey,
		Name:   actionName,
		Config: fmt.Sprintf("actions/%s/action.json", actionKey),
	}
}

func (b *ManifestBuilder) WriteManifest(taskID, packageID string, manifest *Manifest) (string, error) {
	relDir := filepath.Join("desktop-pets", "generation-tasks", taskID, "packages", packageID)
	absDir := filepath.Join(b.dataDir, relDir)
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return "", err
	}
	absPath := filepath.Join(absDir, "manifest.json")
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(absPath, data, 0o644); err != nil {
		return "", err
	}
	relPath := filepath.ToSlash(filepath.Join(relDir, "manifest.json"))
	return relPath, nil
}

func ValidateManifest(manifest *Manifest) error {
	if manifest == nil {
		return fmt.Errorf("manifest is nil")
	}
	if manifest.SchemaVersion != ManifestSchemaVersion {
		return fmt.Errorf("unsupported schemaVersion: %d", manifest.SchemaVersion)
	}
	if !isSafeRelativePath(manifest.Preview) {
		return fmt.Errorf("invalid preview path: %s", manifest.Preview)
	}
	for _, action := range manifest.Actions {
		if !isSafeRelativePath(action.Config) {
			return fmt.Errorf("invalid action config path: %s", action.Config)
		}
	}
	if manifest.DefaultAction == "" {
		return fmt.Errorf("defaultAction is empty")
	}
	found := false
	for _, action := range manifest.Actions {
		if action.Key == manifest.DefaultAction {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("defaultAction %s not found in actions", manifest.DefaultAction)
	}
	return nil
}

func isSafeRelativePath(p string) bool {
	if p == "" {
		return false
	}
	if strings.Contains(p, "..") {
		return false
	}
	if strings.Contains(p, "\\") {
		return false
	}
	if strings.Contains(p, ":") {
		return false
	}
	if strings.HasPrefix(p, "/") {
		return false
	}
	return true
}
