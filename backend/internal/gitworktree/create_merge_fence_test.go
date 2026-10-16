package gitworktree

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCreateRespectsRepositoryMergeFence(t *testing.T) {
	manager, repo := fixture(t)
	second, err := New(manager.dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	unlock, err := manager.lockRepositoryMerge(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()
	_, err = second.Create(ctx, repo, "", "HEAD")
	if !errors.Is(err, context.DeadlineExceeded) {
		unlock()
		t.Fatalf("create bypassed merge fence: %v", err)
	}
	unlock()
	entries, err := manager.List(context.Background(), repo)
	if err != nil || len(entries) != 1 {
		t.Fatalf("blocked creation changed worktree state: %+v %v", entries, err)
	}
	created, err := second.Create(context.Background(), repo, "", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if !created.Managed || created.ID == "" {
		t.Fatalf("creation did not recover: %+v", created)
	}
	if _, err := second.Remove(context.Background(), repo, created.ID); err != nil {
		t.Fatal(err)
	}
}

func TestFailedCreationRecoveryHoldsRepositoryMergeFence(t *testing.T) {
	manager, repo := fixture(t)
	manager.gitPath = createFailureShim(t, "after-add")
	_, err := manager.Create(context.Background(), repo, "", "HEAD")
	if err == nil {
		t.Fatal("expected injected failure")
	}
	entries, err := manager.List(context.Background(), repo)
	if err != nil || len(entries) != 1 {
		t.Fatalf("failed create did not safely recover: %+v %v", entries, err)
	}
}
