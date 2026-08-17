package artifact

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Journal struct {
	RevisionID string
	Path       string
	Entries    []JournalEntry
}

type JournalEntry struct {
	Stage     string `json:"stage"`
	Status    string `json:"status"`
	Detail    string `json:"detail"`
	Timestamp string `json:"timestamp"`
}

func NewJournal(revisionID, path string) *Journal {
	return &Journal{
		RevisionID: revisionID,
		Path:       path,
		Entries:    make([]JournalEntry, 0),
	}
}

func (j *Journal) Record(stage, status, detail string) error {
	if j.Path == "" {
		return fmt.Errorf("artifact: journal path is empty")
	}
	entry := JournalEntry{
		Stage:     stage,
		Status:    status,
		Detail:    detail,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
	}
	j.Entries = append(j.Entries, entry)
	return j.Save()
}

func (j *Journal) Load() error {
	if j.Path == "" {
		return fmt.Errorf("artifact: journal path is empty")
	}
	data, err := os.ReadFile(j.Path)
	if err != nil {
		if os.IsNotExist(err) {
			j.Entries = make([]JournalEntry, 0)
			return nil
		}
		return fmt.Errorf("artifact: load journal %s: %w", j.Path, err)
	}
	var entries []JournalEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("artifact: unmarshal journal %s: %w", j.Path, err)
	}
	j.Entries = entries
	return nil
}

func (j *Journal) Save() error {
	if j.Path == "" {
		return fmt.Errorf("artifact: journal path is empty")
	}
	dir := filepath.Dir(j.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("artifact: create journal dir %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(j.Entries, "", "  ")
	if err != nil {
		return fmt.Errorf("artifact: marshal journal: %w", err)
	}
	if err := os.WriteFile(j.Path, data, 0o600); err != nil {
		return fmt.Errorf("artifact: write journal %s: %w", j.Path, err)
	}
	return nil
}

func (j *Journal) GetLastStage() string {
	var lastStage string
	for _, entry := range j.Entries {
		if entry.Status == "done" {
			lastStage = entry.Stage
		}
	}
	return lastStage
}

func (j *Journal) IsStageDone(stage string) bool {
	for _, entry := range j.Entries {
		if entry.Stage == stage && entry.Status == "done" {
			return true
		}
	}
	return false
}

func (j *Journal) IsStageFailed(stage string) bool {
	for _, entry := range j.Entries {
		if entry.Stage == stage && entry.Status == "failed" {
			return true
		}
	}
	return false
}

func (j *Journal) CanResume() bool {
	if j.IsStageDone("db_committed") {
		return false
	}
	if j.IsStageDone("preparing") || j.IsStageDone("validated") {
		return true
	}
	return false
}
