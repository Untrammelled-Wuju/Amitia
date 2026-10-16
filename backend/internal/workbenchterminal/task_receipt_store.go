package workbenchterminal

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const maxStoredTaskReceipts = 256
const taskReceiptRetention = 7 * 24 * time.Hour
const receiptLockTimeout = 4 * time.Second

type TaskReceipt struct {
	SessionID      string    `json:"sessionId"`
	TaskID         string    `json:"taskId"`
	ConversationID string    `json:"conversationId"`
	WorkspaceURI   string    `json:"workspaceUri"`
	Mode           string    `json:"mode"`
	StartedAt      time.Time `json:"startedAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	State          string    `json:"state"`
	ExitCode       *int      `json:"exitCode,omitempty"`
	LatestSequence uint64    `json:"latestSequence"`
	EvidenceKind   string    `json:"evidenceKind"`
}

type taskReceiptDocument struct {
	Version     int         `json:"version"`
	Receipt     TaskReceipt `json:"receipt"`
	OwnerDigest string      `json:"ownerDigest"`
}

type TaskReceiptJournal struct {
	mu  sync.Mutex
	dir string
}

func NewTaskReceiptJournal(dir string) (*TaskReceiptJournal, error) {
	if dir == "" {
		return nil, ErrInvalid
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || receiptPermissionsUnsafe(info.Mode()) {
		return nil, ErrInvalid
	}
	return &TaskReceiptJournal{dir: dir}, nil
}

func defaultTaskReceiptJournal() (*TaskReceiptJournal, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return NewTaskReceiptJournal(filepath.Join(config, "amitia", "workbench", "terminal-task-receipts"))
}

func ownerDigest(owner string) string {
	h := sha256.Sum256([]byte(owner))
	return hex.EncodeToString(h[:])
}

func (j *TaskReceiptJournal) fileFor(id string) (string, error) {
	if len(id) != 40 {
		return "", ErrInvalid
	}
	for _, c := range id {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return "", ErrInvalid
		}
	}
	return filepath.Join(j.dir, id+".json"), nil
}

func (j *TaskReceiptJournal) readLocked(path string) (taskReceiptDocument, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return taskReceiptDocument{}, err
	}
	if !info.Mode().IsRegular() || receiptPermissionsUnsafe(info.Mode()) || info.Size() > 8192 {
		return taskReceiptDocument{}, ErrInvalid
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return taskReceiptDocument{}, err
	}
	var doc taskReceiptDocument
	if json.Unmarshal(data, &doc) != nil || doc.Version != 1 || doc.Receipt.SessionID == "" || doc.Receipt.TaskID == "" || doc.OwnerDigest == "" {
		return taskReceiptDocument{}, ErrInvalid
	}
	return doc, nil
}

func receiptPermissionsUnsafe(mode os.FileMode) bool {
	return runtime.GOOS != "windows" && mode.Perm()&0077 != 0
}

func (j *TaskReceiptJournal) locked(action func() error) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	unlock, err := lockReceiptJournal(filepath.Join(j.dir, ".journal.lock"), receiptLockTimeout)
	if err != nil {
		return err
	}
	defer unlock()
	return action()
}

func receiptTerminal(state string) bool {
	return state == "exited" || state == "closed" || state == "revoked"
}

func (j *TaskReceiptJournal) Record(owner string, receipt TaskReceipt) error {
	if j == nil || owner == "" || !taskIDPattern.MatchString(receipt.TaskID) || !conversationPattern.MatchString(receipt.ConversationID) {
		return ErrInvalid
	}
	path, err := j.fileFor(receipt.SessionID)
	if err != nil {
		return err
	}
	if receipt.StartedAt.IsZero() || receipt.WorkspaceURI == "" {
		return ErrInvalid
	}
	receipt.EvidenceKind = "terminal-lifecycle-metadata-not-test-proof"
	receipt.UpdatedAt = time.Now().UTC()
	digest := ownerDigest(owner)
	return j.locked(func() error {
		previous, readErr := j.readLocked(path)
		if readErr == nil {
			if previous.OwnerDigest != digest || previous.Receipt.TaskID != receipt.TaskID || previous.Receipt.ConversationID != receipt.ConversationID || previous.Receipt.WorkspaceURI != receipt.WorkspaceURI || !previous.Receipt.StartedAt.Equal(receipt.StartedAt) {
				return ErrTaskBinding
			}
			if receiptTerminal(previous.Receipt.State) {
				return nil
			}
			if previous.Receipt.LatestSequence > receipt.LatestSequence {
				return nil
			}
		} else if !errors.Is(readErr, os.ErrNotExist) {
			return readErr
		}
		doc := taskReceiptDocument{Version: 1, Receipt: receipt, OwnerDigest: digest}
		data, marshalErr := json.Marshal(doc)
		if marshalErr != nil {
			return marshalErr
		}
		tmp, createErr := os.CreateTemp(j.dir, ".receipt-")
		if createErr != nil {
			return createErr
		}
		tmpName := tmp.Name()
		defer os.Remove(tmpName)
		if createErr = tmp.Chmod(0600); createErr == nil {
			_, createErr = tmp.Write(data)
		}
		if createErr == nil {
			createErr = tmp.Sync()
		}
		if closeErr := tmp.Close(); createErr == nil {
			createErr = closeErr
		}
		if createErr != nil {
			return createErr
		}
		if err := os.Rename(tmpName, path); err != nil {
			return err
		}
		return syncReceiptDirectory(j.dir)
	})
}

func (j *TaskReceiptJournal) List(owner, conversationID, taskID string, active map[string]bool) ([]TaskReceipt, error) {
	return j.listRecords(owner, conversationID, taskID, active, true)
}

func (j *TaskReceiptJournal) ListRecorded(owner, conversationID, taskID string) ([]TaskReceipt, error) {
	return j.listRecords(owner, conversationID, taskID, nil, false)
}

func (j *TaskReceiptJournal) listRecords(owner, conversationID, taskID string, active map[string]bool, markInterrupted bool) ([]TaskReceipt, error) {
	if j == nil || owner == "" || !conversationPattern.MatchString(conversationID) || !taskIDPattern.MatchString(taskID) {
		return nil, ErrInvalid
	}
	result := make([]TaskReceipt, 0)
	err := j.locked(func() error {
		entries, readErr := os.ReadDir(j.dir)
		if readErr != nil {
			return readErr
		}
		digest := ownerDigest(owner)
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".json") || len(entry.Name()) != 45 {
				continue
			}
			path, pathErr := j.fileFor(strings.TrimSuffix(entry.Name(), ".json"))
			if pathErr != nil {
				continue
			}
			doc, readErr := j.readLocked(path)
			if readErr != nil {
				continue
			}
			if doc.OwnerDigest != digest || doc.Receipt.ConversationID != conversationID || doc.Receipt.TaskID != taskID || doc.Receipt.SessionID+".json" != entry.Name() {
				continue
			}
			r := doc.Receipt
			if time.Since(r.UpdatedAt) > taskReceiptRetention {
				continue
			}
			if markInterrupted && r.State == "running" && !active[r.SessionID] {
				r.State = "interrupted"
			}
			result = append(result, r)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(result, func(i, k int) bool { return result[i].StartedAt.After(result[k].StartedAt) })
	if len(result) > 32 {
		result = result[:32]
	}
	return result, nil
}

func (j *TaskReceiptJournal) Prune() error {
	if j == nil {
		return ErrInvalid
	}
	return j.locked(func() error {
		entries, err := os.ReadDir(j.dir)
		if err != nil {
			return err
		}
		type candidate struct {
			path    string
			updated time.Time
		}
		candidates := make([]candidate, 0)
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".json") || len(entry.Name()) != 45 {
				continue
			}
			path, pathErr := j.fileFor(strings.TrimSuffix(entry.Name(), ".json"))
			if pathErr != nil {
				continue
			}
			doc, readErr := j.readLocked(path)
			if readErr != nil {
				continue
			}
			candidates = append(candidates, candidate{path, doc.Receipt.UpdatedAt})
		}
		sort.Slice(candidates, func(i, k int) bool { return candidates[i].updated.After(candidates[k].updated) })
		for i, c := range candidates {
			if i >= maxStoredTaskReceipts || time.Since(c.updated) > taskReceiptRetention {
				if err := os.Remove(c.path); err != nil && !errors.Is(err, os.ErrNotExist) {
					return err
				}
			}
		}
		return nil
	})
}
