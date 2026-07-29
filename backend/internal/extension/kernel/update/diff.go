package update

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

type UpdateType string

const (
	UpdateTypePatch       UpdateType = "patch"
	UpdateTypeMinor       UpdateType = "minor"
	UpdateTypeMajor       UpdateType = "major"
	UpdateTypeSecurity    UpdateType = "security"
	UpdateTypeRepair      UpdateType = "repair"
	UpdateTypeDevelopment UpdateType = "development"
)

type DefinitionSnapshot struct {
	ExtensionID     string
	Version         string
	ManifestHash    string
	ContentTreeHash string
	PackageHash     string
	PublisherID     string
	Modules         []ModuleSnapshot
	Contributions   []ContributionSnapshot
	Runtimes        []RuntimeSnapshot
	Dependencies    []DependencySnapshot
	Permissions     []PermissionSnapshot
	Resources       []ResourceSnapshot
	StorageSchemas  []StorageSchemaSnapshot
	Migrations      []MigrationSnapshot
	Platforms       []string
	Architectures   []string
	SignatureKeyID  string
	TrustLevel      string
	GeneratedAt     time.Time
}

type ModuleSnapshot struct {
	ID             string
	Type           string
	Version        string
	EnabledDefault bool
	RuntimeID      string
	Contributions  []string
}

type ContributionSnapshot struct {
	ID           string
	Type         string
	RuntimeID    string
	EntryType    string
	EntryName    string
	InputSchema  string
	OutputSchema string
	RiskLevel    string
	SideEffect   string
}

type RuntimeSnapshot struct {
	ID             string
	Type           string
	Entry          string
	Protocol       string
	InstancePolicy string
	MaxMemoryMB    int
	MaxConcurrent  int
}

type DependencySnapshot struct {
	Type     string
	Target   string
	Version  string
	Required bool
	Scope    string
}

type PermissionSnapshot struct {
	ID          string
	Reason      string
	Constraints string
}

type ResourceSnapshot struct {
	ID   string
	Type string
	Path string
	Mime string
}

type StorageSchemaSnapshot struct {
	Namespace string
	Version   int
	Schema    string
}

type MigrationSnapshot struct {
	ID          string
	FromRange   string
	ToRange     string
	RuntimeType string
	Entry       string
	Reversible  bool
}

type DefinitionDiff struct {
	ExtensionID          string
	OldVersion           string
	NewVersion           string
	UpdateType           UpdateType
	ModulesAdded         []ModuleSnapshot
	ModulesRemoved       []ModuleSnapshot
	ModulesChanged       []ModuleChange
	ContributionsAdded   []ContributionSnapshot
	ContributionsRemoved []ContributionSnapshot
	ContributionsChanged []ContributionChange
	RuntimesAdded        []RuntimeSnapshot
	RuntimesRemoved      []RuntimeSnapshot
	RuntimesChanged      []RuntimeChange
	DependenciesAdded    []DependencySnapshot
	DependenciesRemoved  []DependencySnapshot
	PermissionsAdded     []PermissionSnapshot
	PermissionsRemoved   []PermissionSnapshot
	ResourcesAdded       []ResourceSnapshot
	ResourcesRemoved     []ResourceSnapshot
	StorageSchemaChanged []StorageSchemaChange
	MigrationsAdded      []MigrationSnapshot
	MigrationsRemoved    []MigrationSnapshot
	PublisherChanged     bool
	OldPublisherID       string
	NewPublisherID       string
	SignatureKeyChanged  bool
	OldKeyID             string
	NewKeyID             string
	PlatformAdded        []string
	PlatformRemoved      []string
	BreakingChanges      []BreakingChange
	PermissionExpanded   bool
	ScopeExpanded        bool
	HasBreakingChanges   bool
	HasHighRiskMigration bool
}

type ModuleChange struct {
	ID        string
	FieldType string
	OldValue  string
	NewValue  string
}

