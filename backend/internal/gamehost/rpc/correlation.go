package rpc

import (
	"sync"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
)

type Correlation struct {
	Upstream        RequestKey
	UpstreamPeer    ipc.Peer
	DownstreamPeer  ipc.Peer
	DownstreamReqID string
	CreatedAt       time.Time
}

type CorrelationMap struct {
	mu           sync.RWMutex
	byUpstream   map[RequestKey]*Correlation
	byDownstream map[DownstreamKey]*Correlation
}

type DownstreamKey struct {
	RuntimeID domain.RuntimeInstanceID
	ServiceID domain.ServiceID
	RequestID string
}

func NewCorrelationMap() *CorrelationMap {
	return &CorrelationMap{
		byUpstream:   make(map[RequestKey]*Correlation),
		byDownstream: make(map[DownstreamKey]*Correlation),
	}
}

func (m *CorrelationMap) Add(c *Correlation) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.byUpstream[c.Upstream] = c
	m.byDownstream[DownstreamKey{
		RuntimeID: domain.RuntimeInstanceID(c.DownstreamPeer.RuntimeID),
		ServiceID: domain.ServiceID(c.DownstreamPeer.ServiceID),
		RequestID: c.DownstreamReqID,
	}] = c
}

func (m *CorrelationMap) ByUpstream(key RequestKey) (*Correlation, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	c, ok := m.byUpstream[key]
	if !ok {
		return nil, false
	}
	return c, true
}

func (m *CorrelationMap) ByDownstream(peer ipc.Peer, requestID string) (*Correlation, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	c, ok := m.byDownstream[DownstreamKey{
		RuntimeID: domain.RuntimeInstanceID(peer.RuntimeID),
		ServiceID: domain.ServiceID(peer.ServiceID),
		RequestID: requestID,
	}]
	if !ok {
		return nil, false
	}
	return c, true
}

func (m *CorrelationMap) Remove(key RequestKey) {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.byUpstream[key]
	if !ok {
		return
	}

	delete(m.byUpstream, key)
	delete(m.byDownstream, DownstreamKey{
		RuntimeID: domain.RuntimeInstanceID(c.DownstreamPeer.RuntimeID),
		ServiceID: domain.ServiceID(c.DownstreamPeer.ServiceID),
		RequestID: c.DownstreamReqID,
	})
}

func (m *CorrelationMap) RemoveByDownstream(peer ipc.Peer, requestID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	dk := DownstreamKey{
		RuntimeID: domain.RuntimeInstanceID(peer.RuntimeID),
		ServiceID: domain.ServiceID(peer.ServiceID),
		RequestID: requestID,
	}
	c, ok := m.byDownstream[dk]
	if !ok {
		return
	}

	delete(m.byDownstream, dk)
	delete(m.byUpstream, c.Upstream)
}

func (m *CorrelationMap) RemoveByRuntime(runtimeID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for k, c := range m.byUpstream {
		if string(k.RuntimeID) == runtimeID {
			delete(m.byUpstream, k)
			delete(m.byDownstream, DownstreamKey{
				RuntimeID: domain.RuntimeInstanceID(c.DownstreamPeer.RuntimeID),
				ServiceID: domain.ServiceID(c.DownstreamPeer.ServiceID),
				RequestID: c.DownstreamReqID,
			})
			count++
		}
	}
	return count
}

func (m *CorrelationMap) RemoveByService(runtimeID, serviceID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for k, c := range m.byUpstream {
		if string(k.RuntimeID) == runtimeID && string(k.ServiceID) == serviceID {
			delete(m.byUpstream, k)
			delete(m.byDownstream, DownstreamKey{
				RuntimeID: domain.RuntimeInstanceID(c.DownstreamPeer.RuntimeID),
				ServiceID: domain.ServiceID(c.DownstreamPeer.ServiceID),
				RequestID: c.DownstreamReqID,
			})
			count++
		}
	}
	return count
}

func (m *CorrelationMap) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.byUpstream)
}
