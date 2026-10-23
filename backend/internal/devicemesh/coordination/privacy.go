package coordination

import "encoding/json"

type PrivacyMessage struct {
	ID          string          `json:"id"`
	RoleID      string          `json:"roleId"`
	OwnerID     string          `json:"ownerId"`
	Legacy      bool            `json:"legacy"`
	Revision    int64           `json:"revision"`
	Fingerprint string          `json:"fingerprint"`
	Imported    bool            `json:"imported"`
	Body        json.RawMessage `json:"body"`
	Scope       ExecutionScope  `json:"executionScope"`
}
