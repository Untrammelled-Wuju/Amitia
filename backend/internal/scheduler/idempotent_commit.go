package scheduler

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

type IdempotentOperationKind string

const (
	IdempotentOpMessage    IdempotentOperationKind = "message"
	IdempotentOpEvent      IdempotentOperationKind = "event"
	IdempotentOpTool       IdempotentOperationKind = "tool"
	IdempotentOpOutbox     IdempotentOperationKind = "outbox"
	IdempotentOpSend       IdempotentOperationKind = "send"
	IdempotentOpIndex      IdempotentOperationKind = "index"
	IdempotentOpDelete     IdempotentOperationKind = "delete"
	IdempotentOpReflection IdempotentOperationKind = "reflection"
)

func BuildIdempotencyKey(kind IdempotentOperationKind, userID, characterID, operationID string) string {
	parts := []string{
		string(kind),
		strings.TrimSpace(userID),
		strings.TrimSpace(characterID),
		strings.TrimSpace(operationID),
	}
	raw := strings.Join(parts, "|")
	sum := sha256.Sum256([]byte(raw))
	return "idem-" + string(kind) + "-" + hex.EncodeToString(sum[:])[:24]
}

type CommitStatus string

const (
	CommitPending    CommitStatus = "pending"
	CommitCommitted  CommitStatus = "committed"
	CommitDuplicate  CommitStatus = "duplicate"
	CommitConflict   CommitStatus = "conflict"
	CommitSuperseded CommitStatus = "superseded"
	CommitCancelled  CommitStatus = "cancelled"
)

type IdempotentCommitRecord struct {
	Key             string                   `json:"key"`
	Kind            IdempotentOperationKind  `json:"kind"`
	UserID          string                   `json:"userId"`
	CharacterID     string                   `json:"characterId"`
	StateVersion    int                      `json:"stateVersion"`
	GenerationToken string                   `json:"generationToken,omitempty"`
	CommittedAt     time.Time                `json:"committedAt"`
	Result          interface{}              `json:"result,omitempty"`
	Status          CommitStatus             `json:"status"`
}

type OptimisticLock struct {
	ExpectedVersion int    `json:"expectedVersion"`
	CurrentVersion  int    `json:"currentVersion"`
	Token           string `json:"token"`
}

func NewOptimisticLock(currentVersion int) OptimisticLock {
	return OptimisticLock{
		ExpectedVersion: currentVersion,
		CurrentVersion:  currentVersion,
		Token:           fmt.Sprintf("tok-%d-%d", currentVersion, time.Now().UnixNano()),
	}
}

func (ol OptimisticLock) Matches(actualVersion int) bool {
	return actualVersion == ol.ExpectedVersion
}

func (ol OptimisticLock) Next() OptimisticLock {
	nextVersion := ol.CurrentVersion + 1
	return OptimisticLock{
		ExpectedVersion: nextVersion,
		CurrentVersion:  nextVersion,
		Token:           fmt.Sprintf("tok-%d-%d", nextVersion, time.Now().UnixNano()),
	}
}

type CommitQueue struct {
	mu     sync.Mutex
	queues map[string]chan struct{}
}

func NewCommitQueue() *CommitQueue {
	return &CommitQueue{
		queues: make(map[string]chan struct{}),
	}
}

func commitQueueKey(userID, characterID string) string {
	return "commit:" + strings.TrimSpace(userID) + ":" + strings.TrimSpace(characterID)
}

func (cq *CommitQueue) Serialize(userID, characterID string) func() {
	key := commitQueueKey(userID, characterID)
	cq.mu.Lock()
	ch, exists := cq.queues[key]
	if !exists {
		ch = make(chan struct{}, 1)
		cq.queues[key] = ch
	}
	cq.mu.Unlock()

	ch <- struct{}{}
	return func() {
		<-ch
	}
}

type RetryPolicy struct {
	MaxRetries     int
	BaseDelay      time.Duration
	MaxDelay       time.Duration
	JitterFactor   float64
	CircuitBreaker func() bool
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries:   3,
		BaseDelay:    50 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		JitterFactor: 0.3,
	}
}

func (rp RetryPolicy) ShouldRetry(attempt int, err error) bool {
	if attempt >= rp.MaxRetries {
		return false
	}
	if rp.CircuitBreaker != nil && rp.CircuitBreaker() {
		return false
	}
	return err != nil
}

