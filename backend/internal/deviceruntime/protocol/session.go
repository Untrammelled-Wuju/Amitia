package protocol

type SessionStatus string

const (
	SessionStatusRegistering SessionStatus = "registering"
	SessionStatusSyncing     SessionStatus = "syncing"
	SessionStatusReady       SessionStatus = "ready"
	SessionStatusDegraded    SessionStatus = "degraded"
	SessionStatusClosing     SessionStatus = "closing"
	SessionStatusClosed      SessionStatus = "closed"
	SessionStatusSuperseded  SessionStatus = "superseded"

	RuntimeEventConnected    = "runtime.connected"
	RuntimeEventDisconnected = "runtime.disconnected"
	RuntimeEventHeartbeat    = "runtime.heartbeat"
)

func (s SessionStatus) IsActive() bool {
	switch s {
	case SessionStatusRegistering, SessionStatusSyncing, SessionStatusReady, SessionStatusDegraded:
		return true
	}
	return false
}

func (s SessionStatus) IsTerminal() bool {
	switch s {
	case SessionStatusClosed, SessionStatusSuperseded:
		return true
	}
	return false
}
