package workspace

import (
	"context"
	"io"
	"time"
)

type RemoteProtocol string

const (
	RemoteProtocolSFTP   RemoteProtocol = "sftp"
	RemoteProtocolWebDAV RemoteProtocol = "webdav"
)

type RemoteTLSMode string

const (
	RemoteTLSModeSystem         RemoteTLSMode = "system"
	RemoteTLSModePinned         RemoteTLSMode = "pinned"
	RemoteTLSModeInsecure       RemoteTLSMode = "insecure"
)

type RemoteHostKeyMode string

const (
	RemoteHostKeyModePinned       RemoteHostKeyMode = "pinned"
	RemoteHostKeyModeTOFU         RemoteHostKeyMode = "tofu"
)

type RemoteAuthType string

const (
	RemoteAuthTypePassword  RemoteAuthType = "password"
	RemoteAuthTypePrivateKey RemoteAuthType = "private_key"
	RemoteAuthTypeBearer    RemoteAuthType = "bearer"
)

type RemoteTLSConfig struct {
	Mode        RemoteTLSMode `json:"mode"`
	Certificate string        `json:"certificate,omitempty"`
}

type RemoteSSHConfig struct {
	HostKeyPolicy RemoteHostKeyConfig `json:"hostKeyPolicy,omitempty"`
	AllowInsecure bool                `json:"allowInsecure"`
}

type RemoteHostKeyConfig struct {
	Mode        RemoteHostKeyMode `json:"mode"`
	Fingerprint string            `json:"fingerprint,omitempty"`
}

type RemoteWebDAVConfig struct {
	AllowInsecureTransport bool `json:"allowInsecureTransport"`
}

type RemoteMountConfig struct {
	Protocol      RemoteProtocol     `json:"protocol"`
	Host          string             `json:"host"`
	Port          int                `json:"port,omitempty"`
	BasePath      string             `json:"basePath"`
	CredentialRef string             `json:"credentialRef,omitempty"`
	TLS           RemoteTLSConfig    `json:"tls,omitempty"`
	SSH           RemoteSSHConfig    `json:"ssh,omitempty"`
	WebDAV        RemoteWebDAVConfig `json:"webdav,omitempty"`
}

type RemoteCredential struct {
	Type       RemoteAuthType
	Username   string
	Password   []byte
	PrivateKey []byte
	Passphrase []byte
	BearerToken []byte
}

type RemoteCredentialResolver interface {
	ResolveCredential(ctx context.Context, ref string) (*RemoteCredential, error)
}

type RemotePath struct {
	RemotePath string
}

type RemotePathResolver interface {
	Resolve(config RemoteMountConfig, relativePath string) (RemotePath, error)
}

type RemoteStatResult struct {
	Name       string
	IsDir      bool
	IsSymlink  bool
	SizeBytes  int64
	ModifiedAt time.Time
	MIMEType   string
	Executable bool
}

type RemoteListResult struct {
	Entries    []RemoteStatResult
	NextCursor string
	HasMore    bool
}

type RemoteTransport interface {
	Stat(ctx context.Context, remotePath string) (RemoteStatResult, error)
	List(ctx context.Context, remotePath string, limit int) (RemoteListResult, error)
	Read(ctx context.Context, remotePath string, offset int64, maxBytes int64) (ReadResult, error)
	Write(ctx context.Context, remotePath string, src io.Reader, overwrite bool) error
	Mkdir(ctx context.Context, remotePath string) error
	Rename(ctx context.Context, oldPath string, newPath string) error
	Move(ctx context.Context, sourcePath string, destParentPath string) error
	Copy(ctx context.Context, sourcePath string, destParentPath string) error
	Delete(ctx context.Context, remotePath string, recursive bool) error
	Close() error
	RemoteRoot() string
}
