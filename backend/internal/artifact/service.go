package artifact

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
)

type CreateRequest struct {
	OwnerUserID string
	WorkspaceID string
	Kind        Kind
	MIMEType    string
	Filename    string
	Source      Source
	Width       int
	Height      int
	DurationMS  int64
	Reader      io.Reader
	MaxBytes    int64
}

type Service struct {
	blobStore  BlobStore
	repo       Repository
	limits     UploadLimits
	eventSink  EventSink
}

func NewService(blobStore BlobStore, repo Repository, limits UploadLimits) *Service {
	return &Service{
		blobStore: blobStore,
		repo:      repo,
		limits:    limits,
		eventSink: noopEventSink{},
	}
}

func (s *Service) SetEventSink(sink EventSink) {
	if sink != nil {
		s.eventSink = sink
	}
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (Artifact, error) {
	if req.OwnerUserID == "" {
		return Artifact{}, ErrInvalidUpload("missing owner")
	}
	if req.Reader == nil {
		return Artifact{}, ErrInvalidUpload("missing reader")
	}
	kind := req.Kind
	if kind == "" {
		kind = DeriveKindFromMIME(req.MIMEType)
	}
	maxBytes := req.MaxBytes
	if maxBytes <= 0 {
		maxBytes = s.limits.MaxBytesForKind(kind)
	}
	filename := SanitizeFilename(req.Filename)
	ext := ExtractExtension(filename)
	mimeType := normalizeMIME(req.MIMEType)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	if kind == "" {
		kind = DeriveKindFromMIME(mimeType)
	}
	blobInfo, err := s.blobStore.Put(ctx, req.Reader, maxBytes)
	if err != nil {
		if err.Error() != "" {
			if maxBytes > 0 && err.Error()[0:1] == "e" {
				return Artifact{}, ErrTooLarge(maxBytes)
			}
		}
		return Artifact{}, ErrBlobWriteFailed(err)
	}
	now := time.Now()
	art := &Artifact{
		ID:          ID("art_" + uuid.New().String()),
		OwnerUserID: req.OwnerUserID,
		WorkspaceID: req.WorkspaceID,
		Kind:        kind,
		BlobDigest:  blobInfo.Digest,
		SizeBytes:   blobInfo.SizeBytes,
		MIMEType:    mimeType,
		Filename:    filename,
		Extension:   ext,
		Status:      StatusReady,
		Source:      req.Source,
		Width:       req.Width,
		Height:      req.Height,
		DurationMS:  req.DurationMS,
		Revision:    1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.Create(art); err != nil {
		return Artifact{}, ErrMetadataWriteFailed(err)
	}
	s.eventSink.ArtifactCreated(art)
	return *art, nil
}

func (s *Service) GetByID(ctx context.Context, id ID) (Artifact, error) {
	art, err := s.repo.GetByID(id)
	if err != nil {
		return Artifact{}, ErrNotFound(id)
	}
	if art.Status == StatusDeleted {
		return Artifact{}, ErrDeleted(id)
	}
	return *art, nil
}

func (s *Service) GetOwned(ctx context.Context, ownerUserID string, id ID) (Artifact, error) {
	art, err := s.repo.GetByOwnerAndID(ownerUserID, id)
	if err != nil {
		return Artifact{}, ErrNotFound(id)
	}
	if art.Status == StatusDeleted {
		return Artifact{}, ErrDeleted(id)
	}
	return *art, nil
}

func (s *Service) Delete(ctx context.Context, ownerUserID string, id ID) error {
	art, err := s.repo.GetByOwnerAndID(ownerUserID, id)
	if err != nil {
		return ErrNotFound(id)
	}
	if art.Status == StatusDeleted {
		return ErrDeleted(id)
	}
	count, err := s.repo.CountReferences(id)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrInUse(id)
	}
	if err := s.repo.SoftDelete(id); err != nil {
		return err
	}
	art.Status = StatusDeleted
	s.eventSink.ArtifactDeleted(art)
	return nil
}

func (s *Service) OpenBlob(ctx context.Context, digest BlobDigest) (io.ReadCloser, BlobInfo, error) {
	rc, info, err := s.blobStore.Open(ctx, digest)
	if err != nil {
		return nil, BlobInfo{}, ErrBlobMissing(digest)
	}
	return rc, info, nil
}

func (s *Service) RegisterReference(artifactID ID, refType string, refID string) error {
	return s.repo.InsertReference(&ArtifactReference{
		ArtifactID:    artifactID,
		ReferenceType: refType,
		ReferenceID:   refID,
		CreatedAt:     time.Now(),
	})
}

func normalizeMIME(mime string) string {
	m := ""
	for _, c := range mime {
		if c == ';' {
			break
		}
		m += string(c)
	}
	return m
}

