package migration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

type LegacySourceReference struct {
	EntityType  LegacyEntityType
	LegacyID    string
	LegacyTable string
	RawJSON     json.RawMessage
}

type MigrationOwner struct {
	Type string
	ID   string
	Name string
}

type MigrationEnablement struct {
	Enabled        bool
	Reason         string
	ConflictSource string
}

type MigrationScopeBinding struct {
	Scope      string
	SubjectID  string
	SubjectType string
	State      string
	GrantedAt  *time.Time
}

type MigrationPermissionReference struct {
	Permission string
	SubjectID  string
	SubjectType string
	State      string
	Scope      string
}

type MigrationDependency struct {
	Type    string
	ID      string
	Version string
	Optional bool
}

type MigrationResourceReference struct {
	Type string
	Ref  string
	Hash string
	Size int64
}

type MigrationRuntimeHint struct {
	Kind        string
	DesiredState string
	LastSeen    *time.Time
	Reason       string
}

type MigrationEntity struct {
	MigrationID   string
	EntityType    string
	LegacySource  LegacySourceReference
	CanonicalID   string
	Owner         MigrationOwner
	Version       string
	Definition    json.RawMessage
	Enablement    MigrationEnablement
	Scope         []MigrationScopeBinding
	Permissions   []MigrationPermissionReference
	Dependencies  []MigrationDependency
	Resources     []MigrationResourceReference
	RuntimeHints  []MigrationRuntimeHint
	CollectedAt   time.Time
	Confidence    string
	Warnings      []string
	Errors        []string
}

type MigrationReport struct {
	ReportID      string
	SnapshotID    string
	StartedAt     time.Time
	CompletedAt   *time.Time
	Status        string
	TotalEntities int
	Converted     int
	Skipped       int
	Failed        int
	Conflicts     []MigrationConflict
	Entities      []MigrationEntity
	Summary       string
}

type MigrationConflict struct {
	Type        string
	Severity    string
	Subject     string
	Description string
	Resolution  string
	Evidence    map[string]any
}

type MigrationPlan struct {
	PlanID      string
	SnapshotID  string
	CreatedAt   time.Time
	Entities    []MigrationEntity
	Conflicts   []MigrationConflict
	Strategy    string
	EstimatedSteps int
}

type MigrationValidator interface {
	Validate(ctx context.Context, entity MigrationEntity) []MigrationConflict
}

type MigrationConflictDetector interface {
	Detect(ctx context.Context, entities []MigrationEntity) []MigrationConflict
}

type MigrationPlanner interface {
	Plan(ctx context.Context, snapshotID string, entities []MigrationEntity, conflicts []MigrationConflict) (MigrationPlan, error)
}

type DefaultValidator struct{}

func NewDefaultValidator() *DefaultValidator { return &DefaultValidator{} }

func (v *DefaultValidator) Validate(_ context.Context, entity MigrationEntity) []MigrationConflict {
	var conflicts []MigrationConflict
	if entity.CanonicalID == "" {
		conflicts = append(conflicts, MigrationConflict{
			Type: "missing_canonical_id", Severity: "blocking",
			Subject: entity.LegacySource.LegacyID, Description: "canonical id empty",
			Resolution: "regenerate canonical id",
		})
	}
	if entity.EntityType == "" {
		conflicts = append(conflicts, MigrationConflict{
			Type: "missing_entity_type", Severity: "blocking",
			Subject: entity.LegacySource.LegacyID, Description: "entity type empty",
		})
	}
	if len(entity.Definition) == 0 {
		conflicts = append(conflicts, MigrationConflict{
			Type: "missing_definition", Severity: "blocking",
			Subject: entity.LegacySource.LegacyID, Description: "definition empty",
		})
	}
	if entity.Owner.ID == "" {
		conflicts = append(conflicts, MigrationConflict{
			Type: "missing_owner", Severity: "high",
			Subject: entity.LegacySource.LegacyID, Description: "owner missing",
			Resolution: "assign system owner",
		})
	}
	if entity.Enablement.ConflictSource != "" {
		conflicts = append(conflicts, MigrationConflict{
			Type: "ambiguous_enablement", Severity: "high",
			Subject: entity.LegacySource.LegacyID, Description: "enablement conflict from " + entity.Enablement.ConflictSource,
			Resolution: "use repository value as canonical",
		})
	}
	for _, dep := range entity.Dependencies {
		if dep.ID == "" {
			conflicts = append(conflicts, MigrationConflict{
				Type: "missing_dependency_id", Severity: "medium",
				Subject: entity.LegacySource.LegacyID, Description: "dependency id empty",
			})
		}
	}
	return conflicts
}

