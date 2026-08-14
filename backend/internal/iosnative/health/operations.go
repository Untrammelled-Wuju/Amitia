package health

const (
	OpHealthAuthorizationStatus  = "health.authorization.status"
	OpHealthAuthorizationRequest = "health.authorization.request"
	OpHealthProfileRead          = "health.profile.read"
	OpHealthSamplesQuery         = "health.samples.query"
	OpHealthStatisticsQuery      = "health.statistics.query"
	OpHealthWorkoutsQuery        = "health.workouts.query"
	OpHealthWorkoutsDetail       = "health.workouts.detail"
	OpHealthSleepQuery           = "health.sleep.query"
	OpHealthActivityQuery        = "health.activity.query"
)

func Operations() []string {
	return []string{
		OpHealthAuthorizationStatus,
		OpHealthAuthorizationRequest,
		OpHealthProfileRead,
		OpHealthSamplesQuery,
		OpHealthStatisticsQuery,
		OpHealthWorkoutsQuery,
		OpHealthWorkoutsDetail,
		OpHealthSleepQuery,
		OpHealthActivityQuery,
	}
}
