package delivery

import (
	"fmt"
	"sync"
)

type ChannelResolver interface {
	Resolve(channelName string) ChannelAdapter
	Register(adapter ChannelAdapter)
	Channels() []string
	Has(channelName string) bool
}

type MapChannelResolver struct {
	mu       sync.RWMutex
	adapters map[string]ChannelAdapter
}

func NewMapChannelResolver() *MapChannelResolver {
	return &MapChannelResolver{
		adapters: make(map[string]ChannelAdapter),
	}
}

func NewMapChannelResolverWith(adapters []ChannelAdapter) *MapChannelResolver {
	r := NewMapChannelResolver()
	for _, a := range adapters {
		r.Register(a)
	}
	return r
}

func (r *MapChannelResolver) Register(adapter ChannelAdapter) {
	if adapter == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[adapter.Name()] = adapter
}

func (r *MapChannelResolver) Resolve(channelName string) ChannelAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.adapters[channelName]
}

func (r *MapChannelResolver) Channels() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	channels := make([]string, 0, len(r.adapters))
	for name := range r.adapters {
		channels = append(channels, name)
	}
	return channels
}

func (r *MapChannelResolver) Unregister(channelName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.adapters, channelName)
}

func (r *MapChannelResolver) Has(channelName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.adapters[channelName]
	return ok
}

var ErrChannelNotResolved = fmt.Errorf("channel adapter not resolved")