var _ MigrationValidator = (*DefaultValidator)(nil)

type DefaultConflictDetector struct{}

func NewDefaultConflictDetector() *DefaultConflictDetector { return &DefaultConflictDetector{} }

func (d *DefaultConflictDetector) Detect(_ context.Context, entities []MigrationEntity) []MigrationConflict {
	var conflicts []MigrationConflict
	canonicalMap := make(map[string]string)
	legacyMap := make(map[string]string)
	for _, e := range entities {
		if existing, ok := canonicalMap[e.CanonicalID]; ok && existing != e.LegacySource.LegacyID {
			conflicts = append(conflicts, MigrationConflict{
				Type: "duplicate_canonical_id", Severity: "blocking",
				Subject: e.CanonicalID,
				Description: fmt.Sprintf("canonical id %s shared by %s and %s", e.CanonicalID, existing, e.LegacySource.LegacyID),
				Resolution: "namespace by source",
			})
		}
		canonicalMap[e.CanonicalID] = e.LegacySource.LegacyID
		if existing, ok := legacyMap[e.LegacySource.LegacyID]; ok && existing != e.CanonicalID {
			conflicts = append(conflicts, MigrationConflict{
				Type: "duplicate_legacy_id", Severity: "high",
				Subject: e.LegacySource.LegacyID,
				Description: fmt.Sprintf("legacy id %s maps to %s and %s", e.LegacySource.LegacyID, existing, e.CanonicalID),
			})
		}
		legacyMap[e.LegacySource.LegacyID] = e.CanonicalID
	}
	return conflicts
}

var _ MigrationConflictDetector = (*DefaultConflictDetector)(nil)

type DefaultPlanner struct {
	validator  MigrationValidator
	detector   MigrationConflictDetector
}

func NewDefaultPlanner(validator MigrationValidator, detector MigrationConflictDetector) *DefaultPlanner {
	return &DefaultPlanner{validator: validator, detector: detector}
}

func (p *DefaultPlanner) Plan(ctx context.Context, snapshotID string, entities []MigrationEntity, _ []MigrationConflict) (MigrationPlan, error) {
	if snapshotID == "" {
		return MigrationPlan{}, errors.New("migration: snapshot id required")
	}
	var allConflicts []MigrationConflict
	for _, e := range entities {
		conflicts := p.validator.Validate(ctx, e)
		allConflicts = append(allConflicts, conflicts...)
	}
	allConflicts = append(allConflicts, p.detector.Detect(ctx, entities)...)
	plan := MigrationPlan{
		PlanID:     newMigrationID("plan"),
		SnapshotID: snapshotID,
		CreatedAt:  time.Now().UTC(),
		Entities:   entities,
		Conflicts:  allConflicts,
		Strategy:   "sequential",
		EstimatedSteps: len(entities) + len(allConflicts),
	}
	sort.Slice(plan.Entities, func(i, j int) bool {
		return plan.Entities[i].EntityType < plan.Entities[j].EntityType
	})
	return plan, nil
}

var _ MigrationPlanner = (*DefaultPlanner)(nil)

