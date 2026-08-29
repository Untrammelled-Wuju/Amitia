package wiring

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

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
	return a.resolveActivePet(ctx, userID, characterID, "", "")
}

func (a *V2ActivePetAdapter) ResolveActivePetForEvent(ctx context.Context, event behavior.BehaviorEventEnvelope) (*behavior.ActivePetSnapshot, error) {
	return a.resolveActivePet(ctx, event.UserID, event.CharacterID, event.InstallationID, event.PetInstanceID)
}

func (a *V2ActivePetAdapter) resolveActivePet(ctx context.Context, userID, characterID, installationHint, petInstanceHint string) (*behavior.ActivePetSnapshot, error) {
	if a.installRepo == nil {
		return nil, behavior.NewBehaviorError(behavior.ErrCodeNoActiveInstallation, "installation repository unavailable")
	}

	var selected *installation.Installation
	var selectedRuntimeID string
	runtimeOnline := false

	// Preserve explicit event affinity first. Runtime/playback events already
	// carry installation or pet-instance identity and must never jump devices.
	if installationHint != "" {
		candidate, err := a.installRepo.GetInstallation(installationHint)
		if err != nil {
			return nil, err
		}
		if !v2InstallationMatches(candidate, userID, characterID) {
			return nil, behavior.NewBehaviorError(behavior.ErrCodeNoActiveInstallation, "event installation is not active for character")
		}
		selected = candidate
	}

	connections := []*runtimev2.Connection(nil)
	if a.facade != nil {
		connections = a.facade.ListConnections(userID)
	}

	if selected == nil && petInstanceHint != "" {
		for _, conn := range connections {
			if conn == nil || conn.GetState() != runtimev2.ConnStateConnected || string(conn.RuntimeID) != petInstanceHint {
				continue
			}
			installations, err := a.installRepo.ListInstallationsForUserDevice(userID, string(conn.DeviceID))
			if err != nil {
				return nil, err
			}
			v2SortInstallations(installations)
			for _, candidate := range installations {
				if v2InstallationMatches(candidate, userID, characterID) {
					selected = candidate
					selectedRuntimeID = string(conn.RuntimeID)
					runtimeOnline = true
					break
				}
			}
			if selected != nil {
				break
			}
		}
		if selected == nil {
			return nil, behavior.NewBehaviorError(behavior.ErrCodeNoActiveInstallation, "event runtime is not connected for character")
		}
	}

	if selected == nil {
		// Generic behavior events have no device identity. Choose the most
		// recently enabled active installation across all connected devices,
		// then use stable IDs as a deterministic tie-breaker. This avoids both
		// Go-map randomness and an arbitrary lexical-device preference.
		type onlineCandidate struct {
			installation *installation.Installation
			runtimeID    string
		}
		var candidates []onlineCandidate
		for _, conn := range connections {
			if conn == nil || conn.GetState() != runtimev2.ConnStateConnected {
				continue
			}
			installations, err := a.installRepo.ListInstallationsForUserDevice(userID, string(conn.DeviceID))
			if err != nil {
				return nil, err
			}
			for _, candidate := range installations {
				if v2InstallationMatches(candidate, userID, characterID) {
					candidates = append(candidates, onlineCandidate{installation: candidate, runtimeID: string(conn.RuntimeID)})
				}
			}
		}
		sort.SliceStable(candidates, func(i, j int) bool {
			left, right := candidates[i].installation, candidates[j].installation
			if v2InstallationActivityKey(left) != v2InstallationActivityKey(right) {
				return v2InstallationActivityKey(left) > v2InstallationActivityKey(right)
			}
			if left.DeviceID != right.DeviceID {
				return left.DeviceID < right.DeviceID
			}
			if left.ID != right.ID {
				return left.ID < right.ID
			}
			return candidates[i].runtimeID < candidates[j].runtimeID
		})
		if len(candidates) > 0 {
			selected = candidates[0].installation
			selectedRuntimeID = candidates[0].runtimeID
			runtimeOnline = true
		}
	}

	if selected == nil {
		installations, err := a.installRepo.ListInstallationsByUser(userID)
		if err != nil {
			return nil, err
		}
		v2SortInstallations(installations)
		for _, candidate := range installations {
			if v2InstallationMatches(candidate, userID, characterID) {
				selected = candidate
				break
			}
		}
	}

	if selected == nil {
		return nil, behavior.NewBehaviorError(behavior.ErrCodeNoActiveInstallation, "no active installation for character")
	}

	// If an explicit installation was selected before connection resolution,
	// bind it only to the connection for that device (and, when supplied, that
	// pet instance/runtime). This keeps cloud-triggered behavior device-local.
	if !runtimeOnline {
		for _, conn := range connections {
			if conn == nil || conn.GetState() != runtimev2.ConnStateConnected {
				continue
			}
			if string(conn.DeviceID) != selected.DeviceID {
				continue
			}
			if petInstanceHint != "" && string(conn.RuntimeID) != petInstanceHint {
				continue
			}
			selectedRuntimeID = string(conn.RuntimeID)
			runtimeOnline = true
			break
		}
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
		DeviceID:       selected.DeviceID,
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

func v2SortInstallations(installations []*installation.Installation) {
	sort.SliceStable(installations, func(i, j int) bool {
		if installations[i] == nil {
			return false
		}
		if installations[j] == nil {
			return true
		}
		if v2InstallationActivityKey(installations[i]) != v2InstallationActivityKey(installations[j]) {
			return v2InstallationActivityKey(installations[i]) > v2InstallationActivityKey(installations[j])
		}
		if installations[i].DeviceID != installations[j].DeviceID {
			return installations[i].DeviceID < installations[j].DeviceID
		}
		return installations[i].ID < installations[j].ID
	})
}

func v2InstallationActivityKey(candidate *installation.Installation) string {
	if candidate == nil {
		return ""
	}
	if candidate.LastEnabledAt != "" {
		return candidate.LastEnabledAt
	}
	return candidate.UpdatedAt
}

func v2InstallationMatches(candidate *installation.Installation, userID, characterID string) bool {
	if candidate == nil || candidate.UserID != userID || candidate.Status != installation.StatusEnabled || candidate.IsActive != 1 {
		return false
	}
	return characterID == "" || candidate.CharacterID == characterID
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
var _ behavior.EventTargetedActivePetPort = (*V2ActivePetAdapter)(nil)
