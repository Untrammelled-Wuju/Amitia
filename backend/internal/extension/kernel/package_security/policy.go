package package_security

type ArchivePolicy struct {
	MaxArchiveBytes           int64    `json:"max_archive_bytes"`
	MaxEntryCount             int      `json:"max_entry_count"`
	MaxSingleEntryBytes       int64    `json:"max_single_entry_bytes"`
	MaxTotalUncompressedBytes int64    `json:"max_total_uncompressed_bytes"`
	MaxCompressionRatio       float64  `json:"max_compression_ratio"`
	MaxPathLength             int      `json:"max_path_length"`
	MaxDirectoryDepth         int      `json:"max_directory_depth"`
	AllowedFileTypes          []string `json:"allowed_file_types,omitempty"`
	ForbiddenFileTypes        []string `json:"forbidden_file_types,omitempty"`
	AllowSymlink              bool     `json:"allow_symlink"`
	AllowHardlink             bool     `json:"allow_hardlink"`
	AllowNestedArchive        bool     `json:"allow_nested_archive"`
	AllowExecutable           bool     `json:"allow_executable"`
	AllowDeclaredExecutable   bool     `json:"allow_declared_executable"`
}

func DefaultArchivePolicy() ArchivePolicy {
	return ArchivePolicy{
		MaxArchiveBytes:           100 * 1024 * 1024,
		MaxEntryCount:             2000,
		MaxSingleEntryBytes:       50 * 1024 * 1024,
		MaxTotalUncompressedBytes: 200 * 1024 * 1024,
		MaxCompressionRatio:       100,
		MaxPathLength:             512,
		MaxDirectoryDepth:         32,
		AllowSymlink:              false,
		AllowHardlink:             false,
		AllowNestedArchive:        false,
		AllowExecutable:           false,
		AllowDeclaredExecutable:   true,
	}
}

func RestrictedArchivePolicy() ArchivePolicy {
	return ArchivePolicy{
		MaxArchiveBytes:           10 * 1024 * 1024,
		MaxEntryCount:             200,
		MaxSingleEntryBytes:       5 * 1024 * 1024,
		MaxTotalUncompressedBytes: 20 * 1024 * 1024,
		MaxCompressionRatio:       50,
		MaxPathLength:             256,
		MaxDirectoryDepth:         16,
		AllowSymlink:              false,
		AllowHardlink:             false,
		AllowNestedArchive:        false,
		AllowExecutable:           false,
		AllowDeclaredExecutable:   false,
	}
}