type ContributionChange struct {
	ID        string
	FieldType string
	OldValue  string
	NewValue  string
}

type RuntimeChange struct {
	ID        string
	FieldType string
	OldValue  string
	NewValue  string
}

type StorageSchemaChange struct {
	Namespace  string
	OldVersion int
	NewVersion int
	Compatible bool
}

type BreakingChange struct {
	Field    string
	Reason   string
	Severity string
}

func ComputeDiff(old, new DefinitionSnapshot) DefinitionDiff {
	diff := DefinitionDiff{
		ExtensionID: new.ExtensionID,
		OldVersion:  old.Version,
		NewVersion:  new.Version,
		UpdateType:  classifyUpdate(old.Version, new.Version),
	}

	oldModules := indexBy(old.Modules, func(m ModuleSnapshot) string { return m.ID })
	newModules := indexBy(new.Modules, func(m ModuleSnapshot) string { return m.ID })
	for id, n := range newModules {
		if o, ok := oldModules[id]; !ok {
			diff.ModulesAdded = append(diff.ModulesAdded, n)
		} else {
			changes := compareModule(o, n)
			diff.ModulesChanged = append(diff.ModulesChanged, changes...)
		}
	}
	for id, o := range oldModules {
		if _, ok := newModules[id]; !ok {
			diff.ModulesRemoved = append(diff.ModulesRemoved, o)
			diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
				Field:    "module." + id,
				Reason:   "module removed",
				Severity: "high",
			})
		}
	}

	oldContribs := indexBy(old.Contributions, func(c ContributionSnapshot) string { return c.ID })
	newContribs := indexBy(new.Contributions, func(c ContributionSnapshot) string { return c.ID })
	for id, n := range newContribs {
		if o, ok := oldContribs[id]; !ok {
			diff.ContributionsAdded = append(diff.ContributionsAdded, n)
		} else {
			changes := compareContribution(o, n)
			diff.ContributionsChanged = append(diff.ContributionsChanged, changes...)
			if changedSchema(o.InputSchema, n.InputSchema) || changedSchema(o.OutputSchema, n.OutputSchema) {
				diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
					Field:    "contribution." + id + ".schema",
					Reason:   "input/output schema changed",
					Severity: "high",
				})
			}
		}
	}
	for id, o := range oldContribs {
		if _, ok := newContribs[id]; !ok {
			diff.ContributionsRemoved = append(diff.ContributionsRemoved, o)
			diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
				Field:    "contribution." + id,
				Reason:   "contribution removed",
				Severity: "high",
			})
		}
	}

	oldRuns := indexBy(old.Runtimes, func(r RuntimeSnapshot) string { return r.ID })
	newRuns := indexBy(new.Runtimes, func(r RuntimeSnapshot) string { return r.ID })
	for id, n := range newRuns {
		if o, ok := oldRuns[id]; !ok {
			diff.RuntimesAdded = append(diff.RuntimesAdded, n)
		} else {
			diff.RuntimesChanged = append(diff.RuntimesChanged, compareRuntime(o, n)...)
		}
	}
	for id, o := range oldRuns {
		if _, ok := newRuns[id]; !ok {
			diff.RuntimesRemoved = append(diff.RuntimesRemoved, o)
			diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
				Field:    "runtime." + id,
				Reason:   "runtime entry removed",
				Severity: "high",
			})
		}
	}

	oldDeps := indexBy(old.Dependencies, func(d DependencySnapshot) string { return d.Type + ":" + d.Target })
	newDeps := indexBy(new.Dependencies, func(d DependencySnapshot) string { return d.Type + ":" + d.Target })
	for k, n := range newDeps {
		if _, ok := oldDeps[k]; !ok {
			diff.DependenciesAdded = append(diff.DependenciesAdded, n)
		}
	}
	for k, o := range oldDeps {
		if _, ok := newDeps[k]; !ok {
			diff.DependenciesRemoved = append(diff.DependenciesRemoved, o)
		}
	}

	oldPerms := indexBy(old.Permissions, func(p PermissionSnapshot) string { return p.ID })
	newPerms := indexBy(new.Permissions, func(p PermissionSnapshot) string { return p.ID })
	for id, n := range newPerms {
		if _, ok := oldPerms[id]; !ok {
			diff.PermissionsAdded = append(diff.PermissionsAdded, n)
		}
	}
	for id, o := range oldPerms {
		if _, ok := newPerms[id]; !ok {
			diff.PermissionsRemoved = append(diff.PermissionsRemoved, o)
		}
	}
	if len(diff.PermissionsAdded) > 0 {
		diff.PermissionExpanded = true
	}

	oldRes := indexBy(old.Resources, func(r ResourceSnapshot) string { return r.ID })
	newRes := indexBy(new.Resources, func(r ResourceSnapshot) string { return r.ID })
	for id, n := range newRes {
		if _, ok := oldRes[id]; !ok {
			diff.ResourcesAdded = append(diff.ResourcesAdded, n)
		}
	}
	for id, o := range oldRes {
		if _, ok := newRes[id]; !ok {
			diff.ResourcesRemoved = append(diff.ResourcesRemoved, o)
		}
	}

	oldSchemas := indexBy(old.StorageSchemas, func(s StorageSchemaSnapshot) string { return s.Namespace })
	newSchemas := indexBy(new.StorageSchemas, func(s StorageSchemaSnapshot) string { return s.Namespace })
	for ns, n := range newSchemas {
		if o, ok := oldSchemas[ns]; ok {
			if o.Version != n.Version {
				diff.StorageSchemaChanged = append(diff.StorageSchemaChanged, StorageSchemaChange{
					Namespace:  ns,
					OldVersion: o.Version,
					NewVersion: n.Version,
					Compatible: n.Version > o.Version,
				})
				if n.Version < o.Version {
					diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
						Field:    "storage." + ns,
						Reason:   "schema version downgrade",
						Severity: "critical",
					})
				}
			}
		}
	}

	oldMigs := indexBy(old.Migrations, func(m MigrationSnapshot) string { return m.ID })
	newMigs := indexBy(new.Migrations, func(m MigrationSnapshot) string { return m.ID })
	for id, n := range newMigs {
		if _, ok := oldMigs[id]; !ok {
			diff.MigrationsAdded = append(diff.MigrationsAdded, n)
			if !n.Reversible {
				diff.HasHighRiskMigration = true
			}
		}
	}
	for id, o := range oldMigs {
		if _, ok := newMigs[id]; !ok {
			diff.MigrationsRemoved = append(diff.MigrationsRemoved, o)
		}
	}

	if old.PublisherID != new.PublisherID {
		diff.PublisherChanged = true
		diff.OldPublisherID = old.PublisherID
		diff.NewPublisherID = new.PublisherID
		diff.BreakingChanges = append(diff.BreakingChanges, BreakingChange{
			Field:    "publisher",
			Reason:   "ownership transfer",
			Severity: "critical",
		})
	}

	if old.SignatureKeyID != new.SignatureKeyID {
		diff.SignatureKeyChanged = true
		diff.OldKeyID = old.SignatureKeyID
		diff.NewKeyID = new.SignatureKeyID
	}

	for _, p := range new.Platforms {
		if !contains(old.Platforms, p) {
			diff.PlatformAdded = append(diff.PlatformAdded, p)
		}
	}
	for _, p := range old.Platforms {
		if !contains(new.Platforms, p) {
			diff.PlatformRemoved = append(diff.PlatformRemoved, p)
		}
	}

	diff.HasBreakingChanges = len(diff.BreakingChanges) > 0
	diff.ScopeExpanded = diff.PermissionExpanded
	return diff
}

