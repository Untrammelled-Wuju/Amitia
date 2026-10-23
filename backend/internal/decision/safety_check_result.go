package decision

type SafetyCheckResult struct {
	Passed          bool
	Blocked         bool
	Reason          string
	ConfidenceScore float64
}
