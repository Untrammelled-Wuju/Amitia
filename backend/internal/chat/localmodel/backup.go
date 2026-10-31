package localmodel

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
	amitialog "log"
)

const ModelAssetsComponent = "model.assets"

type ArtifactBackupContributor struct {
	DB   *gorm.DB
	Root string
}

func NewArtifactBackupContributor(db *gorm.DB, dataRoot string) *ArtifactBackupContributor {
	return &ArtifactBackupContributor{DB: db, Root: filepath.Join(dataRoot, "models")}
}

func (c *ArtifactBackupContributor) ID() string {
	return "model.assets"
}

func (c *ArtifactBackupContributor) Name() string {
	return "Local Model Assets"
}

func (c *ArtifactBackupContributor) Dependencies() []string {
	return []string{"model"}
}

func (c *ArtifactBackupContributor) ComponentIDs() []string {
	return []string{ModelAssetsComponent}
}

func (c *ArtifactBackupContributor) Plan(ctx context.Context, req dataportability.BackupRequest) ([]dataportability.BackupComponentPlan, error) {
	if !req.IncludeModelAssets || c.DB == nil {
		return nil, nil
	}
	rows := []ModelArtifact{}
	if err := c.DB.WithContext(ctx).Find(&rows).Error; err != nil {
		return nil, err
	}
	var total int64
	for _, row := range rows {
		total += row.SizeBytes
	}
	return []dataportability.BackupComponentPlan{{
		ID:            ModelAssetsComponent,
		Kind:          dataportability.KindResource,
		LogicalName:   "model.assets.v1.tar",
		Required:      false,
		SourceOfTruth: true,
		ItemCount:     int64(len(rows)),
		EstimatedSize: total + int64(len(rows)*2048),
	}}, nil
}