func classifyUpdate(oldVersion, newVersion string) UpdateType {
	if oldVersion == "" {
		return UpdateTypePatch
	}
	if oldVersion == newVersion {
		return UpdateTypeRepair
	}
	parts := strings.SplitN(newVersion, ".", 3)
	oldParts := strings.SplitN(oldVersion, ".", 3)
	if len(parts) >= 2 && len(oldParts) >= 2 {
		if parts[0] != oldParts[0] {
			return UpdateTypeMajor
		}
		if parts[1] != oldParts[1] {
			return UpdateTypeMinor
		}
		return UpdateTypePatch
	}
	return UpdateTypePatch
}

func compareModule(old, new ModuleSnapshot) []ModuleChange {
	var changes []ModuleChange
	if old.Type != new.Type {
		changes = append(changes, ModuleChange{ID: old.ID, FieldType: "type", OldValue: old.Type, NewValue: new.Type})
	}
	if old.RuntimeID != new.RuntimeID {
		changes = append(changes, ModuleChange{ID: old.ID, FieldType: "runtime", OldValue: old.RuntimeID, NewValue: new.RuntimeID})
	}
	if old.EnabledDefault != new.EnabledDefault {
		changes = append(changes, ModuleChange{ID: old.ID, FieldType: "enabled_default", OldValue: fmt.Sprintf("%v", old.EnabledDefault), NewValue: fmt.Sprintf("%v", new.EnabledDefault)})
	}
	return changes
}

