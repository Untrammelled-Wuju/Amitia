package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/u-ai/backend/config"
	"github.com/u-ai/backend/internal/artifact"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/extension/kernel/event"
	"gorm.io/gorm"
)

type ArtifactRuntime struct {
	Service   *artifact.Service
	Repo      artifact.Repository
	BlobStore artifact.BlobStore
	Resolver  artifact.Resolver
	Handler   *artifact.Handler
}

func BuildArtifactRuntime(db *gorm.DB, blobRoot string, eventPublisher event.DurableEventPublisher) (*ArtifactRuntime, error) {
	if blobRoot == "" {
		if config.AppCfg != nil && config.AppCfg.Storage.DataDir != "" {
			blobRoot = filepath.Join(config.AppCfg.Storage.DataDir, "artifacts", "blobs", "sha256")
		} else {
			blobRoot = filepath.Join("data", "artifacts", "blobs", "sha256")
		}
	}
	if err := os.MkdirAll(blobRoot, 0700); err != nil {
		return nil, err
	}
	blobStore := artifact.NewFilesystemBlobStore(blobRoot)
	repo := artifact.NewRepository(db)
	svc := artifact.NewService(blobStore, repo, artifact.DefaultLimits)
	if eventPublisher != nil {
		svc.SetEventSink(artifact.NewRealEventSink(eventPublisher))
	}
	resolver := artifact.NewResolver(svc)
	handler := artifact.NewHandler(svc)
	return &ArtifactRuntime{
		Service:   svc,
		Repo:      repo,
		BlobStore: blobStore,
		Resolver:  resolver,
		Handler:   handler,
	}, nil
}

func (r *ArtifactRuntime) SetEventPublisher(eventPublisher event.DurableEventPublisher) {
	if r == nil || r.Service == nil || eventPublisher == nil {
		return
	}
	r.Service.SetEventSink(artifact.NewRealEventSink(eventPublisher))
}

type chatArtifactAdapter struct {
	resolver artifact.Resolver
}

func (a *chatArtifactAdapter) Resolve(ctx context.Context, actor string, resourceURI string) (chat.ArtifactResolution, error) {
	if a.resolver == nil {
		return chat.ArtifactResolution{}, nil
	}
	art, err := a.resolver.Resolve(ctx, actor, resourceURI)
	if err != nil {
		return chat.ArtifactResolution{}, err
	}
	return chat.ArtifactResolution{
		ID:           string(art.ID),
		OwnerSpaceID: art.OwnerSpaceID,
		Kind:         string(art.Kind),
		BlobDigest:   string(art.BlobDigest),
		SizeBytes:    art.SizeBytes,
		MIMEType:     art.MIMEType,
		Filename:     art.Filename,
		Status:       string(art.Status),
		Revision:     art.Revision,
	}, nil
}

func (a *chatArtifactAdapter) Open(ctx context.Context, actor string, resourceURI string) (io.ReadCloser, chat.ArtifactResolution, error) {
	if a.resolver == nil {
		return nil, chat.ArtifactResolution{}, nil
	}
	art, err := a.resolver.Resolve(ctx, actor, resourceURI)
	if err != nil {
		return nil, chat.ArtifactResolution{}, err
	}
	res := chat.ArtifactResolution{
		ID:           string(art.ID),
		OwnerSpaceID: art.OwnerSpaceID,
		Kind:         string(art.Kind),
		BlobDigest:   string(art.BlobDigest),
		SizeBytes:    art.SizeBytes,
		MIMEType:     art.MIMEType,
		Filename:     art.Filename,
		Status:       string(art.Status),
		Revision:     art.Revision,
	}
	if art.Status != "ready" {
		return nil, res, fmt.Errorf("artifact not ready")
	}
	rc, _, err := a.resolver.Open(ctx, actor, resourceURI)
	if err != nil {
		return nil, res, err
	}
	return rc, res, nil
}

func (a *chatArtifactAdapter) RegisterReference(artifactID string, refType string, refID string) error {
	if a.resolver == nil {
		return fmt.Errorf("chat artifact adapter: resolver not available")
	}
	return a.resolver.RegisterReference(artifact.ID(artifactID), refType, refID)
}

func (a *chatArtifactAdapter) RegisterReferenceGormTx(tx *gorm.DB, artifactID string, refType string, refID string) error {
	if a.resolver == nil {
		return fmt.Errorf("chat artifact adapter: resolver not available")
	}
	return a.resolver.RegisterReferenceGormTx(tx, artifact.ID(artifactID), refType, refID)
}

func (a *chatArtifactAdapter) UnregisterReferenceGormTx(tx *gorm.DB, artifactID string, refType string, refID string) error {
	if a.resolver == nil {
		return fmt.Errorf("chat artifact adapter: resolver not available")
	}
	return a.resolver.UnregisterReferenceGormTx(tx, artifact.ID(artifactID), refType, refID)
}
