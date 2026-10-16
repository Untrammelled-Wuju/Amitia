package workbenchterminal

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

const maxSessionsPerOwner = 6
const maxSessionsTotal = 48
const sessionTTL = 25 * time.Minute
const absoluteTTL = 4 * time.Hour

type Manager struct {
	mu             sync.Mutex
	sessions       map[string]*session
	stop           chan struct{}
	stopOnce       sync.Once
	receiptJournal *TaskReceiptJournal
	openingTotal   int
	openingByOwner map[string]int
	closed         bool
	startProcess   func(string, uint16, uint16) (process, error)
}

func NewManager() *Manager {
	m := &Manager{sessions: make(map[string]*session), stop: make(chan struct{})}
	go m.janitor()
	return m
}
func (m *Manager) janitor() {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			m.prune()
			m.pruneTaskReceipts()
		case <-m.stop:
			return
		}
	}
}
func (m *Manager) prune() {
	m.mu.Lock()
	now := time.Now()
	var expired []*session
	for id, s := range m.sessions {
		s.mu.Lock()
		idle := (now.Sub(s.lastUsed) > sessionTTL && !(s.taskID != "" && s.running)) || now.Sub(s.created) > absoluteTTL
		s.mu.Unlock()
		if idle {
			s.stopAcceptingCommands()
			delete(m.sessions, id)
			expired = append(expired, s)
		}
	}
	m.mu.Unlock()
	for _, s := range expired {
		s.stopAcceptingCommands()
		m.recordClose(s, "closed")
		_ = s.proc.Close()
	}
}
func (m *Manager) Open(owner, cwd string, rows, cols uint16) (Snapshot, error) {
	return m.OpenTask(owner, cwd, "", "", rows, cols)
}

func (m *Manager) OpenTask(owner, cwd, workspaceURI, taskID string, rows, cols uint16, conversationIDs ...string) (Snapshot, error) {
	conversationID := ""
	if len(conversationIDs) > 0 {
		conversationID = conversationIDs[0]
	}
	if taskID != "" && (!taskIDPattern.MatchString(taskID) || !conversationPattern.MatchString(conversationID)) {
		return Snapshot{}, ErrInvalid
	}
	if owner == "" || cwd == "" || rows < 10 || rows > 120 || cols < 20 || cols > 300 {
		return Snapshot{}, ErrInvalid
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return Snapshot{}, ErrClosed
	}
	if len(m.sessions)+m.openingTotal >= maxSessionsTotal {
		m.mu.Unlock()
		return Snapshot{}, ErrLimit
	}
	count := m.openingByOwner[owner]
	for _, s := range m.sessions {
		if s.owner == owner {
			count++
		}
	}
	if count >= maxSessionsPerOwner {
		m.mu.Unlock()
		return Snapshot{}, ErrLimit
	}
	if m.openingByOwner == nil {
		m.openingByOwner = make(map[string]int)
	}
	m.openingByOwner[owner]++
	m.openingTotal++
	starter := m.startProcess
	if starter == nil {
		starter = launchProcess
	}
	m.mu.Unlock()

	idBytes := make([]byte, 20)
	_, err := rand.Read(idBytes)
	var p process
	if err == nil {
		p, err = starter(cwd, rows, cols)
	}
	m.mu.Lock()
	m.openingByOwner[owner]--
	if m.openingByOwner[owner] == 0 {
		delete(m.openingByOwner, owner)
	}
	m.openingTotal--
	if m.closed {
		m.mu.Unlock()
		if p != nil {
			_ = p.Close()
		}
		return Snapshot{}, ErrClosed
	}
	if err != nil {
		m.mu.Unlock()
		if p != nil {
			_ = p.Close()
		}
		return Snapshot{}, err
	}
	if p == nil {
		m.mu.Unlock()
		return Snapshot{}, ErrUnavailable
	}
	now := time.Now()
	s := &session{receiptJournal: m.receiptJournal, id: hex.EncodeToString(idBytes), owner: owner, taskID: taskID, workspaceURI: workspaceURI, conversationID: conversationID, cwd: cwd, proc: p, running: true, exitCode: -1, readDone: make(chan struct{}), created: now, lastUsed: now}
	s.recordLifecycleLocked(LifecycleEvent{Kind: "session.started", Rows: rows, Cols: cols})
	m.sessions[s.id] = s
	s.recordTaskReceipt("running")
	m.mu.Unlock()
	go s.readOutput()
	go s.watchExit()
	return Snapshot{ID: s.id, TaskID: taskID, WorkspaceURI: workspaceURI, Mode: p.Mode(), Running: true, ExitCode: -1}, nil
}
func (m *Manager) get(owner, id string) (*session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok || s.owner != owner {
		return nil, ErrNotFound
	}
	return s, nil
}
func (m *Manager) Read(owner, id string, cursor int64) (Snapshot, error) {
	if cursor < 0 {
		return Snapshot{}, ErrInvalid
	}
	s, err := m.get(owner, id)
	if err != nil {
		return Snapshot{}, err
	}
	return s.readSnapshot(cursor)
}
func (m *Manager) Input(owner, id string, data []byte) error {
	if len(data) == 0 || len(data) > 4096 {
		return ErrInvalid
	}
	s, err := m.get(owner, id)
	if err != nil {
		return err
	}
	return s.input(data)
}
func (m *Manager) Resize(owner, id string, rows, cols uint16) error {
	if rows < 10 || rows > 120 || cols < 20 || cols > 300 {
		return ErrInvalid
	}
	s, err := m.get(owner, id)
	if err != nil {
		return err
	}
	return s.resize(rows, cols)
}
func (m *Manager) CloseSession(owner, id string) error {
	return m.closeSessionReason(owner, id, "closed")
}
func (m *Manager) RevokeSession(owner, id string) error {
	return m.closeSessionReason(owner, id, "revoked")
}
func (m *Manager) closeSessionReason(owner, id, reason string) error {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if !ok || s.owner != owner {
		m.mu.Unlock()
		return ErrNotFound
	}
	s.stopAcceptingCommands()
	delete(m.sessions, id)
	m.mu.Unlock()
	m.recordClose(s, reason)
	return s.proc.Close()
}
func (m *Manager) Close() {
	m.stopOnce.Do(func() { close(m.stop) })
	m.mu.Lock()
	m.closed = true
	sessions := m.sessions
	for _, s := range sessions {
		s.stopAcceptingCommands()
	}
	m.sessions = make(map[string]*session)
	m.mu.Unlock()
	for _, s := range sessions {
		m.recordClose(s, "closed")
		_ = s.proc.Close()
	}
}
