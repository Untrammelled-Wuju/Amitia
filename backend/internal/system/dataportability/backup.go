package dataportability

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	amitialog "log"
)

const batchSize = 1000

type BackupResult struct {
	BackupID  string          `json:"backupId"`
	Path      string          `json:"path"`
	SizeBytes int64           `json:"sizeBytes"`
	Checksum  string          `json:"checksum"`
	Manifest  *BackupManifest `json:"manifest"`
}

func GenerateBackupID() string {
	return "backup-" + uuid.NewString()
}

func IsValidBackupFileName(name string) bool {
	if filepath.Base(name) != name {
		return false
	}
	value := strings.TrimSuffix(name, ".enc")
	if !strings.HasSuffix(value, ".amitia-backup") {
		return false
	}
	id := strings.TrimSuffix(value, ".amitia-backup")
	if !strings.HasPrefix(id, "backup-") {
		return false
	}
	_, err := uuid.Parse(strings.TrimPrefix(id, "backup-"))
	return err == nil
}

func (c *Coordinator) CreateBackup(ctx context.Context, req BackupRequest, dbPath string, runner SnapshotRunner) (*BackupResult, error) {
	if err := ValidateBackupRequest(req); err != nil {
		return nil, err
	}
	opID := GenerateBackupID()
	op := &BackupOperation{
		ID:        opID,
		Status:    "planning",
		Profile:   string(req.EffectiveProfile()),
		Scope:     string(req.EffectiveScope()),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	c.mu.Lock()
	c.backupOps[opID] = op
	c.mu.Unlock()

	backupDir := filepath.Join(c.DataDir, "backups")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return nil, c.failBackup(opID, err)
	}

	if err := c.CheckDiskSpace(req, backupDir); err != nil {
		return nil, c.failBackup(opID, err)
	}

	op.Status = "running"

	manifest := &BackupManifest{
		Format:            FormatName,
		FormatVersion:     FormatVersion,
		BackupID:          opID,
		CreatedAt:         time.Now().UTC().Format(time.RFC3339),
		AppVersion:        c.AppVersion,
		SchemaFingerprint: c.SchemaFinger,
		Profile:           string(req.EffectiveProfile()),
		Scope:             string(req.EffectiveScope()),
		CharacterID:       req.CharacterID,
		SourcePlatform:    c.Platform,
		Encrypted:         req.Passphrase != "",
		IncludesSecrets:   req.IncludeSecrets,
		SecretsPolicy:     CurrentSecretsPolicy,
	}

	archivePath := filepath.Join(backupDir, opID+".amitia-backup")
	partialPath := filepath.Join(backupDir, "."+opID+".amitia-backup.partial")
	encryptedPartialPath := ""
	published := false
	defer func() {
		if published {
			return
		}
		for _, path := range []string{partialPath, encryptedPartialPath, archivePath} {
			if path == "" {
				continue
			}
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				amitialog.Printf("[original-error] %v", err)
			}
		}
	}()
	writer := NewArchiveWriter(partialPath)
	defer writer.Close()

	componentIDs := make(map[string]bool)
	for _, ct := range c.Contributors {
		plans, err := ct.Plan(ctx, req)
		if err != nil {
			return nil, c.failBackup(opID, fmt.Errorf("contributor %s plan failed: %w", ct.ID(), err))
		}
		captures := make(map[string]*backupComponentCapture, len(plans))
		ordered := make([]*backupComponentCapture, 0, len(plans))
		for _, plan := range plans {
			if componentIDs[plan.ID] {
				return nil, c.failBackup(opID, fmt.Errorf("duplicate backup component: %s", plan.ID))
			}
			componentIDs[plan.ID] = true
			compWriter, err := writer.CreateComponent(plan.ID, plan.LogicalName, plan.Kind)
			if err != nil {
				return nil, c.failBackup(opID, err)
			}
			hash := sha256.New()
			counting := &countingWriter{w: io.MultiWriter(compWriter, hash)}
			capture := &backupComponentCapture{
				plan:     plan,
				writer:   compWriter,
				counting: counting,
				hash:     hash,
			}
			captures[plan.ID] = capture
			ordered = append(ordered, capture)
		}
		adapter := &componentWriterAdapter{captures: captures}
		if err := ct.Export(ctx, req, adapter); err != nil {
			for _, capture := range ordered {
				if err := capture.writer.Close(); err != nil {
					amitialog.Printf("[original-error] %v", err)
				}
			}
			return nil, c.failBackup(opID, fmt.Errorf("contributor %s export failed: %w", ct.ID(), err))
		}
		for _, capture := range ordered {
			if err := capture.writer.Close(); err != nil {
				return nil, c.failBackup(opID, err)
			}
			manifest.Components = append(manifest.Components, BackupComponentManifest{
				ID:            capture.plan.ID,
				Kind:          string(capture.plan.Kind),
				LogicalName:   capture.plan.LogicalName,
				Path:          fmt.Sprintf("datasets/%s.ndjson", capture.plan.ID),
				SizeBytes:     capture.counting.n,
				ItemCount:     capture.plan.ItemCount,
				SHA256:        hex.EncodeToString(capture.hash.Sum(nil)),
				Required:      capture.plan.Required,
				SourceOfTruth: capture.plan.SourceOfTruth,
				Rebuildable:   capture.plan.Rebuildable,
				Sensitive:     capture.plan.Sensitive,
			})
		}
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, c.failBackup(opID, err)
	}
	if err := writer.WriteManifest(manifestData); err != nil {
		return nil, c.failBackup(opID, err)
	}

	if err := writer.Finalize(); err != nil {
		return nil, c.failBackup(opID, err)
	}

	pkgPath := partialPath
	if req.Passphrase != "" {
		encryptedPartialPath = filepath.Join(backupDir, "."+opID+".amitia-backup.enc.partial")
		if err := EncryptFile(partialPath, encryptedPartialPath, req.Passphrase); err != nil {
			if err := os.Remove(encryptedPartialPath); err != nil && !os.IsNotExist(err) {
				amitialog.Printf("[original-error] %v", err)
			}
			return nil, c.failBackup(opID, fmt.Errorf("%w: %v", ErrBackupEncryptionFailed, err))
		}
		if err := os.Remove(partialPath); err != nil {
			if err := os.Remove(encryptedPartialPath); err != nil && !os.IsNotExist(err) {
				amitialog.Printf("[original-error] %v", err)
			}
			return nil, c.failBackup(opID, err)
		}
		archivePath += ".enc"
		pkgPath = encryptedPartialPath
	}
	if err := os.Rename(pkgPath, archivePath); err != nil {
		return nil, c.failBackup(opID, err)
	}
	pkgPath = archivePath

	info, err := os.Stat(pkgPath)
	if err != nil {
		return nil, c.failBackup(opID, err)
	}

	checksum, err := fileSHA256(pkgPath)
	if err != nil {
		return nil, c.failBackup(opID, err)
	}
	op.Status = "completed"
	op.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	published = true

	return &BackupResult{
		BackupID:  opID,
		Path:      pkgPath,
		SizeBytes: info.Size(),
		Checksum:  checksum,
		Manifest:  manifest,
	}, nil
}

