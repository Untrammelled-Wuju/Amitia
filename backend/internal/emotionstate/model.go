package emotionstate

import (
	"math"
	"strings"
	"time"
)

const UserAffectTTL = 45 * time.Minute

type UserAffect struct {
	PrimaryEmotion      string    `json:"primary_emotion"`
	SecondaryEmotion    string    `json:"secondary_emotion,omitempty"`
	Intensity           float64   `json:"intensity"`
	Stress              float64   `json:"stress"`
	Need                string    `json:"need"`
	AdviceWanted        bool      `json:"advice_wanted"`
	Openness            float64   `json:"openness"`
	Severity            float64   `json:"severity"`
	PossibleConcealment bool      `json:"possible_concealment"`
	Confidence          float64   `json:"confidence"`
	Evidence            []string  `json:"evidence,omitempty"`
	UpdatedAt           time.Time `json:"updated_at,omitempty"`
}

func EmptyUserAffect() UserAffect {
	return NormalizeUserAffect(UserAffect{
		PrimaryEmotion: "neutral",
		Need:           "none",
		Openness:       0.5,
		Confidence:     0.5,
	})
}

func NormalizeUserAffect(state UserAffect) UserAffect {
	state.PrimaryEmotion = strings.TrimSpace(state.PrimaryEmotion)
	if state.PrimaryEmotion == "" {
		state.PrimaryEmotion = "neutral"
	}
	state.SecondaryEmotion = strings.TrimSpace(state.SecondaryEmotion)
	state.Need = strings.TrimSpace(state.Need)
	if state.Need == "" {
		state.Need = "none"
	}
	state.Intensity = clamp01(state.Intensity)
	state.Stress = clamp01(state.Stress)
	state.Openness = clamp01(state.Openness)
	state.Severity = clamp01(state.Severity)
	state.Confidence = clamp01(state.Confidence)
	state.Evidence = cleanStrings(state.Evidence, 12)
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = time.Now().UTC()
	}
	return state
}

func UserAffectFresh(state UserAffect, now time.Time) bool {
	if state.UpdatedAt.IsZero() {
		return false
	}
	return now.Sub(state.UpdatedAt) <= UserAffectTTL
}

type RelationshipEmotion struct {
	Concern        float64   `json:"concern"`
	Warmth         float64   `json:"warmth"`
	Hurt           float64   `json:"hurt"`
	Anger          float64   `json:"anger"`
	Disappointment float64   `json:"disappointment"`
	Jealousy       float64   `json:"jealousy"`
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
}

type RelationshipEmotionDelta struct {
	Concern        float64 `json:"concern"`
	Warmth         float64 `json:"warmth"`
	Hurt           float64 `json:"hurt"`
	Anger          float64 `json:"anger"`
	Disappointment float64 `json:"disappointment"`
	Jealousy       float64 `json:"jealousy"`
}

func NormalizeRelationshipEmotion(state RelationshipEmotion) RelationshipEmotion {
	state.Concern = clamp01(state.Concern)
	state.Warmth = clamp01(state.Warmth)
	state.Hurt = clamp01(state.Hurt)
	state.Anger = clamp01(state.Anger)
	state.Disappointment = clamp01(state.Disappointment)
	state.Jealousy = clamp01(state.Jealousy)
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = time.Now().UTC()
	}
	return state
}

func NormalizeRelationshipEmotionDelta(delta RelationshipEmotionDelta) RelationshipEmotionDelta {
	delta.Concern = clampSigned(delta.Concern, 0.18)
	delta.Warmth = clampSigned(delta.Warmth, 0.12)
	delta.Hurt = clampSigned(delta.Hurt, 0.16)
	delta.Anger = clampSigned(delta.Anger, 0.16)
	delta.Disappointment = clampSigned(delta.Disappointment, 0.16)
	delta.Jealousy = clampSigned(delta.Jealousy, 0.12)
	return delta
}

func (delta RelationshipEmotionDelta) IsZero() bool {
	return math.Abs(delta.Concern)+math.Abs(delta.Warmth)+math.Abs(delta.Hurt)+math.Abs(delta.Anger)+math.Abs(delta.Disappointment)+math.Abs(delta.Jealousy) < 0.000001
}

