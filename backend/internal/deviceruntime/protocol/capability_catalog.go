package protocol

import "time"

type CapabilityCatalogPayload struct {
	Revision     int64     `json:"revision"`
	Capabilities []string  `json:"capabilities"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type CapabilityCatalogAckPayload struct {
	Revision int64  `json:"revision"`
	Accepted bool   `json:"accepted"`
	Error    string `json:"error,omitempty"`
}