func (rp RetryPolicy) Delay(attempt int) time.Duration {
	base := rp.BaseDelay * time.Duration(1<<uint(attempt))
	if base > rp.MaxDelay {
		base = rp.MaxDelay
	}
	jitter := time.Duration(float64(base) * rp.JitterFactor * rand.Float64())
	delay := base + jitter
	if delay > rp.MaxDelay {
		delay = rp.MaxDelay
	}
	return delay
}

type CommitValidator struct {
	ExpectedStateVersion int    `json:"expectedStateVersion"`
	ExpectedStatus       string `json:"expectedStatus"`
	GenerationToken      string `json:"generationToken"`
}

func NewCommitValidator(stateVersion int, status string, genToken string) CommitValidator {
	return CommitValidator{
		ExpectedStateVersion: stateVersion,
		ExpectedStatus:       status,
		GenerationToken:      genToken,
	}
}

func (cv CommitValidator) Validate(stateVersion int, status string, genToken string) bool {
	if cv.ExpectedStateVersion > 0 && stateVersion != cv.ExpectedStateVersion {
		return false
	}
	if cv.ExpectedStatus != "" && !strings.EqualFold(strings.TrimSpace(status), cv.ExpectedStatus) {
		return false
	}
	if cv.GenerationToken != "" && genToken != cv.GenerationToken {
		return false
	}
	return true
}

type SupersededAuditRecord struct {
	TaskID        string                  `json:"taskId"`
	UserID        string                  `json:"userId"`
	CharacterID   string                  `json:"characterId"`
	Reason        string                  `json:"reason"`
	SupersededBy  string                  `json:"supersededBy,omitempty"`
	AttemptedAt   time.Time               `json:"attemptedAt"`
	DiscardedAt   time.Time               `json:"discardedAt"`
	StateVersion  int                     `json:"stateVersion"`
	OperationKind IdempotentOperationKind `json:"operationKind"`
}

type SupersededAuditLog struct {
	mu      sync.Mutex
	records []SupersededAuditRecord
	maxSize int
}

func NewSupersededAuditLog(maxSize int) *SupersededAuditLog {
	if maxSize <= 0 {
		maxSize = 5000
	}
	return &SupersededAuditLog{
		maxSize: maxSize,
	}
}

func (sal *SupersededAuditLog) Record(taskID, userID, characterID, reason, supersededBy string, stateVersion int, kind IdempotentOperationKind) {
	sal.mu.Lock()
	defer sal.mu.Unlock()

	record := SupersededAuditRecord{
		TaskID:        strings.TrimSpace(taskID),
		UserID:        strings.TrimSpace(userID),
		CharacterID:   strings.TrimSpace(characterID),
		Reason:        strings.TrimSpace(reason),
		SupersededBy:  strings.TrimSpace(supersededBy),
		AttemptedAt:   time.Now().UTC(),
		DiscardedAt:   time.Now().UTC(),
		StateVersion:  stateVersion,
		OperationKind: kind,
	}

	if len(sal.records) >= sal.maxSize {
		sal.records = sal.records[1:]
	}
	sal.records = append(sal.records, record)
}

func (sal *SupersededAuditLog) RecordsByCharacter(characterID string) []SupersededAuditRecord {
	sal.mu.Lock()
	defer sal.mu.Unlock()

	result := make([]SupersededAuditRecord, 0)
	for _, r := range sal.records {
		if r.CharacterID == characterID {
			result = append(result, r)
		}
	}
	return result
}

type IdempotentExecutor struct {
	mu        sync.Mutex
	committed map[string]IdempotentCommitRecord
	queue     *CommitQueue
	auditLog  *SupersededAuditLog
}

func NewIdempotentExecutor() *IdempotentExecutor {
	return &IdempotentExecutor{
		committed: make(map[string]IdempotentCommitRecord),
		queue:     NewCommitQueue(),
		auditLog:  NewSupersededAuditLog(5000),
	}
}

type IdempotentCommitInput struct {
	Kind            IdempotentOperationKind
	UserID          string
	CharacterID     string
	OperationID     string
	StateVersion    int
	GenerationToken string
	Status          string
	Payload         interface{}
}

type IdempotentCommitResult struct {
	Key       string                 `json:"key"`
	Status    CommitStatus           `json:"status"`
	Duplicate bool                   `json:"duplicate"`
	Record    IdempotentCommitRecord `json:"record"`
}

