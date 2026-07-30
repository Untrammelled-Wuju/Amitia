package trust

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
)

type memoryMutationJournal struct {
	mu           sync.Mutex
	version      uint64
	pending      map[string]PolicyMutation
	failActivate bool
}

func newMemoryMutationJournal() *memoryMutationJournal {
	return &memoryMutationJournal{pending: map[string]PolicyMutation{}}
}

func (j *memoryMutationJournal) ReservePending(_ context.Context, mutation PolicyMutation) (PolicyMutation, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.version++
	mutation.Version = j.version
	mutation.MutationID = mutationID(mutation.Version)
	j.pending[mutation.MutationID] = mutation
	return mutation, nil
}

func (j *memoryMutationJournal) MarkActive(_ context.Context, mutation PolicyMutation) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.failActivate {
		return errors.New("activate failed")
	}
	delete(j.pending, mutation.MutationID)
	return nil
}

func (j *memoryMutationJournal) Pending(_ context.Context) ([]PolicyMutation, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	result := make([]PolicyMutation, 0, len(j.pending))
	for _, mutation := range j.pending {
		result = append(result, mutation)
	}
	return result, nil
}

func TestMutationCoordinatorOrdersPendingApplyInvalidationAndActive(t *testing.T) {
	journal := newMemoryMutationJournal()
	var order []string
	coordinator := NewMutationCoordinator(journal,
		PolicyMutationApplierFunc(func(context.Context, PolicyMutation) (func() error, error) {
			order = append(order, "apply")
			return func() error { order = append(order, "rollback"); return nil }, nil
		}),
		PolicyMutationInvalidatorFunc(func(context.Context, PolicyMutation) error {
			order = append(order, "invalidate")
			return nil
		}))

	active, err := coordinator.Execute(context.Background(), PolicyMutation{Kind: PolicyMutationPublisherTrust, Actor: "user-1", Reason: "approved"})
	if err != nil {
		t.Fatal(err)
	}
	if active.State != PolicyMutationActive || active.Version != 1 || len(order) != 2 || order[0] != "apply" || order[1] != "invalidate" {
		t.Fatalf("unexpected mutation result: %#v order=%v", active, order)
	}
}

func TestMutationCoordinatorRollsBackPermissiveActivationFailure(t *testing.T) {
	journal := newMemoryMutationJournal()
	journal.failActivate = true
	rolledBack := false
	coordinator := NewMutationCoordinator(journal,
		PolicyMutationApplierFunc(func(context.Context, PolicyMutation) (func() error, error) {
			return func() error { rolledBack = true; return nil }, nil
		}), PolicyMutationInvalidatorFunc(func(context.Context, PolicyMutation) error { return nil }))

	_, err := coordinator.Execute(context.Background(), PolicyMutation{Kind: PolicyMutationPublisherTrust, Actor: "user-1", Reason: "approved"})
	if err == nil || !rolledBack {
		t.Fatalf("expected activation error with rollback, err=%v rollback=%v", err, rolledBack)
	}
}

func TestMutationCoordinatorKeepsRestrictiveStateOnActivationFailure(t *testing.T) {
	journal := newMemoryMutationJournal()
	journal.failActivate = true
	rolledBack := false
	coordinator := NewMutationCoordinator(journal,
		PolicyMutationApplierFunc(func(context.Context, PolicyMutation) (func() error, error) {
			return func() error { rolledBack = true; return nil }, nil
		}), PolicyMutationInvalidatorFunc(func(context.Context, PolicyMutation) error { return nil }))

	_, err := coordinator.Execute(context.Background(), PolicyMutation{Kind: PolicyMutationRevocation, Actor: "admin", Reason: "compromised", Restrictive: true})
	if err == nil || rolledBack {
		t.Fatalf("expected fail-closed restriction, err=%v rollback=%v", err, rolledBack)
	}
}

func TestMutationCoordinatorReplaysPendingMutation(t *testing.T) {
	journal := newMemoryMutationJournal()
	pending, _ := journal.ReservePending(context.Background(), PolicyMutation{Kind: PolicyMutationBlocklist, Actor: "admin", Reason: "malware", Restrictive: true})
	applied := ""
	coordinator := NewMutationCoordinator(journal,
		PolicyMutationApplierFunc(func(_ context.Context, mutation PolicyMutation) (func() error, error) {
			applied = mutation.MutationID
			return nil, nil
		}), PolicyMutationInvalidatorFunc(func(context.Context, PolicyMutation) error { return nil }))

	if err := coordinator.ReplayPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	remaining, _ := journal.Pending(context.Background())
	if applied != pending.MutationID || len(remaining) != 0 {
		t.Fatalf("pending mutation not replayed: applied=%s remaining=%d", applied, len(remaining))
	}
}

func TestMutationJournalAllocatesMonotonicVersionsConcurrently(t *testing.T) {
	journal := newMemoryMutationJournal()
	const workers = 32
	versions := make(chan uint64, workers)
	var wait sync.WaitGroup
	for i := 0; i < workers; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			mutation, err := journal.ReservePending(context.Background(), PolicyMutation{Kind: PolicyMutationPublisherTrust})
			if err == nil {
				versions <- mutation.Version
			}
		}()
	}
	wait.Wait()
	close(versions)
	seen := map[uint64]bool{}
	for version := range versions {
		seen[version] = true
	}
	if len(seen) != workers || !seen[1] || !seen[workers] {
		t.Fatalf("versions are not monotonic and unique: %v", seen)
	}
}

func mutationID(version uint64) string {
	return fmt.Sprintf("mutation-%d", version)
}
