package wiring

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/u-ai/backend/internal/desktoppet/behavior"
	"github.com/u-ai/backend/internal/desktoppet/installation"
	"github.com/u-ai/backend/internal/desktoppet/processing"
	runtimev2 "github.com/u-ai/backend/internal/desktoppet/runtime/protocol/v2"
	"github.com/u-ai/backend/log"
)

type V2ActivePetAdapter struct {
	installRepo    installation.Repository
	facade         *runtimev2.RuntimeFacade
	dataDir        string
	manifestReader func(installPath, manifestPath string) (*processing.Manifest, error)
}

func NewV2ActivePetAdapter(installRepo installation.Repository, facade *runtimev2.RuntimeFacade, dataDir string) *V2ActivePetAdapter {
	return &V2ActivePetAdapter{
		installRepo:    installRepo,
		facade:         facade,
		dataDir:        dataDir,
		manifestReader: v2ReadManifestFromDisk,
	}
}

func (a *V2ActivePetAdapter) ResolveActivePet(ctx context.Context, userID, characterID string) (*behavior.ActivePetSnapshot, error) {
	if a.installRepo == nil {
		return nil, behavior.NewBehaviorError(behavior.ErrCodeNoActiveInstallation, "installation repository unavailable")
	}

	var selected *installation.Installation
	var selectedDeviceID string
	var selectedRuntimeID string
	runtimeOnline := false

	if a.facade != nil {
		for _, conn := range a.facade.ListConnections(userID) {
			if conn == nil || conn.State != runtimev2.ConnStateConnected {
				continue
			}
			deviceID := string(conn.DeviceID)
			installations, err := a.installRepo.ListInstallationsForUserDevice(userID, deviceID)
			if err != nil {
				continue
			}
			for _, candidate := range installations {
				if candidate == nil || candidate.Status != installation.StatusEnabled || candidate.IsActive != 1 {
					continue
				}
				if characterID != "" && candidate.CharacterID != characterID {
					continue
				}
				selected = candidate
				selectedDeviceID = deviceID
				selectedRuntimeID = string(conn.RuntimeID)
				runtimeOnline = true
				break
			}
			if selected != nil {
				break
			}
		}
	}

	if selected == nil {
		installations, err := a.installRepo.ListInstallationsByUser(userID)
		if err != nil {
			return nil, err
		}
		for _, candidate := range installations {
			if candidate == nil || candidate.Status != installation.StatusEnabled || candidate.IsActive != 1 {
				continue
			}
			if characterID != "" && candidate.CharacterID != characterID {
				continue
			}
			selected = candidate
			selectedDeviceID = candidate.DeviceID
			break
		}
	}

	if selected == nil {
		return nil, behavior.NewBehaviorError(behavior.ErrCodeNoActiveInstallation, "no active installation for character")
	}

	manifest, err := a.manifestReader(
		filepath.Join(a.dataDir, filepath.FromSlash(selected.InstallPath)),
		filepath.Join(a.dataDir, filepath.FromSlash(selected.ManifestPath)),
	)
	if err != nil {
		log.Logger.Warnf("wiring/active_pet_v2_port: failed to read manifest installationId=%s err=%v", selected.ID, err)
	}

	actions := make(map[string]behavior.ActionCapability)
	if manifest != nil {
		for _, action := range manifest.Actions {
			actions[action.Key] = behavior.ActionCapability{
				Key:         action.Key,
				Name:        action.Name,
				CategoryKey: v2InferCategoryFromKey(action.Key),
				Available:   true,
			}
		}
	}

	stateRevision := int64(selected.StateRevision)
	return &behavior.ActivePetSnapshot{
		UserID:         userID,
		DeviceID:       selectedDeviceID,
		RuntimeID:      selectedRuntimeID,
		InstallationID: selected.ID,
		ReleaseID:      selected.CurrentReleaseID,
		PetInstanceID:  selectedRuntimeID,
		CharacterID:    selected.CharacterID,
		RuntimeOnline:  runtimeOnline,
		StateRevision:  stateRevision,
		DefaultAction:  selected.DefaultActionKey,
		Actions:        actions,
	}, nil
}

func v2ReadManifestFromDisk(installPath, manifestPath string) (*processing.Manifest, error) {
	if manifestPath == "" {
		return nil, nil
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}
	var manifest processing.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	return &manifest, nil
}

func v2InferCategoryFromKey(actionKey string) string {
	switch {
	case actionKey == "" || actionKey == "idle_normal" || actionKey == "idle_blink":
		return "idle"
	case actionKey == "walk_left" || actionKey == "walk_right" || actionKey == "walk_up" || actionKey == "walk_down":
		return "movement"
	case actionKey == "wave" || actionKey == "happy" || actionKey == "sad" || actionKey == "angry" || actionKey == "surprised":
		return "emotion"
	case actionKey == "speaking" || actionKey == "listening":
		return "dialogue"
	case actionKey == "sleeping" || actionKey == "eating" || actionKey == "reading":
		return "life"
	default:
		return "interaction"
	}
}

var _ behavior.ActivePetPort = (*V2ActivePetAdapter)(nil)
