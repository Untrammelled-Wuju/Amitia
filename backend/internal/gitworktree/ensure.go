package gitworktree

import (
	"context"
	"fmt"
	"time"
)

func (m *Manager) EnsureTask(ctx context.Context, repoRoot, branch, ref, taskID, conversationID string) (Entry, bool, error) {
	if !ValidBinding(taskID, conversationID) {
		return Entry{}, false, ErrInvalidArgument
	}
	if ref != "" && !validateRef(ref) || branch != "" && !validateRef(branch) {
		return Entry{}, false, ErrInvalidArgument
	}
	mutationMu.Lock()
	defer mutationMu.Unlock()
	unlock, err := m.lockRepositoryAllocation(ctx, repoRoot)
	if err != nil {
		return Entry{}, false, err
	}
	defer unlock()
	mergeUnlock, err := m.lockRepositoryMerge(ctx, repoRoot)
	if err != nil {
		return Entry{}, false, err
	}
	defer mergeUnlock()
	entries, _, err := m.list(ctx, repoRoot)
	if err != nil {
		return Entry{}, false, err
	}
	var existing *Entry
	for i := range entries {
		item := entries[i]
		if !item.Managed || item.TaskBinding == nil {
			continue
		}
		if item.TaskBinding.TaskID == taskID && item.TaskBinding.ConversationID != conversationID {
			return Entry{}, false, ErrBound
		}
		if item.TaskBinding.TaskID != taskID || item.TaskBinding.ConversationID != conversationID {
			continue
		}
		if existing != nil {
			return Entry{}, false, fmt.Errorf("%w: duplicate task allocations", ErrBound)
		}
		existing = &item
	}
	if existing != nil {
		if branch != "" && existing.Branch != branch {
			return Entry{}, false, ErrBound
		}
		return *existing, false, nil
	}
	item, err := m.createLocked(ctx, repoRoot, branch, ref)
	if err != nil {
		return Entry{}, false, err
	}
	if err := ctx.Err(); err != nil {
		return Entry{}, false, m.rollbackCreatedTask(ctx, repoRoot, item.ID, err)
	}
	binding, err := m.bindLocked(ctx, repoRoot, item.ID, taskID, conversationID)
	if err != nil {
		return Entry{}, false, m.rollbackCreatedTask(ctx, repoRoot, item.ID, err)
	}
	item.TaskBinding = binding
	return item, true, nil
}

func (m *Manager) rollbackCreatedTask(ctx context.Context, repoRoot, id string, originalErr error) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if _, removeErr := m.removeWithMergeLock(cleanupCtx, repoRoot, id); removeErr != nil {
		return fmt.Errorf("bind: %w; worktree rollback: %v", originalErr, removeErr)
	}
	return originalErr
}
