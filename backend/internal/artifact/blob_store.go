package artifact

import (
	"context"
	"io"
)

type BlobInfo struct {
	Digest   BlobDigest
	SizeBytes int64
}

type BlobStore interface {
	Put(ctx context.Context, reader io.Reader, limit int64) (BlobInfo, error)
	Open(ctx context.Context, digest BlobDigest) (io.ReadSeekCloser, BlobInfo, error)
	Stat(ctx context.Context, digest BlobDigest) (BlobInfo, error)
	Delete(ctx context.Context, digest BlobDigest) error
}
