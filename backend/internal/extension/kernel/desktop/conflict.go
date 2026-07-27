package desktop

import (
	"fmt"
	"sync"
	"time"
)

type ConflictType string

const (
	ConflictTypeShortcut      ConflictType = "shortcut"
	ConflictTypeMenuID        ConflictType = "menu_id"
	ConflictTypeActionID     ConflictType = "action_id"
	ConflictTypeTrayID       ConflictType = "tray_id"
)

type ConflictSeverity string

const (
	ConflictSeverityBlock    ConflictSeverity = "block"
	ConflictSeverityWarning  ConflictSeverity = "warning"
)

type ConflictRecord struct {
	ConflictID      string           `json:"conflictId"`
	Type            ConflictType     `json:"type"`
	Severity        ConflictSeverity `json:"severity"`
	Target          string           `json:"target"`
	ExistingContribID string        `json:"existingContribId"`
	ExistingExtID   string           `json:"existingExtId"`
	NewContribID    string           `json:"newContribId"`
	NewExtID        string           `json:"newExtId"`
	Accelerator     string           `json:"accelerator,omitempty"`
	Resolved        bool             `json:"resolved"`
	Resolution      string           `json:"resolution,omitempty"`
	CreatedAt       time.Time        `json:"createdAt"`
	ResolvedAt      *time.Time      `json:"resolvedAt,omitempty"`
}

type ConflictResolver struct {
	mu         sync.RWMutex
	conflicts  map[string]*ConflictRecord
	nextID     int
}

func NewConflictResolver() *ConflictResolver {
	return &ConflictResolver{
		conflicts: make(map[string]*ConflictRecord),
	}
}

func (cr *ConflictResolver) DetectShortcutConflict(existing *DesktopContributionDefinition, new *DesktopContributionDefinition) *ConflictRecord {
	if existing.Shortcut == nil || new.Shortcut == nil {
		return nil
	}
	if !AcceleratorsConflict(existing.Shortcut.Accelerator, new.Shortcut.Accelerator) {
		return nil
	}
	return cr.createConflict(ConflictTypeShortcut, ConflictSeverityBlock,
		new.Target, existing.ContributionID, existing.ExtensionID,
		new.ContributionID, new.ExtensionID, existing.Shortcut.Accelerator)
}

func (cr *ConflictResolver) DetectMenuIDConflict(existing *DesktopContributionDefinition, new *DesktopContributionDefinition) *ConflictRecord {
	if existing.ContributionID == new.ContributionID {
		return cr.createConflict(ConflictTypeMenuID, ConflictSeverityBlock,
			new.Target, existing.ContributionID, existing.ExtensionID,
			new.ContributionID, new.ExtensionID, "")
	}
	if existing.Action.TargetID != "" && existing.Action.TargetID == new.Action.TargetID {
		return cr.createConflict(ConflictTypeActionID, ConflictSeverityBlock,
			new.Target, existing.ContributionID, existing.ExtensionID,
			new.ContributionID, new.ExtensionID, "")
	}
	return nil
}

func (cr *ConflictResolver) createConflict(ct ConflictType, sev ConflictSeverity, target, existingID, existingExt, newID, newExt, accel string) *ConflictRecord {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	cr.nextID++
	id := fmt.Sprintf("dc-%d-%d", time.Now().Unix(), cr.nextID)
	record := &ConflictRecord{
		ConflictID:       id,
		Type:             ct,
		Severity:         sev,
		Target:           target,
		ExistingContribID: existingID,
		ExistingExtID:    existingExt,
		NewContribID:     newID,
		NewExtID:         newExt,
		Accelerator:      accel,
		CreatedAt:        time.Now().UTC(),
	}
	cr.conflicts[id] = record
	return record
}

func (cr *ConflictResolver) Resolve(conflictID, resolution string) error {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	c, ok := cr.conflicts[conflictID]
	if !ok {
		return ErrInvalidConflictResolution
	}
	c.Resolved = true
	c.Resolution = resolution
	now := time.Now().UTC()
	c.ResolvedAt = &now
	return nil
}

func (cr *ConflictResolver) ListUnresolved() []ConflictRecord {
	cr.mu.RLock()
	defer cr.mu.RUnlock()
	result := make([]ConflictRecord, 0)
	for _, c := range cr.conflicts {
		if !c.Resolved {
			result = append(result, *c)
		}
	}
	return result
}

func (cr *ConflictResolver) ListAll() []ConflictRecord {
	cr.mu.RLock()
	defer cr.mu.RUnlock()
	result := make([]ConflictRecord, 0, len(cr.conflicts))
	for _, c := range cr.conflicts {
		result = append(result, *c)
	}
	return result
}

func (cr *ConflictResolver) Get(conflictID string) (*ConflictRecord, bool) {
	cr.mu.RLock()
	defer cr.mu.RUnlock()
	c, ok := cr.conflicts[conflictID]
	if !ok {
		return nil, false
	}
	return c, true
}

func (cr *ConflictResolver) ClearByExtension(extensionID string) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	for id, c := range cr.conflicts {
		if c.ExistingExtID == extensionID || c.NewExtID == extensionID {
			delete(cr.conflicts, id)
		}
	}
}
