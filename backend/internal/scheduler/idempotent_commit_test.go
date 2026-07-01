package scheduler

import (
	"sync"
	"testing"
	"time"
)

type commitTestError struct{ msg string }

func (e *commitTestError) Error() string { return e.msg }

var errIdemTest = &commitTestError{msg: "test"}

func TestBuildIdempotencyKeyDeterministic(t *testing.T) {
	k1 := BuildIdempotencyKey(IdempotentOpMessage, "u1", "c1", "op1")
	k2 := BuildIdempotencyKey(IdempotentOpMessage, "u1", "c1", "op1")
	if k1 != k2 {
		t.Fatalf("expected identical keys, got %s and %s", k1, k2)
	}
}

func TestBuildIdempotencyKeyDifferentKind(t *testing.T) {
	k1 := BuildIdempotencyKey(IdempotentOpMessage, "u1", "c1", "op1")
	k2 := BuildIdempotencyKey(IdempotentOpEvent, "u1", "c1", "op1")
	if k1 == k2 {
		t.Fatal("expected different keys for different kinds")
	}
}

func TestBuildIdempotencyKeyDifferentIDs(t *testing.T) {
	k1 := BuildIdempotencyKey(IdempotentOpTool, "u1", "c1", "op1")
	k2 := BuildIdempotencyKey(IdempotentOpTool, "u1", "c1", "op2")
	if k1 == k2 {
		t.Fatal("expected different keys for different operation IDs")
	}
}

func TestBuildIdempotencyKeyTrimsSpaces(t *testing.T) {
	k1 := BuildIdempotencyKey(IdempotentOpOutbox, " u1 ", " c1 ", " op1 ")
	k2 := BuildIdempotencyKey(IdempotentOpOutbox, "u1", "c1", "op1")
	if k1 != k2 {
		t.Fatalf("expected same keys after trim, got %s and %s", k1, k2)
	}
}

func TestBuildIdempotencyKeyAllKinds(t *testing.T) {
	kinds := []IdempotentOperationKind{
		IdempotentOpMessage, IdempotentOpEvent, IdempotentOpTool,
		IdempotentOpOutbox, IdempotentOpSend, IdempotentOpIndex,
		IdempotentOpDelete, IdempotentOpReflection,
	}
	for _, kind := range kinds {
		k := BuildIdempotencyKey(kind, "u1", "c1", "op1")
		if k == "" {
			t.Fatalf("expected non-empty key for kind %s", kind)
		}
	}
}

func TestOptimisticLockMatches(t *testing.T) {
	lock := NewOptimisticLock(5)
	if !lock.Matches(5) {
		t.Fatal("expected lock to match version 5")
	}
	if lock.Matches(6) {
		t.Fatal("expected lock to not match version 6")
	}
}

func TestOptimisticLockNext(t *testing.T) {
	lock := NewOptimisticLock(5)
	next := lock.Next()
	if next.CurrentVersion != 6 {
		t.Fatalf("expected next version 6, got %d", next.CurrentVersion)
	}
	if !next.Matches(6) {
		t.Fatal("expected next lock to match version 6")
	}
	if next.Token == "" {
		t.Fatal("expected token to be non-empty")
	}
}

func TestOptimisticLockTokenChanges(t *testing.T) {
	lock1 := NewOptimisticLock(3)
	time.Sleep(time.Microsecond)
	lock2 := NewOptimisticLock(3)
	if lock1.Token == lock2.Token {
		t.Fatal("expected different tokens for separate locks")
	}
}

func TestCommitQueueSerialization(t *testing.T) {
	cq := NewCommitQueue()
	var order []int
	var mu sync.Mutex

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			unlock := cq.Serialize("u1", "c1")
			defer unlock()
			mu.Lock()
			order = append(order, idx)
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	if len(order) != 3 {
		t.Fatalf("expected 3 serialized commits, got %d", len(order))
	}
}

