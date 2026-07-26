package dependency

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type Phase string

const (
	PhaseInstall     Phase = "install"
	PhaseEnable      Phase = "enable"
	PhaseStart       Phase = "start"
	PhaseExecute     Phase = "execute"
	PhaseBuild       Phase = "build"
	PhaseDevelopment Phase = "development"
)

type TargetType string

const (
	TargetExtension    TargetType = "extension"
	TargetModule       TargetType = "module"
	TargetContribution TargetType = "contribution"
	TargetTool         TargetType = "tool"
	TargetWorkflow     TargetType = "workflow"
	TargetMCPServer    TargetType = "mcp_server"
	TargetProvider     TargetType = "provider"
	TargetRuntime      TargetType = "runtime"
	TargetHostFeature  TargetType = "host_feature"
	TargetPlatform     TargetType = "platform"
	TargetArchitecture TargetType = "architecture"
)

type ResolutionPolicy string

const (
	PolicyExact              ResolutionPolicy = "exact"
	PolicyHighestCompatible  ResolutionPolicy = "highest_compatible"
	PolicyLowestCompatible   ResolutionPolicy = "lowest_compatible"
	PolicyInstalledPreferred ResolutionPolicy = "installed_preferred"
	PolicySystemPreferred    ResolutionPolicy = "system_preferred"
	PolicyUserSelected       ResolutionPolicy = "user_selected"
	PolicyIsolated           ResolutionPolicy = "isolated"
)

type DependencyScope string

const (
	ScopeShared    DependencyScope = "shared"
	ScopeIsolated  DependencyScope = "isolated"
	ScopeExclusive DependencyScope = "exclusive"
)

type ConflictKind string

const (
	ConflictVersion             ConflictKind = "version_conflict"
	ConflictMissing             ConflictKind = "missing_dependency"
	ConflictCycle               ConflictKind = "dependency_cycle"
	ConflictPlatform            ConflictKind = "platform_conflict"
	ConflictArchitecture        ConflictKind = "architecture_conflict"
	ConflictHostFeatureMissing  ConflictKind = "host_feature_missing"
	ConflictExclusiveProvider   ConflictKind = "exclusive_provider_conflict"
	ConflictRuntime             ConflictKind = "runtime_conflict"
	ConflictOwner               ConflictKind = "owner_conflict"
	ConflictScopeIncompatible   ConflictKind = "scope_incompatible"
	ConflictDuplicateCapability ConflictKind = "duplicate_capability"
	ConflictDependencyDisabled  ConflictKind = "dependency_disabled"
	ConflictDependencyQuarantine ConflictKind = "dependency_quarantined"
)

type Request struct {
	SourceID    string
	Phase       Phase
	Type        TargetType
	Target      string
	VersionRange string
	Required    bool
	Scope       DependencyScope
	Policy      ResolutionPolicy
	Reason      string
}

type Candidate struct {
	TargetID     string
	Type         TargetType
	Version      domain.SemanticVersion
	Origin       string
	Owner        string
	Platform     string
	Trust        string
	Health       string
	Priority     int
	UserSelected bool
	Available    bool
}

type Conflict struct {
	Kind    ConflictKind
	Request Request
	Detail  string
}

type Warning struct {
	Request Request
	Message string
}

type Resolution struct {
	Request    Request
	Status     ResolutionStatus
	Selected   *Candidate
	Candidates []Candidate
	Conflicts  []Conflict
	Warnings   []Warning
}

type ResolutionStatus string

const (
	StatusResolved  ResolutionStatus = "resolved"
	StatusMissing   ResolutionStatus = "missing"
	StatusConflict  ResolutionStatus = "conflict"
	StatusDowngraded ResolutionStatus = "downgraded"
	StatusPending   ResolutionStatus = "pending"
)

type Edge struct {
	From     string
	To       string
	Phase    Phase
	Required bool
	Range    string
	Owner    string
}

type Node struct {
	ID    string
	Type  TargetType
	Owner string
}

type Graph struct {
	Nodes map[string]Node
	Edges []Edge
	Hash  string
}

type Snapshot struct {
	SnapshotID  string
	SourceID    string
	Resolutions []ResolutionRef
	GraphHash   string
	Generation  int64
	CreatedAt   time.Time
}

type ResolutionRef struct {
	RequestID string
	TargetID  string
	Version   string
	Status    ResolutionStatus
}

type AffectedSubject struct {
	SubjectID string
	Type      TargetType
	Phase     Phase
	Required  bool
}

type ResolveRequest struct {
	SourceID string
	Phase    Phase
	Requests []Request
}

type ResolveResult struct {
	SourceID    string
	Phase       Phase
	Resolutions []Resolution
	Conflicts   []Conflict
	Warnings    []Warning
	Graph       Graph
	Generation  int64
}

type Resolver interface {
	Resolve(ctx context.Context, request ResolveRequest) ResolveResult
	BuildGraph(ctx context.Context, roots []string) Graph
	Snapshot(ctx context.Context, sourceID string) (Snapshot, error)
	AffectedBy(ctx context.Context, targetID string) ([]AffectedSubject, error)
}

type CandidateProvider interface {
	FindCandidates(ctx context.Context, target string, targetType TargetType) ([]Candidate, error)
}

type CandidateProviderFunc func(ctx context.Context, target string, targetType TargetType) ([]Candidate, error)

func (f CandidateProviderFunc) FindCandidates(ctx context.Context, target string, targetType TargetType) ([]Candidate, error) {
	return f(ctx, target, targetType)
}
