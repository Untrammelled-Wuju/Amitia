package protocol

type ConnectionState string

const (
	ConnectionStateHandshake ConnectionState = "handshake"
	ConnectionStateConnected ConnectionState = "connected"
	ConnectionStateDegraded  ConnectionState = "degraded"
	ConnectionStateClosing   ConnectionState = "closing"
	ConnectionStateClosed    ConnectionState = "closed"
)

func (s ConnectionState) String() string {
	return string(s)
}

func (s ConnectionState) IsValid() bool {
	switch s {
	case ConnectionStateHandshake, ConnectionStateConnected, ConnectionStateDegraded,
		ConnectionStateClosing, ConnectionStateClosed:
		return true
	}
	return false
}

func (s ConnectionState) IsTerminal() bool {
	switch s {
	case ConnectionStateClosed:
		return true
	}
	return false
}