type MigrationReportStore interface {
	Put(ctx context.Context, report MigrationReport) error
	Get(ctx context.Context, reportID string) (MigrationReport, error)
	List(ctx context.Context, snapshotID string) ([]MigrationReport, error)
}

type InMemoryReportStore struct {
	reports map[string]MigrationReport
}

func NewInMemoryReportStore() *InMemoryReportStore {
	return &InMemoryReportStore{reports: make(map[string]MigrationReport)}
}

func (s *InMemoryReportStore) Put(_ context.Context, report MigrationReport) error {
	s.reports[report.ReportID] = report
	return nil
}

func (s *InMemoryReportStore) Get(_ context.Context, reportID string) (MigrationReport, error) {
	r, ok := s.reports[reportID]
	if !ok {
		return MigrationReport{}, errors.New("migration: report not found")
	}
	return r, nil
}

func (s *InMemoryReportStore) List(_ context.Context, snapshotID string) ([]MigrationReport, error) {
	var out []MigrationReport
	for _, r := range s.reports {
		if r.SnapshotID == snapshotID {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.Before(out[j].StartedAt) })
	return out, nil
}

type MigrationService struct {
	gateway   LegacyReadOnlyGateway
	validator MigrationValidator
	detector  MigrationConflictDetector
	planner   MigrationPlanner
	reports   MigrationReportStore
}

func NewMigrationService(
	gateway LegacyReadOnlyGateway,
	validator MigrationValidator,
	detector MigrationConflictDetector,
	planner MigrationPlanner,
	reports MigrationReportStore,
) *MigrationService {
	return &MigrationService{
		gateway:   gateway,
		validator: validator,
		detector:  detector,
		planner:   planner,
		reports:   reports,
	}
}

func (s *MigrationService) Run(ctx context.Context, snapshotID string, entities []MigrationEntity) (MigrationReport, error) {
	report := MigrationReport{
		ReportID:   newMigrationID("report"),
		SnapshotID: snapshotID,
		StartedAt:  time.Now().UTC(),
		Status:     "running",
		TotalEntities: len(entities),
	}
	plan, err := s.planner.Plan(ctx, snapshotID, entities, nil)
	if err != nil {
		report.Status = "failed"
		completed := time.Now().UTC()
		report.CompletedAt = &completed
		report.Summary = err.Error()
		_ = s.reports.Put(ctx, report)
		return report, err
	}
	report.Entities = plan.Entities
	report.Conflicts = plan.Conflicts
	conflictByLegacy := groupConflictsByLegacy(plan.Conflicts)
	for _, e := range plan.Entities {
		if hasBlockingConflictForEntity(e, conflictByLegacy[e.LegacySource.LegacyID]) {
			report.Failed++
			continue
		}
		if e.CanonicalID == "" || len(e.Definition) == 0 {
			report.Failed++
			continue
		}
		report.Converted++
	}
	report.Skipped = report.TotalEntities - report.Converted - report.Failed
	completed := time.Now().UTC()
	report.CompletedAt = &completed
	if report.Failed > 0 {
		report.Status = "partial"
	} else {
		report.Status = "completed"
	}
	report.Summary = fmt.Sprintf("converted=%d skipped=%d failed=%d conflicts=%d",
		report.Converted, report.Skipped, report.Failed, len(report.Conflicts))
	_ = s.reports.Put(ctx, report)
	return report, nil
}

func groupConflictsByLegacy(conflicts []MigrationConflict) map[string][]MigrationConflict {
	out := make(map[string][]MigrationConflict)
	for _, c := range conflicts {
		out[c.Subject] = append(out[c.Subject], c)
	}
	return out
}

func hasBlockingConflictForEntity(e MigrationEntity, conflicts []MigrationConflict) bool {
	for _, c := range conflicts {
		if c.Severity == "blocking" {
			return true
		}
	}
	return false
}

func (s *MigrationService) Gateway() LegacyReadOnlyGateway {
	return s.gateway
}

func (s *MigrationService) Reports() MigrationReportStore {
	return s.reports
}
