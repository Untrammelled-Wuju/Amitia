package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type ScopeType string

const (
	ScopeGlobal       ScopeType = "global"
	ScopeCharacter    ScopeType = "character"
	ScopeConversation ScopeType = "conversation"
	ScopeExtension    ScopeType = "extension"
	ScopeModule       ScopeType = "module"
	ScopeRuntime      ScopeType = "runtime"
)

type ResourceOwner struct {
	ExtensionID string
	ModuleID    string
	RuntimeID   string
}

func (o ResourceOwner) String() string {
	if o.ModuleID != "" {
		return fmt.Sprintf("%s/%s", o.ExtensionID, o.ModuleID)
	}
	return o.ExtensionID
}

type GetRequest struct {
	Owner       ResourceOwner
	Scope       ScopeType
	ScopeID     string
	Namespace   string
	Key         string
	Consistency string
}

type CASRequest struct {
	Owner     ResourceOwner
	Scope     ScopeType
	ScopeID   string
	Namespace string
	Key       string
	Compare   *ValuePredicate
	Set       json.RawMessage
	TTL       time.Duration
}

type ValuePredicate struct {
	Version int64
	Hash    string
	Exists  *bool
}

type DeleteRequest struct {
	Owner     ResourceOwner
	Scope     ScopeType
	ScopeID   string
	Namespace string
	Key       string
	Version   int64
}

type ListRequest struct {
	Owner     ResourceOwner
	Scope     ScopeType
	ScopeID   string
	Namespace string
	Prefix    string
	PageSize  int
	Cursor    string
}

type Page struct {
	Items      []Value
	NextCursor string
}

type Value struct {
	Key       string
	Version   int64
	Value     json.RawMessage
	Hash      string
	SizeBytes int64
	UpdatedAt time.Time
	ExpiresAt *time.Time
}

type TxOpKind string

const (
	TxOpSet    TxOpKind = "set"
	TxOpDelete TxOpKind = "delete"
)

type TxOp struct {
	Kind      TxOpKind
	Owner     ResourceOwner
	Scope     ScopeType
	ScopeID   string
	Namespace string
	Key       string
	Value     json.RawMessage
	Version   int64
}

type TxRequest struct {
	Ops []TxOp
}

type TxResult struct {
	Applied bool
	Values  []Value
	Error   string
}

type Usage struct {
	Owner      ResourceOwner
	Keys       int
	TotalBytes int64
	QuotaBytes int64
	QuotaKeys  int64
}

type Broker interface {
	Get(ctx context.Context, request GetRequest) (Value, error)
	CompareAndSwap(ctx context.Context, request CASRequest) (Value, error)
	Delete(ctx context.Context, request DeleteRequest) error
	List(ctx context.Context, request ListRequest) (Page, error)
	Transaction(ctx context.Context, request TxRequest) TxResult
	Usage(ctx context.Context, owner ResourceOwner) Usage
	SetQuota(owner ResourceOwner, maxBytes, maxKeys int64)
}

var (
	ErrKeyNotFound      = errors.New("storage: key not found")
	ErrCASConflict      = errors.New("storage: cas conflict")
	ErrQuotaExceeded    = errors.New("storage: quota exceeded")
	ErrInvalidNamespace = errors.New("storage: invalid namespace")
	ErrInvalidOwner     = errors.New("storage: invalid owner")
	ErrTxFailed         = errors.New("storage: transaction failed")
	ErrScopeConflict    = errors.New("storage: scope conflict")
)

type storageKey struct {
	Owner     string
	Scope     ScopeType
	ScopeID   string
	Namespace string
	Key       string
}

func (k storageKey) String() string {
	return fmt.Sprintf("extensions/%s/%s/%s/%s/%s", k.Owner, k.Scope, k.ScopeID, k.Namespace, k.Key)
}

type DefaultBroker struct {
	mu     sync.RWMutex
	data   map[storageKey]Value
	quotas map[string]Usage
}

func NewDefaultBroker() *DefaultBroker {
	return &DefaultBroker{
		data:   make(map[storageKey]Value),
		quotas: make(map[string]Usage),
	}
}

