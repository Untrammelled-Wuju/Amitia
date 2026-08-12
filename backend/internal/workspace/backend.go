package workspace

import (
	"context"
	"io"
)

type WorkspaceBackend interface {
	Kind() WorkspaceKind

	Stat(ctx context.Context, mount WorkspaceMount, path string) (WorkspaceEntry, error)

	List(ctx context.Context, mount WorkspaceMount, path string, opts ListOptions) ([]WorkspaceEntry, error)

	Read(ctx context.Context, mount WorkspaceMount, path string, opts ReadOptions) (ReadResult, error)

	Write(ctx context.Context, mount WorkspaceMount, path string, src io.Reader, opts WriteOptions) (WorkspaceEntry, error)

	Mkdir(ctx context.Context, mount WorkspaceMount, path string) (WorkspaceEntry, error)

	Rename(ctx context.Context, mount WorkspaceMount, path string, newName string) (WorkspaceEntry, error)

	Move(ctx context.Context, mount WorkspaceMount, source string, destinationDir string) (WorkspaceEntry, error)

	Copy(ctx context.Context, mount WorkspaceMount, source string, destinationDir string) (WorkspaceEntry, error)

	Delete(ctx context.Context, mount WorkspaceMount, path string, opts DeleteOptions) error
}
