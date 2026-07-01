package mindruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

type RollbackVersion string

func NewRollbackVersion() RollbackVersion {
	return RollbackVersion(fmt.Sprintf("rollback-engine-v%d", time.Now().UnixNano()))
}

const RollbackEngineVersion RollbackVersion = "rollback-engine-v1"

type CascadeAction string

const (
	CascadeActionInvalidate  CascadeAction = "INVALIDATE"
	CascadeActionRequeue     CascadeAction = "REQUEUE"
	CascadeActionSkip        CascadeAction = "SKIP"
)

type CompensationTarget string

const (
	CompensationTargetBelief    CompensationTarget = "belief"
	CompensationTargetSummary   CompensationTarget = "summary"
	CompensationTargetProfile   CompensationTarget = "profile"
	CompensationTargetReflect   CompensationTarget = "reflect"
	CompensationTargetGrowth    CompensationTarget = "growth"
)

type VersionRecord struct {
	EngineVersion RollbackVersion        `json:"engineVersion"`
	ID            string                 `json:"id"`
	CharacterID   string                 `json:"characterId"`
	Target        SupervisorTarget       `json:"target"`
	Version       int                    `json:"version"`
	SnapshotID    string                 `json:"snapshotId,omitempty"`
	DecisionID    string                 `json:"decisionId,omitempty"`
	CreatedAt     time.Time              `json:"createdAt"`
	RolledBackAt  time.Time              `json:"rolledBackAt,omitempty"`
	RolledBackBy  string                 `json:"rolledBackBy,omitempty"`
	DerivedIDs    []string               `json:"derivedIds,omitempty"`
	Metadata      map[string]string      `json:"metadata,omitempty"`
}

type CompensationEvent struct {
	EngineVersion RollbackVersion      `json:"engineVersion"`
	ID            string               `json:"id"`
	CharacterID   string               `json:"characterId"`
	Target        CompensationTarget   `json:"target"`
	SourceVersion int                  `json:"sourceVersion"`
	TargetVersion int                  `json:"targetVersion"`
	Reason        string               `json:"reason"`
	CreatedAt     time.Time            `json:"createdAt"`
	Processed     bool                 `json:"processed"`
	ProcessedAt   time.Time            `json:"processedAt,omitempty"`
	DerivedEvents []string             `json:"derivedEvents,omitempty"`
	Metadata      map[string]string    `json:"metadata,omitempty"`
}

type VersionHistory struct {
	EngineVersion  RollbackVersion  `json:"engineVersion"`
	CharacterID    string           `json:"characterId"`
	Target         SupervisorTarget `json:"target"`
	Records        []VersionRecord  `json:"records"`
	CurrentVersion int              `json:"currentVersion"`
}

type RollbackPlan struct {
	EngineVersion   RollbackVersion        `json:"engineVersion"`
	ID              string                 `json:"id"`
	CharacterID     string                 `json:"characterId"`
	Target          SupervisorTarget       `json:"target"`
	FromVersion     int                    `json:"fromVersion"`
	ToVersion       int                    `json:"toVersion"`
	Compensations   []CompensationEvent    `json:"compensations"`
	Cascades        []CascadeInstruction   `json:"cascades"`
	CreatedAt       time.Time              `json:"createdAt"`
	Reason          string                 `json:"reason"`
}

type CascadeInstruction struct {
	EngineVersion RollbackVersion `json:"engineVersion"`
	TargetID      string          `json:"targetId"`
	Action        CascadeAction   `json:"action"`
	Reason        string          `json:"reason,omitempty"`
}

func NewVersionHistory(characterID string, target SupervisorTarget) VersionHistory {
	return VersionHistory{
		EngineVersion:  RollbackEngineVersion,
		CharacterID:    characterID,
		Target:         target,
		Records:        make([]VersionRecord, 0),
		CurrentVersion: 0,
	}
}

func (h *VersionHistory) Push(record VersionRecord) VersionHistory {
	updated := VersionHistory{
		EngineVersion:  h.EngineVersion,
		CharacterID:    h.CharacterID,
		Target:         h.Target,
		CurrentVersion: h.CurrentVersion + 1,
	}
	published := record
	published.Version = updated.CurrentVersion
	published.CharacterID = h.CharacterID
	published.Target = h.Target
	published.EngineVersion = RollbackEngineVersion
	if published.ID == "" {
		published.ID = versionRecordID(published)
	}
	updated.Records = append(append([]VersionRecord{}, h.Records...), published)
	return updated
}

func (h *VersionHistory) Latest() (VersionRecord, bool) {
	if len(h.Records) == 0 {
		return VersionRecord{}, false
	}
	return h.Records[len(h.Records)-1], true
}

func (h *VersionHistory) AtVersion(version int) (VersionRecord, bool) {
	for _, r := range h.Records {
		if r.Version == version {
			return r, true
		}
	}
	return VersionRecord{}, false
}

func (h *VersionHistory) ActiveVersions(now time.Time) []VersionRecord {
	result := make([]VersionRecord, 0)
	for _, r := range h.Records {
		if r.RolledBackAt.IsZero() {
			result = append(result, r)
		}
	}
	return result
}

