package interaction

func convergeCoordinationAfterChildUpdate(ac *activeCoordination) bool {
	if ac == nil || ac.status.IsTerminal() || ac.status == CoordinationPaused || ac.cancelRequested {
		return false
	}
	if allTerminal(ac) {
		if ac.status == CoordinationAggregating {
			return false
		}
		ac.status = CoordinationAggregating
		return true
	}
	if ac.status != CoordinationWaiting {
		return false
	}
	for _, assignment := range ac.assignments {
		if assignment.Status == AssignmentWaiting {
			return false
		}
	}
	ac.status = CoordinationRunning
	return true
}
