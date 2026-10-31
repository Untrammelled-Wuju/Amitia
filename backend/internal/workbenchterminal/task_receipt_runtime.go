package workbenchterminal

import (
	"errors"
	amitialog "log"
	"time"
)

var ErrReceiptUnavailable = errors.New("task terminal receipt storage is unavailable")

func (s *session) recordTaskReceipt(state string) {
	if s == nil || s.receiptJournal == nil || s.taskID == "" {
		return
	}
	s.mu.Lock()
	exit := (*int)(nil)
	if !s.running {
		value := s.exitCode
		exit = &value
	}
	receipt := TaskReceipt{
		SessionID: s.id, TaskID: s.taskID, ConversationID: s.conversationID,
		WorkspaceURI: s.workspaceURI, Mode: s.proc.Mode(), StartedAt: s.created.UTC(),
		State: state, ExitCode: exit, LatestSequence: s.lifecycleSequence,
	}
	s.mu.Unlock()
	if err := s.receiptJournal.Record(s.owner, receipt); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
}

func (m *Manager) EnableTaskReceipts(journal *TaskReceiptJournal) {
	m.mu.Lock()
	m.receiptJournal = journal
	m.mu.Unlock()
}

func (m *Manager) TaskReceipts(owner, conversationID, taskID string) ([]TaskReceipt, error) {
	m.mu.Lock()
	journal := m.receiptJournal
	active := make(map[string]bool)
	for _, s := range m.sessions {
		if s.owner == owner && s.taskID == taskID && s.conversationID == conversationID {
			active[s.id] = true
		}
	}
	m.mu.Unlock()
	if journal == nil {
		return nil, ErrReceiptUnavailable
	}
	return journal.List(owner, conversationID, taskID, active)
}

func (m *Manager) pruneTaskReceipts() {
	m.mu.Lock()
	journal := m.receiptJournal
	m.mu.Unlock()
	if journal != nil {
		if err := journal.Prune(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
	}
}

func (m *Manager) receiptStatus() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.receiptJournal != nil {
		return "enabled"
	}
	return "unavailable"
}

func (m *Manager) recordClose(s *session, reason string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	if reason == "revoked" {
		s.recordLifecycleLocked(LifecycleEvent{Kind: "session.revoked", At: time.Now().UTC()})
	}
	s.mu.Unlock()
	s.recordTaskReceipt(reason)
}

func OpenDefaultTaskReceiptJournal() (*TaskReceiptJournal, error) {
	return defaultTaskReceiptJournal()
}
