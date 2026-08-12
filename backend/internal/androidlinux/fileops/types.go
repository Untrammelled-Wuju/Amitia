//go:build linux && !android

package fileops

import "time"

type StatResult struct {
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	Type       string    `json:"type"`
	Size       int64     `json:"size"`
	Mode       uint32    `json:"mode"`
	ModTime    time.Time `json:"modTime"`
	IsDir      bool      `json:"isDir"`
	IsSymlink  bool      `json:"isSymlink"`
	LinkTarget string    `json:"linkTarget,omitempty"`
}

type ListOptions struct {
	Limit          int  `json:"limit"`
	IncludeHidden  bool `json:"includeHidden"`
	FollowSymlinks bool `json:"followSymlinks"`
}

type ReadOptions struct {
	Offset   int64 `json:"offset"`
	MaxBytes int64 `json:"maxBytes"`
}

type WriteOptions struct {
	Overwrite     bool   `json:"overwrite"`
	CreateParents bool   `json:"createParents"`
	Mode          uint32 `json:"mode"`
}

type DeleteOptions struct {
	Recursive bool `json:"recursive"`
}

type SearchOptions struct {
	Query          string `json:"query"`
	MaxDepth       int    `json:"maxDepth"`
	Limit          int    `json:"limit"`
	IncludeHidden  bool   `json:"includeHidden"`
	FollowSymlinks bool   `json:"followSymlinks"`
}

type ReadResult struct {
	Path      string `json:"path"`
	Offset    int64  `json:"offset"`
	BytesRead int    `json:"bytesRead"`
	Content   []byte `json:"content"`
	EOF       bool   `json:"eof"`
	IsBinary  bool   `json:"isBinary"`
}

type MkdirOptions struct {
	Recursive bool   `json:"recursive"`
	Mode      uint32 `json:"mode"`
}

type CopyOptions struct {
	Recursive bool `json:"recursive"`
	Overwrite bool `json:"overwrite"`
}

type MoveOptions struct {
	Overwrite bool `json:"overwrite"`
}

type FileService interface {
	Stat(path string) (StatResult, error)
	List(path string, opts ListOptions) ([]StatResult, error)
	Read(path string, opts ReadOptions) (ReadResult, error)

	Write(path string, data []byte, opts WriteOptions) (StatResult, error)
	Append(path string, data []byte) (StatResult, error)
	Mkdir(path string, opts MkdirOptions) (StatResult, error)
	Touch(path string) (StatResult, error)

	Copy(source, destination string, opts CopyOptions) (StatResult, error)
	Move(source, destination string, opts MoveOptions) (StatResult, error)
	Delete(path string, opts DeleteOptions) error

	Search(root string, opts SearchOptions) ([]StatResult, error)

	Chmod(path string, mode uint32) (StatResult, error)
	Readlink(path string) (string, error)
	Symlink(target, linkPath string) (StatResult, error)
}
