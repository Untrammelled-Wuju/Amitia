package sandbox

import (
	"fmt"
	"regexp"
	"time"
)

const (
	MaxRootfsArchiveBytes = 150 * 1024 * 1024 // 150 MiB
	MaxExpandedBytes      = 512 * 1024 * 1024 // 512 MiB
	MaxSingleFileBytes    = 64 * 1024 * 1024  // 64 MiB
	MaxArchiveEntries     = 200000
	MaxPathLength         = 4096
	MaxDirectoryDepth     = 64
	MaxSymlinkDepth       = 8
	StashVersionKeep      = 2
)

const (
	RootfsErrNotConfigured          = "ROOTFS_NOT_CONFIGURED"
	RootfsErrSourceInvalid          = "ROOTFS_SOURCE_INVALID"
	RootfsErrDownloadFailed         = "ROOTFS_DOWNLOAD_FAILED"
	RootfsErrArchiveTooLarge        = "ROOTFS_ARCHIVE_TOO_LARGE"
	RootfsErrDigestMismatch         = "ROOTFS_DIGEST_MISMATCH"
	RootfsErrArchMismatch           = "ROOTFS_ARCH_MISMATCH"
	RootfsErrArchiveInvalid         = "ROOTFS_ARCHIVE_INVALID"
	RootfsErrPathTraversal          = "ROOTFS_PATH_TRAVERSAL"
	RootfsErrSymlinkEscape          = "ROOTFS_SYMLINK_ESCAPE"
	RootfsErrHardlinkEscape         = "ROOTFS_HARDLINK_ESCAPE"
	RootfsErrExpandedSizeExceeded   = "ROOTFS_EXPANDED_SIZE_EXCEEDED"
	RootfsErrEntryCountExceeded    = "ROOTFS_ENTRY_COUNT_EXCEEDED"
	RootfsErrValidationFailed       = "ROOTFS_VALIDATION_FAILED"
	RootfsErrVersionDigestConflict  = "ROOTFS_VERSION_DIGEST_CONFLICT"
	RootfsErrNotInstalled           = "ROOTFS_NOT_INSTALLED"
	RootfsErrCorrupted              = "ROOTFS_CORRUPTED"
	RootfsErrRuntimeBusy            = "ROOTFS_RUNTIME_BUSY"
	RootfsErrInstallCancelled       = "ROOTFS_INSTALL_CANCELLED"
	RootfsErrCommitFailed           = "ROOTFS_COMMIT_FAILED"
)

var archPattern = regexp.MustCompile(`^[a-z0-9_+-]+$`)

type RootfsSourceType string

const (
	RootfsSourceBundled RootfsSourceType = "bundled"
	RootfsSourceRemote  RootfsSourceType = "remote"
)

type ArchiveFormat string

const (
	ArchiveFormatTarGz  ArchiveFormat = "tar.gz"
	ArchiveFormatTarXz  ArchiveFormat = "tar.xz"
	ArchiveFormatTarBz2 ArchiveFormat = "tar.bz2"
	ArchiveFormatTar    ArchiveFormat = "tar"
)

type RootfsRef struct {
	Version string `json:"version"`
	Digest  string `json:"digest"`
	URI     string `json:"uri,omitempty"`
}

type RootfsInstallSpec struct {
	RootfsVersion      string         `json:"rootfsVersion"`
	AlpineVersion      string         `json:"alpineVersion"`
	GuestArchitecture  string         `json:"guestArchitecture"`
	SourceType         RootfsSourceType `json:"sourceType"`
	SourceURL          string         `json:"sourceURL,omitempty"`
	BundleResource     string         `json:"bundleResource,omitempty"`
	SHA256             string         `json:"sha256"`
	ExpectedSize       int64          `json:"expectedSize"`
	ArchiveFormat      ArchiveFormat  `json:"archiveFormat"`
	Activate           bool           `json:"activate"`
}