func TestCommitQueueDifferentCharactersParallel(t *testing.T) {
	cq := NewCommitQueue()
	var count int32
	var mu sync.Mutex
	done := make(chan struct{}, 2)

	go func() {
		unlock := cq.Serialize("u1", "c1")
		defer unlock()
		mu.Lock()
		count++
		mu.Unlock()
		done <- struct{}{}
	}()

	go func() {
		unlock := cq.Serialize("u1", "c2")
		defer unlock()
		mu.Lock()
		count++
		mu.Unlock()
		done <- struct{}{}
	}()

	<-done
	<-done

	mu.Lock()
	c := count
	mu.Unlock()
	if c != 2 {
		t.Fatalf("expected 2 parallel commits, got %d", c)
	}
}

func TestRetryPolicyDefaults(t *testing.T) {
	rp := DefaultRetryPolicy()
	if rp.MaxRetries != 3 {
		t.Fatalf("expected 3 max retries, got %d", rp.MaxRetries)
	}
	if !rp.ShouldRetry(0, errIdemTest) {
		t.Fatal("expected retry on first attempt with error")
	}
	if rp.ShouldRetry(3, errIdemTest) {
		t.Fatal("expected no retry at max attempts")
	}
	if rp.ShouldRetry(2, nil) {
		t.Fatal("expected no retry when error is nil")
	}
}

func TestRetryPolicyDelayRange(t *testing.T) {
	rp := DefaultRetryPolicy()
	delay0 := rp.Delay(0)
	if delay0 < rp.BaseDelay {
		t.Fatalf("delay(0) too small: %v", delay0)
	}
	delay1 := rp.Delay(1)
	maxExpected := rp.BaseDelay * 2 * 2
	if delay1 > maxExpected || delay1 < rp.BaseDelay {
		t.Fatalf("delay(1) out of range: %v (min=%v max=%v)", delay1, rp.BaseDelay, maxExpected)
	}
	delayLarge := rp.Delay(20)
	if delayLarge > rp.MaxDelay {
		t.Fatalf("delay(20) exceeds max delay: %v > %v", delayLarge, rp.MaxDelay)
	}
}

func TestRetryPolicyCircuitBreaker(t *testing.T) {
	open := true
	rp := RetryPolicy{
		MaxRetries:    3,
		BaseDelay:     10 * time.Millisecond,
		JitterFactor:  0.1,
		CircuitBreaker: func() bool { return open },
	}
	if rp.ShouldRetry(0, errIdemTest) {
		t.Fatal("expected no retry when circuit breaker is open")
	}

	open = false
	rp.CircuitBreaker = func() bool { return open }
	if !rp.ShouldRetry(0, errIdemTest) {
		t.Fatal("expected retry when circuit breaker is closed")
	}
}

func TestCommitValidatorAllPass(t *testing.T) {
	cv := NewCommitValidator(3, "active", "gen-tok")
	if !cv.Validate(3, "active", "gen-tok") {
		t.Fatal("expected validation to pass")
	}
}

func TestCommitValidatorZeroStateVersion(t *testing.T) {
	cv := NewCommitValidator(0, "active", "gen-tok")
	if !cv.Validate(1, "active", "gen-tok") {
		t.Fatal("expected validation to pass when expected state version is 0")
	}
}

func TestCommitValidatorStateVersionMismatch(t *testing.T) {
	cv := NewCommitValidator(3, "active", "gen-tok")
	if cv.Validate(4, "active", "gen-tok") {
		t.Fatal("expected validation to fail on state version mismatch")
	}
}

func TestCommitValidatorStatusMismatch(t *testing.T) {
	cv := NewCommitValidator(3, "active", "gen-tok")
	if cv.Validate(3, "completed", "gen-tok") {
		t.Fatal("expected validation to fail on status mismatch")
	}
}

func TestCommitValidatorStatusCaseInsensitive(t *testing.T) {
	cv := NewCommitValidator(3, "active", "gen-tok")
	if !cv.Validate(3, "ACTIVE", "gen-tok") {
		t.Fatal("expected validation to pass with case-insensitive status")
	}
}

func TestCommitValidatorGenTokenMismatch(t *testing.T) {
	cv := NewCommitValidator(3, "active", "gen-tok")
	if cv.Validate(3, "active", "wrong-tok") {
		t.Fatal("expected validation to fail on generation token mismatch")
	}
}

