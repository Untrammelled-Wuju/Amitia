package desktop_update

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type persistedUpdateState struct {
	UpdateSources    []ExtensionUpdateSource              `json:"update_sources"`
	UpdateCandidates map[string][]ExtensionUpdateMetadata `json:"update_candidates"`
	UpdateOperations []UpdateOperation                    `json:"update_operations"`
	UpdateJournal    []JournalEntry                       `json:"update_journal"`
	DownloadState    []DownloadState                      `json:"download_state"`
	CurrentVersions  map[string]string                    `json:"current_versions"`
	Generation       int64                                `json:"generation"`
}

func (m *UpdateManager) persistencePath() string {
	return filepath.Join(m.dataDir, "update-state.json")
}

func (m *UpdateManager) persistState() {
	m.mu.RLock()
	state := persistedUpdateState{
		UpdateSources: m.sources.List(), UpdateCandidates: make(map[string][]ExtensionUpdateMetadata),
		UpdateOperations: make([]UpdateOperation, 0, len(m.operations)),
		UpdateJournal:    m.journal.ListAll(), DownloadState: m.downloads.ListDownloads(),
		CurrentVersions: make(map[string]string, len(m.currentVersions)), Generation: m.genCounter,
	}
	for id, candidates := range m.candidates {
		state.UpdateCandidates[id] = append([]ExtensionUpdateMetadata(nil), candidates...)
	}
	for _, operation := range m.operations {
		state.UpdateOperations = append(state.UpdateOperations, *operation)
	}
	for id, version := range m.currentVersions {
		state.CurrentVersions[id] = version
	}
	m.mu.RUnlock()
	payload, err := json.Marshal(state)
	if err != nil {
		return
	}
	if err := os.MkdirAll(m.dataDir, 0o755); err != nil {
		return
	}
	tempPath := m.persistencePath() + ".tmp"
	if err := os.WriteFile(tempPath, payload, 0o600); err != nil {
		return
	}
	backupPath := m.persistencePath() + ".bak"
	_ = os.Remove(backupPath)
	if _, err := os.Stat(m.persistencePath()); err == nil {
		if os.Rename(m.persistencePath(), backupPath) != nil {
			return
		}
	}
	if os.Rename(tempPath, m.persistencePath()) != nil {
		_ = os.Rename(backupPath, m.persistencePath())
		return
	}
	_ = os.Remove(backupPath)
}

func (m *UpdateManager) restoreState() {
	payload, err := os.ReadFile(m.persistencePath())
	if err != nil {
		payload, err = os.ReadFile(m.persistencePath() + ".bak")
		if err != nil {
			return
		}
	}
	var state persistedUpdateState
	if json.Unmarshal(payload, &state) != nil {
		return
	}
	for _, source := range state.UpdateSources {
		_ = m.sources.Register(&source)
	}
	m.candidates = state.UpdateCandidates
	if m.candidates == nil {
		m.candidates = make(map[string][]ExtensionUpdateMetadata)
	}
	m.currentVersions = state.CurrentVersions
	if m.currentVersions == nil {
		m.currentVersions = make(map[string]string)
	}
	m.genCounter = state.Generation
	for i := range state.UpdateOperations {
		op := state.UpdateOperations[i]
		m.operations[op.OperationID] = &op
		m.operationsByExt[op.ExtensionID] = append(m.operationsByExt[op.ExtensionID], op.OperationID)
		if !m.stateMachine.IsTerminal(op.Status) {
			m.activeByExt[op.ExtensionID] = op.OperationID
		}
	}
	m.journal.Restore(state.UpdateJournal)
	m.recovery.SetOperations(m.operations)
}
