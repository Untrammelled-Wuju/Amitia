package mindruntime

import (
	"math/rand"
	"sort"
	"time"
)

type SimInteractionFrequency string

const (
	SimFreqHigh   SimInteractionFrequency = "high"
	SimFreqMedium SimInteractionFrequency = "medium"
	SimFreqLow    SimInteractionFrequency = "low"
)

type SimRoleConfig struct {
	RoleID          string                  `json:"roleId"`
	CharacterID     string                  `json:"characterId"`
	Frequency       SimInteractionFrequency `json:"frequency"`
	PersonalityKind string                  `json:"personalityKind"`
	Enabled         bool                    `json:"enabled"`
	SafetyCap       float64                 `json:"safetyCap"`
}

type SimUserBehavior struct {
	BehaviorID   string    `json:"behaviorId"`
	ActionType   string    `json:"actionType"`
	TargetRoleID string    `json:"targetRoleId"`
	Intensity    float64   `json:"intensity"`
	Timestamp    time.Time `json:"timestamp"`
	Channel      string    `json:"channel"`
	MessageLen   int       `json:"messageLen"`
	EmotionTag   string    `json:"emotionTag,omitempty"`
}

type LongitudinalSimConfig struct {
	SimDuration      time.Duration   `json:"simDuration"`
	StepInterval     time.Duration   `json:"stepInterval"`
	Roles            []SimRoleConfig `json:"roles"`
	BehaviorsPerStep int             `json:"behaviorsPerStep"`
	Seed             int64           `json:"seed"`
}

type SimStepResult struct {
	StepIndex    int               `json:"stepIndex"`
	SimTime      time.Time         `json:"simTime"`
	Behaviors    []SimUserBehavior `json:"behaviors"`
	ActiveRoles  int               `json:"activeRoles"`
	MessageCount int               `json:"messageCount"`
}

type LongitudinalSimResult struct {
	Config       SimConfigSnapshot `json:"config"`
	StartedAt    time.Time         `json:"startedAt"`
	EndedAt      time.Time         `json:"endedAt"`
	TotalSteps   int               `json:"totalSteps"`
	Steps        []SimStepResult   `json:"steps"`
	Aggregations SimAggregations   `json:"aggregations"`
	Parameters   CalibratedParams  `json:"parameters"`
}

type SimConfigSnapshot struct {
	Duration         time.Duration `json:"duration"`
	StepInterval     time.Duration `json:"stepInterval"`
	RoleCount        int           `json:"roleCount"`
	BehaviorsPerStep int           `json:"behaviorsPerStep"`
}

type SimAggregations struct {
	TotalMessages      int            `json:"totalMessages"`
	ByChannel          map[string]int `json:"byChannel"`
	ByRole             map[string]int `json:"byRole"`
	ByActionType       map[string]int `json:"byActionType"`
	ByFrequency        map[string]int `json:"byFrequency"`
	PeakMessagesStep   int            `json:"peakMessagesStep"`
	PeakMessagesCount  int            `json:"peakMessagesCount"`
	AvgMessagesPerStep float64        `json:"avgMessagesPerStep"`
}

func DefaultLongitudinalSimConfig() LongitudinalSimConfig {
	return LongitudinalSimConfig{
		SimDuration:      24 * time.Hour,
		StepInterval:     time.Hour,
		Roles:            make([]SimRoleConfig, 0),
		BehaviorsPerStep: 10,
		Seed:             42,
	}
}

