package mindruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

type SnapshotVersion string

const (
	SnapshotVersionV1 SnapshotVersion = "runtime-snapshot-v1"
)

type ReferenceKind string

const (
	ReferenceKindPersonality  ReferenceKind = "personality"
	ReferenceKindAppraisal    ReferenceKind = "appraisal"
	ReferenceKindPsyche       ReferenceKind = "psyche_state"
	ReferenceKindRelationship ReferenceKind = "relationship_state"
	ReferenceKindBehavior     ReferenceKind = "behavior_plan"
	ReferenceKindExpression   ReferenceKind = "expression_plan"
)

type TraceStage string

const (
	TraceStagePersonality  TraceStage = "personality"
	TraceStageAppraisal    TraceStage = "appraisal"
	TraceStagePsyche       TraceStage = "psyche_state"
	TraceStageRelationship TraceStage = "relationship_state"
	TraceStageBehavior     TraceStage = "behavior_plan"
	TraceStageExpression   TraceStage = "expression_plan"
)

type DiagnosticSeverity string

const (
	DiagnosticSeverityInfo    DiagnosticSeverity = "info"
	DiagnosticSeverityWarning DiagnosticSeverity = "warning"
)

type RuntimeReference struct {
	Kind    ReferenceKind `json:"kind"`
	ID      string        `json:"id,omitempty"`
	Version string        `json:"version,omitempty"`
	Summary string        `json:"summary,omitempty"`
}

type RuntimeDiagnostic struct {
	Severity DiagnosticSeverity `json:"severity"`
	Code     string             `json:"code"`
	Message  string             `json:"message"`
	Source   string             `json:"source,omitempty"`
}

type TraceFrame struct {
	Index       int                  `json:"index"`
	Stage       TraceStage           `json:"stage"`
	Reference   RuntimeReference     `json:"reference"`
	Diagnostics []RuntimeDiagnostic  `json:"diagnostics,omitempty"`
	Metadata    []TraceFrameMetadata `json:"metadata,omitempty"`
}

type TraceFrameMetadata struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type RuntimeSnapshotInput struct {
	UserID            string
	CharacterID       string
	InteractionID     string
	CreatedAt         time.Time
	PreviousVersion   int
	PersonalityRef    RuntimeReference
	AppraisalRef      RuntimeReference
	PsycheStateRef    RuntimeReference
	RelationshipRef   RuntimeReference
	BehaviorPlanRef   RuntimeReference
	ExpressionPlanRef RuntimeReference
	Frames            []TraceFrame
}

type RuntimeSnapshot struct {
	Version           SnapshotVersion     `json:"version"`
	ID                string              `json:"id"`
	UserID            string              `json:"userId,omitempty"`
	CharacterID       string              `json:"characterId,omitempty"`
	InteractionID     string              `json:"interactionId,omitempty"`
	StateVersion      int                 `json:"stateVersion"`
	CreatedAt         time.Time           `json:"createdAt"`
	PersonalityRef    RuntimeReference    `json:"personalityRef"`
	AppraisalRef      RuntimeReference    `json:"appraisalRef"`
	PsycheStateRef    RuntimeReference    `json:"psycheStateRef"`
	RelationshipRef   RuntimeReference    `json:"relationshipRef"`
	BehaviorPlanRef   RuntimeReference    `json:"behaviorPlanRef"`
	ExpressionPlanRef RuntimeReference    `json:"expressionPlanRef"`
	Trace             []TraceFrame        `json:"trace"`
	Diagnostics       []RuntimeDiagnostic `json:"diagnostics,omitempty"`
}

func BuildRuntimeSnapshot(input RuntimeSnapshotInput) RuntimeSnapshot {
	input.PersonalityRef.Kind = ReferenceKindPersonality
	input.AppraisalRef.Kind = ReferenceKindAppraisal
	input.PsycheStateRef.Kind = ReferenceKindPsyche
	input.RelationshipRef.Kind = ReferenceKindRelationship
	input.BehaviorPlanRef.Kind = ReferenceKindBehavior
	input.ExpressionPlanRef.Kind = ReferenceKindExpression

	diagnostics := collectReferenceDiagnostics(input)
	trace := BuildTraceFrames(input)
	stateVersion := input.PreviousVersion + 1
	if stateVersion < 1 {
		stateVersion = 1
	}

	snapshot := RuntimeSnapshot{
		Version:           SnapshotVersionV1,
		UserID:            input.UserID,
		CharacterID:       input.CharacterID,
		InteractionID:     input.InteractionID,
		StateVersion:      stateVersion,
		CreatedAt:         input.CreatedAt.UTC(),
		PersonalityRef:    input.PersonalityRef,
		AppraisalRef:      input.AppraisalRef,
		PsycheStateRef:    input.PsycheStateRef,
		RelationshipRef:   input.RelationshipRef,
		BehaviorPlanRef:   input.BehaviorPlanRef,
		ExpressionPlanRef: input.ExpressionPlanRef,
		Trace:             trace,
		Diagnostics:       diagnostics,
	}
	snapshot.ID = snapshotID(snapshot)
	return snapshot
}

