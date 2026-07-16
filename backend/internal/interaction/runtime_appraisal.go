package interaction

import (
	"strings"

	"github.com/u-ai/backend/internal/psyche/appraisal"
	"github.com/u-ai/backend/internal/psyche/budget"
)

type AppraisalResult struct {
	PsycheDelta       float64            `json:"psycheDelta"`
	RelationshipDelta float64            `json:"relationshipDelta"`
	NeedDeltas        map[string]float64 `json:"needDeltas,omitempty"`
	Severity          float64            `json:"severity"`
	EventType         string             `json:"eventType"`
	BudgetAllocated   float64            `json:"budgetAllocated"`
}

type AppraisalEventCategory string

const (
	AppraisalCatPraise        AppraisalEventCategory = "praise"
	AppraisalCatCold          AppraisalEventCategory = "cold"
	AppraisalCatHelp          AppraisalEventCategory = "help"
	AppraisalCatBoundaryCross AppraisalEventCategory = "boundary_cross"
	AppraisalCatApology       AppraisalEventCategory = "apology"
	AppraisalCatComplaint     AppraisalEventCategory = "complaint"
	AppraisalCatEmotional     AppraisalEventCategory = "emotional"
	AppraisalCatChat          AppraisalEventCategory = "chat"
)

type appraisalSensitivities struct {
	boundaryStrength  float64
	warmth            float64
	rejectionSens     float64
	affection         float64
	conflictAvoidance float64
}

func (p *RuntimePipeline) runAppraisal(snapshot ContextSnapshot, scope InteractionScope, req *ProcessRequest, path PathType) *AppraisalResult {
	if p.appraisalEngine == nil || req.IsInternal {
		return nil
	}
	sens := extractAppraisalSensitivities(snapshot)
	eventCat := classifyAppraisalEvent(req.Message, path)
	a := p.appraisalEngine.Evaluate(appraisal.AppraisalInput{
		EventType: string(eventCat), Source: "user_message", IsUserInitiated: true,
		RelatesToGoal:             semanticRelatesToGoal(req.Message, snapshot, eventCat),
		GoalCongruent:             semanticGoalCongruent(req.Message, snapshot, eventCat, sens),
		IsExpected:                semanticIsExpected(req.Message, snapshot, eventCat, sens),
		Controllable:              semanticControllable(req.Message, eventCat),
		Responsibility:            semanticResponsibility(req.Message, snapshot, eventCat, sens),
		Uncertainty:               semanticUncertainty(req.Message, snapshot, eventCat, sens),
		InvolvesRelation:          snapshot.Relationship.Status == LoadStatusReady && snapshot.Relationship.Value.Familiarity > 0.1,
		NormViolated:              semanticNormViolated(req.Message, snapshot, eventCat, sens),
		BoundaryViolated:          semanticBoundaryViolated(req.Message, snapshot, eventCat, sens),
		HasAlternativeExplanation: semanticHasAlternativeExplanation(req.Message, eventCat),
		SimilarPastEvents:         semanticSimilarPastCount(req.Message, snapshot, eventCat),
	})
	severity := budget.ComputeEventSeverity(a.OverallSeverity, a.GoalRelevance, a.NormViolation, a.BoundaryViolation)
	psyScale, relScale, needScales := eventCategoryScales(eventCat, sens)
	result := &AppraisalResult{PsycheDelta: (a.GoalCongruence - 0.5) * psyScale, RelationshipDelta: (a.RelationshipRelevance - 0.5) * relScale, Severity: severity, EventType: string(eventCat)}
	result.NeedDeltas = map[string]float64{"reassurance": (a.GoalCongruence - 0.5) * needScales["reassurance"], "connection": (a.RelationshipRelevance - 0.5) * needScales["connection"], "autonomy": (a.Controllability - 0.5) * needScales["autonomy"], "clarity": ((1.0 - a.CausalUncertainty) - 0.5) * needScales["clarity"], "novelty": (a.Novelty - 0.5) * needScales["novelty"], "expression": (a.Responsibility - 0.5) * needScales["expression"], "rest": -severity * 0.05}
	if p.budgetController != nil {
		candidates := []budget.CandidateDelta{{Module: "psyche", Delta: result.PsycheDelta, Priority: 1, Reason: "interaction_appraisal"}, {Module: "relationship", Delta: result.RelationshipDelta, Priority: 2, Reason: "interaction_appraisal"}}
		budgetResult := p.budgetController.Allocate(severity, candidates)
		result.BudgetAllocated = budgetResult.TotalAllocated
		for _, final := range budgetResult.FinalDeltas {
			if final.Module == "psyche" {
				result.PsycheDelta = final.Delta
			} else if final.Module == "relationship" {
				result.RelationshipDelta = final.Delta
			}
		}
		for _, rejected := range budgetResult.Rejected {
			if rejected.Module == "psyche" {
				result.PsycheDelta = 0
			} else if rejected.Module == "relationship" {
				result.RelationshipDelta = 0
			}
		}
	}
	return result
}

func extractAppraisalSensitivities(snapshot ContextSnapshot) appraisalSensitivities {
	sensitivities := appraisalSensitivities{boundaryStrength: 0.7, warmth: 0.5, rejectionSens: 0.5, affection: 0.45, conflictAvoidance: 0.5}
	if snapshot.RuntimeProfile.Status != LoadStatusReady {
		return sensitivities
	}
	config := snapshot.RuntimeProfile.Value.PersonalityConfig
	if config == nil {
		return sensitivities
	}
	sensitivities.boundaryStrength = extractSensFloat(config, "boundary", sensitivities.boundaryStrength)
	sensitivities.warmth = extractSensFloat(config, "warmth", sensitivities.warmth)
	sensitivities.affection = extractSensFloat(config, "affection", sensitivities.affection)
	sensitivities.conflictAvoidance = extractSensFloat(config, "conflictAvoidance", sensitivities.conflictAvoidance)
	directness := extractSensFloat(config, "directness", 0.5)
	sensitivities.rejectionSens = sensitivities.conflictAvoidance*0.6 + (1.0-directness)*0.4
	return sensitivities
}

