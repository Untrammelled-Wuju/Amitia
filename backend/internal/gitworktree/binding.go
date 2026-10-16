package gitworktree

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var ErrBound = errors.New("worktree is attached to an active task")
var ErrBindingMismatch = errors.New("worktree task binding does not match")
var ErrReviewOutdated = errors.New("worktree revision changed since review request")
var bindingIdentifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
var mutationMu sync.Mutex

type TaskBinding struct {
	TaskID         string    `json:"taskId"`
	ConversationID string    `json:"conversationId"`
	BoundAt        time.Time `json:"boundAt"`
	BaseHead       string    `json:"baseHead,omitempty"`
}

func ValidBinding(taskID, conversationID string) bool {
	return bindingIdentifier.MatchString(taskID) && bindingIdentifier.MatchString(conversationID)
}

func validBaseHead(head string) bool {
	if head == "" {
		return true
	}
	if len(head) != 40 && len(head) != 64 {
		return false
	}
	return strings.Trim(head, "0123456789abcdef") == ""
}

func (m *Manager) bindingPath(key, id string) string {
	return filepath.Join(m.managedRoot(key), ".bindings", id+".json")
}

func (m *Manager) readBinding(key, id string) (*TaskBinding, error) {
	path := m.bindingPath(key, id)
	directory := filepath.Dir(path)
	dirInfo, dirErr := os.Lstat(directory)
	if os.IsNotExist(dirErr) {
		return nil, nil
	}
	if dirErr != nil {
		return nil, dirErr
	}
	if !dirInfo.IsDir() || dirInfo.Mode()&os.ModeSymlink != 0 {
		return nil, ErrInvalidArgument
	}
	realDir, err := filepath.EvalSymlinks(directory)
	if err != nil || filepath.Clean(realDir) != filepath.Clean(directory) {
		return nil, ErrInvalidArgument
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 4096 || metadataFilePermissionsUnsafe(info.Mode().Perm()) {
		return nil, fmt.Errorf("%w: invalid task binding storage", ErrInvalidArgument)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var binding TaskBinding
	if err := json.Unmarshal(data, &binding); err != nil || !ValidBinding(binding.TaskID, binding.ConversationID) || binding.BoundAt.IsZero() || !validBaseHead(binding.BaseHead) {
		return nil, fmt.Errorf("%w: corrupted task binding", ErrInvalidArgument)
	}
	return &binding, nil
}

func (m *Manager) Bind(ctx context.Context, repoRoot, id, taskID, conversationID string) (*TaskBinding, error) {
	mutationMu.Lock()
	defer mutationMu.Unlock()
	unlock, err := m.lockRepositoryAllocation(ctx, repoRoot)
	if err != nil {
		return nil, err
	}
	defer unlock()
	mergeUnlock, err := m.lockRepositoryMerge(ctx, repoRoot)
	if err != nil {
		return nil, err
	}
	defer mergeUnlock()
	return m.bindLocked(ctx, repoRoot, id, taskID, conversationID)
}

func (m *Manager) bindLocked(ctx context.Context, repoRoot, id, taskID, conversationID string) (*TaskBinding, error) {
	if !managedID.MatchString(id) || !ValidBinding(taskID, conversationID) {
		return nil, ErrInvalidArgument
	}
	entries, canonical, err := m.list(ctx, repoRoot)
	if err != nil {
		return nil, err
	}
	found := false
	baseHead := ""
	for _, entry := range entries {
		if entry.Managed && entry.ID == id {
			baseHead = entry.Head
			if entry.TaskBinding != nil {
				if entry.TaskBinding.TaskID == taskID && entry.TaskBinding.ConversationID == conversationID {
					return entry.TaskBinding, nil
				}
				return nil, ErrBound
			}
			found = true
			break
		}
	}
	if !found {
		return nil, ErrNotManaged
	}
	_, key, err := m.repository(ctx, canonical)
	if err != nil {
		return nil, err
	}
	audit, err := m.readMergeAudit(key, id)
	if err != nil {
		return nil, err
	}
	if audit != nil {
		return nil, ErrMergePreviouslyAttempted
	}
	checkpoint, err := m.readCheckpoint(key, id)
	if err != nil {
		return nil, err
	}
	if checkpoint != nil {
		return nil, ErrCheckpointRequired
	}
	path := m.bindingPath(key, id)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil || filepath.Clean(realDir) != filepath.Clean(dir) {
		return nil, ErrInvalidArgument
	}
	binding := &TaskBinding{TaskID: taskID, ConversationID: conversationID, BoundAt: time.Now().UTC(), BaseHead: baseHead}
	data, err := json.Marshal(binding)
	if err != nil {
		return nil, err
	}
	temp, err := os.CreateTemp(dir, ".binding-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(temp.Name())
	if err := temp.Chmod(0600); err != nil {
		temp.Close()
		return nil, err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return nil, err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return nil, err
	}
	if err := temp.Close(); err != nil {
		return nil, err
	}
	if _, err := os.Lstat(path); err == nil {
		return nil, ErrBound
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		return nil, err
	}
	if err := syncManagedMetadataDirectory(dir); err != nil {
		return nil, err
	}
	return binding, nil
}

func (m *Manager) Release(ctx context.Context, repoRoot, id, taskID, conversationID string) error {
	if !managedID.MatchString(id) || !ValidBinding(taskID, conversationID) {
		return ErrInvalidArgument
	}
	finish, err := m.lockCheckpointExecution(ctx, repoRoot, id)
	if err != nil {
		return err
	}
	defer finish()
	mutationMu.Lock()
	defer mutationMu.Unlock()
	unlock, err := m.lockRepositoryAllocation(ctx, repoRoot)
	if err != nil {
		return err
	}
	defer unlock()
	entries, canonical, err := m.list(ctx, repoRoot)
	if err != nil {
		return err
	}
	var attached *TaskBinding
	found := false
	for _, entry := range entries {
		if entry.Managed && entry.ID == id {
			attached = entry.TaskBinding
			found = true
			break
		}
	}
	if !found {
		return ErrNotManaged
	}
	if attached == nil || attached.TaskID != taskID || attached.ConversationID != conversationID {
		return ErrBindingMismatch
	}
	_, key, err := m.repository(ctx, canonical)
	if err != nil {
		return err
	}
	mergeUnlock, err := m.lockRepositoryMerge(ctx, repoRoot)
	if err != nil {
		return err
	}
	defer mergeUnlock()
	if err := m.refusePendingMergeAudit(key, id); err != nil {
		return err
	}
	if err := m.invalidateCheckpointByKey(key, id); err != nil {
		return err
	}
	if err := os.Remove(m.bindingPath(key, id)); err != nil {
		return err
	}
	return syncManagedMetadataDirectory(filepath.Dir(m.bindingPath(key, id)))
}
