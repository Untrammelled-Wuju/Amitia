package javascript_main

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

type HandlerType string

const (
	HandlerTypeTool    HandlerType = "tool"
	HandlerTypeEvent   HandlerType = "event"
	HandlerTypeHook    HandlerType = "hook"
	HandlerTypeProvider HandlerType = "provider"
	HandlerTypeSchedule HandlerType = "schedule"
)

type HandlerBinding struct {
	HandlerType HandlerType
	EntryName   string
	BoundAt     time.Time
}

type HandlerFunc func(input interface{}, ctx InvocationContext) (interface{}, error)

type HandlerRegistry struct {
	mu           sync.RWMutex
	allowed      map[string]AllowedContribution
	bound        map[string]HandlerBinding
	handlers     map[string]HandlerFunc
}

func NewHandlerRegistry(allowed []AllowedContribution) *HandlerRegistry {
	r := &HandlerRegistry{
		allowed:  make(map[string]AllowedContribution),
		bound:    make(map[string]HandlerBinding),
		handlers: make(map[string]HandlerFunc),
	}
	for _, c := range allowed {
		key := handlerKey(c.EntryType, c.EntryName)
		r.allowed[key] = c
	}
	return r
}

func handlerKey(handlerType, entryName string) string {
	return fmt.Sprintf("%s:%s", handlerType, entryName)
}

func (r *HandlerRegistry) Bind(handlerType HandlerType, entryName string, handler HandlerFunc) error {
	if handlerType == "" {
		return errors.New("javascript_main: handler type required")
	}
	if entryName == "" {
		return errors.New("javascript_main: entry name required")
	}
	if handler == nil {
		return errors.New("javascript_main: handler required")
	}
	key := handlerKey(string(handlerType), entryName)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, allowed := r.allowed[key]; !allowed {
		return fmt.Errorf("javascript_main: entry_not_declared %s", key)
	}
	if _, exists := r.bound[key]; exists {
		return fmt.Errorf("javascript_main: handler already bound for %s", key)
	}
	r.bound[key] = HandlerBinding{
		HandlerType: handlerType,
		EntryName:   entryName,
		BoundAt:     time.Now().UTC(),
	}
	r.handlers[key] = handler
	return nil
}

func (r *HandlerRegistry) Unbind(handlerType HandlerType, entryName string) error {
	key := handlerKey(string(handlerType), entryName)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.bound[key]; !exists {
		return fmt.Errorf("javascript_main: handler not bound for %s", key)
	}
	delete(r.bound, key)
	delete(r.handlers, key)
	return nil
}

func (r *HandlerRegistry) Get(handlerType HandlerType, entryName string) (HandlerFunc, error) {
	key := handlerKey(string(handlerType), entryName)
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, exists := r.handlers[key]
	if !exists {
		return nil, fmt.Errorf("javascript_main: handler not found for %s", key)
	}
	return handler, nil
}

func (r *HandlerRegistry) IsBound(handlerType HandlerType, entryName string) bool {
	key := handlerKey(string(handlerType), entryName)
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.bound[key]
	return exists
}

func (r *HandlerRegistry) ListBindings() []HandlerBinding {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]HandlerBinding, 0, len(r.bound))
	for _, b := range r.bound {
		result = append(result, b)
	}
	return result
}

func (r *HandlerRegistry) VerifyCompleteness() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var missing []string
	for key := range r.allowed {
		if _, bound := r.bound[key]; !bound {
			missing = append(missing, key)
		}
	}
	return missing
}

func (r *HandlerRegistry) BoundCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.bound)
}

func (r *HandlerRegistry) AllowedCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.allowed)
}
