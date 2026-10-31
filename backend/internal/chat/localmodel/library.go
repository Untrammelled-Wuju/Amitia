package localmodel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	amitialog "log"
)

const maxModelArtifactBytes int64 = 32 << 30

type ModelArtifact struct {
	ID           string `gorm:"column:id;primaryKey" json:"id"`
	OwnerSpaceID string `gorm:"column:owner_space_id;not null" json:"ownerSpaceId"`
	Name         string `gorm:"column:name;not null" json:"name"`
	Filename     string `gorm:"column:filename;not null" json:"filename"`
	Format       string `gorm:"column:format;not null" json:"format"`
	SizeBytes    int64  `gorm:"column:size_bytes;not null" json:"sizeBytes"`
	SHA256       string `gorm:"column:sha256;not null" json:"sha256"`
	ResourceURI  string `gorm:"column:resource_uri;not null" json:"resourceUri"`
	Status       string `gorm:"column:status;not null" json:"status"`
	CreatedAt    string `gorm:"column:created_at;not null" json:"createdAt"`
	UpdatedAt    string `gorm:"column:updated_at;not null" json:"updatedAt"`
}

func (ModelArtifact) TableName() string {
	return "model_artifacts"
}

type Library struct {
	db   *gorm.DB
	root string
}

func NewLibrary(db *gorm.DB, dataRoot string) *Library {
	root := strings.TrimSpace(dataRoot)
	if root == "" {
		return nil
	}
	return &Library{db: db, root: filepath.Join(root, "models")}
}

func (l *Library) List(ctx context.Context, owner string) ([]ModelArtifact, error) {
	if l == nil || l.db == nil {
		return nil, errors.New("本地模型库不可用")
	}
	rows := []ModelArtifact{}
	err := l.db.WithContext(ctx).Where("owner_space_id = ? AND status = ?", strings.TrimSpace(owner), "ready").Order("created_at DESC, id DESC").Find(&rows).Error
	return rows, err
}

func (l *Library) Get(ctx context.Context, owner, id string) (*ModelArtifact, error) {
	if l == nil || l.db == nil {
		return nil, errors.New("本地模型库不可用")
	}
	var row ModelArtifact
	if err := l.db.WithContext(ctx).Where("id = ? AND owner_space_id = ?", strings.TrimSpace(id), strings.TrimSpace(owner)).Take(&row).Error; err != nil {
		return nil, err
	}
	if row.Status != "ready" {
		return nil, errors.New("本地模型文件尚未就绪")
	}
	return &row, nil
}

