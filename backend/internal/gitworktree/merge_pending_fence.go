package gitworktree

func (m *Manager) refusePendingMergeAudit(key, id string) error {
	audit, err := m.readMergeAudit(key, id)
	if err != nil {
		return err
	}
	if audit != nil && audit.Result == "pending" {
		return ErrMergePreviouslyAttempted
	}
	return nil
}
