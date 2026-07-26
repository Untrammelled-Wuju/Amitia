package package_security

type EntryKind string

const (
	EntryKindFile      EntryKind = "file"
	EntryKindDirectory EntryKind = "directory"
	EntryKindSymlink   EntryKind = "symlink"
	EntryKindHardlink  EntryKind = "hardlink"
	EntryKindDevice    EntryKind = "device"
	EntryKindUnknown   EntryKind = "unknown"
)

func (e EntryKind) IsValid() bool {
	switch e {
	case EntryKindFile, EntryKindDirectory, EntryKindSymlink,
		EntryKindHardlink, EntryKindDevice, EntryKindUnknown:
		return true
	}
	return false
}

type ArchiveEntryInfo struct {
	Path             string    `json:"path"`
	NormalizedPath   string    `json:"normalized_path"`
	Kind             EntryKind `json:"kind"`
	CompressedSize   int64     `json:"compressed_size"`
	UncompressedSize int64     `json:"uncompressed_size"`
	Mode             uint32    `json:"mode"`
	MIMEType         string    `json:"mime_type"`
	Hash             string    `json:"hash"`
	CRC32            uint32    `json:"crc32,omitempty"`
}

func (e ArchiveEntryInfo) IsDirectory() bool {
	return e.Kind == EntryKindDirectory
}

func (e ArchiveEntryInfo) IsSymlink() bool {
	return e.Kind == EntryKindSymlink
}

func (e ArchiveEntryInfo) IsSpecial() bool {
	return e.Kind == EntryKindDevice || e.Kind == EntryKindSymlink || e.Kind == EntryKindHardlink
}