func RunLongitudinalSim(config LongitudinalSimConfig) LongitudinalSimResult {
	rng := rand.New(rand.NewSource(config.Seed))

	startedAt := time.Now().UTC()
	simStart := startedAt

	totalSteps := int(config.SimDuration / config.StepInterval)
	if totalSteps < 1 {
		totalSteps = 1
	}

	steps := make([]SimStepResult, 0, totalSteps)
	byChannel := make(map[string]int)
	byRole := make(map[string]int)
	byActionType := make(map[string]int)
	byFrequency := make(map[string]int)
	totalMessages := 0
	peakStep := 0
	peakCount := 0

	for stepIdx := 0; stepIdx < totalSteps; stepIdx++ {
		simTime := simStart.Add(time.Duration(stepIdx) * config.StepInterval)
		activeRoles := activeRolesAt(config.Roles, simTime, rng)

		behaviors := generateSimBehaviors(activeRoles, config.BehaviorsPerStep, simTime, rng)

		stepResult := SimStepResult{
			StepIndex:    stepIdx,
			SimTime:      simTime,
			Behaviors:    behaviors,
			ActiveRoles:  len(activeRoles),
			MessageCount: len(behaviors),
		}
		steps = append(steps, stepResult)

		totalMessages += len(behaviors)
		if len(behaviors) > peakCount {
			peakCount = len(behaviors)
			peakStep = stepIdx
		}

		for _, b := range behaviors {
			byChannel[b.Channel]++
			byRole[b.TargetRoleID]++
			byActionType[b.ActionType]++
		}
		for _, r := range activeRoles {
			byFrequency[string(r.Frequency)]++
		}
	}

	calibrated := CalibrateParameters(CalibrationInput{
		ObservedMessageCount: totalMessages,
		ObservedPeakMessages: peakCount,
		ObservedDuration:     config.SimDuration,
		ActiveRoleCount:      len(config.Roles),
		DefaultConfig:        DefaultCalibrationConfig(),
	})

	endedAt := simStart.Add(config.SimDuration)

	return LongitudinalSimResult{
		Config: SimConfigSnapshot{
			Duration:         config.SimDuration,
			StepInterval:     config.StepInterval,
			RoleCount:        len(config.Roles),
			BehaviorsPerStep: config.BehaviorsPerStep,
		},
		StartedAt:  startedAt,
		EndedAt:    endedAt,
		TotalSteps: totalSteps,
		Steps:      steps,
		Aggregations: SimAggregations{
			TotalMessages:      totalMessages,
			ByChannel:          byChannel,
			ByRole:             byRole,
			ByActionType:       byActionType,
			ByFrequency:        byFrequency,
			PeakMessagesStep:   peakStep,
			PeakMessagesCount:  peakCount,
			AvgMessagesPerStep: safeDiv(float64(totalMessages), float64(totalSteps)),
		},
		Parameters: calibrated,
	}
}

func generateSimBehaviors(roles []SimRoleConfig, behaviorsPerStep int, simTime time.Time, rng *rand.Rand) []SimUserBehavior {
	if len(roles) == 0 {
		return nil
	}

	actionTypes := []string{"chat", "command", "correction", "reaction", "silence", "farewell"}
	channels := []string{"wechat", "qq", "web", "api"}
	emotions := []string{"neutral", "happy", "sad", "angry", "anxious", "excited", "calm"}

	count := len(roles) * behaviorsPerStep / 10
	if count < len(roles) {
		count = len(roles)
	}
	if count > 200 {
		count = 200
	}

	behaviors := make([]SimUserBehavior, 0, count)
	for i := 0; i < count; i++ {
		role := roles[rng.Intn(len(roles))]
		mult := freqMultiplier(role.Frequency, rng)
		if mult == 0 {
			continue
		}

		b := SimUserBehavior{
			BehaviorID:   simBehaviorID(role.RoleID, simTime, i),
			ActionType:   actionTypes[rng.Intn(len(actionTypes))],
			TargetRoleID: role.RoleID,
			Intensity:    rng.Float64(),
			Timestamp:    simTime,
			Channel:      channels[rng.Intn(len(channels))],
			MessageLen:   5 + rng.Intn(100),
			EmotionTag:   emotions[rng.Intn(len(emotions))],
		}
		behaviors = append(behaviors, b)
	}

	sort.SliceStable(behaviors, func(i, j int) bool {
		return behaviors[i].Timestamp.Before(behaviors[j].Timestamp)
	})
	return behaviors
}

func activeRolesAt(roles []SimRoleConfig, simTime time.Time, rng *rand.Rand) []SimRoleConfig {
	active := make([]SimRoleConfig, 0, len(roles))
	for _, r := range roles {
		if r.Enabled && rng.Float64() < 0.95 {
			active = append(active, r)
		}
	}
	return active
}

func freqMultiplier(freq SimInteractionFrequency, rng *rand.Rand) int {
	switch freq {
	case SimFreqHigh:
		return 1 + rng.Intn(3)
	case SimFreqMedium:
		if rng.Float64() < 0.7 {
			return 1
		}
		return 0
	case SimFreqLow:
		if rng.Float64() < 0.3 {
			return 1
		}
		return 0
	default:
		return 1
	}
}

func simBehaviorID(roleID string, simTime time.Time, idx int) string {
	ts := simTime.Format("20060102T150405")
	return "sim-behavior-" + roleID + "-" + ts + "-" + intToStr(idx)
}

func ComparePersonalityTemplates(templates []SimRoleConfig, simConfig LongitudinalSimConfig) []LongitudinalSimResult {
	results := make([]LongitudinalSimResult, 0, len(templates))
	for _, tmpl := range templates {
		cfg := simConfig
		cfg.Roles = []SimRoleConfig{tmpl}
		result := RunLongitudinalSim(cfg)
		results = append(results, result)
	}
	return results
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func intToStr(val int) string {
	if val == 0 {
		return "0"
	}
	neg := val < 0
	if neg {
		val = -val
	}
	digits := make([]byte, 0, 10)
	for val > 0 {
		digits = append([]byte{byte('0' + val%10)}, digits...)
		val /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