func (b *DefaultBroker) Get(_ context.Context, request GetRequest) (Value, error) {
	if err := validateRequest(request.Owner, request.Scope, request.Namespace, request.Key); err != nil {
		return Value{}, err
	}
	k := storageKey{
		Owner:     request.Owner.String(),
		Scope:     request.Scope,
		ScopeID:   request.ScopeID,
		Namespace: request.Namespace,
		Key:       request.Key,
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	v, ok := b.data[k]
	if !ok {
		return Value{}, ErrKeyNotFound
	}
	if v.ExpiresAt != nil && time.Now().UTC().After(*v.ExpiresAt) {
		return Value{}, ErrKeyNotFound
	}
	return v, nil
}

func (b *DefaultBroker) CompareAndSwap(_ context.Context, request CASRequest) (Value, error) {
	if err := validateRequest(request.Owner, request.Scope, request.Namespace, request.Key); err != nil {
		return Value{}, err
	}
	k := storageKey{
		Owner:     request.Owner.String(),
		Scope:     request.Scope,
		ScopeID:   request.ScopeID,
		Namespace: request.Namespace,
		Key:       request.Key,
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	current, exists := b.data[k]
	if request.Compare != nil {
		if request.Compare.Version != 0 && (!exists || current.Version != request.Compare.Version) {
			return Value{}, ErrCASConflict
		}
		if request.Compare.Hash != "" && (!exists || current.Hash != request.Compare.Hash) {
			return Value{}, ErrCASConflict
		}
		if request.Compare.Exists != nil {
			wantExists := *request.Compare.Exists
			if wantExists != exists {
				return Value{}, ErrCASConflict
			}
		}
	}
	if err := b.checkQuotaLocked(request.Owner, int64(len(request.Set))); err != nil {
		return Value{}, err
	}
	newVersion := int64(1)
	if exists {
		newVersion = current.Version + 1
	}
	now := time.Now().UTC()
	value := Value{
		Key:       request.Key,
		Version:   newVersion,
		Value:     request.Set,
		Hash:      hashBytes(request.Set),
		SizeBytes: int64(len(request.Set)),
		UpdatedAt: now,
	}
	if request.TTL > 0 {
		expires := now.Add(request.TTL)
		value.ExpiresAt = &expires
	}
	b.data[k] = value
	b.updateUsageLocked(request.Owner)
	return value, nil
}

func (b *DefaultBroker) Delete(_ context.Context, request DeleteRequest) error {
	if err := validateRequest(request.Owner, request.Scope, request.Namespace, request.Key); err != nil {
		return err
	}
	k := storageKey{
		Owner:     request.Owner.String(),
		Scope:     request.Scope,
		ScopeID:   request.ScopeID,
		Namespace: request.Namespace,
		Key:       request.Key,
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if request.Version != 0 {
		v, ok := b.data[k]
		if !ok || v.Version != request.Version {
			return ErrCASConflict
		}
	}
	delete(b.data, k)
	b.updateUsageLocked(request.Owner)
	return nil
}

func (b *DefaultBroker) List(_ context.Context, request ListRequest) (Page, error) {
	if request.Owner.ExtensionID == "" {
		return Page{}, ErrInvalidOwner
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	var items []Value
	prefix := fmt.Sprintf("extensions/%s/%s/%s/%s/", request.Owner.String(), request.Scope, request.ScopeID, request.Namespace)
	if request.Prefix != "" {
		prefix += request.Prefix
	}
	for k, v := range b.data {
		if !strings.HasPrefix(k.String(), prefix) {
			continue
		}
		if v.ExpiresAt != nil && time.Now().UTC().After(*v.ExpiresAt) {
			continue
		}
		items = append(items, v)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Key < items[j].Key
	})
	pageSize := request.PageSize
	if pageSize <= 0 {
		pageSize = len(items)
	}
	cursor := 0
	if request.Cursor != "" {
		fmt.Sscanf(request.Cursor, "%d", &cursor)
	}
	if cursor >= len(items) {
		return Page{Items: []Value{}}, nil
	}
	end := cursor + pageSize
	if end > len(items) {
		end = len(items)
	}
	page := Page{Items: items[cursor:end]}
	if end < len(items) {
		page.NextCursor = fmt.Sprintf("%d", end)
	}
	return page, nil
}

func (b *DefaultBroker) Transaction(_ context.Context, request TxRequest) TxResult {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := TxResult{Applied: true}
	var backup []struct {
		k  storageKey
		v  Value
		ok bool
	}
	for _, op := range request.Ops {
		if err := validateRequest(op.Owner, op.Scope, op.Namespace, op.Key); err != nil {
			return TxResult{Applied: false, Error: err.Error()}
		}
		k := storageKey{
			Owner:     op.Owner.String(),
			Scope:     op.Scope,
			ScopeID:   op.ScopeID,
			Namespace: op.Namespace,
			Key:       op.Key,
		}
		current, exists := b.data[k]
		backup = append(backup, struct {
			k  storageKey
			v  Value
			ok bool
		}{k, current, exists})
		switch op.Kind {
		case TxOpSet:
			if err := b.checkQuotaLocked(op.Owner, int64(len(op.Value))); err != nil {
				b.rollbackLocked(backup)
				return TxResult{Applied: false, Error: err.Error()}
			}
			newVersion := int64(1)
			if exists {
				newVersion = current.Version + 1
			}
			now := time.Now().UTC()
			value := Value{
				Key:       op.Key,
				Version:   newVersion,
				Value:     op.Value,
				Hash:      hashBytes(op.Value),
				SizeBytes: int64(len(op.Value)),
				UpdatedAt: now,
			}
			b.data[k] = value
			result.Values = append(result.Values, value)
		case TxOpDelete:
			delete(b.data, k)
		}
	}
	for _, op := range request.Ops {
		b.updateUsageLocked(op.Owner)
	}
	return result
}

func (b *DefaultBroker) Usage(_ context.Context, owner ResourceOwner) Usage {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.computeUsageLocked(owner)
}

func (b *DefaultBroker) SetQuota(owner ResourceOwner, maxBytes, maxKeys int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	key := owner.String()
	quota := b.quotas[key]
	quota.Owner = owner
	quota.QuotaBytes = maxBytes
	quota.QuotaKeys = maxKeys
	b.quotas[key] = quota
}

func (b *DefaultBroker) rollbackLocked(backup []struct {
	k  storageKey
	v  Value
	ok bool
}) {
	for _, b_ := range backup {
		if b_.ok {
			b.data[b_.k] = b_.v
		} else {
			delete(b.data, b_.k)
		}
	}
}

func (b *DefaultBroker) checkQuotaLocked(owner ResourceOwner, additional int64) error {
	key := owner.String()
	quota, ok := b.quotas[key]
	if !ok {
		return nil
	}
	if quota.QuotaBytes > 0 {
		usage := b.computeUsageLocked(owner)
		if usage.TotalBytes+additional > quota.QuotaBytes {
			return ErrQuotaExceeded
		}
	}
	if quota.QuotaKeys > 0 {
		usage := b.computeUsageLocked(owner)
		if int64(usage.Keys)+1 > quota.QuotaKeys {
			return ErrQuotaExceeded
		}
	}
	return nil
}

func (b *DefaultBroker) updateUsageLocked(owner ResourceOwner) {
	key := owner.String()
	quota, ok := b.quotas[key]
	if !ok {
		return
	}
	usage := b.computeUsageLocked(owner)
	quota.Keys = usage.Keys
	quota.TotalBytes = usage.TotalBytes
	quota.Owner = owner
	b.quotas[key] = quota
}

func (b *DefaultBroker) computeUsageLocked(owner ResourceOwner) Usage {
	prefix := fmt.Sprintf("extensions/%s/", owner.String())
	var keys int
	var bytes int64
	for k, v := range b.data {
		if !strings.HasPrefix(k.String(), prefix) {
			continue
		}
		keys++
		bytes += v.SizeBytes
	}
	usage := Usage{Owner: owner, Keys: keys, TotalBytes: bytes}
	if quota, ok := b.quotas[owner.String()]; ok {
		usage.QuotaBytes = quota.QuotaBytes
		usage.QuotaKeys = quota.QuotaKeys
	}
	return usage
}

func validateRequest(owner ResourceOwner, scope ScopeType, namespace, key string) error {
	if owner.ExtensionID == "" {
		return ErrInvalidOwner
	}
	if scope == "" {
		return ErrInvalidNamespace
	}
	if namespace == "" {
		return ErrInvalidNamespace
	}
	if key == "" {
		return ErrInvalidNamespace
	}
	if scope == ScopeConversation && owner.ModuleID == "" && namespace == "" {
		return ErrScopeConflict
	}
	return nil
}

func hashBytes(data []byte) string {
	return fmt.Sprintf("%x", data)
}

var _ Broker = (*DefaultBroker)(nil)