func BuildTraceFrames(input RuntimeSnapshotInput) []TraceFrame {
	frames := append([]TraceFrame{}, input.Frames...)
	if len(frames) == 0 {
		frames = []TraceFrame{
			frameFor(TraceStagePersonality, input.PersonalityRef),
			frameFor(TraceStageAppraisal, input.AppraisalRef),
			frameFor(TraceStagePsyche, input.PsycheStateRef),
			frameFor(TraceStageRelationship, input.RelationshipRef),
			frameFor(TraceStageBehavior, input.BehaviorPlanRef),
			frameFor(TraceStageExpression, input.ExpressionPlanRef),
		}
	}
	sort.SliceStable(frames, func(i, j int) bool {
		left := traceStageOrder(frames[i].Stage)
		right := traceStageOrder(frames[j].Stage)
		if left == right {
			if frames[i].Reference.ID == frames[j].Reference.ID {
				return string(frames[i].Stage) < string(frames[j].Stage)
			}
			return frames[i].Reference.ID < frames[j].Reference.ID
		}
		return left < right
	})
	for i := range frames {
		frames[i].Index = i + 1
	}
	return frames
}

func frameFor(stage TraceStage, ref RuntimeReference) TraceFrame {
	return TraceFrame{Stage: stage, Reference: ref}
}

func collectReferenceDiagnostics(input RuntimeSnapshotInput) []RuntimeDiagnostic {
	refs := []RuntimeReference{
		input.PersonalityRef,
		input.AppraisalRef,
		input.PsycheStateRef,
		input.RelationshipRef,
		input.BehaviorPlanRef,
		input.ExpressionPlanRef,
	}
	diagnostics := make([]RuntimeDiagnostic, 0)
	for _, ref := range refs {
		if strings.TrimSpace(ref.ID) == "" {
			source := string(ref.Kind)
			diagnostics = append(diagnostics, RuntimeDiagnostic{
				Severity: DiagnosticSeverityWarning,
				Code:     "missing_" + source + "_reference",
				Message:  source + " reference is missing",
				Source:   source,
			})
		}
	}
	return diagnostics
}

func traceStageOrder(stage TraceStage) int {
	switch stage {
	case TraceStagePersonality:
		return 10
	case TraceStageAppraisal:
		return 20
	case TraceStagePsyche:
		return 30
	case TraceStageRelationship:
		return 40
	case TraceStageBehavior:
		return 50
	case TraceStageExpression:
		return 60
	default:
		return 100
	}
}

func snapshotID(snapshot RuntimeSnapshot) string {
	parts := []string{
		string(snapshot.Version),
		snapshot.UserID,
		snapshot.CharacterID,
		snapshot.InteractionID,
		fmt.Sprint(snapshot.StateVersion),
		snapshot.CreatedAt.Format(time.RFC3339Nano),
		referenceKey(snapshot.PersonalityRef),
		referenceKey(snapshot.AppraisalRef),
		referenceKey(snapshot.PsycheStateRef),
		referenceKey(snapshot.RelationshipRef),
		referenceKey(snapshot.BehaviorPlanRef),
		referenceKey(snapshot.ExpressionPlanRef),
	}
	for _, frame := range snapshot.Trace {
		parts = append(parts, fmt.Sprintf("%03d:%s:%s", frame.Index, frame.Stage, referenceKey(frame.Reference)))
	}
	for _, diagnostic := range snapshot.Diagnostics {
		parts = append(parts, string(diagnostic.Severity)+":"+diagnostic.Code+":"+diagnostic.Source)
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return "runtime-snapshot-" + hex.EncodeToString(sum[:])[:16]
}

func referenceKey(ref RuntimeReference) string {
	return string(ref.Kind) + ":" + ref.ID + ":" + ref.Version + ":" + ref.Summary
}
