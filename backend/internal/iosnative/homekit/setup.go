package homekit

const (
	SetupStatusSuccess    = "success"
	SetupStatusFailed     = "failed"
	SetupStatusCancelled  = "cancelled"
	SetupStatusPending    = "pending"
)

type SetupSession struct {
	SessionID string `json:"sessionId"`

	Status string `json:"status"`

	HomeID string `json:"homeId,omitempty"`
	RoomID string `json:"roomId,omitempty"`

	PairedAccessoryID string `json:"pairedAccessoryId,omitempty"`

	Error string `json:"error,omitempty"`
}

func NewSetupSession(homeID, roomID string) *SetupSession {
	return &SetupSession{
		Status: SetupStatusPending,
		HomeID: homeID,
		RoomID: roomID,
	}
}

func (s *SetupSession) MarkSuccess(accessoryID string) {
	s.Status = SetupStatusSuccess
	s.PairedAccessoryID = accessoryID
}

func (s *SetupSession) MarkFailed(err string) {
	s.Status = SetupStatusFailed
	s.Error = err
}

func (s *SetupSession) MarkCancelled() {
	s.Status = SetupStatusCancelled
}

func (s *SetupSession) IsComplete() bool {
	return s.Status != SetupStatusPending
}
