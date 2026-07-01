package system

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/affect"
	"github.com/u-ai/backend/internal/need"
	"github.com/u-ai/backend/internal/relationship"
)

type PsycheSnapshotOutput struct {
	Emotion      affect.EmotionState           `json:"emotion"`
	Mood         affect.MoodState              `json:"mood"`
	Stress       float64                       `json:"stress"`
	AffectLabel  string                        `json:"affectLabel"`
	Needs        map[string]float64            `json:"needs"`
	Beliefs      []beliefSnapshotEntry         `json:"beliefs"`
	Relationship relationship.RelationshipState `json:"relationship"`
	CollectedAt  string                        `json:"collectedAt"`
}

type beliefSnapshotEntry struct {
	Key         string  `json:"key"`
	Value       string  `json:"value"`
	Confidence  float64 `json:"confidence"`
	Conflicted  bool    `json:"conflicted"`
}

func RegisterPsycheSnapshotRouter(r *gin.RouterGroup) {
	r.GET("/psyche/snapshot", handlePsycheSnapshot)
}

func handlePsycheSnapshot(c *gin.Context) {
	now := time.Now().UTC()
	affectDefault := affect.DefaultState(now)
	needDefault := need.DefaultSnapshot(now)
	relDefault := relationship.DefaultState()

	needs := make(map[string]float64, len(needDefault.States))
	for kind, state := range needDefault.States {
		needs[string(kind)] = state.Level
	}

	snapshot := PsycheSnapshotOutput{
		Emotion:      affectDefault.Emotion,
		Mood:         affectDefault.Mood,
		Stress:       affectDefault.Stress,
		AffectLabel:  "平静",
		Needs:        needs,
		Beliefs:      []beliefSnapshotEntry{},
		Relationship: relDefault,
		CollectedAt:  now.Format("2006-01-02T15:04:05Z07:00"),
	}

	if snapshot.Emotion.Positive > 0.55 {
		snapshot.AffectLabel = "积极"
	} else if snapshot.Emotion.Negative > 0.55 {
		snapshot.AffectLabel = "消极"
	} else if snapshot.Stress > 0.55 {
		snapshot.AffectLabel = "紧张"
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": snapshot, "msg": "操作成功"})
}