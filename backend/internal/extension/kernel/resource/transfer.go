package resource

import "time"

type TransferAction string

const (
	TransferAdopt  TransferAction = "adopt"
	TransferClone  TransferAction = "clone"
	TransferDetach TransferAction = "detach"
)

func (ta TransferAction) IsValid() bool {
	switch ta {
	case TransferAdopt, TransferClone, TransferDetach:
		return true
	}
	return false
}

type OwnershipTransferRequest struct {
	ResourceID string         `json:"resource_id"`
	FromOwner  ResourceOwner  `json:"from_owner"`
	ToOwner    ResourceOwner  `json:"to_owner"`
	Action     TransferAction `json:"action"`
	Reason     string         `json:"reason,omitempty"`
	CloneID    string         `json:"clone_id,omitempty"`
}

type OwnershipTransferRecord struct {
	TransferID string         `json:"transfer_id"`
	ResourceID string         `json:"resource_id"`
	FromOwner  ResourceOwner  `json:"from_owner"`
	ToOwner    ResourceOwner  `json:"to_owner"`
	Action     TransferAction `json:"action"`
	Reason     string         `json:"reason,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}
