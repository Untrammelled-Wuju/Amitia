package system

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/need"
	"github.com/u-ai/backend/internal/psyche"
	"github.com/u-ai/backend/internal/relationship"
	"gorm.io/gorm"
)

type PsycheSnapshotOutput struct {
	Emotion      PsycheEmotionOutput            `json:"emotion"`
	Mood         PsycheMoodOutput               `json:"mood"`
	Stress       float64                        `json:"stress"`
	Energy       float64                        `json:"energy"`
	AffectLabel  string                         `json:"affectLabel"`
	Needs        map[string]float64             `json:"needs"`
	Beliefs      []beliefSnapshotEntry          `json:"beliefs"`
	Relationship relationship.RelationshipState `json:"relationship"`
	CollectedAt  string                         `json:"collectedAt"`
}

type PsycheEmotionOutput struct {
	Positive  float64   `json:"positive"`
	Negative  float64   `json:"negative"`
	Arousal   float64   `json:"arousal"`
	Dominance float64   `json:"dominance"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type PsycheMoodOutput struct {
	Valence   float64   `json:"valence"`
	Tension   float64   `json:"tension"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type beliefSnapshotEntry struct {
	Key        string  `json:"key"`
	Value      string  `json:"value"`
	Confidence float64 `json:"confidence"`
	Conflicted bool    `json:"conflicted"`
}

func RegisterPsycheSnapshotRouter(r *gin.RouterGroup, db *gorm.DB) {
	handler := newPsycheSnapshotHandler(db)
	r.GET("/psyche/state", handler.handle)
	r.GET("/psyche/snapshot", handler.handle)
}

type psycheSnapshotHandler struct {
	db *gorm.DB
}

func newPsycheSnapshotHandler(db *gorm.DB) *psycheSnapshotHandler {
	return &psycheSnapshotHandler{db: db}
}

func (h *psycheSnapshotHandler) handle(c *gin.Context) {
	characterID := c.Query("characterId")
	now := time.Now().UTC()

	if characterID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "characterId is required"})
		return
	}

	store := psyche.NewSQLitePsycheStore(h.db)
	state, err := store.LoadState(characterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "failed to load psyche state", "error": err.Error()})
		return
	}

	var collectedAt string
	if !state.UpdatedAt.IsZero() {
		collectedAt = state.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
	} else {
		collectedAt = now.Format("2006-01-02T15:04:05Z07:00")
	}

	valence := state.Emotion.Valence
	positive := valence
	negative := 1.0 - valence

	affectLabel := "平静"
	if valence > 0.55 {
		affectLabel = "积极"
	} else if valence < 0.45 {
		affectLabel = "消极"
	}
	if state.Stress > 0.55 {
		affectLabel = "紧张"
	}

	needDefault := need.DefaultSnapshot(now)
	needs := make(map[string]float64, len(needDefault.States))
	for kind, ns := range needDefault.States {
		needs[string(kind)] = ns.Level
	}

	relDefault := relationship.DefaultState()

	snapshot := PsycheSnapshotOutput{
		Emotion: PsycheEmotionOutput{
			Positive:  positive,
			Negative:  negative,
			Arousal:   state.Emotion.Arousal,
			Dominance: state.Emotion.Dominance,
			UpdatedAt: state.UpdatedAt,
		},
		Mood: PsycheMoodOutput{
			Valence:   state.Mood.MoodValence,
			Tension:   state.Mood.MoodArousal,
			UpdatedAt: state.UpdatedAt,
		},
		Stress:       state.Stress,
		Energy:       state.Energy,
		AffectLabel:  affectLabel,
		Needs:        needs,
		Beliefs:      []beliefSnapshotEntry{},
		Relationship: relDefault,
		CollectedAt:  collectedAt,
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": snapshot, "msg": "操作成功"})
}