func ValidateBackupRequest(req BackupRequest) error {
	switch req.EffectiveProfile() {
	case ProfileFull, ProfilePortable:
	default:
		return ErrBackupProfileInvalid
	}
	switch req.EffectiveScope() {
	case ScopeAll, ScopeCharacter, ScopeSettings, ScopeMemory:
	default:
		return ErrBackupScopeInvalid
	}
	if req.EffectiveScope() == ScopeCharacter && req.CharacterID == "" {
		return ErrBackupScopeInvalid
	}
	if req.EffectiveScope() == ScopeSettings && req.CharacterID != "" {
		return ErrBackupScopeInvalid
	}
	if req.Passphrase != "" && len([]rune(req.Passphrase)) < 8 {
		return ErrBackupPassphraseWeak
	}
	if req.IncludeSecrets && req.Passphrase == "" && req.Purpose != PurposePreImport && req.Purpose != PurposePreRestore {
		return ErrBackupEncryptionRequired
	}
	return nil
}

func (c *Coordinator) failBackup(opID string, err error) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if op, ok := c.backupOps[opID]; ok {
		op.Status = "failed"
		op.Error = err.Error()
		op.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return err
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (w *countingWriter) Write(p []byte) (int, error) {
	n, err := w.w.Write(p)
	w.n += int64(n)
	return n, err
}

type componentWriterAdapter struct {
	captures map[string]*backupComponentCapture
}

func (a *componentWriterAdapter) CreateComponent(id, logicalName string, kind ComponentKind) (io.WriteCloser, error) {
	capture, ok := a.captures[id]
	if !ok {
		return nil, fmt.Errorf("%w: undeclared component %s", ErrBackupComponentFailed, id)
	}
	return &nopWriteCloser{capture.counting}, nil
}

func (a *componentWriterAdapter) WriteJSON(id string, v interface{}) error {
	capture, ok := a.captures[id]
	if !ok {
		return fmt.Errorf("%w: undeclared component %s", ErrBackupComponentFailed, id)
	}
	return json.NewEncoder(capture.counting).Encode(v)
}

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }

type backupComponentCapture struct {
	plan     BackupComponentPlan
	writer   io.WriteCloser
	counting *countingWriter
	hash     hash.Hash
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type SnapshotRunner interface {
	CheckpointWAL()
	RunIntegrityCheck()
	RunForeignKeyCheck()
	GetSQLiteFiles() []SQLiteFileInfo
	BackupTo(destPath string) error
	Migrate() error
}

type SQLiteFileInfo struct {
	Name string
	Path string
}
