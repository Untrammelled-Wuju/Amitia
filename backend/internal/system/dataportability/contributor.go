package dataportability

import (
	"context"
	"io"
)

type BackupRequest struct {
	Scope              BackupScope
	Profile            BackupProfile
	CharacterID        string
	IncludeSecrets     bool
	IncludeModelAssets bool
	Purpose            BackupPurpose
	Passphrase         string
}

func (r BackupRequest) EffectiveScope() BackupScope {
	if r.Scope == "" {
		return ScopeAll
	}
	return r.Scope
}

func (r BackupRequest) EffectiveProfile() BackupProfile {
	if r.Profile == "" {
		return ProfileFull
	}
	return r.Profile
}

type BackupComponentPlan struct {
	ID            string
	Kind          ComponentKind
	LogicalName   string
	Required      bool
	SourceOfTruth bool
	Rebuildable   bool
	Sensitive     bool
	EstimatedSize int64
	ItemCount     int64
}

type ImportPreviewRequest struct {
	StagingPath string
	Manifest    *BackupManifest
}

type ImportComponentPreview struct {
	ComponentID string               `json:"componentId"`
	Kind        ComponentKind        `json:"kind"`
	LogicalName string               `json:"logicalName"`
	ItemCount   int64                `json:"itemCount"`
	Collisions  []ComponentCollision `json:"collisions"`
	Warnings    []string             `json:"warnings"`
}

type ComponentCollision struct {
	SourceID   string          `json:"sourceId"`
	TargetID   string          `json:"targetId"`
	EntityType string          `json:"entityType"`
	Policy     CollisionPolicy `json:"policy"`
}

type CollisionPolicy string

const (
	CollisionDuplicate CollisionPolicy = "duplicate"
	CollisionSkip      CollisionPolicy = "skip"
	CollisionReplace   CollisionPolicy = "replace"
)

type ImportRequest struct {
	OperationID        string
	StagingPath        string
	Manifest           *BackupManifest
	CollisionPolicy    CollisionPolicy
	CharacterPolicy    CollisionPolicy
	DefaultCharacterID string
	ActivateImported   bool
	SecretProvider     SecretProvider
	IdentityMap        *ImportIdentityMap
	Purpose            BackupPurpose
}

type SecretProvider interface {
	Reauthorize(ctx context.Context, ref SecretRef) (string, error)
	MarkReauthorizationRequired(ctx context.Context, ref SecretRef) error
}

type SecretRef struct {
	EntityType string
	EntityID   string
	SecretType string
}

type BackupWriter interface {
	CreateComponent(id, logicalName string, kind ComponentKind) (io.WriteCloser, error)
	WriteJSON(id string, v interface{}) error
}

type BackupReader interface {
	ReadComponent(id string) (io.ReadCloser, error)
	ReadJSON(id string, v interface{}) error
	ListComponents() []string
}

type BackupContributor interface {
	ID() string
	Name() string
	Dependencies() []string
	ComponentIDs() []string
	Plan(ctx context.Context, req BackupRequest) ([]BackupComponentPlan, error)
	Export(ctx context.Context, req BackupRequest, out BackupWriter) error
	PreviewImport(ctx context.Context, req ImportPreviewRequest, in BackupReader) ([]ImportComponentPreview, error)
	Import(ctx context.Context, req ImportRequest, in BackupReader) error
}
