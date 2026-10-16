//go:build workbench_isolated

package workbenchterminal

import (
	"errors"
	"regexp"
)

var conversationPattern = regexp.MustCompile(`^[a-zA-Z0-9_:.-]{1,160}$`)

type TaskReceiptJournal struct{}

func (s *session) recordTaskReceipt(string)              {}
func (m *Manager) recordClose(s *session, reason string) {}
func (m *Manager) pruneTaskReceipts()                    {}
func launchProcess(string, uint16, uint16) (process, error) {
	return nil, errors.New("process launcher unavailable in isolated tests")
}
