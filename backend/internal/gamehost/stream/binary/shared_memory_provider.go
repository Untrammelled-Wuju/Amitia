package binary

import (
	"context"
	"io"
)

type SharedMemoryProvider struct {
	resolver SharedMemoryResolver
}

type SharedMemoryResolver interface {
	Map(ctx context.Context, name string, size int64) (SharedMemoryRegion, error)
	Unmap(region SharedMemoryRegion) error
}

type SharedMemoryRegion interface {
	Bytes() []byte
	Size() int64
	Name() string
}

func NewSharedMemoryProvider(resolver SharedMemoryResolver) (*SharedMemoryProvider, error) {
	if resolver == nil {
		return nil, ErrUnsupportedPlatform
	}
	return &SharedMemoryProvider{resolver: resolver}, nil
}

func DefaultSharedMemoryProvider() (*SharedMemoryProvider, error) {
	resolver, err := newDefaultSharedMemoryResolver()
	if err != nil {
		return nil, err
	}
	return &SharedMemoryProvider{resolver: resolver}, nil
}

func (p *SharedMemoryProvider) Kind() BinaryStorageKind {
	return BinaryStorageSharedMemory
}

func (p *SharedMemoryProvider) Create(
	ctx context.Context,
	owner BinaryOwner,
	request CreateRequest,
) (WritingHandle, error) {
	if err := owner.Validate(); err != nil {
		return WritingHandle{}, err
	}

	name := generateSharedMemoryName(owner, request)
	size := request.ExpectedSize
	if size <= 0 {
		size = 1024
	}

	handle := &sharedMemoryWritingHandle{
		owner:    owner,
		name:     name,
		size:     size,
		provider: p,
	}
	return handle.toPublicHandle(), nil
}

func (p *SharedMemoryProvider) Resolve(
	ctx context.Context,
	owner BinaryOwner,
	ref BinaryReference,
) (ResolvedBinary, error) {
	if err := owner.Validate(); err != nil {
		return ResolvedBinary{}, err
	}

	region, err := p.resolver.Map(ctx, ref.ID.String(), ref.Size)
	if err != nil {
		return ResolvedBinary{}, ErrObjectNotFound
	}

	reader := &sharedMemoryReader{region: region}
	return ResolvedBinary{
		Reference: ref,
		Reader:    reader,
	}, nil
}

func (p *SharedMemoryProvider) Release(
	ctx context.Context,
	owner BinaryOwner,
	id BinaryObjectID,
) error {
	_ = ctx
	_ = owner
	_ = id
	return nil
}

func (p *SharedMemoryProvider) Shutdown(ctx context.Context) error {
	return nil
}

func generateSharedMemoryName(owner BinaryOwner, request CreateRequest) string {
	return string(owner.RuntimeID) + "_" + string(owner.ServiceID) + "_" + NewBinaryObjectID().String()
}

type sharedMemoryWritingHandle struct {
	owner    BinaryOwner
	name     string
	size     int64
	region   SharedMemoryRegion
	provider *SharedMemoryProvider
}

func (h *sharedMemoryWritingHandle) toPublicHandle() WritingHandle {
	return WritingHandle{
		ObjectID: BinaryObjectID(h.name),
		Writer:   &sharedMemoryWriter{handle: h},
		Seal: func(actualSize int64, checksum *Checksum) (BinaryReference, error) {
			return BinaryReference{
				ID:       BinaryObjectID(h.name),
				Kind:     BinaryStorageSharedMemory,
				Size:     actualSize,
				MediaType: "",
				Checksum:  checksum,
				Lifetime:  BinaryLifetimeMessage,
			}, nil
		},
	}
}

type sharedMemoryWriter struct {
	handle *sharedMemoryWritingHandle
}

func (w *sharedMemoryWriter) Write(p []byte) (int, error) {
	return 0, ErrUnsupportedPlatform
}

func (w *sharedMemoryWriter) Close() error {
	return nil
}

type sharedMemoryReader struct {
	region SharedMemoryRegion
	offset int
	io.Reader
}

func (r *sharedMemoryReader) Read(p []byte) (int, error) {
	if r.region == nil {
		return 0, io.EOF
	}
	data := r.region.Bytes()
	if r.offset >= len(data) {
		return 0, io.EOF
	}
	n := copy(p, data[r.offset:])
	r.offset += n
	return n, nil
}

func (r *sharedMemoryReader) Close() error {
	if r.region != nil {
		return nil
	}
	return nil
}
