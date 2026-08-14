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
	install, err := a.installRepo.GetActiveInstallation(userID)
	if err != nil {
		if err == installation.ErrInstallationNotFound {
			return nil, behavior.NewBehaviorError(behavior.ErrCodeNoActiveInstallation, "no active installation")
		}
		return nil, err
	}

	if install.Status != installation.StatusEnabled {
		return nil, behavior.NewBehaviorError(behavior.ErrCodeNoActiveInstallation, "active installation not enabled")
	}

	runtimeOnline := false
	petInstanceID := ""

	if a.facade != nil {
		conns := a.facade.ListConnections(userID)
		for _, conn := range conns {
			if conn != nil && conn.State == runtimev2.ConnStateConnected {
				runtimeOnline = true
				petInstanceID = string(conn.RuntimeID)
				break
			}
		}
	}

	manifest, err := a.manifestReader(
		filepath.Join(a.dataDir, filepath.FromSlash(install.InstallPath)),
		filepath.Join(a.dataDir, filepath.FromSlash(install.ManifestPath)),
	)
	if err != nil {
		log.Logger.Warnf("wiring/active_pet_v2_port: failed to read manifest installationId=%s err=%v", install.ID, err)
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

	stateRevision := int64(install.StateRevision)

	return &behavior.ActivePetSnapshot{
		InstallationID: install.ID,
		ReleaseID:      install.CurrentReleaseID,
		PetInstanceID:  petInstanceID,
		CharacterID:    install.CharacterID,
		RuntimeOnline:  runtimeOnline,
		StateRevision:  stateRevision,
		DefaultAction:  install.DefaultActionKey,
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