func (c *ArtifactBackupContributor) Export(ctx context.Context, req dataportability.BackupRequest, out dataportability.BackupWriter) error {
	if !req.IncludeModelAssets || c.DB == nil {
		return nil
	}
	rows := []ModelArtifact{}
	if err := c.DB.WithContext(ctx).Find(&rows).Error; err != nil {
		return err
	}
	writer, err := out.CreateComponent(ModelAssetsComponent, "model.assets.v1.tar", dataportability.KindResource)
	if err != nil {
		return err
	}
	archive := tar.NewWriter(writer)
	metadata, err := json.Marshal(rows)
	if err != nil {
		if err := archive.Close(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		if err := writer.Close(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		return err
	}
	if err := archive.WriteHeader(&tar.Header{Name: "model-artifacts.json", Mode: 0o600, Size: int64(len(metadata))}); err != nil {
		if err := archive.Close(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		if err := writer.Close(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		return err
	}
	if _, err := archive.Write(metadata); err != nil {
		if err := archive.Close(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		if err := writer.Close(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		return err
	}
	for _, row := range rows {
		path := c.artifactPath(row)
		info, err := os.Stat(path)
		if err != nil {
			if err := archive.Close(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			if err := writer.Close(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			return fmt.Errorf("open model artifact %s: %w", row.ID, err)
		}
		if err := archive.WriteHeader(&tar.Header{Name: "models/" + row.ID + "/" + safeModelFilename(row.Filename), Mode: 0o600, Size: info.Size()}); err != nil {
			if err := archive.Close(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			if err := writer.Close(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			if err := archive.Close(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			if err := writer.Close(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			return err
		}
		_, copyErr := io.Copy(archive, file)
		fileErr := file.Close()
		if copyErr != nil {
			if err := archive.Close(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			if err := writer.Close(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			return copyErr
		}
		if fileErr != nil {
			if err := archive.Close(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			if err := writer.Close(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			return fileErr
		}
	}
	if err := archive.Close(); err != nil {
		if err := writer.Close(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		return err
	}
	return writer.Close()
}

func (c *ArtifactBackupContributor) PreviewImport(ctx context.Context, req dataportability.ImportPreviewRequest, in dataportability.BackupReader) ([]dataportability.ImportComponentPreview, error) {
	reader, err := in.ReadComponent(ModelAssetsComponent)
	if err != nil {
		return nil, nil
	}
	defer reader.Close()
	preview := dataportability.ImportComponentPreview{ComponentID: ModelAssetsComponent, Kind: dataportability.KindResource, LogicalName: "model.assets.v1.tar"}
	archive := tar.NewReader(reader)
	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return []dataportability.ImportComponentPreview{preview}, err
		}
		if header.Name == "model-artifacts.json" {
			data, err := io.ReadAll(io.LimitReader(archive, 16<<20))
			if err != nil {
				return []dataportability.ImportComponentPreview{preview}, err
			}
			rows := []ModelArtifact{}
			if err := json.Unmarshal(data, &rows); err != nil {
				return []dataportability.ImportComponentPreview{preview}, err
			}
			preview.ItemCount = int64(len(rows))
			break
		}
	}
	return []dataportability.ImportComponentPreview{preview}, nil
}

func (c *ArtifactBackupContributor) Import(ctx context.Context, req dataportability.ImportRequest, in dataportability.BackupReader) error {
	if c.DB == nil {
		return nil
	}
	reader, err := in.ReadComponent(ModelAssetsComponent)
	if err != nil {
		return nil
	}
	defer reader.Close()
	if err := os.MkdirAll(c.Root, 0o700); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(c.Root, ".restore-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	archive := tar.NewReader(reader)
	rows := []ModelArtifact{}
	rowIndex := map[string]int{}
	stagedFiles := map[string]string{}
	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg {
			return fmt.Errorf("model artifact archive contains unsupported entry: %s", header.Name)
		}
		if header.Name == "model-artifacts.json" {
			data, err := io.ReadAll(io.LimitReader(archive, 16<<20))
			if err != nil {
				return err
			}
			if err := json.Unmarshal(data, &rows); err != nil {
				return err
			}
			for index := range rows {
				rows[index].ID = strings.TrimSpace(rows[index].ID)
				rows[index].Filename = safeModelFilename(rows[index].Filename)
				if !validModelArtifactID(rows[index].ID) || rows[index].Filename == "" {
					return errors.New("model artifact metadata contains invalid identity")
				}
				if rows[index].SizeBytes <= 0 || rows[index].SizeBytes > maxModelArtifactBytes {
					return fmt.Errorf("model artifact metadata contains invalid size: %s", rows[index].ID)
				}
				if len(rows[index].SHA256) != sha256.Size*2 {
					return fmt.Errorf("model artifact metadata contains invalid hash: %s", rows[index].ID)
				}
				if _, err := hex.DecodeString(rows[index].SHA256); err != nil {
					return fmt.Errorf("model artifact metadata contains invalid hash: %s", rows[index].ID)
				}
				if _, exists := rowIndex[rows[index].ID]; exists {
					return fmt.Errorf("model artifact metadata contains duplicate id: %s", rows[index].ID)
				}
				rowIndex[rows[index].ID] = index
			}
			continue
		}
		parts := strings.Split(header.Name, "/")
		if len(parts) != 3 || parts[0] != "models" {
			return fmt.Errorf("model artifact archive contains invalid path: %s", header.Name)
		}
		id := parts[1]
		filename := safeModelFilename(parts[2])
		index, exists := rowIndex[id]
		if !validModelArtifactID(id) || !exists {
			return fmt.Errorf("model artifact archive contains unknown id: %s", id)
		}
		if filename == "" || filename != rows[index].Filename {
			return fmt.Errorf("model artifact archive contains unexpected filename: %s", header.Name)
		}
		if _, exists := stagedFiles[id]; exists {
			return fmt.Errorf("model artifact archive contains duplicate file: %s", id)
		}
		target, err := os.CreateTemp(staging, id+"-*")
		if err != nil {
			return err
		}
		digest := sha256.New()
		written, err := io.Copy(io.MultiWriter(target, digest), io.LimitReader(archive, rows[index].SizeBytes+1))
		if err != nil {
			if err := target.Close(); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			return err
		}
		if err := target.Close(); err != nil {
			return err
		}
		if rows[index].SizeBytes != written || !strings.EqualFold(rows[index].SHA256, hex.EncodeToString(digest.Sum(nil))) {
			return fmt.Errorf("model artifact integrity mismatch: %s", id)
		}
		stagedFiles[id] = target.Name()
	}
	if len(rows) != len(stagedFiles) {
		return errors.New("model artifact archive is missing model files")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	type restoreMove struct {
		target string
		old    string
		dir    string
		dirNew bool
	}
	moved := make([]restoreMove, 0, len(rows))
	err = c.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for index := range rows {
			row := rows[index]
			dir := filepath.Join(c.Root, row.ID)
			_, statErr := os.Stat(dir)
			dirNew := os.IsNotExist(statErr)
			if err := os.MkdirAll(dir, 0o700); err != nil {
				return err
			}
			target := filepath.Join(dir, row.Filename)
			oldPath := ""
			if _, err := os.Stat(target); err == nil {
				old, err := os.CreateTemp(staging, "old-*")
				if err != nil {
					return err
				}
				oldPath = old.Name()
				if err := old.Close(); err != nil {
					return err
				}
				if err := os.Rename(target, oldPath); err != nil {
					return err
				}
			}
			if err := os.Rename(stagedFiles[row.ID], target); err != nil {
				if oldPath != "" {
					if err := os.Rename(oldPath, target); err != nil {
						amitialog.Printf("[original-error] %v", err)
					}
				}
				return err
			}
			moved = append(moved, restoreMove{target: target, old: oldPath, dir: dir, dirNew: dirNew})
			row.ResourceURI = ModelArtifactURI(row.ID, row.Filename)
			row.Status = "ready"
			row.UpdatedAt = now
			if err := tx.Save(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		for index := len(moved) - 1; index >= 0; index-- {
			move := moved[index]
			if err := os.Remove(move.target); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
			if move.old != "" {
				if err := os.Rename(move.old, move.target); err != nil {
					amitialog.Printf("[original-error] %v", err)
				}
			} else if move.dirNew {
				if err := os.RemoveAll(move.dir); err != nil {
					amitialog.Printf("[original-error] %v", err)
				}
			}
		}
		return err
	}
	return nil
}

func (c *ArtifactBackupContributor) artifactPath(row ModelArtifact) string {
	return filepath.Join(c.Root, row.ID, safeModelFilename(row.Filename))
}

func validModelArtifactID(id string) bool {
	if !strings.HasPrefix(id, "local_model_") || len(id) > 128 {
		return false
	}
	for _, char := range id {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}

var _ dataportability.BackupContributor = (*ArtifactBackupContributor)(nil)
