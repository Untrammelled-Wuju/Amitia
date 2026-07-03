package mindruntime

import (
	"math/rand"
	"sort"
	"time"
)

type LoadProfile string

const (
	LoadProfileBurst     LoadProfile = "burst"
	LoadProfileSustained LoadProfile = "sustained"
	LoadProfileStep      LoadProfile = "step"
	LoadProfileChaotic   LoadProfile = "chaotic"
)

type FaultType string

const (
	FaultChannelOffline FaultType = "channel_offline"
	FaultDependencyFail FaultType = "dependency_fail"
	FaultTimeout        FaultType = "timeout"
	FaultPartition      FaultType = "partition"
)

type InjectedFault struct {
	FaultType FaultType `json:"faultType"`
	Target    string    `json:"target"`
	StartAt   time.Time `json:"startAt"`
	EndAt     time.Time `json:"endAt"`
	Active    bool      `json:"active"`
}

type LoadInjectorConfig struct {
	Profile       LoadProfile     `json:"profile"`
	Duration      time.Duration   `json:"duration"`
	BurstRate     int             `json:"burstRate"`
	BurstInterval time.Duration   `json:"burstInterval"`
	SustainedRPS  int             `json:"sustainedRps"`
	StepIncrement int             `json:"stepIncrement"`
	StepInterval  time.Duration   `json:"stepInterval"`
	Faults        []InjectedFault `json:"faults"`
	BatchSize     int             `json:"batchSize"`
	IndexJobs     int             `json:"indexJobs"`
	DeleteJobs    int             `json:"deleteJobs"`
}

type LoadInjectionResult struct {
	Profile          LoadProfile           `json:"profile"`
	StartedAt        time.Time             `json:"startedAt"`
	EndedAt          time.Time             `json:"endedAt"`
	TotalMessages    int                   `json:"totalMessages"`
	PeakRPS          int                   `json:"peakRps"`
	FaultsTriggered  int                   `json:"faultsTriggered"`
	BatchOpsComplete int                   `json:"batchOpsComplete"`
	TimeSeries       []LoadTimeSeriesPoint `json:"timeSeries"`
}

type LoadTimeSeriesPoint struct {
	Timestamp    time.Time `json:"timestamp"`
	MessageRate  int       `json:"messageRate"`
	QueueDepth   int       `json:"queueDepth"`
	ActiveFaults int       `json:"activeFaults"`
}

func DefaultLoadInjectorConfig() LoadInjectorConfig {
	return LoadInjectorConfig{
		Profile:       LoadProfileBurst,
		Duration:      5 * time.Minute,
		BurstRate:     100,
		BurstInterval: 10 * time.Second,
		SustainedRPS:  50,
		StepIncrement: 10,
		StepInterval:  30 * time.Second,
		Faults:        make([]InjectedFault, 0),
		BatchSize:     500,
		IndexJobs:     3,
		DeleteJobs:    2,
	}
}

func InjectLoad(config LoadInjectorConfig) LoadInjectionResult {
	now := time.Now().UTC()
	result := LoadInjectionResult{
		Profile:    config.Profile,
		StartedAt:  now,
		TimeSeries: make([]LoadTimeSeriesPoint, 0),
	}

	rng := rand.New(rand.NewSource(now.UnixNano()))

	totalIntervals := int(config.Duration.Seconds())
	if totalIntervals < 1 {
		totalIntervals = 1
	}

	totalMessages := 0
	peakRPS := 0
	faultsTriggered := 0
	currentQDepth := 0

	for i := 0; i < totalIntervals; i++ {
		second := now.Add(time.Duration(i) * time.Second)

		msgRate := computeLoadRate(config, i, totalIntervals, rng)
		if msgRate > peakRPS {
			peakRPS = msgRate
		}
		totalMessages += msgRate

		currentQDepth += msgRate
		drainRate := config.SustainedRPS / 2
		if drainRate < 1 {
			drainRate = 1
		}
		currentQDepth -= drainRate
		if currentQDepth < 0 {
			currentQDepth = 0
		}

		activeFaults := countActiveFaults(config.Faults, second)
		if activeFaults > 0 {
			faultsTriggered += activeFaults
		}

		point := LoadTimeSeriesPoint{
			Timestamp:    second,
			MessageRate:  msgRate,
			QueueDepth:   currentQDepth,
			ActiveFaults: activeFaults,
		}
		result.TimeSeries = append(result.TimeSeries, point)
	}

	result.EndedAt = now.Add(config.Duration)
	result.TotalMessages = totalMessages
	result.PeakRPS = peakRPS
	result.FaultsTriggered = faultsTriggered
	result.BatchOpsComplete = config.IndexJobs + config.DeleteJobs

	return result
}

func computeLoadRate(config LoadInjectorConfig, currentIdx, totalIntervals int, rng *rand.Rand) int {
	switch config.Profile {
	case LoadProfileBurst:
		burstPhase := int(config.BurstInterval.Seconds())
		if burstPhase < 1 {
			burstPhase = 1
		}
		if currentIdx%burstPhase == 0 {
			return config.BurstRate + rng.Intn(config.BurstRate/5)
		}
		return rng.Intn(5)

	case LoadProfileSustained:
		return config.SustainedRPS + rng.Intn(config.SustainedRPS/10)

	case LoadProfileStep:
		stepPhase := int(config.StepInterval.Seconds())
		if stepPhase < 1 {
			stepPhase = 1
		}
		stepNum := currentIdx / stepPhase
		rate := config.SustainedRPS + stepNum*config.StepIncrement
		return rate + rng.Intn(rate/10+1)

	case LoadProfileChaotic:
		return rng.Intn(config.BurstRate*2 + 1)

	default:
		return config.SustainedRPS
	}
}

func countActiveFaults(faults []InjectedFault, now time.Time) int {
	count := 0
	for _, f := range faults {
		if f.Active && (f.StartAt.IsZero() || !now.Before(f.StartAt)) && (f.EndAt.IsZero() || now.Before(f.EndAt)) {
			count++
		}
	}
	return count
}

func BuildBurstFaults(baseTime time.Time) []InjectedFault {
	return []InjectedFault{
		{FaultType: FaultChannelOffline, Target: "wechat", StartAt: baseTime.Add(10 * time.Second), EndAt: baseTime.Add(20 * time.Second), Active: true},
		{FaultType: FaultDependencyFail, Target: "qdrant", StartAt: baseTime.Add(30 * time.Second), EndAt: baseTime.Add(45 * time.Second), Active: true},
		{FaultType: FaultTimeout, Target: "llm", StartAt: baseTime.Add(50 * time.Second), EndAt: baseTime.Add(60 * time.Second), Active: true},
		{FaultType: FaultPartition, Target: "redis", StartAt: baseTime.Add(70 * time.Second), EndAt: baseTime.Add(90 * time.Second), Active: true},
	}
}

func BuildSustainedFaults(baseTime time.Time) []InjectedFault {
	return []InjectedFault{
		{FaultType: FaultDependencyFail, Target: "surrealdb", StartAt: baseTime.Add(60 * time.Second), EndAt: baseTime.Add(120 * time.Second), Active: true},
		{FaultType: FaultTimeout, Target: "embedding", StartAt: baseTime.Add(120 * time.Second), EndAt: baseTime.Add(150 * time.Second), Active: true},
	}
}

func SortFaultsByStart(faults []InjectedFault) []InjectedFault {
	sorted := make([]InjectedFault, len(faults))
	copy(sorted, faults)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].StartAt.Before(sorted[j].StartAt)
	})
	return sorted
}
