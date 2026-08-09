package ipc

import (
	"fmt"
	"sync"
)

type ConnectionRegistry struct {
	mu       sync.RWMutex
	byID     map[ConnectionID]*Connection
	byPeer   map[PeerKey]ConnectionID
}

func NewConnectionRegistry() *ConnectionRegistry {
	return &ConnectionRegistry{
		byID:   make(map[ConnectionID]*Connection),
		byPeer: make(map[PeerKey]ConnectionID),
	}
}

func (r *ConnectionRegistry) Register(conn *Connection) error {
	if conn == nil {
		return fmt.Errorf("connection must not be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byID[conn.ID]; exists {
		return fmt.Errorf("connection id already registered: %s", conn.ID)
	}

	key := conn.Peer.Key()
	if existingID, exists := r.byPeer[key]; exists {
		if existing, ok := r.byID[existingID]; ok && existing.IsActive() {
			return fmt.Errorf("active connection already exists for peer %s/%s", key.RuntimeID, key.ServiceID)
		}
		delete(r.byID, existingID)
		delete(r.byPeer, key)
	}

	r.byID[conn.ID] = conn
	r.byPeer[key] = conn.ID
	return nil
}

func (r *ConnectionRegistry) Get(id ConnectionID) (*Connection, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	conn, ok := r.byID[id]
	return conn, ok
}

func (r *ConnectionRegistry) GetByPeer(runtimeID PeerKey) (*Connection, bool) {
	r.mu.RLock()
	id, ok := r.byPeer[runtimeID]
	if !ok {
		r.mu.RUnlock()
		return nil, false
	}
	conn, ok := r.byID[id]
	r.mu.RUnlock()
	return conn, ok
}

func (r *ConnectionRegistry) Remove(id ConnectionID) (*Connection, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	conn, ok := r.byID[id]
	if !ok {
		return nil, false
	}

	delete(r.byID, id)
	key := conn.Peer.Key()
	if currentID, exists := r.byPeer[key]; exists && currentID == id {
		delete(r.byPeer, key)
	}

	return conn, true
}

func (r *ConnectionRegistry) RemoveByPeer(key PeerKey) (*Connection, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id, ok := r.byPeer[key]
	if !ok {
		return nil, false
	}

	conn, ok := r.byID[id]
	if !ok {
		delete(r.byPeer, key)
		return nil, false
	}

	delete(r.byID, id)
	delete(r.byPeer, key)
	return conn, true
}

func (r *ConnectionRegistry) List() []*Connection {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Connection, 0, len(r.byID))
	for _, conn := range r.byID {
		result = append(result, conn)
	}
	return result
}

func (r *ConnectionRegistry) ActiveCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, conn := range r.byID {
		if conn.IsActive() {
			count++
		}
	}
	return count
}

func (r *ConnectionRegistry) PeerExists(key PeerKey) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, ok := r.byPeer[key]
	if !ok {
		return false
	}
	conn, ok := r.byID[id]
	if !ok {
		return false
	}
	return conn.IsActive()
}
