package workspace

import (
	"mime"
	"path/filepath"
	"strings"

	"github.com/u-ai/backend/pkg/resourceuri"
)

const (
	SAFMIMETypeDir = "vnd.android.document/directory"

	SAFFlagSupportsDelete    = 0x00000002
	SAFFlagSupportsCreate    = 0x00000004
	SAFFlagSupportsRename    = 0x00000040
	SAFFlagSupportsMove      = 0x00000080
	SAFFlagSupportsCopy      = 0x00000100
	SAFFlagSupportsWrite     = 0x00000006
	SAFFlagVirtualDocument   = 0x00000200
	SAFFlagSupportsIsVirtual = 0x00000400
)

const (
	SAFStatTimeout  = 5 * 1000_000_000
	SAFListTimeout  = 10 * 1000_000_000
	SAFReadTimeout  = 15 * 1000_000_000
	SAFWriteTimeout = 30 * 1000_000_000
	SAFMkdirTimeout = 10 * 1000_000_000
	SAFRenameTimeout = 10 * 1000_000_000
	SAFMoveTimeout  = 30 * 1000_000_000
	SAFCopyTimeout  = 30 * 1000_000_000
	SAFDeleteTimeout = 30 * 1000_000_000
)

const (
	SAFTempFilePrefix = ".amitia-"
	SAFTempFileSuffix = ".tmp"
)

func IsSAFMIMEDirectory(mimeType string) bool {
	return mimeType == SAFMIMETypeDir
}

func IsSAFVirtual(flags int64) bool {
	return flags&SAFFlagVirtualDocument != 0
}

func IsSAFSupportsDelete(flags int64) bool {
	return flags&SAFFlagSupportsDelete != 0
}

func IsSAFSupportsCreate(flags int64) bool {
	return flags&SAFFlagSupportsCreate != 0
}

func IsSAFSupportsRename(flags int64) bool {
	return flags&SAFFlagSupportsRename != 0
}

func IsSAFSupportsMove(flags int64) bool {
	return flags&SAFFlagSupportsMove != 0
}

func IsSAFSupportsCopy(flags int64) bool {
	return flags&SAFFlagSupportsCopy != 0
}

func IsSAFWritable(flags int64) bool {
	return flags&SAFFlagSupportsWrite != 0
}

func InferMIMEType(displayName string) string {
	ext := filepath.Ext(displayName)
	if ext != "" {
		mt := mime.TypeByExtension(ext)
		if mt != "" {
			return mt
		}
	}
	return "application/octet-stream"
}

func BuildSAFEntryURI(mount WorkspaceMount, relativePath string) string {
	base := mount.RootURI
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return base + relativePath
}

func BuildSAFDirectoryEntryURI(mount WorkspaceMount, relativePath string) string {
	uri := BuildSAFEntryURI(mount, relativePath)
	if !strings.HasSuffix(uri, "/") {
		uri += "/"
	}
	return uri
}

func ParseSAFEntryURI(uriStr string) (mountID WorkspaceID, relativePath string, err error) {
	uri, err := resourceuri.Parse(uriStr)
	if err != nil {
		return "", "", err
	}
	if uri.Root() != resourceuri.ResourceRootWorkspace {
		return "", "", ErrInvalidURI
	}
	rel := uri.RelativePath()
	if !strings.HasPrefix(rel, "@") {
		return "", "", ErrInvalidURI
	}
	slashIdx := strings.Index(rel, "/")
	if slashIdx < 0 {
		return "", "", ErrInvalidURI
	}
	mountID = WorkspaceID(rel[1:slashIdx])
	relativePath = rel[slashIdx+1:]
	return mountID, relativePath, nil
}

func SplitPathParent(path string) (parent string, name string) {
	if path == "" {
		return "", ""
	}
	if !strings.Contains(path, "/") {
		return "", path
	}
	idx := strings.LastIndex(path, "/")
	return path[:idx], path[idx+1:]
}
