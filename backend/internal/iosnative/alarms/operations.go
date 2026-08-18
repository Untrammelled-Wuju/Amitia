package alarms

const (
	OperationStatus               = "media.alarms.status"
	OperationAuthorizationStatus  = "media.alarms.authorization.status"
	OperationAuthorizationRequest = "media.alarms.authorization.request"
	OperationList                 = "media.alarms.list"
	OperationGet                  = "media.alarms.get"
	OperationSchedule             = "media.alarms.schedule"
	OperationStop                 = "media.alarms.stop"
	OperationCancel               = "media.alarms.cancel"
	// Internal legacy operation constants are retained for compatibility but are not advertised until a Live Activity widget extension exists.
	OperationCountdown = "media.alarms.countdown"
	OperationPause     = "media.alarms.pause"
	OperationResume    = "media.alarms.resume"
)

func Operations() []string {
	return []string{
		OperationStatus,
		OperationAuthorizationStatus,
		OperationAuthorizationRequest,
		OperationList,
		OperationGet,
		OperationSchedule,
		OperationStop,
		OperationCancel,
	}
}