func (l *Library) Import(ctx context.Context, owner, name, filename string, body io.Reader) (*ModelArtifact, error) {
	if l == nil || l.db == nil || body == nil {
		return nil, errors.New("本地模型导入参数无效")
	}
	owner = strings.TrimSpace(owner)
	filename = safeModelFilename(filename)
	format := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	if owner == "" || filename == "" || (format != "gguf" && format != "mnn") {
		return nil, errors.New("仅支持 GGUF 或 MNN 模型文件")
	}
	if strings.TrimSpace(name) == "" {
		name = strings.TrimSuffix(filename, filepath.Ext(filename))
	}
	if err := os.MkdirAll(l.root, 0o700); err != nil {
		return nil, err
	}
	temp, err := os.CreateTemp(l.root, ".model-import-*")
	if err != nil {
		return nil, err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(temp, hash), io.LimitReader(body, maxModelArtifactBytes+1))
	closeErr := temp.Close()
	if copyErr != nil {
		return nil, copyErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if written > maxModelArtifactBytes {
		return nil, errors.New("模型文件超过 32 GiB 限制")
	}
	if written < 1024 {
		return nil, errors.New("模型文件过小或已损坏")
	}
	if err := validateModelHeader(tempPath, format); err != nil {
		return nil, err
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	var existing ModelArtifact
	err = l.db.WithContext(ctx).Where("owner_space_id = ? AND sha256 = ?", owner, digest).Take(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	id := "local_model_" + uuid.NewString()
	dir := filepath.Join(l.root, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	target := filepath.Join(dir, filename)
	if err := os.Rename(tempPath, target); err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	row := &ModelArtifact{
		ID:           id,
		OwnerSpaceID: owner,
		Name:         strings.TrimSpace(name),
		Filename:     filename,
		Format:       format,
		SizeBytes:    written,
		SHA256:       digest,
		ResourceURI:  "amitia://data/models/" + id + "/" + url.PathEscape(filename),
		Status:       "ready",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := l.db.WithContext(ctx).Create(row).Error; err != nil {
		if err := os.Remove(target); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		if err := os.Remove(dir); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		return nil, err
	}
	return row, nil
}

func (l *Library) Delete(ctx context.Context, owner, id string) error {
	row, err := l.Get(ctx, owner, id)
	if err != nil {
		return err
	}
	var references int64
	if err := l.db.WithContext(ctx).Table("model_artifact_references").Where("artifact_id = ?", row.ID).Count(&references).Error; err != nil {
		return err
	}
	if references > 0 {
		return errors.New("模型仍被模型配置引用，不能删除")
	}
	if err := l.db.WithContext(ctx).Delete(&ModelArtifact{}, "id = ? AND owner_space_id = ?", row.ID, strings.TrimSpace(owner)).Error; err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(l.root, row.ID))
}

func (l *Library) FindByURI(ctx context.Context, owner, resourceURI string) (*ModelArtifact, error) {
	if l == nil || l.db == nil {
		return nil, errors.New("本地模型库不可用")
	}
	var row ModelArtifact
	query := l.db.WithContext(ctx).Where("resource_uri = ? AND status = ?", strings.TrimSpace(resourceURI), "ready")
	if strings.TrimSpace(owner) != "" {
		query = query.Where("owner_space_id = ?", strings.TrimSpace(owner))
	}
	if err := query.Take(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (l *Library) Detect(ctx context.Context, owner, provider string) ([]ModelArtifact, error) {
	format := ""
	switch strings.TrimSpace(provider) {
	case "llama_cpp":
		format = "gguf"
	case "mnn":
		format = "mnn"
	}
	rows, err := l.List(ctx, owner)
	if err != nil {
		return nil, err
	}
	if format == "" {
		return rows, nil
	}
	filtered := make([]ModelArtifact, 0, len(rows))
	for _, row := range rows {
		if row.Format == format {
			filtered = append(filtered, row)
		}
	}
	return filtered, nil
}

func ReferencedArtifactIDs(raw string) []string {
	var value any
	if json.Unmarshal([]byte(strings.TrimSpace(raw)), &value) != nil {
		return nil
	}
	seen := map[string]struct{}{}
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case map[string]any:
			for key, item := range typed {
				if key == "resourceUri" || key == "modelResourceUri" || key == "mmprojUri" {
					if uri, ok := item.(string); ok {
						if id := ArtifactIDFromURI(uri); id != "" {
							seen[id] = struct{}{}
						}
					}
				}
				walk(item)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(value)
	result := make([]string, 0, len(seen))
	for id := range seen {
		result = append(result, id)
	}
	return result
}

func ArtifactIDFromURI(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "amitia" || parsed.Host != "data" {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 3 || parts[0] != "models" || strings.TrimSpace(parts[1]) == "" {
		return ""
	}
	return parts[1]
}

func safeModelFilename(value string) string {
	base := filepath.Base(strings.TrimSpace(value))
	base = strings.ReplaceAll(base, "\x00", "")
	base = strings.ReplaceAll(base, "\n", "")
	base = strings.ReplaceAll(base, "\r", "")
	if len(base) > 255 {
		base = base[len(base)-255:]
	}
	return base
}

func validateModelHeader(path, format string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	header := make([]byte, 16)
	n, err := io.ReadFull(file, header)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return err
	}
	if n < 4 {
		return errors.New("模型文件头无效")
	}
	if format == "gguf" && string(header[:4]) != "GGUF" {
		return errors.New("GGUF 文件头无效")
	}
	return nil
}

func ModelArtifactURI(id, filename string) string {
	return "amitia://data/models/" + strings.TrimSpace(id) + "/" + url.PathEscape(filepath.Base(filename))
}

func FormatForProvider(provider string) string {
	switch strings.TrimSpace(provider) {
	case "llama_cpp":
		return "gguf"
	case "mnn":
		return "mnn"
	default:
		return ""
	}
}

func ValidateArtifactForProvider(row *ModelArtifact, provider string) error {
	if row == nil {
		return errors.New("模型文件不存在")
	}
	expected := FormatForProvider(provider)
	if expected == "" {
		return nil
	}
	if row.Format != expected {
		return fmt.Errorf("%s 需要 %s 模型文件", provider, strings.ToUpper(expected))
	}
	return nil
}