func TestCommitValidatorEmptyGenToken(t *testing.T) {
	cv := NewCommitValidator(3, "active", "")
	if !cv.Validate(3, "active", "") {
		t.Fatal("expected validation to pass when generation token is empty on both sides")
	}
	if !cv.Validate(3, "active", "any-tok") {
		t.Fatal("expected validation to pass when expected generation token is empty")
	}
}

func TestIdempotentExecutorDuplicate(t *testing.T) {
	ie := NewIdempotentExecutor()
	input := IdempotentCommitInput{
		Kind:         IdempotentOpMessage,
		UserID:       "u1",
		CharacterID:  "c1",
		OperationID:  "op1",
		StateVersion: 1,
		Status:       "active",
	}

	r1 := ie.Commit(input)
	if r1.Status != CommitCommitted || r1.Duplicate {
		t.Fatalf("expected committed, got %s duplicate=%v", r1.Status, r1.Duplicate)
	}

	r2 := ie.Commit(input)
	if r2.Status != CommitDuplicate || !r2.Duplicate {
		t.Fatalf("expected duplicate, got %s duplicate=%v", r2.Status, r2.Duplicate)
	}
}

func TestIdempotentExecutorSupersededDiscard(t *testing.T) {
	ie := NewIdempotentExecutor()
	input := IdempotentCommitInput{
		Kind:         IdempotentOpTool,
		UserID:       "u1",
		CharacterID:  "c1",
		OperationID:  "op2",
		StateVersion: 2,
		Status:       "superseded",
	}

	r := ie.Commit(input)
	if r.Status != CommitSuperseded {
		t.Fatalf("expected superseded, got %s", r.Status)
	}

	auditRecords := ie.AuditRecords("c1")
	if len(auditRecords) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(auditRecords))
	}
	if auditRecords[0].Reason != "superseded" {
		t.Fatalf("expected audit reason superseded, got %s", auditRecords[0].Reason)
	}
}

func TestIdempotentExecutorCancelledDiscard(t *testing.T) {
	ie := NewIdempotentExecutor()
	input := IdempotentCommitInput{
		Kind:         IdempotentOpOutbox,
		UserID:       "u1",
		CharacterID:  "c1",
		OperationID:  "op3",
		StateVersion: 3,
		Status:       "cancelled",
	}

	r := ie.Commit(input)
	if r.Status != CommitCancelled {
		t.Fatalf("expected cancelled, got %s", r.Status)
	}

	auditRecords := ie.AuditRecords("c1")
	if len(auditRecords) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(auditRecords))
	}
}

func TestIdempotentExecutorCommitWithLock(t *testing.T) {
	ie := NewIdempotentExecutor()
	lock := NewOptimisticLock(5)

	input := IdempotentCommitInput{
		Kind:            IdempotentOpSend,
		UserID:          "u1",
		CharacterID:     "c1",
		OperationID:     "op4",
		StateVersion:    5,
		GenerationToken: lock.Token,
		Status:          "active",
	}

	r := ie.CommitWithLock(input, lock)
	if r.Status != CommitCommitted {
		t.Fatalf("expected committed with lock, got %s", r.Status)
	}
}

func TestIdempotentExecutorCommitWithLockConflict(t *testing.T) {
	ie := NewIdempotentExecutor()
	lock := NewOptimisticLock(5)

	input := IdempotentCommitInput{
		Kind:         IdempotentOpDelete,
		UserID:       "u1",
		CharacterID:  "c1",
		OperationID:  "op5",
		StateVersion: 6,
		Status:       "active",
	}

	r := ie.CommitWithLock(input, lock)
	if r.Status != CommitConflict {
		t.Fatalf("expected conflict, got %s", r.Status)
	}
}

func TestIdempotentExecutorDifferentKindsDifferentKeys(t *testing.T) {
	ie := NewIdempotentExecutor()

	r1 := ie.Commit(IdempotentCommitInput{
		Kind:        IdempotentOpMessage,
		UserID:      "u1",
		CharacterID: "c1",
		OperationID: "op",
		Status:      "active",
	})

	r2 := ie.Commit(IdempotentCommitInput{
		Kind:        IdempotentOpEvent,
		UserID:      "u1",
		CharacterID: "c1",
		OperationID: "op",
		Status:      "active",
	})

	if r1.Key == r2.Key {
		t.Fatal("expected different keys for different operation kinds")
	}
	if r1.Status != CommitCommitted || r2.Status != CommitCommitted {
		t.Fatal("expected both operations to be committed")
	}
}

