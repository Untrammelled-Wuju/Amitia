package workspace

import (
	"context"
	"fmt"
	"time"
)

type SAFGrantStatus struct {
	Valid             bool
	Readable          bool
	Writable          bool
	ProviderAvailable bool
	RootExists        bool
}

type SAFDocumentRef struct {
	GrantID     string
	DocumentID  string
	Name        string
	MIMEType    string
	Flags       int64
	IsDirectory bool
}

type SAFStatResult struct {
	Name        string
	MIMEType    string
	SizeBytes   *int64
	ModifiedAt  *time.Time
	Flags       int64
	IsDirectory bool
	IsVirtual   bool
}

type SAFEntryRef struct {
	DocumentID  string
	Name        string
	MIMEType    string
	SizeBytes   *int64
	ModifiedAt  *time.Time
	Flags       int64
	IsDirectory bool
	IsVirtual   bool
}

type SAFCreateFileInput struct {
	ParentDocumentID string
	DisplayName      string
	MIMEType         string
}

type SAFCreateDirInput struct {
	ParentDocumentID string
	DisplayName      string
}

type SAFWriteSource struct {
	Stream   []byte
	Resource string
}

type SAFBridge interface {
	GrantStatus(ctx context.Context, grantID string) (SAFGrantStatus, error)

	Stat(ctx context.Context, grantID string, documentID string, name string) (SAFStatResult, error)
	List(ctx context.Context, grantID string, documentID string, limit int) ([]SAFEntryRef, string, error)
	Read(ctx context.Context, grantID string, documentID string, offset int64, maxBytes int64) ([]byte, string, bool, error)
	Write(ctx context.Context, grantID string, documentID string, targetName string, source SAFWriteSource, overwrite bool) (SAFStatResult, error)
	Mkdir(ctx context.Context, grantID string, input SAFCreateDirInput) (SAFStatResult, error)
	Rename(ctx context.Context, grantID string, documentID string, newName string) (SAFStatResult, error)
	Move(ctx context.Context, grantID string, documentID string, targetParentDocumentID string) (SAFStatResult, error)
	Copy(ctx context.Context, grantID string, documentID string, targetParentDocumentID string) (SAFStatResult, error)
	Delete(ctx context.Context, grantID string, documentID string) error

	ResolvePath(ctx context.Context, grantID string, relativePath string) (SAFDocumentRef, error)
	CreateFile(ctx context.Context, grantID string, input SAFCreateFileInput) (SAFStatResult, error)
}

type SAFNativeError struct {
	Code                string
	Message             string
	PermissionRevoked   bool
	ProviderUnavailable bool
	Retryable           bool
}

func (e SAFNativeError) Error() string {
	return fmt.Sprintf("saf native error [%s]: %s", e.Code, e.Message)
}

var _ error = SAFNativeError{}
