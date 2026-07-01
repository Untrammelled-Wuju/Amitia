package system

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/mindruntime"
)

func (h *Handler) ShadowModeStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":          "shadow",
		"currentPhase":    string(mindruntime.ShadowPhaseInteraction),
		"phasesCompleted": []string{},
		"activeSince":     time.Now().UTC().Format(time.RFC3339),
		"rollbacks":       0,
	})
}

func (h *Handler) ShadowModeStart(c *gin.Context) {
	var body struct {
		Phase string `json:"phase"`
	}
	c.ShouldBindJSON(&body)
	if body.Phase == "" {
		body.Phase = string(mindruntime.ShadowPhaseInteraction)
	}
	c.JSON(http.StatusOK, gin.H{
		"started": true,
		"phase":   body.Phase,
		"startedAt": time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *Handler) ShadowModeStop(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"stopped":  true,
		"stoppedAt": time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *Handler) ShadowModePhaseAdvance(c *gin.Context) {
	phases := mindruntime.AllShadowPhases()
	currentPhase := c.Query("current")
	nextPhase := ""
	for i, p := range phases {
		if string(p) == currentPhase && i+1 < len(phases) {
			nextPhase = string(phases[i+1])
			break
		}
	}
	if nextPhase == "" {
		c.JSON(http.StatusOK, gin.H{
			"advanced": false,
			"message":  "no more phases",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"advanced":    true,
		"fromPhase":   currentPhase,
		"toPhase":     nextPhase,
	})
}

func (h *Handler) ShadowModeThresholds(c *gin.Context) {
	thresholds := mindruntime.DefaultAutoRollbackThresholds()
	c.JSON(http.StatusOK, gin.H{
		"thresholds": thresholds,
	})
}

func (h *Handler) ShadowModeUpdateThresholds(c *gin.Context) {
	var body struct {
		MaxErrorRate         float64 `json:"maxErrorRate"`
		MaxP95LatencyMs      int64   `json:"maxP95LatencyMs"`
		MaxDuplicateDeliveries int    `json:"maxDuplicateDeliveries"`
		MaxUnknownBacklog    int     `json:"maxUnknownBacklog"`
		MaxConsistencyDiffs  int     `json:"maxConsistencyDiffs"`
		MaxPostCancelSubmit  int     `json:"maxPostCancelSubmit"`
		MaxQueueAgeMs        int64   `json:"maxQueueAgeMs"`
	}
	c.ShouldBindJSON(&body)
	thresholds := mindruntime.DefaultAutoRollbackThresholds()
	if body.MaxErrorRate > 0 {
		thresholds.MaxErrorRate = body.MaxErrorRate
	}
	if body.MaxP95LatencyMs > 0 {
		thresholds.MaxP95Latency = time.Duration(body.MaxP95LatencyMs) * time.Millisecond
	}
	if body.MaxDuplicateDeliveries > 0 {
		thresholds.MaxDuplicateDeliveries = body.MaxDuplicateDeliveries
	}
	if body.MaxUnknownBacklog > 0 {
		thresholds.MaxUnknownBacklog = body.MaxUnknownBacklog
	}
	if body.MaxConsistencyDiffs > 0 {
		thresholds.MaxConsistencyDiffs = body.MaxConsistencyDiffs
	}
	if body.MaxPostCancelSubmit > 0 {
		thresholds.MaxPostCancelSubmit = body.MaxPostCancelSubmit
	}
	if body.MaxQueueAgeMs > 0 {
		thresholds.MaxQueueAge = time.Duration(body.MaxQueueAgeMs) * time.Millisecond
	}
	c.JSON(http.StatusOK, gin.H{
		"updated":    true,
		"thresholds": thresholds,
	})
}

func (h *Handler) ShadowModeCompare(c *gin.Context) {
	oldMetrics := mindruntime.ShadowMetrics{
		LatencyMs:      150,
		ErrorCount:     2,
		CancelCount:    1,
		QueueDepth:     10,
		DeliveryStatus: "delivered",
		SafetyScore:    0.95,
		ConsistencyDiffs: 0,
		UnknownBacklog: 0,
		DuplicateDeliveries: 0,
		QueueAgeMs:     500,
	}
	newMetrics := mindruntime.ShadowMetrics{
		LatencyMs:      120,
		ErrorCount:     1,
		CancelCount:    0,
		QueueDepth:     8,
		DeliveryStatus: "delivered",
		SafetyScore:    0.97,
		ConsistencyDiffs: 0,
		UnknownBacklog: 0,
		DuplicateDeliveries: 0,
		QueueAgeMs:     400,
	}
	comparison := mindruntime.CompareShadowResults(oldMetrics, newMetrics)
	c.JSON(http.StatusOK, gin.H{
		"comparison": comparison,
	})
}

func (h *Handler) ShadowModeRollbacks(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"rollbacks": []interface{}{},
	})
}

func (h *Handler) ShadowModeLoadSim(c *gin.Context) {
	config := mindruntime.DefaultLoadInjectorConfig()
	result := mindruntime.InjectLoad(config)
	c.JSON(http.StatusOK, gin.H{
		"result": result,
	})
}

func (h *Handler) ShadowModeLongitudinalSim(c *gin.Context) {
	config := mindruntime.DefaultLongitudinalSimConfig()
	config.Roles = []mindruntime.SimRoleConfig{
		{RoleID: "role-1", CharacterID: "char-1", Frequency: mindruntime.SimFreqHigh, PersonalityKind: "warm", Enabled: true, SafetyCap: 0.8},
		{RoleID: "role-2", CharacterID: "char-2", Frequency: mindruntime.SimFreqMedium, PersonalityKind: "cool", Enabled: true, SafetyCap: 0.9},
		{RoleID: "role-3", CharacterID: "char-3", Frequency: mindruntime.SimFreqLow, PersonalityKind: "neutral", Enabled: true, SafetyCap: 0.85},
	}
	result := mindruntime.RunLongitudinalSim(config)
	c.JSON(http.StatusOK, gin.H{
		"result": result,
	})
}
