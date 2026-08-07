// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only

package runtime

import "sync"

type RuntimeRegistry struct {
	mu          sync.RWMutex
	byRuntime   map[string]*Connection
	bySession   map[string]*Connection
	userRuntime map[string]string
}

func NewRuntimeRegistry() *RuntimeRegistry {
	return &RuntimeRegistry{
		byRuntime:   make(map[string]*Connection),
		bySession:   make(map[string]*Connection),
		userRuntime: make(map[string]string),
	}
}

func (r *RuntimeRegistry) Register(conn *Connection) (superseded *Connection) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.byRuntime[conn.RuntimeID()]; ok {
		superseded = existing
	}

	r.byRuntime[conn.RuntimeID()] = conn
	r.bySession[conn.SessionID()] = conn

	if uid := conn.UserID(); uid != "" {
		r.userRuntime[uid] = conn.RuntimeID()
	}

	return superseded
}

func (r *RuntimeRegistry) Unregister(sessionID string) *Connection {
	r.mu.Lock()
	defer r.mu.Unlock()

	conn, ok := r.bySession[sessionID]
	if !ok {
		return nil
	}

	delete(r.bySession, sessionID)

	if existing, ok := r.byRuntime[conn.RuntimeID()]; ok && existing.SessionID() == sessionID {
		delete(r.byRuntime, conn.RuntimeID())
	}

	if uid := conn.UserID(); uid != "" {
		if bound, ok := r.userRuntime[uid]; ok && bound == conn.RuntimeID() {
			delete(r.userRuntime, uid)
		}
	}

	return conn
}

func (r *RuntimeRegistry) GetByRuntime(runtimeID string) *Connection {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byRuntime[runtimeID]
}

func (r *RuntimeRegistry) GetBySession(sessionID string) *Connection {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.bySession[sessionID]
}

func (r *RuntimeRegistry) GetByUser(userID string) *Connection {
	r.mu.RLock()
	defer r.mu.RUnlock()

	runtimeID, ok := r.userRuntime[userID]
	if !ok {
		return nil
	}
	return r.byRuntime[runtimeID]
}

func (r *RuntimeRegistry) GetForUser(userID, runtimeID string) *Connection {
	r.mu.RLock()
	defer r.mu.RUnlock()

	conn, ok := r.byRuntime[runtimeID]
	if !ok {
		return nil
	}

	if conn.UserID() != userID {
		return nil
	}

	return conn
}

func (r *RuntimeRegistry) SelectRuntime(userID string) (*Connection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if userID == "" {
		return nil, ErrRuntimeOffline
	}

	runtimeID, ok := r.userRuntime[userID]
	if !ok {
		return nil, ErrRuntimeOffline
	}

	candidate, ok := r.byRuntime[runtimeID]
	if !ok {
		return nil, ErrRuntimeOffline
	}

	return candidate, nil
}

func (r *RuntimeRegistry) ListAll() []*Connection {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*Connection, 0, len(r.byRuntime))
	for _, conn := range r.byRuntime {
		list = append(list, conn)
	}
	return list
}

func (r *RuntimeRegistry) ListByState(state SessionState) []*Connection {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*Connection, 0)
	for _, conn := range r.byRuntime {
		if conn.State() == state {
			list = append(list, conn)
		}
	}
	return list
}

func (r *RuntimeRegistry) ListForUser(userID string) []*Connection {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]*Connection, 0)
	for _, conn := range r.byRuntime {
		if conn.UserID() == userID {
			list = append(list, conn)
		}
	}
	return list
}

func (r *RuntimeRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byRuntime)
}

func (r *RuntimeRegistry) CountByState(state SessionState) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, conn := range r.byRuntime {
		if conn.State() == state {
			count++
		}
	}
	return count
}

func (r *RuntimeRegistry) CloseAll(code int, reason string) {
	r.mu.Lock()
	list := make([]*Connection, 0, len(r.byRuntime))
	for _, conn := range r.byRuntime {
		list = append(list, conn)
	}
	r.mu.Unlock()

	for _, conn := range list {
		conn.Close(code, reason)
	}
}

func (r *RuntimeRegistry) ForEach(fn func(conn *Connection)) {
	r.mu.RLock()
	list := make([]*Connection, 0, len(r.byRuntime))
	for _, conn := range r.byRuntime {
		list = append(list, conn)
	}
	r.mu.RUnlock()

	for _, conn := range list {
		fn(conn)
	}
}

func (r *RuntimeRegistry) BindUserRuntime(userID, runtimeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.userRuntime[userID] = runtimeID
}

func (r *RuntimeRegistry) GetUserRuntime(userID string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.userRuntime[userID]
}

func (r *RuntimeRegistry) Supersede(oldConn *Connection, newConn *Connection) {
	r.mu.Lock()
	defer r.mu.Unlock()

	oldConn.SetState(SessionStateClosing)

	if existing, ok := r.byRuntime[oldConn.RuntimeID()]; ok && existing == oldConn {
		delete(r.byRuntime, oldConn.RuntimeID())
	}
	if existing, ok := r.bySession[oldConn.SessionID()]; ok && existing == oldConn {
		delete(r.bySession, oldConn.SessionID())
	}

	r.byRuntime[newConn.RuntimeID()] = newConn
	r.bySession[newConn.SessionID()] = newConn

	if uid := newConn.UserID(); uid != "" {
		r.userRuntime[uid] = newConn.RuntimeID()
	}
}
