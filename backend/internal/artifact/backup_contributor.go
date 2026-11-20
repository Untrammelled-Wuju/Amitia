package artifact

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/system/dataportability"
	"gorm.io/gorm"
)

const chatArtifactsComponent = "artifact.chat"

type BackupContributor struct {
	DB   *gorm.DB
	Root string
}

func NewBackupContributor(db *gorm.DB, dataDir string) *BackupContributor {
	return &BackupContributor{DB: db, Root: filepath.Join(dataDir, "artifacts", "blobs", "sha256")}
}

func (*BackupContributor) ID() string             { return "artifact" }
func (*BackupContributor) Name() string           { return "Chat Attachments" }
func (*BackupContributor) Dependencies() []string { return []string{"chat"} }
func (*BackupContributor) ComponentIDs() []string { return []string{chatArtifactsComponent} }

type backupArtifactDocument struct {
	Artifacts  []Artifact          `json:"artifacts"`
	References []ArtifactReference `json:"references"`
}

func (c *BackupContributor) Plan(ctx context.Context, req dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	if c == nil || c.DB == nil || req.EffectiveScope() == dataportability.ScopeSettings {
		return nil, nil
	}
	var rows []Artifact
	if err := c.DB.WithContext(ctx).Where("source = ? AND status <> ?", SourceChatUpload, StatusDeleted).Find(&rows).Error; err != nil {
		return nil, err
	}
	var size int64
	for _, row := range rows {
		size += row.SizeBytes
	}
	return []dataportability.BackupComponentPlan{{
		ID:            chatArtifactsComponent,
		Kind:          dataportability.KindResource,
		LogicalName:   "artifact.chat.v1.tar",
		Required:      true,
		SourceOfTruth: true,
		ItemCount:     int64(len(rows)),
		EstimatedSize: size + int64(len(rows)*4096),
	}}, nil
}

