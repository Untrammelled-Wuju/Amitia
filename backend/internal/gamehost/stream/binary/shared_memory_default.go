package binary

import (
	"context"
	"fmt"
)

func newDefaultSharedMemoryResolver() (SharedMemoryResolver, error) {
	return nil, ErrUnsupportedPlatform
}

type unsupportedSharedMemoryResolver struct{}

func (r *unsupportedSharedMemoryResolver) Map(ctx context.Context, name string, size int64) (SharedMemoryRegion, error) {
	return nil, fmt.Errorf("%w: shared memory not available", ErrUnsupportedPlatform)
}

func (r *unsupportedSharedMemoryResolver) Unmap(region SharedMemoryRegion) error {
	return fmt.Errorf("%w: shared memory not available", ErrUnsupportedPlatform)
}