func (h *VersionHistory) RolledBackVersions() []VersionRecord {
	result := make([]VersionRecord, 0)
	for _, r := range h.Records {
		if !r.RolledBackAt.IsZero() {
			result = append(result, r)
		}
	}
	return result
}

type VersionRollbackEngine struct {
	Config RollbackEngineConfig
}

type RollbackEngineConfig struct {
	Enabled                  bool
	MaxHistoryPerTarget      int
	RequireCompensation      bool
	AutoCascade              bool
	DefaultCascadeAction     CascadeAction
}

func DefaultRollbackEngineConfig() RollbackEngineConfig {
	return RollbackEngineConfig{
		Enabled:              true,
		MaxHistoryPerTarget:  50,
		RequireCompensation:  true,
		AutoCascade:          true,
		DefaultCascadeAction: CascadeActionInvalidate,
	}
}

func NewVersionRollbackEngine(config RollbackEngineConfig) *VersionRollbackEngine {
	return &VersionRollbackEngine{Config: config}
}

func (e *VersionRollbackEngine) PlanRollback(
	history VersionHistory,
	targetVersion int,
	reason string,
	derivedRecords []VersionRecord,
) RollbackPlan {
	latest, hasLatest := history.Latest()
	if !hasLatest {
		return RollbackPlan{}
	}

	fromVersion := latest.Version
	if targetVersion >= fromVersion {
		return RollbackPlan{}
	}

	compEvents := make([]CompensationEvent, 0)
	if e.Config.RequireCompensation {
		compEvents = append(compEvents, CompensationEvent{
			EngineVersion: RollbackEngineVersion,
			ID:            compensationEventID(history.CharacterID, string(history.Target), fromVersion, targetVersion),
			CharacterID:   history.CharacterID,
			Target:        mapSupervisorTargetToCompensation(history.Target),
			SourceVersion: fromVersion,
			TargetVersion: targetVersion,
			Reason:        reason,
			CreatedAt:     time.Now().UTC(),
		})
	}

	cascades := make([]CascadeInstruction, 0)
	if e.Config.AutoCascade {
		for _, derived := range derivedRecords {
			cascades = append(cascades, CascadeInstruction{
				EngineVersion: RollbackEngineVersion,
				TargetID:      derived.ID,
				Action:        e.Config.DefaultCascadeAction,
				Reason:        fmt.Sprintf("parent version %d rolled back to %d", fromVersion, targetVersion),
			})
		}
	}

	planID := rollbackPlanID(history.CharacterID, string(history.Target), fromVersion, targetVersion)

	return RollbackPlan{
		EngineVersion: RollbackEngineVersion,
		ID:            planID,
		CharacterID:   history.CharacterID,
		Target:        history.Target,
		FromVersion:   fromVersion,
		ToVersion:     targetVersion,
		Compensations: compEvents,
		Cascades:      cascades,
		CreatedAt:     time.Now().UTC(),
		Reason:        reason,
	}
}

func (e *VersionRollbackEngine) ExecuteRollback(
	history VersionHistory,
	targetVersion int,
	reason string,
	rollbackBy string,
	derivedRecords []VersionRecord,
) (VersionHistory, RollbackPlan, []CompensationEvent) {
	plan := e.PlanRollback(history, targetVersion, reason, derivedRecords)
	if plan.ID == "" {
		return history, plan, nil
	}

	updated := VersionHistory{
		EngineVersion:  history.EngineVersion,
		CharacterID:    history.CharacterID,
		Target:         history.Target,
		CurrentVersion: history.CurrentVersion,
	}
	updated.Records = make([]VersionRecord, len(history.Records))
	copy(updated.Records, history.Records)

	now := time.Now().UTC()
	for i, r := range updated.Records {
		if r.Version > targetVersion && r.Version <= plan.FromVersion && r.RolledBackAt.IsZero() {
			updated.Records[i].RolledBackAt = now
			updated.Records[i].RolledBackBy = rollbackBy
		}
	}

	return updated, plan, plan.Compensations
}

func (e *VersionRollbackEngine) TrimHistory(history VersionHistory) VersionHistory {
	if e.Config.MaxHistoryPerTarget <= 0 || len(history.Records) <= e.Config.MaxHistoryPerTarget {
		return history
	}
	excess := len(history.Records) - e.Config.MaxHistoryPerTarget
	trimmed := make([]VersionRecord, 0, e.Config.MaxHistoryPerTarget)
	for i, r := range history.Records {
		if i < excess && !r.RolledBackAt.IsZero() {
			continue
		}
		trimmed = append(trimmed, r)
	}
	if len(trimmed) > e.Config.MaxHistoryPerTarget {
		trimmed = trimmed[len(trimmed)-e.Config.MaxHistoryPerTarget:]
	}
	return VersionHistory{
		EngineVersion:  RollbackEngineVersion,
		CharacterID:    history.CharacterID,
		Target:         history.Target,
		Records:        trimmed,
		CurrentVersion: trimmed[len(trimmed)-1].Version,
	}
}

