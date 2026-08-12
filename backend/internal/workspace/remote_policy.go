package workspace

import (
	"path"
	"strings"
	"time"
)

const (
	DefaultSFTPPort                    = 22
	DefaultWebDAVPort                  = 443
	MaxRemoteListEntries               = 1000
	MaxRemoteRecursiveDepth            = 32
	MaxRemoteRecursiveFiles            = 10000
	MaxRemoteReadBytes                 = 64 * 1024 * 1024
	MaxRemoteWebDAVResponseBodyBytes   = 64 * 1024
	MaxRemoteConnectionsPerMount       = 4
	MaxRemoteGlobalConnections         = 16
	MaxRemoteConnectTimeout            = 10 * time.Second
	MaxRemoteOperationTimeout          = 60 * time.Second
	MaxRemoteIdleTTL                   = 5 * time.Minute
	RemoteTempSuffix                   = ".amitia-"
	RemoteTempSuffixPattern            = ".amitia-*.tmp"
)

type RemotePolicy struct {
	MaxListEntries       int
	MaxRecursiveDepth    int
	MaxRecursiveFiles    int
	MaxReadBytes         int64
	MaxConnectionsPerMount int
	MaxGlobalConnections int
	ConnectTimeout       time.Duration
	OperationTimeout     time.Duration
	IdleTTL              time.Duration
}

var DefaultRemotePolicy = RemotePolicy{
	MaxListEntries:       MaxRemoteListEntries,
	MaxRecursiveDepth:    MaxRemoteRecursiveDepth,
	MaxRecursiveFiles:    MaxRemoteRecursiveFiles,
	MaxReadBytes:         MaxRemoteReadBytes,
	MaxConnectionsPerMount: MaxRemoteConnectionsPerMount,
	MaxGlobalConnections: MaxRemoteGlobalConnections,
	ConnectTimeout:       MaxRemoteConnectTimeout,
	OperationTimeout:     MaxRemoteOperationTimeout,
	IdleTTL:              MaxRemoteIdleTTL,
}

func ResolveRemotePathSFTP(basePath string, relativePath string) string {
	cleaned := path.Clean("/" + relativePath)
	if cleaned == "/" {
		return path.Clean(basePath)
	}
	return path.Clean(basePath + "/" + cleaned[1:])
}

func ResolveRemotePathWebDAV(basePath string, relativePath string) string {
	cleaned := path.Clean("/" + relativePath)
	if cleaned == "/" {
		return strings.TrimRight(basePath, "/")
	}
	return strings.TrimRight(basePath, "/") + "/" + cleaned[1:]
}

func ValidateRemotePath(relativePath string) error {
	if relativePath == "" {
		return nil
	}
	if strings.Contains(relativePath, "..") {
		return ErrRemoteBoundaryEscaped
	}
	if path.IsAbs(relativePath) {
		return ErrRemoteBoundaryEscaped
	}
	return nil
}

func IsPathUnderBase(childPath string, basePath string) bool {
	normalizedBase := path.Clean(basePath)
	normalizedChild := path.Clean(childPath)
	if normalizedBase == normalizedChild {
		return true
	}
	if !strings.HasPrefix(normalizedChild, normalizedBase+"/") {
		return false
	}
	return true
}

func InferRemoteMIMEType(name string) string {
	ext := strings.ToLower(path.Ext(name))
	switch ext {
	case ".txt":
		return "text/plain"
	case ".md":
		return "text/markdown"
	case ".go":
		return "text/x-go"
	case ".js":
		return "application/javascript"
	case ".ts":
		return "application/typescript"
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "application/yaml"
	case ".xml":
		return "application/xml"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".py":
		return "text/x-python"
	case ".java":
		return "text/x-java"
	case ".c":
		return "text/x-c"
	case ".cpp", ".cc":
		return "text/x-c++"
	case ".h":
		return "text/x-c-header"
	case ".rs":
		return "text/x-rust"
	case ".rb":
		return "text/x-ruby"
	case ".sh":
		return "application/x-sh"
	case ".bat":
		return "application/bat"
	case ".ps1":
		return "application/x-powershell"
	case ".sql":
		return "application/sql"
	case ".log":
		return "text/plain"
	case ".csv":
		return "text/csv"
	case ".pdf":
		return "application/pdf"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".ico":
		return "image/x-icon"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg":
		return "audio/ogg"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".avi":
		return "video/x-msvideo"
	case ".mov":
		return "video/quicktime"
	case ".zip":
		return "application/zip"
	case ".tar":
		return "application/x-tar"
	case ".gz":
		return "application/gzip"
	case ".bz2":
		return "application/x-bzip2"
	case ".7z":
		return "application/x-7z-compressed"
	case ".rar":
		return "application/vnd.rar"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xls":
		return "application/vnd.ms-excel"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".ppt":
		return "application/vnd.ms-powerpoint"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	default:
		return "application/octet-stream"
	}
}