func compareContribution(old, new ContributionSnapshot) []ContributionChange {
	var changes []ContributionChange
	if old.RuntimeID != new.RuntimeID {
		changes = append(changes, ContributionChange{ID: old.ID, FieldType: "runtime", OldValue: old.RuntimeID, NewValue: new.RuntimeID})
	}
	if old.EntryName != new.EntryName {
		changes = append(changes, ContributionChange{ID: old.ID, FieldType: "entry", OldValue: old.EntryName, NewValue: new.EntryName})
	}
	if old.RiskLevel != new.RiskLevel {
		changes = append(changes, ContributionChange{ID: old.ID, FieldType: "risk_level", OldValue: old.RiskLevel, NewValue: new.RiskLevel})
	}
	return changes
}

func compareRuntime(old, new RuntimeSnapshot) []RuntimeChange {
	var changes []RuntimeChange
	if old.Entry != new.Entry {
		changes = append(changes, RuntimeChange{ID: old.ID, FieldType: "entry", OldValue: old.Entry, NewValue: new.Entry})
	}
	if old.Type != new.Type {
		changes = append(changes, RuntimeChange{ID: old.ID, FieldType: "type", OldValue: old.Type, NewValue: new.Type})
	}
	if old.MaxMemoryMB != new.MaxMemoryMB {
		changes = append(changes, RuntimeChange{ID: old.ID, FieldType: "max_memory", OldValue: fmt.Sprintf("%d", old.MaxMemoryMB), NewValue: fmt.Sprintf("%d", new.MaxMemoryMB)})
	}
	return changes
}

func changedSchema(old, new string) bool {
	if old == "" || new == "" {
		return false
	}
	return old != new
}

func indexBy[T any](items []T, keyFn func(T) string) map[string]T {
	result := make(map[string]T, len(items))
	for _, item := range items {
		result[keyFn(item)] = item
	}
	return result
}

func contains(slice []string, v string) bool {
	for _, s := range slice {
		if s == v {
			return true
		}
	}
	return false
}

func (d DefinitionDiff) SortedBreakingChanges() []BreakingChange {
	out := make([]BreakingChange, len(d.BreakingChanges))
	copy(out, d.BreakingChanges)
	sort.Slice(out, func(i, j int) bool {
		severityOrder := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3}
		si := severityOrder[out[i].Severity]
		sj := severityOrder[out[j].Severity]
		if si != sj {
			return si < sj
		}
		return out[i].Field < out[j].Field
	})
	return out
}

var (
	ErrInvalidDefinition = errors.New("update: invalid definition snapshot")
)