func ApplyRelationshipDelta(state RelationshipEmotion, delta RelationshipEmotionDelta, now time.Time) RelationshipEmotion {
	state = DecayRelationshipEmotion(state, now)
	delta = NormalizeRelationshipEmotionDelta(delta)
	state.Concern = clamp01(state.Concern + delta.Concern)
	state.Warmth = clamp01(state.Warmth + delta.Warmth)
	state.Hurt = clamp01(state.Hurt + delta.Hurt)
	state.Anger = clamp01(state.Anger + delta.Anger)
	state.Disappointment = clamp01(state.Disappointment + delta.Disappointment)
	state.Jealousy = clamp01(state.Jealousy + delta.Jealousy)
	state.UpdatedAt = now
	return state
}

func DecayRelationshipEmotion(state RelationshipEmotion, now time.Time) RelationshipEmotion {
	if state.UpdatedAt.IsZero() {
		state.UpdatedAt = now
		return state
	}
	hours := now.Sub(state.UpdatedAt).Hours()
	if hours <= 0 {
		return state
	}
	decay := func(value, halfLife float64) float64 {
		if value <= 0 || halfLife <= 0 {
			return clamp01(value)
		}
		return clamp01(value * math.Pow(0.5, hours/halfLife))
	}
	state.Concern = decay(state.Concern, 4)
	state.Hurt = decay(state.Hurt, 48)
	state.Anger = decay(state.Anger, 18)
	state.Disappointment = decay(state.Disappointment, 72)
	state.Jealousy = 0
	state.Warmth = state.Warmth * math.Pow(0.5, hours/(24*45))
	state.Warmth = clamp01(state.Warmth)
	state.UpdatedAt = now
	return state
}

type Signals struct {
	DeviceClass           string   `json:"device_class,omitempty"`
	VolumeRMSMean         float64  `json:"volume_rms_mean"`
	SpeechRateCharsPerSec float64  `json:"speech_rate_chars_per_sec"`
	UtteranceDurationMS   int64    `json:"utterance_duration_ms"`
	ResponseLatencyMS     int64    `json:"response_latency_ms"`
	InterruptionCount     int      `json:"interruption_count"`
	VolumeDeviation       float64  `json:"volume_deviation"`
	SpeechRateDeviation   float64  `json:"speech_rate_deviation"`
	Evidence              []string `json:"evidence,omitempty"`
}

type Baseline struct {
	VolumeRMSMean         float64 `json:"volume_rms_mean"`
	SpeechRateCharsPerSec float64 `json:"speech_rate_chars_per_sec"`
	SampleCount           int     `json:"sample_count"`
}

func ApplyBaseline(current Baseline, signals Signals) Baseline {
	if signals.VolumeRMSMean <= 0 && signals.SpeechRateCharsPerSec <= 0 {
		return current
	}
	if current.SampleCount <= 0 {
		return Baseline{
			VolumeRMSMean:         signals.VolumeRMSMean,
			SpeechRateCharsPerSec: signals.SpeechRateCharsPerSec,
			SampleCount:           1,
		}
	}
	nextSamples := current.SampleCount + 1
	weight := 1.0 / float64(nextSamples)
	if weight < 0.05 {
		weight = 0.05
	}
	current.VolumeRMSMean = current.VolumeRMSMean*(1-weight) + signals.VolumeRMSMean*weight
	current.SpeechRateCharsPerSec = current.SpeechRateCharsPerSec*(1-weight) + signals.SpeechRateCharsPerSec*weight
	current.SampleCount++
	return current
}

type State struct {
	UserID              string
	CharacterID         string
	UserAffect          UserAffect
	RelationshipEmotion RelationshipEmotion
	Signals             Signals
	Baseline            Baseline
	Version             int64
	UpdatedAt           time.Time
}

type Record struct {
	UserID                  string    `gorm:"column:user_id;primaryKey"`
	CharacterID             string    `gorm:"column:character_id;primaryKey"`
	UserAffectJSON          string    `gorm:"column:user_affect_json"`
	RelationshipEmotionJSON string    `gorm:"column:relationship_emotion_json"`
	SignalsJSON             string    `gorm:"column:signals_json"`
	BaselineJSON            string    `gorm:"column:baseline_json"`
	Version                 int64     `gorm:"column:version"`
	UpdatedAt               time.Time `gorm:"column:updated_at"`
}

func (Record) TableName() string {
	return "emotion_states"
}

func cleanStrings(values []string, limit int) []string {
	if limit <= 0 {
		limit = len(values)
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		result = append(result, value)
		if len(result) >= limit {
			break
		}
	}
	return result
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func clampSigned(value, limit float64) float64 {
	if value > limit {
		return limit
	}
	if value < -limit {
		return -limit
	}
	return value
}
