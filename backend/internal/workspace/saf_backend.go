package workspace

import (
	"context"
	"fmt"
	"io"
)

type SAFBackend struct{}

func NewSAFBackend() *SAFBackend {
	return &SAFBackend{}
}

func (b *SAFBackend) Kind() WorkspaceKind {
	return WorkspaceKindSAF
}

func (b *SAFBackend) Stat(ctx context.Context, mount WorkspaceMount, path string) (WorkspaceEntry, error) {
	return WorkspaceEntry{}, fmt.Errorf("%w: SAF native support not available", ErrSAFUnavailable)
}

func (b *SAFBackend) List(ctx context.Context, mount WorkspaceMount, path string, opts ListOptions) ([]WorkspaceEntry, error) {
	return nil, fmt.Errorf("%w: SAF native support not available", ErrSAFUnavailable)
}

func (b *SAFBackend) Read(ctx context.Context, mount WorkspaceMount, path string, opts ReadOptions) (ReadResult, error) {
	return ReadResult{}, fmt.Errorf("%w: SAF native support not available", ErrSAFUnavailable)
}

func (b *SAFBackend) Write(ctx context.Context, mount WorkspaceMount, path string, src io.Reader, opts WriteOptions) (WorkspaceEntry, error) {
	return WorkspaceEntry{}, fmt.Errorf("%w: SAF native support not available", ErrSAFUnavailable)
}

func (b *SAFBackend) Mkdir(ctx context.Context, mount WorkspaceMount, path string) (WorkspaceEntry, error) {
	return WorkspaceEntry{}, fmt.Errorf("%w: SAF native support not available", ErrSAFUnavailable)
}

func (b *SAFBackend) Rename(ctx context.Context, mount WorkspaceMount, path string, newName string) (WorkspaceEntry, error) {
	return WorkspaceEntry{}, fmt.Errorf("%w: SAF native support not available", ErrSAFUnavailable)
}

func (b *SAFBackend) Move(ctx context.Context, mount WorkspaceMount, source string, destinationDir string) (WorkspaceEntry, error) {
	return WorkspaceEntry{}, fmt.Errorf("%w: SAF native support not available", ErrSAFUnavailable)
}

func (b *SAFBackend) Copy(ctx context.Context, mount WorkspaceMount, source string, destinationDir string) (WorkspaceEntry, error) {
	return WorkspaceEntry{}, fmt.Errorf("%w: SAF native support not available", ErrSAFUnavailable)
}

func (b *SAFBackend) Delete(ctx context.Context, mount WorkspaceMount, path string, opts DeleteOptions) error {
	return fmt.Errorf("%w: SAF native support not available", ErrSAFUnavailable)
}
