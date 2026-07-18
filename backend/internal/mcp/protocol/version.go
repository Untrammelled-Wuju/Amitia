package protocol

import "slices"

const LatestProtocolVersion = "2025-11-25"

var SupportedProtocolVersions = []string{
	LatestProtocolVersion,
	"2025-06-18",
	"2025-03-26",
	"2024-11-05",
}

func SupportsVersion(version string) bool {
	return slices.Contains(SupportedProtocolVersions, version)
}

type Implementation struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Version string `json:"version"`
}

type ClientCapabilities struct {
	Experimental map[string]any `json:"experimental,omitempty"`
	Roots        map[string]any `json:"roots,omitempty"`
	Sampling     map[string]any `json:"sampling,omitempty"`
	Elicitation  map[string]any `json:"elicitation,omitempty"`
	Tasks        map[string]any `json:"tasks,omitempty"`
}

type InitializeParams struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ClientCapabilities `json:"capabilities"`
	ClientInfo      Implementation     `json:"clientInfo"`
}

type InitializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ServerInfo      Implementation `json:"serverInfo"`
	Instructions    string         `json:"instructions,omitempty"`
}

type ProgressParams struct {
	ProgressToken any     `json:"progressToken"`
	Progress      float64 `json:"progress"`
	Total         float64 `json:"total,omitempty"`
	Message       string  `json:"message,omitempty"`
}

type CancelledParams struct {
	RequestID any    `json:"requestId"`
	Reason    string `json:"reason,omitempty"`
}
