// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package system

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/affect"
	"github.com/u-ai/backend/internal/belief"
	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/need"
	"github.com/u-ai/backend/internal/relationship"
	"github.com/u-ai/backend/pkg/comment/response"
	"github.com/u-ai/backend/pkg/util"
)

func RegisterPsycheAPIRouter(r *gin.RouterGroup) {
	h := NewPsycheQueryHandler()

	psyche := r.Group("/psyche")
	{
		affectG := psyche.Group("/affect")
		affectG.POST("/compute", h.ComputeAffect)
		affectG.GET("/default", h.DefaultAffectState)
		affectG.POST("/pad-label", h.PADLabel)

		beliefG := psyche.Group("/belief")
		beliefG.POST("/resolve", h.ResolveBelief)
		beliefG.POST("/batch", h.ResolveBeliefBatch)

		needG := psyche.Group("/need")
		needG.POST("/update", h.UpdateNeeds)
		needG.GET("/default", h.DefaultNeedSnapshot)

		decisionG := psyche.Group("/decision")
		decisionG.POST("/score", h.ScoreBehavior)
		decisionG.POST("/select", h.SelectBehavior)

		relG := psyche.Group("/relationship")
		relG.POST("/update", h.UpdateRelationship)
		relG.GET("/default", h.DefaultRelationshipState)
	}
}

type PsycheQueryHandler struct{}

func NewPsycheQueryHandler() *PsycheQueryHandler {
	return &PsycheQueryHandler{}
}

func (h *PsycheQueryHandler) ComputeAffect(c *gin.Context) {
	var req struct {
		Current     affect.AffectState          `json:"current"`
		Personality affect.PersonalityReference `json:"personality"`
		Appraisal   affect.EventAppraisal       `json:"appraisal"`
		Budget      affect.ChangeBudget         `json:"budget"`
		Now         time.Time                   `json:"now"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数错误", err.Error())
		return
	}
	now := req.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	input := affect.EngineInput{
		Current:     req.Current,
		Personality: req.Personality,
		Appraisal:   req.Appraisal,
		Budget:      req.Budget,
		Now:         now,
	}
	output := affect.ComputeNextState(input)
	c.JSON(http.StatusOK, gin.H{
		"code": response.OK,
		"msg":  "操作成功",
		"data": output,
	})
}

func (h *PsycheQueryHandler) DefaultAffectState(c *gin.Context) {
	state := affect.DefaultState(time.Now().UTC())
	c.JSON(http.StatusOK, gin.H{
		"code": response.OK,
		"msg":  "操作成功",
		"data": state,
	})
}

func (h *PsycheQueryHandler) PADLabel(c *gin.Context) {
	var req struct {
		Pleasure  float64 `json:"pleasure"`
		Arousal   float64 `json:"arousal"`
		Dominance float64 `json:"dominance"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数错误", err.Error())
		return
	}
	label := affect.PADLabel(req.Pleasure, req.Arousal, req.Dominance)
	c.JSON(http.StatusOK, gin.H{
		"code": response.OK,
		"msg":  "操作成功",
		"data": gin.H{"label": label},
	})
}

