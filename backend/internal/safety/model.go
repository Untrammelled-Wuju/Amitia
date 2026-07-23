package safety

import "gorm.io/gorm"

type HardConstraint struct {
	RuleID       string `json:"ruleId"`
	CandidateKey string `json:"candidateKey"`
	Reason       string `json:"reason"`
	Severity     string `json:"severity"`
}

type SoftPreference struct {
	Dimension        string  `json:"dimension"`
	RawScore         float64 `json:"rawScore"`
	NormalizedWeight float64 `json:"normalizedWeight"`
	Contribution     float64 `json:"contribution"`
}

type CopingStrategy struct {
	Selected        string   `json:"selected"`
	Alternatives    []string `json:"alternatives"`
	SelectionReason string   `json:"selectionReason"`
}

type EmotionExpression struct {
	DisplayMode       string `json:"displayMode"`
	InternalIntensity int    `json:"internalIntensity"`
	DisplayIntensity  int    `json:"displayIntensity"`
	OverrideReason    string `json:"overrideReason"`
}

type BdiConfig struct {
	HardConstraints   []HardConstraint   `json:"hardConstraints"`
	SoftPreferences   []SoftPreference   `json:"softPreferences"`
	CopingStrategy    *CopingStrategy    `json:"copingStrategy"`
	EmotionExpression *EmotionExpression `json:"emotionExpression"`
}

type AuditLog struct {
	ID     uint   `json:"id" gorm:"primaryKey"`
	Time   string `json:"time"`
	RuleID string `json:"ruleId"`
	Action string `json:"action"`
}

type SafetyConfig struct {
	PreventEmotionalBlackmail        bool   `json:"preventEmotionalBlackmail"`
	PreventExclusiveDependency       bool   `json:"preventExclusiveDependency"`
	PreventRealityIsolation          bool   `json:"preventRealityIsolation"`
	PreventPunitiveExpression        bool   `json:"preventPunitiveExpression"`
	PreventPretendingHuman           bool   `json:"preventPretendingHuman"`
	PreventSensitiveProactiveMention bool   `json:"preventSensitiveProactiveMention"`
	RestrictAdultContent             bool   `json:"restrictAdultContent"`
	NegativeEmotionCap               int    `json:"negativeEmotionCap"`
	IntimacyExpressionCap            int    `json:"intimacyExpressionCap"`
	ViolationAction                  string `json:"violationAction"`
	AuditLogRetentionDays            int    `json:"auditLogRetentionDays"`
}

type Handler struct {
	db *gorm.DB
}

func NewHandler(db *gorm.DB) *Handler {
	return &Handler{db: db}
}