func (ie *IdempotentExecutor) Commit(input IdempotentCommitInput) IdempotentCommitResult {
	key := BuildIdempotencyKey(input.Kind, input.UserID, input.CharacterID, input.OperationID)

	ie.mu.Lock()
	if existing, ok := ie.committed[key]; ok {
		ie.mu.Unlock()
		return IdempotentCommitResult{
			Key:       key,
			Status:    CommitDuplicate,
			Duplicate: true,
			Record:    existing,
		}
	}

	unlock := ie.queue.Serialize(input.UserID, input.CharacterID)
	ie.mu.Unlock()
	defer unlock()

	ie.mu.Lock()
	defer ie.mu.Unlock()

	if existing, ok := ie.committed[key]; ok {
		return IdempotentCommitResult{
			Key:       key,
			Status:    CommitDuplicate,
			Duplicate: true,
			Record:    existing,
		}
	}

	if strings.EqualFold(strings.TrimSpace(input.Status), "superseded") {
		ie.auditLog.Record(key, input.UserID, input.CharacterID, "superseded", "", input.StateVersion, input.Kind)
		record := IdempotentCommitRecord{
			Key:             key,
			Kind:            input.Kind,
			UserID:          strings.TrimSpace(input.UserID),
			CharacterID:     strings.TrimSpace(input.CharacterID),
			StateVersion:    input.StateVersion,
			GenerationToken: strings.TrimSpace(input.GenerationToken),
			CommittedAt:     time.Now().UTC(),
			Status:          CommitSuperseded,
		}
		ie.committed[key] = record
		return IdempotentCommitResult{Key: key, Status: CommitSuperseded, Record: record}
	}

	if strings.EqualFold(strings.TrimSpace(input.Status), "cancelled") {
		ie.auditLog.Record(key, input.UserID, input.CharacterID, "cancelled", "", input.StateVersion, input.Kind)
		record := IdempotentCommitRecord{
			Key:             key,
			Kind:            input.Kind,
			UserID:          strings.TrimSpace(input.UserID),
			CharacterID:     strings.TrimSpace(input.CharacterID),
			StateVersion:    input.StateVersion,
			GenerationToken: strings.TrimSpace(input.GenerationToken),
			CommittedAt:     time.Now().UTC(),
			Status:          CommitCancelled,
		}
		ie.committed[key] = record
		return IdempotentCommitResult{Key: key, Status: CommitCancelled, Record: record}
	}

	record := IdempotentCommitRecord{
		Key:             key,
		Kind:            input.Kind,
		UserID:          strings.TrimSpace(input.UserID),
		CharacterID:     strings.TrimSpace(input.CharacterID),
		StateVersion:    input.StateVersion,
		GenerationToken: strings.TrimSpace(input.GenerationToken),
		CommittedAt:     time.Now().UTC(),
		Status:          CommitCommitted,
		Result:          input.Payload,
	}
	ie.committed[key] = record

	return IdempotentCommitResult{Key: key, Status: CommitCommitted, Record: record}
}

func (ie *IdempotentExecutor) CommitWithLock(input IdempotentCommitInput, lock OptimisticLock) IdempotentCommitResult {
	validator := NewCommitValidator(lock.ExpectedVersion, input.Status, input.GenerationToken)
	if !validator.Validate(input.StateVersion, input.Status, input.GenerationToken) {
		return IdempotentCommitResult{
			Key:    BuildIdempotencyKey(input.Kind, input.UserID, input.CharacterID, input.OperationID),
			Status: CommitConflict,
		}
	}
	return ie.Commit(input)
}

func (ie *IdempotentExecutor) IsCommitted(key string) bool {
	ie.mu.Lock()
	defer ie.mu.Unlock()
	_, ok := ie.committed[key]
	return ok
}

func (ie *IdempotentExecutor) GetRecord(key string) (IdempotentCommitRecord, bool) {
	ie.mu.Lock()
	defer ie.mu.Unlock()
	record, ok := ie.committed[key]
	return record, ok
}

func (ie *IdempotentExecutor) AuditRecords(characterID string) []SupersededAuditRecord {
	return ie.auditLog.RecordsByCharacter(characterID)
}

func (ie *IdempotentExecutor) Reset() {
	ie.mu.Lock()
	defer ie.mu.Unlock()
	ie.committed = make(map[string]IdempotentCommitRecord)
}