func (h *PsycheQueryHandler) ResolveBelief(c *gin.Context) {
	var req struct {
		Key        string                `json:"key"`
		Candidates []belief.Candidate    `json:"candidates"`
		Policy     belief.ResolverPolicy `json:"policy"`
		Now        time.Time             `json:"now"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数错误", err.Error())
		return
	}
	now := req.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	policy := req.Policy
	if policy.MinimumConfidence == 0 && policy.MaxCandidates == 0 {
		policy = belief.DefaultPolicy()
	}
	input := belief.ResolveInput{
		Key:        req.Key,
		Candidates: req.Candidates,
		Policy:     policy,
		Now:        now,
	}
	result := belief.ResolveBelief(input)
	c.JSON(http.StatusOK, gin.H{
		"code": response.OK,
		"msg":  "操作成功",
		"data": result,
	})
}

func (h *PsycheQueryHandler) ResolveBeliefBatch(c *gin.Context) {
	var req belief.BatchInput
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数错误", err.Error())
		return
	}
	for i := range req.Beliefs {
		if req.Beliefs[i].Policy.MinimumConfidence == 0 && req.Beliefs[i].Policy.MaxCandidates == 0 {
			req.Beliefs[i].Policy = belief.DefaultPolicy()
		}
		if req.Beliefs[i].Now.IsZero() {
			req.Beliefs[i].Now = time.Now().UTC()
		}
	}
	result := belief.ResolveBatch(req)
	c.JSON(http.StatusOK, gin.H{
		"code": response.OK,
		"msg":  "操作成功",
		"data": result,
	})
}

func (h *PsycheQueryHandler) UpdateNeeds(c *gin.Context) {
	var req struct {
		Current     need.NeedSnapshot   `json:"current"`
		Signals     []need.NeedSignal   `json:"signals,omitempty"`
		Personality need.PersonalityRef `json:"personality"`
		Budget      need.ChangeBudget   `json:"budget"`
		Now         time.Time           `json:"now"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数错误", err.Error())
		return
	}
	now := req.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	budget := req.Budget
	if budget.MaxLevelDelta == 0 {
		budget = need.DefaultBudget()
	}
	input := need.UpdateInput{
		Current:     req.Current,
		Signals:     req.Signals,
		Personality: req.Personality,
		Budget:      budget,
		Now:         now,
	}
	result := need.UpdateNeeds(input)
	c.JSON(http.StatusOK, gin.H{
		"code": response.OK,
		"msg":  "操作成功",
		"data": result,
	})
}

func (h *PsycheQueryHandler) DefaultNeedSnapshot(c *gin.Context) {
	snapshot := need.DefaultSnapshot(time.Now().UTC())
	c.JSON(http.StatusOK, gin.H{
		"code": response.OK,
		"msg":  "操作成功",
		"data": snapshot,
	})
}

func (h *PsycheQueryHandler) ScoreBehavior(c *gin.Context) {
	var req struct {
		Candidates []decision.BehaviorCandidate    `json:"candidates"`
		Options    decision.BehaviorScoringOptions `json:"options"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数错误", err.Error())
		return
	}
	options := req.Options
	if options.BaseWeight == 0 && options.PersonalityWeight == 0 {
		options = decision.DefaultBehaviorScoringOptions()
	}
	scored := decision.ScoreBehaviorCandidates(req.Candidates, options)
	c.JSON(http.StatusOK, gin.H{
		"code": response.OK,
		"msg":  "操作成功",
		"data": scored,
	})
}

func (h *PsycheQueryHandler) SelectBehavior(c *gin.Context) {
	var req struct {
		Candidates []decision.BehaviorCandidate `json:"candidates"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数错误", err.Error())
		return
	}
	now := time.Now().UTC()
	scored := make([]decision.BehaviorCandidate, len(req.Candidates))
	copy(scored, req.Candidates)
	hasV2 := false
	for _, cand := range scored {
		if cand.ScoringVersion == decision.BehaviorFormulaVersionV2 {
			hasV2 = true
			break
		}
	}
	if !hasV2 {
		for i := range scored {
			scored[i].ScoringVersion = decision.BehaviorFormulaVersionV2
		}
	}
	layer := decision.DefaultArbitrationLayer()
	arbResult, err := layer.Arbitrate(decision.ArbitrationInput{
		Candidates: scored,
		Filter:     decision.DefaultHardConstraintFilter(),
		Now:        now,
	})
	if err != nil {
		util.ErrorResponse(c, response.InvalidParams, "决策仲裁失败", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": response.OK,
		"msg":  "操作成功",
		"data": arbResult,
	})
}

func (h *PsycheQueryHandler) UpdateRelationship(c *gin.Context) {
	var req relationship.UpdateInput
	if err := c.ShouldBindJSON(&req); err != nil {
		util.ErrorResponse(c, response.InvalidParams, "请求参数错误", err.Error())
		return
	}
	if req.Budget.MaxPositiveDelta == 0 && req.Budget.MaxNegativeDelta == 0 {
		req.Budget = relationship.DefaultBudget()
	}
	result := relationship.UpdateRelationship(req)
	c.JSON(http.StatusOK, gin.H{
		"code": response.OK,
		"msg":  "操作成功",
		"data": result,
	})
}

func (h *PsycheQueryHandler) DefaultRelationshipState(c *gin.Context) {
	state := relationship.DefaultState()
	c.JSON(http.StatusOK, gin.H{
		"code": response.OK,
		"msg":  "操作成功",
		"data": state,
	})
}
