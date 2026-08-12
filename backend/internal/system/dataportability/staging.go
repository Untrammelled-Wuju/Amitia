package dataportability

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type StagingManager struct {
	BaseDir string
}

func NewStagingManager(baseDir string) *StagingManager {
	return &StagingManager{BaseDir: baseDir}
}

func randomID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *StagingManager) CreateStaging() (string, error) {
	id := randomID()
	path := filepath.Join(s.BaseDir, "imports", ".staging", id)
	if err := os.MkdirAll(path, 0o700); err != nil {
		return "", err
	}
	return path, nil
}

func (s *StagingManager) CleanupStaging(path string) error {
	if path == "" {
		return nil
	}
	if !strings.HasPrefix(path, filepath.Join(s.BaseDir, "imports", ".staging")) {
		return ErrImportStagingFailed
	}
	return os.RemoveAll(path)
}

func SanitizeArchivePath(name string) error {
	if name == "" {
		return ErrImportArchiveInvalid
	}
	if strings.Contains(name, "..") {
		return ErrImportArchiveInvalid
	}
	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, `\`) {
		return ErrImportArchiveInvalid
	}
	if len(name) > 1 && name[1] == ':' {
		return ErrImportArchiveInvalid
	}
	for _, c := range name {
		if c == 0 {
			return ErrImportArchiveInvalid
		}
	}
	return nil
}

type DataRestoreTicket struct {
	OperationID              string `json:"operationId"`
	BackupID                 string `json:"backupId"`
	StagingPath              string `json:"stagingPath"`
	ManifestHash             string `json:"manifestHash"`
	SourceSchemaFingerprint  string `json:"sourceSchemaFingerprint"`
	ExpectedCurrentFingerprint string `json:"expectedCurrentFingerprint"`
	CreatedAt                string `json:"createdAt"`
}

func (s *StagingManager) WriteRestoreTicket(ticket *DataRestoreTicket) error {
	dir := filepath.Join(s.BaseDir, ".restore")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(ticket)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "ticket.json"), data, 0o600)
}

func (s *StagingManager) ReadRestoreTicket() (*DataRestoreTicket, error) {
	path := filepath.Join(s.BaseDir, ".restore", "ticket.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ticket DataRestoreTicket
	if err := json.Unmarshal(data, &ticket); err != nil {
		return nil, err
	}
	return &ticket, nil
}

func (s *StagingManager) ClearRestoreTicket() error {
	path := filepath.Join(s.BaseDir, ".restore", "ticket.json")
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *StagingManager) HasRestoreTicket() bool {
	_, err := os.Stat(filepath.Join(s.BaseDir, ".restore", "ticket.json"))
	return err == nil
}
