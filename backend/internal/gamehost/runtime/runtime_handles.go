package runtime

import (
	"sync"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type RuntimeHandleStore struct {
	mu      sync.Mutex
	handles map[domain.RuntimeInstanceID]map[domain.ServiceID]*ServiceExecutionHandle
}

func NewRuntimeHandleStore() *RuntimeHandleStore {
	return &RuntimeHandleStore{
		handles: make(map[domain.RuntimeInstanceID]map[domain.ServiceID]*ServiceExecutionHandle),
	}
}

func (s *RuntimeHandleStore) Put(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, handle *ServiceExecutionHandle) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.handles[runtimeID]; !ok {
		s.handles[runtimeID] = make(map[domain.ServiceID]*ServiceExecutionHandle)
	}
	s.handles[runtimeID][serviceID] = handle
}

func (s *RuntimeHandleStore) Get(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (*ServiceExecutionHandle, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	runtimeHandles, ok := s.handles[runtimeID]
	if !ok {
		return nil, false
	}
	handle, ok := runtimeHandles[serviceID]
	return handle, ok
}

func (s *RuntimeHandleStore) Remove(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if runtimeHandles, ok := s.handles[runtimeID]; ok {
		delete(runtimeHandles, serviceID)
		if len(runtimeHandles) == 0 {
			delete(s.handles, runtimeID)
		}
	}
}

func (s *RuntimeHandleStore) RemoveRuntime(runtimeID domain.RuntimeInstanceID) []*ServiceExecutionHandle {
	s.mu.Lock()
	defer s.mu.Unlock()

	runtimeHandles, ok := s.handles[runtimeID]
	if !ok {
		return nil
	}
	handles := make([]*ServiceExecutionHandle, 0, len(runtimeHandles))
	for _, h := range runtimeHandles {
		handles = append(handles, h)
	}
	delete(s.handles, runtimeID)
	return handles
}

func (s *RuntimeHandleStore) ListByRuntime(runtimeID domain.RuntimeInstanceID) []*ServiceExecutionHandle {
	s.mu.Lock()
	defer s.mu.Unlock()

	runtimeHandles, ok := s.handles[runtimeID]
	if !ok {
		return nil
	}
	handles := make([]*ServiceExecutionHandle, 0, len(runtimeHandles))
	for _, h := range runtimeHandles {
		handles = append(handles, h)
	}
	return handles
}

func (s *RuntimeHandleStore) ListAll() map[domain.RuntimeInstanceID][]*ServiceExecutionHandle {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make(map[domain.RuntimeInstanceID][]*ServiceExecutionHandle)
	for runtimeID, runtimeHandles := range s.handles {
		handles := make([]*ServiceExecutionHandle, 0, len(runtimeHandles))
		for _, h := range runtimeHandles {
			handles = append(handles, h)
		}
		result[runtimeID] = handles
	}
	return result
}

func (s *RuntimeHandleStore) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, runtimeHandles := range s.handles {
		count += len(runtimeHandles)
	}
	return count
}

func (s *RuntimeHandleStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handles = make(map[domain.RuntimeInstanceID]map[domain.ServiceID]*ServiceExecutionHandle)
}
