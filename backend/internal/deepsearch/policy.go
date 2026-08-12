package deepsearch

import "time"

type DeepSearchPolicy struct {
	MaxRounds          int
	MaxQueriesPerRound int
	MaxSearchCalls     int
	ResultsPerQuery    int
	MaxSources         int
	MaxTaskDuration    time.Duration
	MaxCheckpointBytes int
	MaxResultBytes     int
	MaxQueryChars      int
	MaxFocusAreas      int
	FocusHitThreshold  int
	MaxPerDomain       int
	EarlyStopNewSource int
}

func DefaultDeepSearchPolicy() DeepSearchPolicy {
	return DeepSearchPolicy{
		MaxRounds:          3,
		MaxQueriesPerRound: 4,
		MaxSearchCalls:     12,
		ResultsPerQuery:    8,
		MaxSources:         40,
		MaxTaskDuration:    5 * time.Minute,
		MaxCheckpointBytes: 512 * 1024,
		MaxResultBytes:     192 * 1024,
		MaxQueryChars:      2048,
		MaxFocusAreas:      8,
		FocusHitThreshold:  2,
		MaxPerDomain:       5,
		EarlyStopNewSource: 2,
	}
}

func (p DeepSearchPolicy) SanitizeRequest(req *DeepSearchRequest) {
	if req.MaxRounds <= 0 {
		req.MaxRounds = p.MaxRounds
	}
	if req.MaxRounds > 5 {
		req.MaxRounds = 5
	}
	if req.MaxQueriesPerRound <= 0 {
		req.MaxQueriesPerRound = p.MaxQueriesPerRound
	}
	if req.MaxQueriesPerRound > 6 {
		req.MaxQueriesPerRound = 6
	}
	if req.ResultsPerQuery <= 0 {
		req.ResultsPerQuery = p.ResultsPerQuery
	}
	if req.ResultsPerQuery > 20 {
		req.ResultsPerQuery = 20
	}
	if req.MaxSources <= 0 {
		req.MaxSources = p.MaxSources
	}
	if req.MaxSources > 100 {
		req.MaxSources = 100
	}
}
