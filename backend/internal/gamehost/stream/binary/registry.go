package binary

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type ObjectState string

const (
	ObjectStateWriting  ObjectState = "writing"
	ObjectStateReady    ObjectState = "ready"
	ObjectStateReleasing ObjectState = "releasing"
	ObjectStateReleased  ObjectState = "released"
)

type BinaryObjectRecord struct {
	ID          BinaryObjectID
	Kind        BinaryStorageKind
	Owner       BinaryOwner
	Size        int64
	MediaType   string
	Lifetime    BinaryLifetime
	Checksum    *Checksum
	Metadata    map[string]json.RawMessage
	State       ObjectState
	CreatedAt   time.Time
	ReleasedAt  *time.Time

	Internal interface{}
}

type ObjectRegistry interface {
	InsertWriting(ctx context.Context, record BinaryObjectRecord) error
	SealObject(ctx context.Context, id BinaryObjectID, actualSize int64, checksum *Checksum) error
	Get(ctx context.Context, id BinaryObjectID) (BinaryObjectRecord, error)
	Release(ctx context.Context, id BinaryObjectID) error
	ListByRuntime(runtimeID domain.RuntimeInstanceID) ([]BinaryObjectRecord, error)
	ListByService(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (map[BinaryObjectID]BinaryObjectRecord, error)
	CountActive() int
	LimitActive() int
	RemoveByRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) (int, error)
	RemoveByService(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (int, error)
	GetActiveObjects() []BinaryObjectRecord
}

type Options struct {
	MaxActiveObjects   int
	MaxObjectSize      int64
}

type OptionsFunc func(*Options)

type memoryObjectRegistry struct {
	mu            sync.RWMutex
	index         map[BinaryObjectID]BinaryObjectRecord
	runtimeIndex  map[domain.RuntimeInstanceID]map[BinaryObjectID]struct{}
	serviceIndex  map[domain.RuntimeInstanceID]map[domain.ServiceID]map[BinaryObjectID]struct{}
	opts          Options
}

func NewObjectRegistry(opts Options) ObjectRegistry {
	if opts.MaxActiveObjects <= 0 {
		opts.MaxActiveObjects = 1024
	}
	if opts.MaxObjectSize <= 0 {
		opts.MaxObjectSize = 1 << 30
	}
	return &memoryObjectRegistry{
		index:        make(map[BinaryObjectID]BinaryObjectRecord),
		runtimeIndex: make(map[domain.RuntimeInstanceID]map[BinaryObjectID]struct{}),
		serviceIndex: make(map[domain.RuntimeInstanceID]map[domain.ServiceID]map[BinaryObjectID]struct{}),
		opts:         opts,
	}
}

func (r *memoryObjectRegistry) InsertWriting(ctx context.Context, record BinaryObjectRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := record.Owner.Validate(); err != nil {
		return err
	}
	if record.ID.IsEmpty() {
		return ErrIDEmpty
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	active := r.countActiveLocked()
	if active >= r.opts.MaxActiveObjects {
		return ErrActiveObjectLimit
	}

	now := time.Now().UTC()
	record.State = ObjectStateWriting
	record.CreatedAt = now
	if record.Metadata == nil {
		record.Metadata = nil
	}

	r.index[record.ID] = record
	r.addToIndicesLocked(record.Owner, record.ID)
	return nil
}

func (r *memoryObjectRegistry) SealObject(ctx context.Context, id BinaryObjectID, actualSize int64, checksum *Checksum) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if actualSize < 0 {
		return ErrSizeNegative
	}
	if actualSize > r.opts.MaxObjectSize {
		return ErrSizeTooLarge
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	record, ok := r.index[id]
	if !ok {
		return ErrObjectNotFound
	}
	if record.State != ObjectStateWriting {
		return ErrObjectNotReady
	}

	record.State = ObjectStateReady
	record.Size = actualSize
	record.Checksum = checksum
	r.index[id] = record
	return nil
}

func (r *memoryObjectRegistry) Get(ctx context.Context, id BinaryObjectID) (BinaryObjectRecord, error) {
	if err := ctx.Err(); err != nil {
		return BinaryObjectRecord{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.getLocked(id)
}

func (r *memoryObjectRegistry) getLocked(id BinaryObjectID) (BinaryObjectRecord, error) {
	record, ok := r.index[id]
	if !ok {
		return BinaryObjectRecord{}, ErrObjectNotFound
	}
	if record.State == ObjectStateReleased {
		return BinaryObjectRecord{}, ErrObjectReleased
	}
	return record, nil
}

func (r *memoryObjectRegistry) Release(ctx context.Context, id BinaryObjectID) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	record, ok := r.index[id]
	if !ok {
		return nil
	}
	if record.State == ObjectStateReleased {
		return nil
	}

	now := time.Now().UTC()
	record.State = ObjectStateReleased
	record.ReleasedAt = &now
	r.index[id] = record

	r.removeFromIndicesLocked(record.Owner, id)
	return nil
}

func (r *memoryObjectRegistry) ListByRuntime(runtimeID domain.RuntimeInstanceID) ([]BinaryObjectRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.runtimeIndex[runtimeID]
	result := make([]BinaryObjectRecord, 0, len(ids))
	for id := range ids {
		if record, ok := r.index[id]; ok {
			result = append(result, record)
		}
	}
	return result, nil
}

func (r *memoryObjectRegistry) ListByService(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (map[BinaryObjectID]BinaryObjectRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.serviceIndex[runtimeID][serviceID]
	result := make(map[BinaryObjectID]BinaryObjectRecord, len(ids))
	for id := range ids {
		if record, ok := r.index[id]; ok {
			result[id] = record
		}
	}
	return result, nil
}

func (r *memoryObjectRegistry) CountActive() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.countActiveLocked()
}

func (r *memoryObjectRegistry) LimitActive() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.opts.MaxActiveObjects
}

func (r *memoryObjectRegistry) countActiveLocked() int {
	count := 0
	for _, record := range r.index {
		if record.State != ObjectStateReleased {
			count++
		}
	}
	return count
}

func (r *memoryObjectRegistry) RemoveByRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	ids := r.runtimeIndex[runtimeID]
	count := len(ids)
	for id := range ids {
		record := r.index[id]
		now := time.Now().UTC()
		record.State = ObjectStateReleased
		record.ReleasedAt = &now
		r.index[id] = record
	}
	delete(r.runtimeIndex, runtimeID)
	for id := range ids {
		record := r.index[id]
		r.removeFromServiceIndexLocked(record.Owner.RuntimeID, record.Owner.ServiceID, id)
	}
	return count, nil
}

func (r *memoryObjectRegistry) removeFromServiceIndexLocked(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, id BinaryObjectID) {
	if si, ok := r.serviceIndex[runtimeID]; ok {
		if ci, ok := si[serviceID]; ok {
			delete(ci, id)
			if len(ci) == 0 {
				delete(si, serviceID)
			}
		}
		if len(si) == 0 {
			delete(r.serviceIndex, runtimeID)
		}
	}
}

func (r *memoryObjectRegistry) RemoveByService(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	ids := r.serviceIndex[runtimeID][serviceID]
	count := len(ids)
	for id := range ids {
		record := r.index[id]
		now := time.Now().UTC()
		record.State = ObjectStateReleased
		record.ReleasedAt = &now
		r.index[id] = record
		r.removeFromIndicesLocked(record.Owner, id)
	}
	return count, nil
}

func (r *memoryObjectRegistry) GetActiveObjects() []BinaryObjectRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]BinaryObjectRecord, 0)
	for _, record := range r.index {
		if record.State == ObjectStateReady {
			result = append(result, record)
		}
	}
	return result
}

