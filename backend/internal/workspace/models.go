package workspace

import (
	"time"
)

type WorkspaceID string

type WorkspaceKind string

const (
	WorkspaceKindLocal    WorkspaceKind = "local"
	WorkspaceKindSAF      WorkspaceKind = "saf"
	WorkspaceKindRemote   WorkspaceKind = "remote"
	WorkspaceKindIsolated WorkspaceKind = "isolated"
)

type WorkspaceStatus string

const (
	WorkspaceStatusReady             WorkspaceStatus = "ready"
	WorkspaceStatusReadOnly          WorkspaceStatus = "read_only"
	WorkspaceStatusUnavailable       WorkspaceStatus = "unavailable"
	WorkspaceStatusPermissionRevoked WorkspaceStatus = "permission_revoked"
	WorkspaceStatusMissing           WorkspaceStatus = "missing"
	WorkspaceStatusUnsupported       WorkspaceStatus = "unsupported"
	WorkspaceStatusDisabled          WorkspaceStatus = "disabled"
)

type WorkspaceMount struct {
	ID            WorkspaceID     `json:"id"`
	Name          string          `json:"name"`
	Kind          WorkspaceKind   `json:"kind"`
	ReadOnly      bool            `json:"readOnly"`
	Available     bool            `json:"available"`
	Status        WorkspaceStatus `json:"status"`
	StatusReason  string          `json:"statusReason,omitempty"`
	RootURI       string          `json:"rootUri"`
	NativeGrant   string          `json:"-"`
	LocalRoot     string          `json:"-"`
	BackendConfig string          `json:"-"`
	CredentialRef string          `json:"-"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
	LastUsedAt    time.Time       `json:"lastUsedAt"`
}

func (m WorkspaceMount) backendConfig() (string, bool) {
	return m.BackendConfig, m.BackendConfig != ""
}

type WorkspaceEntryType string

const (
	WorkspaceEntryTypeFile      WorkspaceEntryType = "file"
	WorkspaceEntryTypeDirectory WorkspaceEntryType = "directory"
)

type WorkspaceEntry struct {
	URI        string             `json:"uri"`
	Name       string             `json:"name"`
	Type       WorkspaceEntryType `json:"type"`
	MIMEType   string             `json:"mimeType,omitempty"`
	SizeBytes  *int64             `json:"sizeBytes,omitempty"`
	ModifiedAt *time.Time         `json:"modifiedAt,omitempty"`
	Readable   bool               `json:"readable"`
	Writable   bool               `json:"writable"`
}

type ListOptions struct {
	Limit  int
	Cursor string
}

type ReadOptions struct {
	Offset   int64
	MaxBytes int64
	Encoding string
}

type WriteOptions struct {
	Overwrite bool
	Atomic    bool
}

type DeleteOptions struct {
	Recursive bool
}

type ReadResult struct {
	Content  []byte
	Resource string
	IsText   bool
}

type ListResult struct {
	Entries    []WorkspaceEntry
	NextCursor string
	HasMore    bool
}

type persistenceRecord struct {
	id            string
	name          string
	kind          WorkspaceKind
	localRoot     string
	nativeGrant   string
	readOnly      bool
	enabled       bool
	createdAt     time.Time
	updatedAt     time.Time
	lastUsedAt    time.Time
	backendConfig string
	credentialRef string
}
