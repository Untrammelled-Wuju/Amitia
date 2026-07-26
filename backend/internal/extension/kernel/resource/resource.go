package resource

import "time"

type ResourceRecord struct {
	ResourceID     string         `json:"resource_id"`
	ResourceType   ResourceType   `json:"resource_type"`
	Owner          ResourceOwner  `json:"owner"`
	RuntimeManager string         `json:"runtime_manager,omitempty"`
	StorageManager string         `json:"storage_manager,omitempty"`
	State          ResourceState  `json:"state"`
	DeleteStrategy DeleteStrategy `json:"delete_strategy"`
	Version        string         `json:"version,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      *time.Time     `json:"deleted_at,omitempty"`
	ExpiresAt      *time.Time     `json:"expires_at,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

func (r ResourceRecord) IsActive() bool {
	return r.State == StateActive
}

func (r ResourceRecord) IsTerminal() bool {
	return r.State.IsTerminal()
}

func (r ResourceRecord) IsOwnedBy(owner ResourceOwner) bool {
	return r.Owner.Equals(owner)
}

func (r ResourceRecord) IsExpired() bool {
	if r.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*r.ExpiresAt)
}
