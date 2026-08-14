package control

import (
	"fmt"
	"sync"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type ControlSinkDescriptor struct {
	SinkID      string
	RuntimeID   domain.RuntimeInstanceID
	PluginID    domain.PluginID
	ServiceID   domain.ServiceID
	Kind        ControlOutputKind
	Generation  uint64
}

type ControlSinkRegistry struct {
	mu    sync.RWMutex
	sinks map[string]ControlSinkDescriptor
}

func NewControlSinkRegistry() *ControlSinkRegistry {
	return &ControlSinkRegistry{
		sinks: make(map[string]ControlSinkDescriptor),
	}
}

func (r *ControlSinkRegistry) Register(sink ControlSinkDescriptor) error {
	if sink.SinkID == "" {
		return fmt.Errorf("sink id must not be empty")
	}
	if sink.RuntimeID == "" {
		return fmt.Errorf("runtime id must not be empty")
	}
	if sink.PluginID == "" {
		return fmt.Errorf("plugin id must not be empty")
	}
	if !IsValidPublicOutputKind(sink.Kind) {
		return fmt.Errorf("invalid output kind: %s", sink.Kind)
	}
	if sink.Generation == 0 {
		return fmt.Errorf("generation must not be zero")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := r.sinkKey(sink.RuntimeID, sink.ServiceID, sink.SinkID)
	if _, exists := r.sinks[key]; exists {
		return fmt.Errorf("sink already registered: %s", key)
	}
	r.sinks[key] = sink
	return nil
}

func (r *ControlSinkRegistry) Resolve(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, sinkID string) (ControlSinkDescriptor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key := r.sinkKey(runtimeID, serviceID, sinkID)
	sink, ok := r.sinks[key]
	return sink, ok
}

func (r *ControlSinkRegistry) Remove(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, sinkID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := r.sinkKey(runtimeID, serviceID, sinkID)
	delete(r.sinks, key)
}

func (r *ControlSinkRegistry) RemoveByRuntime(runtimeID domain.RuntimeInstanceID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, sink := range r.sinks {
		if sink.RuntimeID == runtimeID {
			delete(r.sinks, key)
		}
	}
}

func (r *ControlSinkRegistry) RemoveByGeneration(runtimeID domain.RuntimeInstanceID, generation uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, sink := range r.sinks {
		if sink.RuntimeID == runtimeID && sink.Generation != generation {
			delete(r.sinks, key)
		}
	}
}

func (r *ControlSinkRegistry) ListByRuntime(runtimeID domain.RuntimeInstanceID) []ControlSinkDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]ControlSinkDescriptor, 0)
	for _, sink := range r.sinks {
		if sink.RuntimeID == runtimeID {
			result = append(result, sink)
		}
	}
	return result
}

func (r *ControlSinkRegistry) sinkKey(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, sinkID string) string {
	return string(runtimeID) + "/" + string(serviceID) + "/" + sinkID
}