func extractSensFloat(config map[string]interface{}, key string, defaultValue float64) float64 {
	value, ok := config[key]
	if !ok {
		return defaultValue
	}
	switch raw := value.(type) {
	case float64:
		if raw > 1 {
			return clampFloat(raw/100, 0, 1)
		}
		return clampFloat(raw, 0, 1)
	case int:
		value := float64(raw)
		if value > 1 {
			return clampFloat(value/100, 0, 1)
		}
		return clampFloat(value, 0, 1)
	case int64:
		value := float64(raw)
		if value > 1 {
			return clampFloat(value/100, 0, 1)
		}
		return clampFloat(value, 0, 1)
	default:
		return defaultValue
	}
}

func clampFloat(value, low, high float64) float64 {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func classifyAppraisalEvent(message string, path PathType) AppraisalEventCategory {
	if path == PathTypeDeep {
		return AppraisalCatEmotional
	}
	message = strings.ToLower(message)
	if containsAny(message, praiseMarkers) {
		return AppraisalCatPraise
	}
	if containsAny(message, apologyMarkers) {
		return AppraisalCatApology
	}
	if containsAny(message, boundaryCrossMarkers) {
		return AppraisalCatBoundaryCross
	}
	if containsAny(message, complaintMarkers) {
		return AppraisalCatComplaint
	}
	if containsAny(message, helpMarkers) {
		return AppraisalCatHelp
	}
	if containsAny(message, coldMarkers) {
		return AppraisalCatCold
	}
	if isEmotionalMessage(message) {
		return AppraisalCatEmotional
	}
	return AppraisalCatChat
}

var praiseMarkers = []string{"谢谢", "感谢", "很棒", "厉害", "优秀", "佩服", "真好", "太棒", "多谢", "thank", "great", "awesome", "amazing", "wonderful", "称赞", "表扬"}
var apologyMarkers = []string{"对不起", "抱歉", "我的错", "怪我", "不好意思", "sorry", "apologize", "原谅", "forgive", "我错了", "悔", "道歉"}
var boundaryCrossMarkers = []string{"爱你", "想见你", "私聊", "加好友", "私人", "私下", "单独", "love you", "private", "personal", "电话号码", "地址"}
var complaintMarkers = []string{"不满", "失望", "讨厌", "烦", "生气", "无语", "糟糕", "差劲", "hate", "disappointed", "angry", "upset", "抱怨", "投诉"}
var helpMarkers = []string{"帮我", "求助", "怎么办", "不知道", "教", "请问", "help", "need", "需要", "帮忙", "能不能", "可以吗", "建议", "advice"}
var coldMarkers = []string{"哦", "嗯", "行吧", "随便", "无所谓", "fine", "whatever", "k", "呵呵", "好吧"}

func eventCategoryScales(category AppraisalEventCategory, sensitivities appraisalSensitivities) (float64, float64, map[string]float64) {
	switch category {
	case AppraisalCatPraise:
		return 0.35, 0.40, map[string]float64{"reassurance": 0.35, "connection": 0.40, "autonomy": 0.15, "clarity": 0.20, "novelty": 0.10, "expression": 0.25}
	case AppraisalCatApology:
		return 0.45, 0.50, map[string]float64{"reassurance": 0.45, "connection": 0.50, "autonomy": 0.20, "clarity": 0.35, "novelty": 0.15, "expression": 0.40}
	case AppraisalCatComplaint:
		return 0.35, -0.40, map[string]float64{"reassurance": -0.35, "connection": -0.40, "autonomy": -0.10, "clarity": -0.25, "novelty": -0.10, "expression": -0.30}
	case AppraisalCatBoundaryCross:
		scale := 0.4 + sensitivities.boundaryStrength*0.4
		return scale, -scale, map[string]float64{"reassurance": -scale, "connection": -scale, "autonomy": -scale * 0.5, "clarity": -scale * 0.6, "novelty": -scale * 0.4, "expression": -scale * 0.5}
	case AppraisalCatCold:
		scale := 0.25 + sensitivities.rejectionSens*0.5
		return scale, -scale, map[string]float64{"reassurance": -scale, "connection": -scale, "autonomy": -0.10, "clarity": -0.15, "novelty": -0.05, "expression": -scale * 0.6}
	case AppraisalCatHelp:
		return 0.20, 0.25, map[string]float64{"reassurance": 0.20, "connection": 0.25, "autonomy": 0.10, "clarity": 0.30, "novelty": 0.15, "expression": 0.15}
	case AppraisalCatEmotional:
		scale := 0.30 + sensitivities.warmth*0.3
		return scale, scale * 0.8, map[string]float64{"reassurance": scale, "connection": scale * 0.8, "autonomy": 0.10, "clarity": 0.15, "novelty": scale * 0.5, "expression": scale}
	default:
		return 0.10, 0.10, map[string]float64{"reassurance": 0.10, "connection": 0.10, "autonomy": 0.05, "clarity": 0.08, "novelty": 0.08, "expression": 0.10}
	}
}
