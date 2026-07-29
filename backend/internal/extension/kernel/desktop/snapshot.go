package desktop

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

type DesktopSnapshot struct {
	Generation    int64                                    `json:"generation"`
	Contributions []ResolvedDesktopContribution            `json:"contributions"`
	Hash          string                                   `json:"hash"`
	CreatedAt     time.Time                                `json:"createdAt"`
	MenuTree      map[string][]ResolvedDesktopContribution `json:"menuTree"`
	TrayTree      map[string][]ResolvedDesktopContribution `json:"trayTree"`
	Shortcuts     []ResolvedDesktopContribution            `json:"shortcuts"`
	Conflicts     []ConflictRecord                         `json:"conflicts"`
}

func (s *DesktopSnapshot) ComputeHash() string {
	data, err := json.Marshal(s.Contributions)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

type SnapshotBuilder struct {
	generation int64
}

func NewSnapshotBuilder(generation int64) *SnapshotBuilder {
	return &SnapshotBuilder{generation: generation}
}

func (b *SnapshotBuilder) Build(
	contributions []ResolvedDesktopContribution,
	conflicts []ConflictRecord,
	sortCtx SortContext,
) *DesktopSnapshot {
	menuTree := make(map[string][]ResolvedDesktopContribution)
	trayTree := make(map[string][]ResolvedDesktopContribution)
	var shortcuts []ResolvedDesktopContribution

	menuTargets := []string{
		"app.menu.file.extensions", "app.menu.edit.extensions",
		"app.menu.view.extensions", "app.menu.tools.extensions",
		"app.menu.help.extensions",
		"context.chat.message.extensions", "context.chat.composer.extensions",
		"context.extension.detail.extensions",
	}
	for _, target := range menuTargets {
		items := SortMenuItems(contributions, target, sortCtx)
		items = insertSeparators(items)
		if len(items) > 0 {
			menuTree[target] = items
		}
	}

	trayTargets := []string{"tray.quick_actions", "tray.extensions", "tray.status"}
	for _, target := range trayTargets {
		items := SortTrayItems(contributions, target, sortCtx)
		if len(items) > 0 {
			trayTree[target] = items
		}
	}

	shortcuts = SortShortcuts(contributions, sortCtx)

	snapshot := &DesktopSnapshot{
		Generation:    b.generation,
		Contributions: contributions,
		MenuTree:      menuTree,
		TrayTree:      trayTree,
		Shortcuts:     shortcuts,
		Conflicts:     conflicts,
		CreatedAt:     time.Now().UTC(),
	}
	snapshot.Hash = snapshot.ComputeHash()
	return snapshot
}

type SnapshotDiff struct {
	AddedContributions   []string
	RemovedContributions []string
	ChangedContributions []string
	AddedShortcuts       []string
	RemovedShortcuts     []string
	MenuTargetsChanged   []string
	TrayTargetsChanged   []string
	HashChanged          bool
}

func DiffSnapshots(old, new *DesktopSnapshot) *SnapshotDiff {
	diff := &SnapshotDiff{}
	if old == nil && new == nil {
		return diff
	}
	if old == nil {
		for _, c := range new.Contributions {
			diff.AddedContributions = append(diff.AddedContributions, c.Definition.ContributionID)
		}
		diff.HashChanged = true
		return diff
	}
	if new == nil {
		for _, c := range old.Contributions {
			diff.RemovedContributions = append(diff.RemovedContributions, c.Definition.ContributionID)
		}
		diff.HashChanged = true
		return diff
	}
	oldMap := make(map[string]ResolvedDesktopContribution)
	for _, c := range old.Contributions {
		oldMap[c.Definition.ContributionID] = c
	}
	newMap := make(map[string]ResolvedDesktopContribution)
	for _, c := range new.Contributions {
		newMap[c.Definition.ContributionID] = c
	}
	for id, oldC := range oldMap {
		if newC, ok := newMap[id]; !ok {
			diff.RemovedContributions = append(diff.RemovedContributions, id)
			if newC.Definition.Shortcut != nil {
				diff.RemovedShortcuts = append(diff.RemovedShortcuts, id)
			}
		} else {
			if oldC.Definition.DefinitionHash != newC.Definition.DefinitionHash {
				diff.ChangedContributions = append(diff.ChangedContributions, id)
			}
			if oldC.Definition.Shortcut != nil && newC.Definition.Shortcut == nil {
				diff.RemovedShortcuts = append(diff.RemovedShortcuts, id)
			}
			if oldC.Definition.Shortcut == nil && newC.Definition.Shortcut != nil {
				diff.AddedShortcuts = append(diff.AddedShortcuts, id)
			}
			if oldC.Definition.Shortcut != nil && newC.Definition.Shortcut != nil {
				if oldC.Definition.Shortcut.Accelerator != newC.Definition.Shortcut.Accelerator {
					diff.RemovedShortcuts = append(diff.RemovedShortcuts, id)
					diff.AddedShortcuts = append(diff.AddedShortcuts, id)
				}
			}
		}
	}
	for id := range newMap {
		if _, ok := oldMap[id]; !ok {
			diff.AddedContributions = append(diff.AddedContributions, id)
			if newMap[id].Definition.Shortcut != nil {
				diff.AddedShortcuts = append(diff.AddedShortcuts, id)
			}
		}
	}
	oldMenuTargets := make(map[string]bool)
	for t := range old.MenuTree {
		oldMenuTargets[t] = true
	}
	for t := range new.MenuTree {
		if !oldMenuTargets[t] {
			diff.MenuTargetsChanged = append(diff.MenuTargetsChanged, t)
		} else {
			oldHash := hashContributions(old.MenuTree[t])
			newHash := hashContributions(new.MenuTree[t])
			if oldHash != newHash {
				diff.MenuTargetsChanged = append(diff.MenuTargetsChanged, t)
			}
		}
	}
	for t := range oldMenuTargets {
		if _, ok := new.MenuTree[t]; !ok {
			diff.MenuTargetsChanged = append(diff.MenuTargetsChanged, t)
		}
	}
	diff.HashChanged = old.Hash != new.Hash
	return diff
}

func hashContributions(contribs []ResolvedDesktopContribution) string {
	data, err := json.Marshal(contribs)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
