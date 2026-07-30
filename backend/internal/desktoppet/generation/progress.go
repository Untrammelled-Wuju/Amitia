package generation

var stageOrder = []AttemptStatus{
	AttemptStatusPending,
	AttemptStatusPreparingReference,
	AttemptStatusBuildingPrompt,
	AttemptStatusWaitingRateLimit,
	AttemptStatusSubmitting,
	AttemptStatusUnknownSubmission,
	AttemptStatusPolling,
	AttemptStatusResultReceived,
	AttemptStatusPersisting,
	AttemptStatusSucceeded,
}

var stageWeights = map[AttemptStatus]float64{
	AttemptStatusPending:            0.0,
	AttemptStatusPreparingReference: 0.05,
	AttemptStatusBuildingPrompt:     0.05,
	AttemptStatusWaitingRateLimit:   0.05,
	AttemptStatusSubmitting:         0.15,
	AttemptStatusUnknownSubmission:  0.0,
	AttemptStatusPolling:            0.30,
	AttemptStatusResultReceived:     0.10,
	AttemptStatusPersisting:         0.25,
	AttemptStatusSucceeded:          0.05,
}

func CalculateActionProgress(status AttemptStatus) int {
	if status == AttemptStatusSucceeded {
		return 100
	}
	var cumulative float64
	found := false
	for _, s := range stageOrder {
		if s == status {
			found = true
			break
		}
		cumulative += stageWeights[s]
	}
	if !found {
		return 0
	}
	progress := int(cumulative * 100)
	if progress > 99 {
		progress = 99
	}
	if progress < 0 {
		progress = 0
	}
	return progress
}

func CalculateActionProgressFromString(status string) int {
	return CalculateActionProgress(AttemptStatus(status))
}

func StageWeight(status AttemptStatus) float64 {
	if w, ok := stageWeights[status]; ok {
		return w
	}
	return 0
}

func CumulativeProgressBefore(status AttemptStatus) float64 {
	var cumulative float64
	for _, s := range stageOrder {
		if s == status {
			break
		}
		cumulative += stageWeights[s]
	}
	return cumulative
}