func (c *BackupContributor) Export(ctx context.Context, req dataportability.BackupRequest, out dataportability.BackupWriter) (returnErr error) {
	if c == nil || c.DB == nil || req.EffectiveScope() == dataportability.ScopeSettings {
		return nil
	}
	var document backupArtifactDocument
	if err := c.DB.WithContext(ctx).Where("source = ? AND status <> ?", SourceChatUpload, StatusDeleted).Find(&document.Artifacts).Error; err != nil {
		return err
	}
	ids := make([]ID, 0, len(document.Artifacts))
	for _, row := range document.Artifacts {
		ids = append(ids, row.ID)
	}
	if len(ids) > 0 {
		if err := c.DB.WithContext(ctx).Where("artifact_id IN ?", ids).Order("artifact_id, reference_type, reference_id").Find(&document.References).Error; err != nil {
			return err
		}
	}
	writer, err := out.CreateComponent(chatArtifactsComponent, "artifact.chat.v1.tar", dataportability.KindResource)
	if err != nil {
		return err
	}
	archive := tar.NewWriter(writer)
	defer func() {
		returnErr = errors.Join(returnErr, archive.Close(), writer.Close())
	}()
	metadata, err := json.Marshal(document)
	if err != nil {
		return err
	}
	if err := archive.WriteHeader(&tar.Header{Name: "metadata.json", Mode: 0o600, Size: int64(len(metadata)), ModTime: time.Now()}); err != nil {
		return err
	}
	if _, err := archive.Write(metadata); err != nil {
		return err
	}
	seen := map[BlobDigest]bool{}
	for _, row := range document.Artifacts {
		if seen[row.BlobDigest] {
			continue
		}
		seen[row.BlobDigest] = true
		file, err := os.Open(c.blobPath(row.BlobDigest))
		if err != nil {
			return fmt.Errorf("open attachment blob %s: %w", row.BlobDigest, err)
		}
		info, err := file.Stat()
		if err != nil {
			return errors.Join(fmt.Errorf("stat attachment blob %s: %w", row.BlobDigest, err), file.Close())
		}
		if err := archive.WriteHeader(&tar.Header{Name: "blobs/" + strings.TrimPrefix(string(row.BlobDigest), "sha256:"), Mode: 0o600, Size: info.Size(), ModTime: info.ModTime()}); err != nil {
			return errors.Join(err, file.Close())
		}
		if _, err := io.Copy(archive, file); err != nil {
			return errors.Join(fmt.Errorf("copy attachment blob %s: %w", row.BlobDigest, err), file.Close())
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (c *BackupContributor) PreviewImport(ctx context.Context, req dataportability.ImportPreviewRequest, in dataportability.BackupReader) ([]dataportability.ImportComponentPreview, error) {
	reader, err := in.ReadComponent(chatArtifactsComponent)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	preview := dataportability.ImportComponentPreview{ComponentID: chatArtifactsComponent, Kind: dataportability.KindResource, LogicalName: "artifact.chat.v1.tar"}
	archive := tar.NewReader(reader)
	for {
		header, err := archive.Next()
		if err == io.EOF {
			return []dataportability.ImportComponentPreview{preview}, nil
		}
		if err != nil {
			return []dataportability.ImportComponentPreview{preview}, err
		}
		if header.Name != "metadata.json" {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(archive, 16<<20))
		if err != nil {
			return []dataportability.ImportComponentPreview{preview}, err
		}
		var document backupArtifactDocument
		if err := json.Unmarshal(data, &document); err != nil {
			return []dataportability.ImportComponentPreview{preview}, err
		}
		preview.ItemCount = int64(len(document.Artifacts))
		return []dataportability.ImportComponentPreview{preview}, nil
	}
}

func (c *BackupContributor) Import(ctx context.Context, req dataportability.ImportRequest, in dataportability.BackupReader) (returnErr error) {
	if c == nil || c.DB == nil {
		return nil
	}
	reader, err := in.ReadComponent(chatArtifactsComponent)
	if err != nil {
		return err
	}
	defer reader.Close()
	if err := os.MkdirAll(c.Root, 0o700); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(c.Root, ".restore-*")
	if err != nil {
		return err
	}
	defer func() {
		returnErr = errors.Join(returnErr, os.RemoveAll(staging))
	}()
	archive := tar.NewReader(reader)
	var document backupArtifactDocument
	metadataSeen := false
	staged := map[BlobDigest]string{}
	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg {
			return fmt.Errorf("attachment archive contains unsupported entry: %s", header.Name)
		}
		if header.Name == "metadata.json" {
			if metadataSeen {
				return errors.New("attachment archive contains duplicate metadata")
			}
			data, err := io.ReadAll(io.LimitReader(archive, 16<<20))
			if err != nil {
				return err
			}
			if err := json.Unmarshal(data, &document); err != nil {
				return err
			}
			metadataSeen = true
			continue
		}
		digestHex := strings.TrimPrefix(header.Name, "blobs/")
		if digestHex == header.Name || len(digestHex) != 64 {
			return fmt.Errorf("attachment archive contains invalid blob path: %s", header.Name)
		}
		if _, err := hex.DecodeString(digestHex); err != nil {
			return fmt.Errorf("attachment archive contains invalid digest: %s", header.Name)
		}
		digest := BlobDigest("sha256:" + strings.ToLower(digestHex))
		if _, exists := staged[digest]; exists {
			return fmt.Errorf("attachment archive contains duplicate blob: %s", digest)
		}
		target, err := os.CreateTemp(staging, digestHex+"-*")
		if err != nil {
			return err
		}
		hash := sha256.New()
		written, copyErr := io.Copy(io.MultiWriter(target, hash), archive)
		closeErr := target.Close()
		if err := errors.Join(copyErr, closeErr); err != nil {
			return err
		}
		if written != header.Size || hex.EncodeToString(hash.Sum(nil)) != strings.ToLower(digestHex) {
			return fmt.Errorf("attachment blob integrity mismatch: %s", digest)
		}
		staged[digest] = target.Name()
	}
	if !metadataSeen {
		return errors.New("attachment archive is missing metadata")
	}
	if err := validateBackupArtifacts(document, staged); err != nil {
		return err
	}
	for digest, source := range staged {
		target := c.blobPath(digest)
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		if _, err := os.Stat(target); err == nil {
			if err := os.Remove(source); err != nil {
				return err
			}
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.Rename(source, target); err != nil {
			return err
		}
	}
	return c.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, row := range document.Artifacts {
			if err := tx.Save(&row).Error; err != nil {
				return err
			}
		}
		for _, reference := range document.References {
			if err := tx.Where("artifact_id = ? AND reference_type = ? AND reference_id = ?", reference.ArtifactID, reference.ReferenceType, reference.ReferenceID).FirstOrCreate(&reference).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func validateBackupArtifacts(document backupArtifactDocument, staged map[BlobDigest]string) error {
	requires := map[ID]bool{}
	for index := range document.Artifacts {
		row := &document.Artifacts[index]
		if row.ID == "" || row.OwnerSpaceID == "" || row.Source != SourceChatUpload || row.Status == StatusDeleted || row.SizeBytes <= 0 || !strings.HasPrefix(string(row.BlobDigest), "sha256:") || len(row.BlobDigest) != 71 {
			return fmt.Errorf("attachment metadata is invalid: %s", row.ID)
		}
		if _, err := hex.DecodeString(strings.TrimPrefix(string(row.BlobDigest), "sha256:")); err != nil {
			return fmt.Errorf("attachment metadata digest is invalid: %s", row.ID)
		}
		if staged[row.BlobDigest] == "" {
			return fmt.Errorf("attachment archive is missing blob: %s", row.BlobDigest)
		}
		requires[row.ID] = true
	}
	owners := map[ID]int{}
	storages := map[ID]int{}
	for _, reference := range document.References {
		if !requires[reference.ArtifactID] {
			return fmt.Errorf("attachment archive contains reference to unknown artifact: %s", reference.ArtifactID)
		}
		switch reference.ReferenceType {
		case chatUploadOwnerReference:
			owners[reference.ArtifactID]++
		case chatUploadStorageReference:
			storages[reference.ArtifactID]++
		}
	}
	for id := range requires {
		if owners[id] != 1 || storages[id] != 1 {
			return fmt.Errorf("attachment archive ownership is incomplete: %s", id)
		}
	}
	return nil
}

func (c *BackupContributor) blobPath(digest BlobDigest) string {
	value := strings.TrimPrefix(string(digest), "sha256:")
	if len(value) < 4 {
		return filepath.Join(c.Root, value)
	}
	return filepath.Join(c.Root, value[0:2], value[2:4], value)
}

var _ dataportability.BackupContributor = (*BackupContributor)(nil)
