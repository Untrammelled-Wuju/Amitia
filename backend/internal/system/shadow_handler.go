package system

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/mindruntime"
)

func (h *Handler) ShadowModeStatus(c *gin.Context) {
	h.shadowMu.RLock()
	state := h.shadowState
	h.shadowMu.RUnlock()
	c.JSON(http.StatusOK, state)
}

func validShadowPhase(raw string) (mindruntime.ShadowPhase, bool) {
	for _, phase := range mindruntime.AllShadowPhases() {
		if string(phase) == raw {
			return phase, true
		}
	}
	return "", false
}

func (h *Handler) ShadowModeStart(c *gin.Context) {
	var body struct {
		Phase string `json:"phase"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.Phase == "" {
		body.Phase = string(mindruntime.ShadowPhaseInteraction)
	}
	phase, ok := validShadowPhase(body.Phase)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid shadow phase"})
		return
	}
	h.shadowMu.Lock()
	h.shadowState.CurrentPhase = phase
	h.shadowState.Status = mindruntime.ShadowModeShadow
	startedAt := time.Now().UTC()
	h.shadowState.ActiveSince = startedAt
	h.shadowMu.Unlock()
	c.JSON(http.StatusOK, gin.H{"started": true, "phase": phase, "startedAt": startedAt})
}

func (h *Handler) ShadowModeStop(c *gin.Context) {
	h.shadowMu.Lock()
	h.shadowState.Status = mindruntime.ShadowModeOff
	h.shadowMu.Unlock()
	c.JSON(http.StatusOK, gin.H{"stopped": true, "stoppedAt": time.Now().UTC()})
}

func (h *Handler) ShadowModePhaseAdvance(c *gin.Context) {
	h.shadowMu.Lock()
	defer h.shadowMu.Unlock()
	phases := mindruntime.AllShadowPhases()
	current := h.shadowState.CurrentPhase
	for i, phase := range phases {
		if phase == current && i+1 < len(phases) {
			h.shadowState.PhasesCompleted = append(h.shadowState.PhasesCompleted, current)
			h.shadowState.CurrentPhase = phases[i+1]
			c.JSON(http.StatusOK, gin.H{"advanced": true, "fromPhase": current, "toPhase": phases[i+1]})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"advanced": false, "message": "no more phases"})
}

func (h *Handler) ShadowModeThresholds(c *gin.Context) {
	h.shadowMu.RLock()
	thresholds := h.shadowState.Thresholds
	h.shadowMu.RUnlock()
	c.JSON(http.StatusOK, gin.H{"thresholds": thresholds})
}

func (h *Handler) ShadowModeUpdateThresholds(c *gin.Context) {
	var body struct {
		MaxErrorRate           *float64 `json:"maxErrorRate"`
		MaxP95LatencyMs        *int64   `json:"maxP95LatencyMs"`
		MaxDuplicateDeliveries *int     `json:"maxDuplicateDeliveries"`
		MaxUnknownBacklog      *int     `json:"maxUnknownBacklog"`
		MaxConsistencyDiffs    *int     `json:"maxConsistencyDiffs"`
		MaxPostCancelSubmit    *int     `json:"maxPostCancelSubmit"`
		MaxQueueAgeMs          *int64   `json:"maxQueueAgeMs"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.shadowMu.Lock()
	t := h.shadowState.Thresholds
	if body.MaxErrorRate != nil && *body.MaxErrorRate >= 0 {
		t.MaxErrorRate = *body.MaxErrorRate
	}
	if body.MaxP95LatencyMs != nil && *body.MaxP95LatencyMs >= 0 {
		t.MaxP95Latency = time.Duration(*body.MaxP95LatencyMs) * time.Millisecond
	}
	if body.MaxDuplicateDeliveries != nil && *body.MaxDuplicateDeliveries >= 0 {
		t.MaxDuplicateDeliveries = *body.MaxDuplicateDeliveries
	}
	if body.MaxUnknownBacklog != nil && *body.MaxUnknownBacklog >= 0 {
		t.MaxUnknownBacklog = *body.MaxUnknownBacklog
	}
	if body.MaxConsistencyDiffs != nil && *body.MaxConsistencyDiffs >= 0 {
		t.MaxConsistencyDiffs = *body.MaxConsistencyDiffs
	}
	if body.MaxPostCancelSubmit != nil && *body.MaxPostCancelSubmit >= 0 {
		t.MaxPostCancelSubmit = *body.MaxPostCancelSubmit
	}
	if body.MaxQueueAgeMs != nil && *body.MaxQueueAgeMs >= 0 {
		t.MaxQueueAge = time.Duration(*body.MaxQueueAgeMs) * time.Millisecond
	}
	h.shadowState.Thresholds = t
	h.shadowMu.Unlock()
	c.JSON(http.StatusOK, gin.H{"updated": true, "thresholds": t})
}

func (h *Handler) ShadowModeCompare(c *gin.Context) {
	var body struct {
		OldMetrics mindruntime.ShadowMetrics `json:"oldMetrics"`
		NewMetrics mindruntime.ShadowMetrics `json:"newMetrics"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	comparison := mindruntime.CompareShadowResults(body.OldMetrics, body.NewMetrics)
	h.shadowMu.Lock()
	stateBefore := h.shadowState
	rollback, event := mindruntime.CheckAutoRollback(stateBefore, body.NewMetrics, stateBefore.Thresholds)
	h.shadowState.Comparisons = append(h.shadowState.Comparisons, comparison)
	h.shadowState.MetricsSnapshot = body.NewMetrics
	if rollback {
		h.shadowState.Rollbacks = append(h.shadowState.Rollbacks, event)
		if event.ToStatus != "" {
			h.shadowState.Status = event.ToStatus
		}
	}
	h.shadowMu.Unlock()
	c.JSON(http.StatusOK, gin.H{"comparison": comparison, "autoRollback": rollback, "rollback": event})
}

func (h *Handler) ShadowModeRollbacks(c *gin.Context) {
	h.shadowMu.RLock()
	items := append([]mindruntime.ShadowRollbackEvent(nil), h.shadowState.Rollbacks...)
	h.shadowMu.RUnlock()
	c.JSON(http.StatusOK, gin.H{"rollbacks": items})
}

func (h *Handler) ShadowModeLoadSim(c *gin.Context) {
	var body struct {
		Profile         string `json:"profile"`
		DurationSeconds int    `json:"durationSeconds"`
		SustainedRPS    int    `json:"sustainedRps"`
		BurstRate       int    `json:"burstRate"`
	}
	_ = c.ShouldBindJSON(&body)
	cfg := mindruntime.DefaultLoadInjectorConfig()
	if body.Profile != "" {
		cfg.Profile = mindruntime.LoadProfile(body.Profile)
	}
	if body.DurationSeconds > 0 {
		if body.DurationSeconds > 300 {
			body.DurationSeconds = 300
		}
		cfg.Duration = time.Duration(body.DurationSeconds) * time.Second
	}
	if body.SustainedRPS > 0 {
		cfg.SustainedRPS = body.SustainedRPS
	}
	if body.BurstRate > 0 {
		cfg.BurstRate = body.BurstRate
	}
	c.JSON(http.StatusOK, gin.H{"result": mindruntime.InjectLoad(cfg)})
}

func (h *Handler) ShadowModeLongitudinalSim(c *gin.Context) {
	cfg := mindruntime.DefaultLongitudinalSimConfig()
	result := mindruntime.RunLongitudinalSim(cfg)
	c.JSON(http.StatusOK, gin.H{"result": result})
}