func (s RootfsInstallSpec) Validate() error {
	if s.RootfsVersion == "" {
		return &RootfsError{Code: RootfsErrArchiveInvalid, Message: "rootfsVersion is required"}
	}
	if !archPattern.MatchString(s.GuestArchitecture) {
		return &RootfsError{Code: RootfsErrArchMismatch, Message: fmt.Sprintf("invalid guest architecture: %s", s.GuestArchitecture)}
	}
	if s.SourceType != RootfsSourceBundled && s.SourceType != RootfsSourceRemote {
		return &RootfsError{Code: RootfsErrSourceInvalid, Message: fmt.Sprintf("unsupported source type: %s", s.SourceType)}
	}
	if s.SourceType == RootfsSourceRemote && s.SourceURL == "" {
		return &RootfsError{Code: RootfsErrSourceInvalid, Message: "sourceURL is required for remote rootfs"}
	}
	if s.SourceType == RootfsSourceBundled && s.BundleResource == "" {
		return &RootfsError{Code: RootfsErrSourceInvalid, Message: "bundleResource is required for bundled rootfs"}
	}
	if len(s.SHA256) != 64 {
		return &RootfsError{Code: RootfsErrDigestMismatch, Message: "sha256 must be 64 hex characters"}
	}
	if s.ExpectedSize <= 0 || s.ExpectedSize > MaxRootfsArchiveBytes {
		return &RootfsError{Code: RootfsErrArchiveTooLarge, Message: fmt.Sprintf("expectedSize must be between 1 and %d", MaxRootfsArchiveBytes)}
	}
	switch s.ArchiveFormat {
	case ArchiveFormatTarGz, ArchiveFormatTarXz, ArchiveFormatTarBz2, ArchiveFormatTar:
	default:
		return &RootfsError{Code: RootfsErrArchiveInvalid, Message: fmt.Sprintf("unsupported archive format: %s", s.ArchiveFormat)}
	}
	return nil
}

type RootfsStatus struct {
	Installed         bool      `json:"installed"`
	ActiveVersion     string    `json:"activeVersion,omitempty"`
	ActiveDigest      string    `json:"activeDigest,omitempty"`
	RunningVersion    string    `json:"runningVersion,omitempty"`
	RunningDigest     string    `json:"runningDigest,omitempty"`
	RestartRequired   bool      `json:"restartRequired"`
	Corrupted         bool      `json:"corrupted"`
	AvailableVersions []string  `json:"availableVersions,omitempty"`
}

type RootfsVersionMetadata struct {
	SchemaVersion     int       `json:"schemaVersion"`
	InstallationID    string    `json:"installationId"`
	RootfsVersion     string    `json:"rootfsVersion"`
	AlpineVersion     string    `json:"alpineVersion"`
	GuestArchitecture string    `json:"guestArchitecture"`
	ArchiveDigest     string    `json:"archiveDigest"`
	InstalledAt       time.Time `json:"installedAt"`
	Validated         bool      `json:"validated"`
	ArchiveSizeBytes  int64     `json:"archiveSizeBytes"`
	ArchiveFormat     string    `json:"archiveFormat"`
	SourceType        string    `json:"sourceType"`
	SourceRef         string    `json:"sourceRef"`
}

type RootfsInstallResult struct {
	InstallationID string    `json:"installationId"`
	Version        string    `json:"version"`
	Digest         string    `json:"digest"`
	InstalledAt    time.Time `json:"installedAt"`
	Activated      bool      `json:"activated"`
}

type RootfsProgress struct {
	InstallationID string  `json:"installationId"`
	Phase          string  `json:"phase"`
	BytesWritten   int64   `json:"bytesWritten"`
	TotalBytes     int64   `json:"totalBytes"`
	Message        string  `json:"message,omitempty"`
	Done           bool    `json:"done"`
	Failed         bool    `json:"failed"`
}

type RootfsError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Cause   error  `json:"-"`
}

func (e *RootfsError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *RootfsError) Unwrap() error {
	return e.Cause
}

const rootfsURIPrefix = "amitia://runtime/ios/rootfs/"

func ParseRootfsURI(uri string) (installationID string, err error) {
	if len(uri) <= len(rootfsURIPrefix) {
		return "", &RootfsError{Code: RootfsErrNotConfigured, Message: "rootfs URI too short"}
	}
	prefix := uri[:len(rootfsURIPrefix)]
	if prefix != rootfsURIPrefix {
		return "", &RootfsError{Code: RootfsErrNotConfigured, Message: "rootfs URI must start with amitia://runtime/ios/rootfs/"}
	}
	id := uri[len(rootfsURIPrefix):]
	if id == "" {
		return "", &RootfsError{Code: RootfsErrNotConfigured, Message: "rootfs URI missing installation ID"}
	}
	return id, nil
}

func BuildRootfsURI(installationID string) string {
	return rootfsURIPrefix + installationID
}
