package artifact

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
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
	eventSink  *RealEventSink
}

func NewService(blobStore BlobStore, repo Repository, limits UploadLimits) *Service {
	return &Service{
		blobStore: blobStore,
		repo:      repo,
		limits:    limits,
		eventSink: nil,
	}
}

func (s *Service) SetEventSink(sink *RealEventSink) {
	s.eventSink = sink
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
	sniffReader := req.Reader
	if mimeType == "" {
		var buf [512]byte
		n, _ := io.ReadFull(sniffReader, buf[:])
		if n > 0 {
			mimeType = http.DetectContentType(buf[:n])
			sniffReader = io.MultiReader(bytes.NewReader(buf[:n]), sniffReader)
		}
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	if kind == "" {
		kind = DeriveKindFromMIME(mimeType)
	}
	blobInfo, err := s.blobStore.Put(ctx, sniffReader, maxBytes)
	if err != nil {
		if isBlobTooLarge(err) {
			return Artifact{}, ErrTooLarge(maxBytes)
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

	sqlDB, err := s.repo.SqlDB()
	if err != nil {
		return Artifact{}, ErrMetadataWriteFailed(err)
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return Artifact{}, ErrMetadataWriteFailed(err)
	}
	defer tx.Rollback()

	if err := s.repo.CreateSqlTx(tx, art); err != nil {
		return Artifact{}, ErrMetadataWriteFailed(err)
	}

	if s.eventSink != nil {
		if err := s.eventSink.PublishCreated(ctx, tx, art); err != nil {
			return Artifact{}, ErrMetadataWriteFailed(err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Artifact{}, ErrMetadataWriteFailed(err)
	}

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

	sqlDB, err := s.repo.SqlDB()
	if err != nil {
		return err
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := s.repo.SoftDeleteSqlTx(tx, id); err != nil {
		return err
	}

	if s.eventSink != nil {
		if err := s.eventSink.PublishDeleted(ctx, tx, art); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	art.Status = StatusDeleted
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
	sqlDB, err := s.repo.SqlDB()
	if err != nil {
		return ErrMetadataWriteFailed(err)
	}
	tx, err := sqlDB.BeginTx(context.Background(), nil)
	if err != nil {
		return ErrMetadataWriteFailed(err)
	}
	defer tx.Rollback()
	if err := s.repo.InsertReferenceSqlTx(tx, &ArtifactReference{
		ArtifactID:    artifactID,
		ReferenceType: refType,
		ReferenceID:   refID,
		CreatedAt:     time.Now(),
	}); err != nil {
		return ErrMetadataWriteFailed(err)
	}
	return tx.Commit()
}

func (s *Service) RegisterReferenceSqlTx(tx *sql.Tx, artifactID ID, refType string, refID string) error {
	return s.repo.InsertReferenceSqlTx(tx, &ArtifactReference{
		ArtifactID:    artifactID,
		ReferenceType: refType,
		ReferenceID:   refID,
		CreatedAt:     time.Now(),
	})
}

func (s *Service) UnregisterReference(artifactID ID, refType string, refID string) error {
	sqlDB, err := s.repo.SqlDB()
	if err != nil {
		return ErrMetadataWriteFailed(err)
	}
	tx, err := sqlDB.BeginTx(context.Background(), nil)
	if err != nil {
		return ErrMetadataWriteFailed(err)
	}
	defer tx.Rollback()
	if err := s.repo.RemoveReferenceSqlTx(tx, artifactID, refType, refID); err != nil {
		return ErrMetadataWriteFailed(err)
	}
	return tx.Commit()
}

func (s *Service) UnregisterReferenceSqlTx(tx *sql.Tx, artifactID ID, refType string, refID string) error {
	return s.repo.RemoveReferenceSqlTx(tx, artifactID, refType, refID)
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

func isBlobTooLarge(err error) bool {
	return errors.Is(err, ErrBlobTooLarge)
}
