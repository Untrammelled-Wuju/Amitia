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
	ObjectStateWriting   ObjectState = "writing"
	ObjectStateReady     ObjectState = "ready"
	ObjectStateReleasing ObjectState = "releasing"
	ObjectStateReleased  ObjectState = "released"
)

type BinaryObjectRecord struct {
	ID         BinaryObjectID
	Kind       BinaryStorageKind
	Owner      BinaryOwner
	Size       int64
	MediaType  string
	Lifetime   BinaryLifetime
	Checksum   *Checksum
	Metadata   map[string]json.RawMessage
	State      ObjectState
	CreatedAt  time.Time
	ReleasedAt *time.Time

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
	ActiveBytes() int64
	LimitActiveBytes() int64
	CountByRuntime(runtimeID domain.RuntimeInstanceID) int
	ActiveBytesByRuntime(runtimeID domain.RuntimeInstanceID) int64
	RemoveByRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) (int, error)
	RemoveByService(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (int, error)
	GetActiveObjects() []BinaryObjectRecord
}

type Options struct {
	MaxActiveObjects int
	MaxObjectSize    int64
	MaxActiveBytes   int64
}

type OptionsFunc func(*Options)

type memoryObjectRegistry struct {
	mu           sync.RWMutex
	index        map[BinaryObjectID]BinaryObjectRecord
	runtimeIndex map[domain.RuntimeInstanceID]map[BinaryObjectID]struct{}
	serviceIndex map[domain.RuntimeInstanceID]map[domain.ServiceID]map[BinaryObjectID]struct{}
	opts         Options
}

func NewObjectRegistry(opts Options) ObjectRegistry {
	if opts.MaxActiveObjects <= 0 {
		opts.MaxActiveObjects = 1024
	}
	if opts.MaxObjectSize <= 0 {
		opts.MaxObjectSize = 1 << 30
	}
	if opts.MaxActiveBytes <= 0 {
		opts.MaxActiveBytes = 4 << 30
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
	if record.Size < 0 {
		return ErrSizeNegative
	}
	if record.Size > r.opts.MaxObjectSize {
		return ErrSizeTooLarge
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	active := r.countActiveLocked()
	if active >= r.opts.MaxActiveObjects {
		return ErrActiveObjectLimit
	}
	if record.Size > 0 && r.opts.MaxActiveBytes > 0 {
		used := r.activeBytesLocked()
		if used > r.opts.MaxActiveBytes-record.Size {
			return ErrActiveBytesLimit
		}
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
	if r.opts.MaxActiveBytes > 0 {
		usedWithoutCurrent := r.activeBytesLocked() - record.Size
		if usedWithoutCurrent < 0 {
			usedWithoutCurrent = 0
		}
		if actualSize > r.opts.MaxActiveBytes || usedWithoutCurrent > r.opts.MaxActiveBytes-actualSize {
			return ErrActiveBytesLimit
		}
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

	// Provider resources are released before the registry entry reaches this
	// method. Remove the authoritative record instead of retaining an unbounded
	// RELEASED tombstone for every frame/audio chunk ever transferred. Random
	// object IDs make tombstones unnecessary for replay protection, and repeated
	// release remains idempotent because a missing record already returns nil.
	r.removeFromIndicesLocked(record.Owner, id)
	delete(r.index, id)
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

func (r *memoryObjectRegistry) ActiveBytes() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.activeBytesLocked()
}

func (r *memoryObjectRegistry) LimitActiveBytes() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.opts.MaxActiveBytes
}

func (r *memoryObjectRegistry) CountByRuntime(runtimeID domain.RuntimeInstanceID) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for id := range r.runtimeIndex[runtimeID] {
		if record, ok := r.index[id]; ok && record.State != ObjectStateReleased {
			count++
		}
	}
	return count
}

func (r *memoryObjectRegistry) ActiveBytesByRuntime(runtimeID domain.RuntimeInstanceID) int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var total int64
	for id := range r.runtimeIndex[runtimeID] {
		if record, ok := r.index[id]; ok && record.State != ObjectStateReleased && record.Size > 0 {
			total += record.Size
		}
	}
	return total
}

func (r *memoryObjectRegistry) activeBytesLocked() int64 {
	var total int64
	for _, record := range r.index {
		if record.State != ObjectStateReleased && record.Size > 0 {
			total += record.Size
		}
	}
	return total
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
		record, ok := r.index[id]
		if !ok {
			continue
		}
		r.removeFromServiceIndexLocked(record.Owner.RuntimeID, record.Owner.ServiceID, id)
		delete(r.index, id)
	}
	delete(r.runtimeIndex, runtimeID)
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
	// Copy IDs first because removeFromIndicesLocked mutates the same service
	// index map while we remove records. Deleting during map iteration is legal in
	// Go, but the explicit copy keeps cleanup deterministic and easy to audit.
	objectIDs := make([]BinaryObjectID, 0, len(ids))
	for id := range ids {
		objectIDs = append(objectIDs, id)
	}
	for _, id := range objectIDs {
		record, ok := r.index[id]
		if !ok {
			continue
		}
		r.removeFromIndicesLocked(record.Owner, id)
		delete(r.index, id)
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
