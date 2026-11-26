package capability

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

const SchemaRefPrefix = "sha256:"

const DefaultSchemaTTL = 24 * time.Hour

var (
	ErrSchemaBodyInvalid  = errors.New("能力索引：Schema 内容为空或不是合法 JSON")
	ErrSchemaRefInvalid   = errors.New("能力索引：schemaRef 不是 sha256 内容地址")
	ErrSchemaRefMissing   = errors.New("能力索引：schemaRef 在存储中不存在")
	ErrSchemaRefMismatch  = errors.New("能力索引：写入内容与 schemaRef 的哈希不一致")
	ErrSchemaRefImmutable = errors.New("能力索引：历史 schemaRef 不可被覆写")
	ErrSchemaExpired      = errors.New("能力索引：schemaRef 已超过 TTL，需要重新索引")
	ErrDescriptorInvalid  = errors.New("能力索引：描述符缺少必填字段")
	ErrDescriptorUnknown  = errors.New("能力索引：请求的工具不在当前索引中")
)

type SchemaRef string

func NewSchemaRef(body []byte) SchemaRef {
	sum := sha256.Sum256(body)
	return SchemaRef(SchemaRefPrefix + hex.EncodeToString(sum[:]))
}

func (r SchemaRef) String() string {
	return string(r)
}

func (r SchemaRef) Valid() bool {
	text := string(r)
	if len(text) != len(SchemaRefPrefix)+64 || text[:len(SchemaRefPrefix)] != SchemaRefPrefix {
		return false
	}
	_, err := hex.DecodeString(text[len(SchemaRefPrefix):])
	return err == nil
}

type schemaBlob struct {
	body      []byte
	createdAt time.Time
	expiresAt time.Time
}

type SchemaStore struct {
	mu     sync.RWMutex
	ttl    time.Duration
	now    func() time.Time
	blobs  map[SchemaRef]schemaBlob
	reads  atomic.Int64
	writes atomic.Int64
}

func NewSchemaStore(ttl time.Duration) *SchemaStore {
	return NewSchemaStoreWithClock(ttl, time.Now)
}

func NewSchemaStoreWithClock(ttl time.Duration, now func() time.Time) *SchemaStore {
	if ttl <= 0 {
		ttl = DefaultSchemaTTL
	}
	if now == nil {
		now = time.Now
	}
	return &SchemaStore{
		ttl:   ttl,
		now:   now,
		blobs: map[SchemaRef]schemaBlob{},
	}
}

func (s *SchemaStore) TTL() time.Duration {
	return s.ttl
}

func (s *SchemaStore) Reads() int64 {
	return s.reads.Load()
}

func (s *SchemaStore) Writes() int64 {
	return s.writes.Load()
}

func (s *SchemaStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.blobs)
}

func validateSchemaBody(body []byte) error {
	if len(body) == 0 || !json.Valid(body) {
		return ErrSchemaBodyInvalid
	}
	return nil
}

func (s *SchemaStore) Put(body []byte) (SchemaRef, bool, error) {
	if err := validateSchemaBody(body); err != nil {
		return "", false, err
	}
	ref := NewSchemaRef(body)
	created, err := s.PutAt(ref, body)
	if err != nil {
		return "", false, err
	}
	return ref, created, nil
}

func (s *SchemaStore) PutAt(ref SchemaRef, body []byte) (bool, error) {
	if !ref.Valid() {
		return false, fmt.Errorf("%w: %q", ErrSchemaRefInvalid, ref)
	}
	if err := validateSchemaBody(body); err != nil {
		return false, err
	}
	if NewSchemaRef(body) != ref {
		return false, fmt.Errorf("%w: ref=%s", ErrSchemaRefMismatch, ref)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now().UTC()
	if existing, ok := s.blobs[ref]; ok {
		if string(existing.body) != string(body) {
			return false, fmt.Errorf("%w: ref=%s", ErrSchemaRefImmutable, ref)
		}
		return false, nil
	}
	copied := make([]byte, len(body))
	copy(copied, body)
	s.blobs[ref] = schemaBlob{
		body:      copied,
		createdAt: now,
		expiresAt: now.Add(s.ttl),
	}
	s.writes.Add(1)
	return true, nil
}

func (s *SchemaStore) Get(ref SchemaRef) ([]byte, bool) {
	s.reads.Add(1)
	s.mu.RLock()
	defer s.mu.RUnlock()
	blob, ok := s.blobs[ref]
	if !ok {
		return nil, false
	}
	copied := make([]byte, len(blob.body))
	copy(copied, blob.body)
	return copied, true
}

func (s *SchemaStore) Fresh(ref SchemaRef, at time.Time) (bool, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	blob, ok := s.blobs[ref]
	if !ok {
		return false, false
	}
	return !at.UTC().After(blob.expiresAt), true
}

func (s *SchemaStore) RefTTL(ref SchemaRef) (time.Time, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	blob, ok := s.blobs[ref]
	if !ok {
		return time.Time{}, false
	}
	return blob.expiresAt, true
}

func (s *SchemaStore) Load(ref SchemaRef, requireFresh bool) ([]byte, error) {
	if !ref.Valid() {
		return nil, fmt.Errorf("%w: %q", ErrSchemaRefInvalid, ref)
	}
	body, ok := s.Get(ref)
	if !ok {
		return nil, fmt.Errorf("%w: ref=%s", ErrSchemaRefMissing, ref)
	}
	if requireFresh {
		if fresh, _ := s.Fresh(ref, s.now()); !fresh {
			return nil, fmt.Errorf("%w: ref=%s", ErrSchemaExpired, ref)
		}
	}
	return body, nil
}