func (r *memoryObjectRegistry) addToIndicesLocked(owner BinaryOwner, id BinaryObjectID) {
	if r.runtimeIndex[owner.RuntimeID] == nil {
		r.runtimeIndex[owner.RuntimeID] = make(map[BinaryObjectID]struct{})
	}
	r.runtimeIndex[owner.RuntimeID][id] = struct{}{}

	if r.serviceIndex[owner.RuntimeID] == nil {
		r.serviceIndex[owner.RuntimeID] = make(map[domain.ServiceID]map[BinaryObjectID]struct{})
	}
	if r.serviceIndex[owner.RuntimeID][owner.ServiceID] == nil {
		r.serviceIndex[owner.RuntimeID][owner.ServiceID] = make(map[BinaryObjectID]struct{})
	}
	r.serviceIndex[owner.RuntimeID][owner.ServiceID][id] = struct{}{}
}

func (r *memoryObjectRegistry) removeFromIndicesLocked(owner BinaryOwner, id BinaryObjectID) {
	delete(r.runtimeIndex[owner.RuntimeID], id)
	if len(r.runtimeIndex[owner.RuntimeID]) == 0 {
		delete(r.runtimeIndex, owner.RuntimeID)
	}

	if si, ok := r.serviceIndex[owner.RuntimeID]; ok {
		delete(si[owner.ServiceID], id)
		if len(si[owner.ServiceID]) == 0 {
			delete(si, owner.ServiceID)
		}
		if len(si) == 0 {
			delete(r.serviceIndex, owner.RuntimeID)
		}
	}
}
