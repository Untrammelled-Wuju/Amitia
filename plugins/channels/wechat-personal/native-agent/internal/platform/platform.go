package platform

import "encoding/json"

type ClientStatus struct {
	Found          bool   `json:"found"`
	Running        bool   `json:"running"`
	PID            int    `json:"pid,omitempty"`
	Path           string `json:"path,omitempty"`
	Version        string `json:"version,omitempty"`
	Strategy       string `json:"strategy,omitempty"`
	WindowManaged  bool   `json:"windowManaged"`
	PreloadCapable bool   `json:"preloadCapable,omitempty"`
	Message        string `json:"message,omitempty"`
}

type DriverStatus struct {
	Available       bool            `json:"available"`
	Attached        bool            `json:"attached"`
	Kind            string          `json:"kind,omitempty"`
	Version         string          `json:"version,omitempty"`
	ClientVersion   string          `json:"clientVersion,omitempty"`
	VersionVerified bool            `json:"versionVerified"`
	Endpoint        string          `json:"endpoint,omitempty"`
	Capabilities    map[string]bool `json:"capabilities,omitempty"`
	Message         string          `json:"message,omitempty"`
}

type StartOptions struct {
	// DriverLibraryPath is a host-verified native companion path supplied by
	// the plugin service. Platform implementations may load it using their
	// native mechanism (for example LD_PRELOAD on Linux or controlled DLL
	// loading on Windows). The Agent never discovers arbitrary libraries.
	DriverLibraryPath string
}

type DriverRequest struct {
	Op      string          `json:"op"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

func ProbeDriverForClient(client ClientStatus) DriverStatus {
	return ProbeDriver(client)
}

func CallDriver(client ClientStatus, req DriverRequest) (json.RawMessage, error) {
	return callDriver(client, req)
}
