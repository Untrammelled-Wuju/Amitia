//go:build linux && !android

package archive

import "time"

type Format string

const (
	FormatZIP    Format = "zip"
	FormatTAR    Format = "tar"
	FormatTARGZ  Format = "tar.gz"
	FormatTARBZ2 Format = "tar.bz2"
	FormatTARXZ  Format = "tar.xz"
	FormatTARZST Format = "tar.zst"

	FormatGZIP  Format = "gzip"
	FormatBZIP2 Format = "bzip2"
	FormatXZ    Format = "xz"
	FormatZSTD  Format = "zstd"
)

type DetectRequest struct {
	Path string `json:"path"`
}

type DetectResult struct {
	Path        string `json:"path"`
	Format      Format `json:"format"`
	MIMEType    string `json:"mimeType,omitempty"`
	Archive     bool   `json:"archive"`
	Compressed  bool   `json:"compressed"`
	SizeBytes   int64  `json:"sizeBytes"`
	EntryCount  *int   `json:"entryCount,omitempty"`
	Encrypted   bool   `json:"encrypted,omitempty"`
}

type Entry struct {
	Name       string     `json:"name"`
	Path       string     `json:"path"`
	Type       string     `json:"type"`
	SizeBytes  int64      `json:"sizeBytes"`
	Mode       string     `json:"mode,omitempty"`
	ModifiedAt *time.Time `json:"modifiedAt,omitempty"`
	LinkTarget string     `json:"linkTarget,omitempty"`
	Encrypted  bool       `json:"encrypted,omitempty"`
}

type ListRequest struct {
	Path              string `json:"path"`
	Limit             int    `json:"limit,omitempty"`
	Offset            int    `json:"offset,omitempty"`
	IncludeDirectories bool  `json:"includeDirectories,omitempty"`
}

type ExtractRequest struct {
	Path           string   `json:"path"`
	Target         string   `json:"target"`
	Overwrite      bool     `json:"overwrite,omitempty"`
	StripOptions   int      `json:"stripComponents,omitempty"`
	Include        []string `json:"include,omitempty"`
	Exclude        []string `json:"exclude,omitempty"`
	AllowSymlinks  bool     `json:"allowSymlinks,omitempty"`
	MaxEntries     *int     `json:"maxEntries,omitempty"`
	MaxBytes       *int64   `json:"maxBytes,omitempty"`
}

type CreateRequest struct {
	Sources          []string `json:"sources"`
	Target           string   `json:"target"`
	Format           Format   `json:"format"`
	CompressionLevel *int     `json:"compressionLevel,omitempty"`
	BasePath         string   `json:"basePath,omitempty"`
	IncludeHidden    bool     `json:"includeHidden,omitempty"`
	FollowSymlinks   bool     `json:"followSymlinks,omitempty"`
	Overwrite        bool     `json:"overwrite,omitempty"`
}

type VerifyResult struct {
	Valid                 bool     `json:"valid"`
	Format                Format   `json:"format"`
	EntryCount            int      `json:"entryCount"`
	TotalUncompressedBytes int64   `json:"totalUncompressedBytes"`
	UnsafeEntries         int      `json:"unsafeEntries"`
	CorruptEntries        int      `json:"corruptEntries"`
	EncryptedEntries      int      `json:"encryptedEntries"`
	Warnings              []string `json:"warnings,omitempty"`
}