func (e *VersionRollbackEngine) BuildCompensationChain(
	sourceEvent CompensationEvent,
	derivedTargets []CompensationTarget,
) []CompensationEvent {
	events := make([]CompensationEvent, 0)
	now := time.Now().UTC()
	for _, dt := range derivedTargets {
		ev := CompensationEvent{
			EngineVersion: RollbackEngineVersion,
			ID:            compensationEventID(sourceEvent.CharacterID, string(dt), sourceEvent.SourceVersion, sourceEvent.TargetVersion),
			CharacterID:   sourceEvent.CharacterID,
			Target:        dt,
			SourceVersion: sourceEvent.SourceVersion,
			TargetVersion: sourceEvent.TargetVersion,
			Reason:        fmt.Sprintf("cascaded from %s rollback", sourceEvent.Target),
			CreatedAt:     now,
			DerivedEvents: []string{sourceEvent.ID},
		}
		events = append(events, ev)
	}
	return events
}

type RollbackSummary struct {
	EngineVersion             RollbackVersion    `json:"engineVersion"`
	TotalHistories            int                `json:"totalHistories"`
	TotalRecords              int                `json:"totalRecords"`
	ActiveRecords             int                `json:"activeRecords"`
	RolledBackRecords         int                `json:"rolledBackRecords"`
	PendingCompensations      int                `json:"pendingCompensations"`
	ProcessedCompensations    int                `json:"processedCompensations"`
}

func (e *VersionRollbackEngine) Summary(
	histories []VersionHistory,
	compensations []CompensationEvent,
) RollbackSummary {
	totalRecords := 0
	activeRecords := 0
	rolledBackRecords := 0
	now := time.Now().UTC()

	for _, h := range histories {
		totalRecords += len(h.Records)
		activeRecords += len(h.ActiveVersions(now))
		rolledBackRecords += len(h.RolledBackVersions())
	}

	pending := 0
	processed := 0
	for _, c := range compensations {
		if c.Processed {
			processed++
		} else {
			pending++
		}
	}

	return RollbackSummary{
		EngineVersion:          RollbackEngineVersion,
		TotalHistories:         len(histories),
		TotalRecords:           totalRecords,
		ActiveRecords:          activeRecords,
		RolledBackRecords:      rolledBackRecords,
		PendingCompensations:   pending,
		ProcessedCompensations: processed,
	}
}

func MarkCompensationProcessed(event CompensationEvent) CompensationEvent {
	event.Processed = true
	event.ProcessedAt = time.Now().UTC()
	return event
}

func MergeCompensationEvents(events []CompensationEvent) []CompensationEvent {
	seen := make(map[string]CompensationEvent)
	for _, e := range events {
		key := e.CharacterID + "|" + string(e.Target) + "|" + fmt.Sprint(e.SourceVersion)
		if existing, ok := seen[key]; ok {
			if !e.Processed && existing.Processed {
				seen[key] = existing
			} else {
				seen[key] = e
			}
		} else {
			seen[key] = e
		}
	}
	result := make([]CompensationEvent, 0, len(seen))
	for _, e := range seen {
		result = append(result, e)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result
}

func versionsBetween(history VersionHistory, from, to int) []VersionRecord {
	result := make([]VersionRecord, 0)
	for _, r := range history.Records {
		if r.Version > to && r.Version <= from {
			result = append(result, r)
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Version < result[j].Version
	})
	return result
}

func mapSupervisorTargetToCompensation(t SupervisorTarget) CompensationTarget {
	switch t {
	case SupervisorTargetPersonality:
		return CompensationTargetProfile
	case SupervisorTargetSummary:
		return CompensationTargetSummary
	case SupervisorTargetReflection:
		return CompensationTargetReflect
	case SupervisorTargetGrowth:
		return CompensationTargetGrowth
	default:
		return CompensationTargetBelief
	}
}

func versionRecordID(record VersionRecord) string {
	parts := []string{
		"ver",
		record.CharacterID,
		string(record.Target),
		fmt.Sprint(record.Version),
		record.SnapshotID,
		record.DecisionID,
	}
	raw := strings.Join(parts, "|")
	sum := sha256.Sum256([]byte(raw))
	return "version-" + hex.EncodeToString(sum[:])[:16]
}

func rollbackPlanID(characterID, target string, fromVersion, toVersion int) string {
	raw := fmt.Sprintf("rollback-plan|%s|%s|%d|%d|%d", characterID, target, fromVersion, toVersion, time.Now().UnixNano())
	sum := sha256.Sum256([]byte(raw))
	return "rollback-plan-" + hex.EncodeToString(sum[:])[:16]
}

func compensationEventID(characterID, target string, fromVersion, toVersion int) string {
	parts := []string{
		"comp",
		characterID,
		target,
		fmt.Sprint(fromVersion),
		fmt.Sprint(toVersion),
	}
	raw := strings.Join(parts, "|")
	sum := sha256.Sum256([]byte(raw))
	return "comp-" + hex.EncodeToString(sum[:])[:16]
}