func TestIdempotentExecutorIsCommitted(t *testing.T) {
	ie := NewIdempotentExecutor()
	key := BuildIdempotencyKey(IdempotentOpMessage, "u1", "c1", "op")

	if ie.IsCommitted(key) {
		t.Fatal("expected not committed before commit")
	}

	ie.Commit(IdempotentCommitInput{
		Kind:        IdempotentOpMessage,
		UserID:      "u1",
		CharacterID: "c1",
		OperationID: "op",
		Status:      "active",
	})

	if !ie.IsCommitted(key) {
		t.Fatal("expected committed after commit")
	}
}

func TestIdempotentExecutorGetRecord(t *testing.T) {
	ie := NewIdempotentExecutor()

	ie.Commit(IdempotentCommitInput{
		Kind:        IdempotentOpReflection,
		UserID:      "u1",
		CharacterID: "c1",
		OperationID: "reflect-1",
		StateVersion: 7,
		Status:      "active",
	})

	key := BuildIdempotencyKey(IdempotentOpReflection, "u1", "c1", "reflect-1")
	record, ok := ie.GetRecord(key)
	if !ok {
		t.Fatal("expected record to exist")
	}
	if record.Kind != IdempotentOpReflection || record.StateVersion != 7 {
		t.Fatalf("unexpected record: kind=%s version=%d", record.Kind, record.StateVersion)
	}
}

func TestIdempotentExecutorGetRecordNotFound(t *testing.T) {
	ie := NewIdempotentExecutor()
	_, ok := ie.GetRecord("nonexistent-key")
	if ok {
		t.Fatal("expected no record for nonexistent key")
	}
}

func TestSupersededAuditLogMaxSize(t *testing.T) {
	sal := NewSupersededAuditLog(3)
	for i := 0; i < 5; i++ {
		sal.Record("task-"+string(rune('a'+i)), "u1", "c1", "superseded", "", 1, IdempotentOpReflection)
	}
	records := sal.RecordsByCharacter("c1")
	if len(records) > 3 {
		t.Fatalf("expected max 3 audit records, got %d", len(records))
	}
}

func TestSupersededAuditLogRecordsByCharacterEmpty(t *testing.T) {
	sal := NewSupersededAuditLog(10)
	sal.Record("task-1", "u1", "c1", "superseded", "", 1, IdempotentOpMessage)
	records := sal.RecordsByCharacter("c2")
	if len(records) != 0 {
		t.Fatalf("expected no records for character c2, got %d", len(records))
	}
}

func TestIdempotentExecutorReset(t *testing.T) {
	ie := NewIdempotentExecutor()

	ie.Commit(IdempotentCommitInput{
		Kind:        IdempotentOpIndex,
		UserID:      "u1",
		CharacterID: "c1",
		OperationID: "idx-1",
		Status:      "active",
	})

	key := BuildIdempotencyKey(IdempotentOpIndex, "u1", "c1", "idx-1")
	if !ie.IsCommitted(key) {
		t.Fatal("expected committed before reset")
	}

	ie.Reset()

	if ie.IsCommitted(key) {
		t.Fatal("expected not committed after reset")
	}
}

func TestIdempotentExecutorParallelSubmits(t *testing.T) {
	ie := NewIdempotentExecutor()
	var wg sync.WaitGroup
	results := make([]IdempotentCommitResult, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = ie.Commit(IdempotentCommitInput{
				Kind:        IdempotentOpMessage,
				UserID:      "u1",
				CharacterID: "c1",
				OperationID: "parallel",
				Status:      "active",
			})
		}(i)
	}
	wg.Wait()

	committed := 0
	duplicates := 0
	for _, r := range results {
		if r.Status == CommitCommitted {
			committed++
		} else if r.Status == CommitDuplicate {
			duplicates++
		}
	}
	if committed != 1 || duplicates != 4 {
		t.Fatalf("expected 1 committed and 4 duplicates, got committed=%d duplicates=%d", committed, duplicates)
	}
}