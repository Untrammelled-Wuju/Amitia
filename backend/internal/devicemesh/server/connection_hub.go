package server

import (
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	meshprotocol "github.com/u-ai/backend/internal/devicemesh/protocol"
	protocol "github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type ConnectionHub struct {
	mu          sync.RWMutex
	connections map[string]*MeshConnection
}

func NewConnectionHub() *ConnectionHub {
	return &ConnectionHub{
		connections: make(map[string]*MeshConnection),
	}
}

func (h *ConnectionHub) Attach(sessionID runtimeidentity.RuntimeSessionID, conn *MeshConnection) (*MeshConnection, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := sessionID.String()
	existing, ok := h.connections[key]
	if ok && existing.Generation >= conn.Generation {
		return existing, false
	}

	if existing != nil && existing.Generation < conn.Generation {
		_ = existing.Close(4001, "session_superseded")
	}

	h.connections[key] = conn
	return nil, true
}

func (h *ConnectionHub) Detach(sessionID runtimeidentity.RuntimeSessionID, generation int64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := sessionID.String()
	if existing, ok := h.connections[key]; ok && existing.Generation == generation {
		delete(h.connections, key)
	}
}

func (h *ConnectionHub) GetBySession(sessionID runtimeidentity.RuntimeSessionID) (*MeshConnection, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	c, ok := h.connections[sessionID.String()]
	return c, ok
}

// GetByRuntime returns the newest active connection for one concrete runtime.
// A superseded websocket may coexist briefly until CloseSuperseded completes;
// choosing by map iteration would make routed invocations nondeterministic.
func (h *ConnectionHub) GetByRuntime(spaceID runtimeidentity.SpaceID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID) (*MeshConnection, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var selected *MeshConnection
	for _, c := range h.connections {
		if c == nil || c.SpaceID != spaceID || c.DeviceID != deviceID || c.RuntimeID != runtimeID {
			continue
		}
		if selected == nil || c.Generation > selected.Generation {
			selected = c
		}
	}
	return selected, selected != nil
}

// GetByDevice returns the active Device Agent connection for a space-owned
// device. Device Agent identity is single-runtime per device; if stale
// connections coexist transiently, the highest connection generation wins.
func (h *ConnectionHub) GetByDevice(spaceID runtimeidentity.SpaceID, deviceID runtimeidentity.DeviceID) (*MeshConnection, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var selected *MeshConnection
	for _, c := range h.connections {
		if c == nil || c.SpaceID != spaceID || c.DeviceID != deviceID {
			continue
		}
		if selected == nil || c.Generation > selected.Generation {
			selected = c
		}
	}
	return selected, selected != nil
}

// ListBySpace returns at most one active connection per device for a Space.
// The highest connection generation wins for each device, and the result is
// sorted by device ID so callers never depend on Go map iteration order.
func (h *ConnectionHub) ListBySpace(spaceID runtimeidentity.SpaceID) []*MeshConnection {
	h.mu.RLock()
	byDevice := make(map[string]*MeshConnection)
	for _, c := range h.connections {
		if c == nil || c.SpaceID != spaceID {
			continue
		}
		key := c.DeviceID.String()
		if existing := byDevice[key]; existing == nil || c.Generation > existing.Generation {
			byDevice[key] = c
		}
	}
	h.mu.RUnlock()

	keys := make([]string, 0, len(byDevice))
	for key := range byDevice {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]*MeshConnection, 0, len(keys))
	for _, key := range keys {
		result = append(result, byDevice[key])
	}
	return result
}

// GetPreferredBySpace returns one deterministic active Device Agent for a Space.
// The freshest connection wins; generation and device ID provide stable
// tie-breakers so generic cloud control-plane routing never depends on map iteration.
// Desktop-pet behavior routing MUST NOT use this helper; it is affinity-routed.
func (h *ConnectionHub) GetPreferredBySpace(spaceID runtimeidentity.SpaceID) (*MeshConnection, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var selected *MeshConnection
	for _, c := range h.connections {
		if c == nil || c.SpaceID != spaceID {
			continue
		}
		if selected == nil || c.LastPongAt.After(selected.LastPongAt) ||
			(c.LastPongAt.Equal(selected.LastPongAt) && c.Generation > selected.Generation) ||
			(c.LastPongAt.Equal(selected.LastPongAt) && c.Generation == selected.Generation && c.DeviceID.String() < selected.DeviceID.String()) {
			selected = c
		}
	}
	return selected, selected != nil
}

func (h *ConnectionHub) Send(sessionID runtimeidentity.RuntimeSessionID, generation int64, data []byte) bool {
	h.mu.RLock()
	c, ok := h.connections[sessionID.String()]
	h.mu.RUnlock()

	if !ok || c.Generation != generation {
		return false
	}
	return c.Send(data) == nil
}

func (h *ConnectionHub) CloseSuperseded(sessionID runtimeidentity.RuntimeSessionID, generation int64) {
	h.mu.RLock()
	c, ok := h.connections[sessionID.String()]
	h.mu.RUnlock()

	if ok && c.Generation < generation {
		_ = c.Close(4001, "session_superseded")
		h.Detach(sessionID, c.Generation)
	}
}

func (h *ConnectionHub) CloseAll() {
	h.mu.Lock()
	conns := make([]*MeshConnection, 0, len(h.connections))
	for _, c := range h.connections {
		conns = append(conns, c)
	}
	h.connections = make(map[string]*MeshConnection)
	h.mu.Unlock()

	for _, c := range conns {
		_ = c.Close(4000, "server_shutdown")
	}
}

func (h *ConnectionHub) SendEnvelope(sessionID runtimeidentity.RuntimeSessionID, generation int64, msgType protocol.MessageType, payload interface{}) bool {
	h.mu.RLock()
	c, ok := h.connections[sessionID.String()]
	h.mu.RUnlock()

	if !ok || c.Generation != generation {
		return false
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return false
	}

	seq := c.nextOutboundSequence()

	env := protocol.Envelope{
		EnvelopeVersion:      meshprotocol.EnvelopeVersion,
		Protocol:             meshprotocol.ProtocolName,
		MessageType:          msgType,
		MessageID:            uuid.New().String(),
		SpaceID:              c.SpaceID,
		DeviceID:             c.DeviceID,
		RuntimeID:            c.RuntimeID,
		RuntimeSessionID:     c.SessionID,
		ConnectionGeneration: c.Generation,
		Sequence:             seq,
		PayloadSchemaVersion: 1,
		PayloadHash:          protocol.ComputePayloadHash(payloadBytes),
		SentAt:               time.Now().UTC(),
		Payload:              payloadBytes,
	}

	envBytes, err := json.Marshal(env)
	if err != nil {
		return false
	}

	return c.Send(envBytes) == nil
}

func (h *ConnectionHub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections)
}

func (h *ConnectionHub) IsSessionActive(sessionID runtimeidentity.RuntimeSessionID, generation int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	c, ok := h.connections[sessionID.String()]
	if !ok {
		return false
	}
	return c.Generation == generation
}

func (h *ConnectionHub) GetLastPongAt(sessionID runtimeidentity.RuntimeSessionID, generation int64) (time.Time, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	c, ok := h.connections[sessionID.String()]
	if !ok || c.Generation != generation {
		return time.Time{}, false
	}
	return c.LastPongAt, true
}

func (h *ConnectionHub) HasRecentPong(sessionID runtimeidentity.RuntimeSessionID, generation int64, within time.Duration) bool {
	lastPong, ok := h.GetLastPongAt(sessionID, generation)
	if !ok {
		return false
	}
	return time.Since(lastPong) <= within
}
