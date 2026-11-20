package artifact

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/u-ai/backend/internal/timeoutpolicy"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type CreateRequest struct {
	OwnerSpaceID  string
	OwnerDeviceID string
	WorkspaceID   string
	Kind          Kind
	MIMEType      string
	Filename      string
	Source        Source
	Width         int
	Height        int
	DurationMS    int64
	Reader        io.Reader
	MaxBytes      int64
	Unbounded     bool
}

type ImportURLRequest struct {
	OwnerSpaceID string
	WorkspaceID  string
	URL          string
	Kind         Kind
	MIMEType     string
	Filename     string
	Source       Source
	MaxBytes     int64
}

type Service struct {
	blobStore       BlobStore
	repo            Repository
	limits          UploadLimits
	storageDeviceID string
	eventSink       *RealEventSink
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

func (s *Service) SetStorageDeviceID(deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" || len(deviceID) > 256 || strings.ContainsAny(deviceID, "\x00\r\n/\\") {
		return ErrInvalidUpload("invalid artifact storage device identity")
	}
	s.storageDeviceID = deviceID
	return nil
}

func (s *Service) Create(ctx context.Context, req CreateRequest) (Artifact, error) {
	if req.OwnerSpaceID == "" {
		return Artifact{}, ErrInvalidUpload("missing owner")
	}
	if req.Reader == nil {
		return Artifact{}, ErrInvalidUpload("missing reader")
	}
	if req.Source == SourceChatUpload && strings.TrimSpace(req.OwnerDeviceID) == "" {
		return Artifact{}, ErrInvalidUpload("chat upload requires device ownership")
	}
	if req.Source == SourceChatUpload && s.storageDeviceID == "" {
		return Artifact{}, ErrInvalidUpload("chat upload requires storage device identity")
	}
	kind := req.Kind
	if kind == "" {
		kind = DeriveKindFromMIME(req.MIMEType)
	}
	maxBytes := req.MaxBytes
	if req.Unbounded {
		maxBytes = -1
	} else if maxBytes <= 0 {
		maxBytes = s.limits.MaxBytesForKind(kind)
	}
	filename := SanitizeFilename(req.Filename)
	ext := ExtractExtension(filename)
	mimeType := normalizeMIME(req.MIMEType)
	sniffReader := req.Reader
	if mimeType == "" {
		var buf [512]byte
		n, err := io.ReadFull(sniffReader, buf[:])
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			return Artifact{}, fmt.Errorf("artifact: read content for type detection: %w", err)
		}
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
			return Artifact{}, &ArtifactError{Code: "too_large", Msg: err.Error(), Cause: err}
		}
		return Artifact{}, ErrBlobWriteFailed(err)
	}
	now := time.Now()
	art := &Artifact{
		ID:              ID("art_" + uuid.New().String()),
		OwnerSpaceID:    req.OwnerSpaceID,
		OwnerDeviceID:   req.OwnerDeviceID,
		StorageDeviceID: s.storageDeviceID,
		WorkspaceID:     req.WorkspaceID,
		Kind:            kind,
		BlobDigest:      blobInfo.Digest,
		SizeBytes:       blobInfo.SizeBytes,
		MIMEType:        mimeType,
		Filename:        filename,
		Extension:       ext,
		Status:          StatusReady,
		Source:          req.Source,
		Width:           req.Width,
		Height:          req.Height,
		DurationMS:      req.DurationMS,
		Revision:        1,
		CreatedAt:       now,
		UpdatedAt:       now,
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
	if req.Source == SourceChatUpload {
		ref := &ArtifactReference{ArtifactID: art.ID, ReferenceType: chatUploadOwnerReference, ReferenceID: req.OwnerDeviceID, CreatedAt: now}
		if err := s.repo.InsertReferenceSqlTx(tx, ref); err != nil {
			return Artifact{}, ErrMetadataWriteFailed(err)
		}
		ref = &ArtifactReference{ArtifactID: art.ID, ReferenceType: chatUploadStorageReference, ReferenceID: s.storageDeviceID, CreatedAt: now}
		if err := s.repo.InsertReferenceSqlTx(tx, ref); err != nil {
			return Artifact{}, ErrMetadataWriteFailed(err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Artifact{}, ErrMetadataWriteFailed(err)
	}
	if s.eventSink != nil {
		if err := s.eventSink.PublishCreated(ctx, art); err != nil {
			log.Printf("artifact: publish created event failed: %v", err)
		}
	}

	return *art, nil
}

func (s *Service) ImportURL(ctx context.Context, req ImportURLRequest) (Artifact, error) {
	rawURL := strings.TrimSpace(req.URL)
	if rawURL == "" {
		return Artifact{}, ErrInvalidUpload("missing url")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return Artifact{}, ErrInvalidUpload("invalid url")
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "https" && scheme != "http" {
		return Artifact{}, ErrInvalidUpload("unsupported url scheme")
	}
	maxBytes := req.MaxBytes
	if maxBytes <= 0 {
		maxBytes = s.limits.MaxBytesForKind(req.Kind)
	}
	if maxBytes <= 0 {
		maxBytes = 32 * 1024 * 1024
	}
	client := timeoutpolicy.Client(&http.Client{
		Timeout: 20 * time.Second,
		Transport: &http.Transport{
			Proxy:               nil,
			ForceAttemptHTTP2:   true,
			DialContext:         safeArtifactDialContext,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Artifact{}, ErrInvalidUpload(err.Error())
	}
	httpReq.Header.Set("User-Agent", "Amitia/1.0")
	resp, err := client.Do(httpReq)
	if err != nil {
		return Artifact{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Artifact{}, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return Artifact{}, err
	}
	if int64(len(data)) > maxBytes {
		return Artifact{}, ErrTooLarge(maxBytes)
	}
	mimeType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	if strings.TrimSpace(req.MIMEType) != "" {
		mimeType = strings.TrimSpace(req.MIMEType)
	}
	filename := strings.TrimSpace(req.Filename)
	if filename == "" {
		filename = path.Base(parsed.Path)
	}
	if filename == "." || filename == "/" || filename == "" {
		filename = "attachment"
	}
	source := req.Source
	if source == "" {
		source = SourceImport
	}
	return s.Create(ctx, CreateRequest{
		OwnerSpaceID: req.OwnerSpaceID,
		WorkspaceID:  req.WorkspaceID,
		Kind:         req.Kind,
		MIMEType:     mimeType,
		Filename:     filename,
		Source:       source,
		Reader:       bytes.NewReader(data),
		MaxBytes:     maxBytes,
	})
}

func safeArtifactDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		if !isPublicArtifactIP(ip.IP) {
			return nil, fmt.Errorf("blocked non-public artifact address: %s", ip.IP)
		}
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	return dialer.DialContext(ctx, network, address)
}

func isPublicArtifactIP(ip net.IP) bool {
	return ip != nil &&
		!ip.IsPrivate() &&
		!ip.IsLoopback() &&
		!ip.IsUnspecified() &&
		!ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() &&
		!ip.IsMulticast()
}

func (s *Service) GetByID(ctx context.Context, id ID) (Artifact, error) {
	art, err := s.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Artifact{}, &ArtifactError{Code: "not_found", Msg: fmt.Sprintf("%s: %v", id, err), Cause: err}
		}
		return Artifact{}, fmt.Errorf("artifact: get metadata %s: %w", id, err)
	}
	if art.Status == StatusDeleted {
		return Artifact{}, ErrDeleted(id)
	}
	return *art, nil
}

func (s *Service) GetOwned(ctx context.Context, ownerSpaceID string, id ID) (Artifact, error) {
	art, err := s.repo.GetByOwnerAndID(ownerSpaceID, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Artifact{}, &ArtifactError{Code: "not_found", Msg: fmt.Sprintf("%s: %v", id, err), Cause: err}
		}
		return Artifact{}, fmt.Errorf("artifact: get owned metadata: %w", err)
	}
	if art.Status == StatusDeleted {
		return Artifact{}, ErrDeleted(id)
	}
	if art.Source == SourceChatUpload {
		owner, storage, err := s.chatUploadLocations(ctx, id)
		if err != nil {
			return Artifact{}, err
		}
		art.OwnerDeviceID, art.StorageDeviceID = owner, storage
	}
	return *art, nil
}

const chatUploadOwnerReference = "chat_upload_owner_device"
const chatUploadStorageReference = "chat_upload_storage_device"

func (s *Service) GetOwnedForDevice(ctx context.Context, ownerSpaceID, ownerDeviceID string, id ID) (Artifact, error) {
	art, err := s.GetOwned(ctx, ownerSpaceID, id)
	if err != nil {
		return Artifact{}, err
	}
	if art.Source != SourceChatUpload {
		return art, nil
	}
	if strings.TrimSpace(ownerDeviceID) == "" {
		return Artifact{}, ErrNotOwned(id)
	}
	var owners []ArtifactReference
	err = s.repo.DB().WithContext(ctx).Where("artifact_id = ? AND reference_type = ?", id, chatUploadOwnerReference).Limit(2).Find(&owners).Error
	if err != nil {
		return Artifact{}, fmt.Errorf("artifact: verify chat upload device owner: %w", err)
	}
	if len(owners) != 1 || owners[0].ReferenceID != ownerDeviceID {
		return Artifact{}, ErrNotOwned(id)
	}
	return art, nil
}

func (s *Service) chatUploadLocations(ctx context.Context, id ID) (string, string, error) {
	var rows []ArtifactReference
	if err := s.repo.DB().WithContext(ctx).Where("artifact_id = ? AND reference_type IN ?", id, []string{chatUploadOwnerReference, chatUploadStorageReference}).Find(&rows).Error; err != nil {
		return "", "", fmt.Errorf("artifact: read chat upload ownership: %w", err)
	}
	owner, storage := "", ""
	for _, row := range rows {
		switch row.ReferenceType {
		case chatUploadOwnerReference:
			if owner != "" {
				return "", "", ErrInvalidReference("duplicate chat upload owner")
			}
			owner = row.ReferenceID
		case chatUploadStorageReference:
			if storage != "" {
				return "", "", ErrInvalidReference("duplicate chat upload storage")
			}
			storage = row.ReferenceID
		}
	}
	if owner == "" || storage == "" {
		return "", "", ErrInvalidReference("chat upload ownership is incomplete")
	}
	return owner, storage, nil
}

func (s *Service) GetChatUploadReference(ctx context.Context, ownerSpaceID, ownerDeviceID, storageDeviceID string, id ID) (Artifact, error) {
	art, err := s.GetOwned(ctx, ownerSpaceID, id)
	if err != nil {
		return Artifact{}, err
	}
	if art.Source != SourceChatUpload || art.OwnerDeviceID != ownerDeviceID || art.StorageDeviceID != storageDeviceID {
		return Artifact{}, ErrNotOwned(id)
	}
	return art, nil
}

func (s *Service) Delete(ctx context.Context, ownerSpaceID string, id ID) error {
	art, err := s.repo.GetByOwnerAndID(ownerSpaceID, id)
	if err != nil {
		return &ArtifactError{Code: "not_found", Msg: fmt.Sprintf("%s: %v", id, err), Cause: err}
	}
	if art.Status == StatusDeleted {
		return ErrDeleted(id)
	}
	count, err := s.repo.CountReferences(id)
	if err != nil {
		return fmt.Errorf("artifact: count references before deletion: %w", err)
	}
	if count > 0 {
		return ErrInUse(id)
	}

	sqlDB, err := s.repo.SqlDB()
	if err != nil {
		return ErrMetadataWriteFailed(err)
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return ErrMetadataWriteFailed(err)
	}
	defer tx.Rollback()

	if err := s.repo.SoftDeleteSqlTx(tx, id); err != nil {
		return ErrMetadataWriteFailed(err)
	}

	if err := tx.Commit(); err != nil {
		return ErrMetadataWriteFailed(err)
	}
	if s.eventSink != nil {
		if err := s.eventSink.PublishDeleted(ctx, art); err != nil {
			log.Printf("artifact: publish deleted event failed: %v", err)
		}
	}

	art.Status = StatusDeleted
	return nil
}

func (s *Service) OpenBlob(ctx context.Context, digest BlobDigest) (io.ReadSeekCloser, BlobInfo, error) {
	rc, info, err := s.blobStore.Open(ctx, digest)
	if err != nil {
		return nil, BlobInfo{}, &ArtifactError{Code: "blob_missing", Msg: fmt.Sprintf("%s: %v", digest, err), Cause: err}
	}
	return rc, info, nil
}

func (s *Service) RegisterReference(artifactID ID, refType string, refID string) error {
	if refType == chatUploadOwnerReference {
		return ErrInvalidReference("chat upload ownership references are reserved")
	}
	if refType == chatUploadStorageReference {
		return ErrInvalidReference("chat upload storage references are reserved")
	}
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
	if refType == chatUploadOwnerReference {
		return ErrInvalidReference("chat upload ownership references are reserved")
	}
	if refType == chatUploadStorageReference {
		return ErrInvalidReference("chat upload storage references are reserved")
	}
	return s.repo.InsertReferenceSqlTx(tx, &ArtifactReference{
		ArtifactID:    artifactID,
		ReferenceType: refType,
		ReferenceID:   refID,
		CreatedAt:     time.Now(),
	})
}

func (s *Service) RegisterReferenceGormTx(tx *gorm.DB, artifactID ID, refType string, refID string) error {
	if refType == chatUploadOwnerReference {
		return ErrInvalidReference("chat upload ownership references are reserved")
	}
	if refType == chatUploadStorageReference {
		return ErrInvalidReference("chat upload storage references are reserved")
	}
	return tx.Create(&ArtifactReference{
		ArtifactID:    artifactID,
		ReferenceType: refType,
		ReferenceID:   refID,
		CreatedAt:     time.Now(),
	}).Error
}

func (s *Service) UnregisterReferenceGormTx(tx *gorm.DB, artifactID ID, refType string, refID string) error {
	if refType == chatUploadOwnerReference {
		return ErrInvalidReference("chat upload ownership references are reserved")
	}
	if refType == chatUploadStorageReference {
		return ErrInvalidReference("chat upload storage references are reserved")
	}
	return tx.Where("artifact_id = ? AND reference_type = ? AND reference_id = ?", artifactID, refType, refID).Delete(&ArtifactReference{}).Error
}

func (s *Service) UnregisterReference(artifactID ID, refType string, refID string) error {
	if refType == chatUploadOwnerReference {
		return ErrInvalidReference("chat upload ownership references are reserved")
	}
	if refType == chatUploadStorageReference {
		return ErrInvalidReference("chat upload storage references are reserved")
	}
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
	if refType == chatUploadOwnerReference {
		return ErrInvalidReference("chat upload ownership references are reserved")
	}
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
