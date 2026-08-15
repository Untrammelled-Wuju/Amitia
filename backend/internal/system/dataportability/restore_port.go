package dataportability

import (
	"context"
	"fmt"
	"io"
)

type RestorePurpose string

const (
	RestorePurposeOrdinary RestorePurpose = "ordinary"
	RestorePurposeFull     RestorePurpose = "full_restore"
)

type RestoreOptions struct {
	OperationID      string
	Purpose          RestorePurpose
	CharacterPolicy  CollisionPolicy
	DefaultCharacterID string
	ActivateImported bool
	IdentityMap      *ImportIdentityMap
	SecretProvider   SecretProvider
}

type CharacterRestorePort interface {
	RestoreCharacters(ctx context.Context, in BackupReader, opts RestoreOptions) error
}

type ChatRestorePort interface {
	RestoreChats(ctx context.Context, in BackupReader, opts RestoreOptions) error
}

type EpisodicRestorePort interface {
	RestoreEpisodic(ctx context.Context, in BackupReader, opts RestoreOptions) error
}

type PsycheRestorePort interface {
	RestorePsyche(ctx context.Context, in BackupReader, opts RestoreOptions) error
}

type RelationshipRestorePort interface {
	RestoreRelationships(ctx context.Context, in BackupReader, opts RestoreOptions) error
}

type WorldbookRestorePort interface {
	RestoreWorldbook(ctx context.Context, in BackupReader, opts RestoreOptions) error
}

type ModelConfigRestorePort interface {
	RestoreModelConfigs(ctx context.Context, in BackupReader, opts RestoreOptions) error
}

type VoiceRestorePort interface {
	RestoreVoices(ctx context.Context, in BackupReader, opts RestoreOptions) error
}

type EmbeddingRestorePort interface {
	RestoreEmbeddings(ctx context.Context, in BackupReader, opts RestoreOptions) error
}

type ResourceRestorePort interface {
	RestoreResources(ctx context.Context, in BackupReader, opts RestoreOptions) error
}

type ExtensionRestorePort interface {
	RestoreExtensions(ctx context.Context, in BackupReader, opts RestoreOptions) error
}

type WorkspaceRestorePort interface {
	RestoreWorkspaces(ctx context.Context, in BackupReader, opts RestoreOptions) error
}

type MemoryRestorePort interface {
	RestoreMemories(ctx context.Context, in BackupReader, opts RestoreOptions) error
}

type RestorePorts struct {
	Character    CharacterRestorePort
	Chat         ChatRestorePort
	Episodic     EpisodicRestorePort
	Psyche       PsycheRestorePort
	Relationship RelationshipRestorePort
	Worldbook    WorldbookRestorePort
	ModelConfig  ModelConfigRestorePort
	Voice        VoiceRestorePort
	Embedding    EmbeddingRestorePort
	Resource     ResourceRestorePort
	Extension    ExtensionRestorePort
	Workspace    WorkspaceRestorePort
	Memory       MemoryRestorePort
}

func (p RestorePorts) HasAny() bool {
	return p.Character != nil || p.Chat != nil || p.Episodic != nil ||
		p.Psyche != nil || p.Relationship != nil || p.Worldbook != nil ||
		p.ModelConfig != nil || p.Voice != nil || p.Embedding != nil ||
		p.Resource != nil || p.Extension != nil || p.Workspace != nil ||
		p.Memory != nil
}

type noOpReader struct{}

func (noOpReader) ReadComponent(id string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("component not available: %s", id)
}

func (noOpReader) ReadJSON(id string, v interface{}) error {
	return fmt.Errorf("component not available: %s", id)
}

func (noOpReader) ListComponents() []string {
	return nil
}

var _ BackupReader = (*noOpReader)(nil)
