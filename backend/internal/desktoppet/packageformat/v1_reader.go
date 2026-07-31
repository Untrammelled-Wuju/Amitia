package packageformat

import (
	"encoding/json"
	"fmt"
)

type v1Canvas struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type v1Action struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Config   string `json:"config"`
	LoopType string `json:"loopType"`
}

type v1Capabilities struct {
	HasTransparentBackground bool `json:"hasTransparentBackground"`
	SupportsFrameSequence    bool `json:"supportsFrameSequence"`
}

type v1Manifest struct {
	SchemaVersion     int            `json:"schemaVersion"`
	PackageID         string         `json:"packageId"`
	Name              string         `json:"name"`
	CharacterID       string         `json:"characterId"`
	GenerationTaskID  string         `json:"generationTaskId"`
	ProcessingVersion int            `json:"processingVersion"`
	CreatedAt         string         `json:"createdAt"`
	Canvas            v1Canvas       `json:"canvas"`
	DefaultAction     string         `json:"defaultAction"`
	Preview           string         `json:"preview"`
	Actions           []v1Action     `json:"actions"`
	Capabilities      v1Capabilities `json:"capabilities"`
}

type V1Reader struct{}

func (r *V1Reader) SchemaVersion() int { return 1 }

func (r *V1Reader) ReadManifest(data []byte) (*Manifest, error) {
	var v1 v1Manifest
	if err := json.Unmarshal(data, &v1); err != nil {
		return nil, NewPackageError(ErrCodePackageManifestInvalid, "failed to unmarshal v1 manifest", err)
	}

	if v1.SchemaVersion != 1 {
		return nil, NewPackageError(
			ErrCodePackageSchemaUnsupported,
			fmt.Sprintf("v1 reader expects schemaVersion=1, got %d", v1.SchemaVersion),
			nil,
		)
	}

	manifest := NewManifest()
	manifest.SchemaVersion = ManifestSchemaVersion
	manifest.ManifestFormat = ManifestFormatCanonical
	manifest.ReleaseID = v1.PackageID
	manifest.PetID = v1.CharacterID
	manifest.Name = v1.Name
	manifest.Version = fmt.Sprintf("legacy-v%d", v1.ProcessingVersion)
	manifest.Description = ""
	manifest.Author = ManifestAuthor{
		Name: "legacy_inferred",
		ID:   "legacy",
	}
	manifest.License = ManifestLicense{
		SPDX:       "legacy_inferred",
		NoticePath: "",
	}
	manifest.Compatibility = ManifestCompatibility{
		MinRuntimeVersion: "0.0.0",
		RenderMode:        RenderModeSprite,
	}
	manifest.Binding = ManifestBinding{
		Policy:            BindingPolicyInferred,
		SourceCharacterID: v1.CharacterID,
	}
	manifest.Canvas = ManifestCanvas{
		Width:           v1.Canvas.Width,
		Height:          v1.Canvas.Height,
		CoordinateSystem: CoordinateSystemTopLeft,
	}
	manifest.DefaultAction = v1.DefaultAction
	manifest.Preview = v1.Preview

	actions := make([]ManifestActionEntry, 0, len(v1.Actions))
	for _, a := range v1.Actions {
		loopType := NormalizePlaybackMode(a.LoopType)
		if loopType == "" {
			loopType = LoopTypeLoop
		}
		entry := ManifestActionEntry{
			Key:            a.Key,
			Name:           a.Name,
			Config:         a.Config,
			RevisionID:     "legacy_inferred",
			QualityVerdict: QualityVerdictSkipped,
			PlaybackMode:   loopType,
		}
		actions = append(actions, entry)
	}
	manifest.Actions = actions

	manifest.Capabilities = ManifestCapabilities{
		TransparentBackground: v1.Capabilities.HasTransparentBackground,
		FrameSequence:         v1.Capabilities.SupportsFrameSequence,
		PerFrameDuration:      false,
		Audio:                 false,
	}

	manifest.Integrity = ManifestIntegrity{
		Algorithm: IntegrityAlgorithmV1Legacy,
	}

	manifest.Provenance = ManifestProvenance{
		SourceType:       "legacy_v1",
		GenerationTaskID: v1.GenerationTaskID,
		ProcessingTaskID: "",
		BuiltAt:          v1.CreatedAt,
		Builder:          "v1-compat-reader",
	}

	return manifest, nil
}
