package binary

import (
	"context"
	"encoding/json"
	"io"
)

type CreateRequest struct {
	ExpectedSize int64
	MediaType    string
	Metadata     map[string]json.RawMessage
}

type ResolvedBinary struct {
	Reference BinaryReference

	Reader io.ReadCloser
}

type BinaryProvider interface {
	Kind() BinaryStorageKind

	Create(
		ctx context.Context,
		owner BinaryOwner,
		request CreateRequest,
	) (WritingHandle, error)

	Resolve(
		ctx context.Context,
		owner BinaryOwner,
		ref BinaryReference,
	) (ResolvedBinary, error)

	Release(
		ctx context.Context,
		owner BinaryOwner,
		id BinaryObjectID,
	) error

	Shutdown(ctx context.Context) error
}

type WritingHandle struct {
	ObjectID BinaryObjectID
	Writer   io.WriteCloser
	Seal     func(actualSize int64, checksum *Checksum) (BinaryReference, error)
}
