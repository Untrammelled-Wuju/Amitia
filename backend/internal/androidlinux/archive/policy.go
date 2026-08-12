//go:build linux && !android

package archive

type Policy struct {
	MaxEntries              int
	MaxTotalUncompressedBytes int64
	MaxSingleEntryBytes     int64
	MaxCompressionRatio     float64
	MaxArchiveBytes         int64
	MaxSources              int
	MaxPatterns             int
	MaxPatternBytes         int
	DefaultListLimit        int
	MaxListLimit            int
	StripSetuid             bool
	StripSetgid             bool
	FileModeMask            uint32
	DirModeMask             uint32
}

func DefaultPolicy() Policy {
	return Policy{
		MaxEntries:              100000,
		MaxTotalUncompressedBytes: 4 * 1024 * 1024 * 1024,
		MaxSingleEntryBytes:     1 * 1024 * 1024 * 1024,
		MaxCompressionRatio:     1000,
		MaxArchiveBytes:         2 * 1024 * 1024 * 1024,
		MaxSources:              10000,
		MaxPatterns:             100,
		MaxPatternBytes:         4096,
		DefaultListLimit:        100,
		MaxListLimit:            1000,
		StripSetuid:             true,
		StripSetgid:             true,
		FileModeMask:            0755,
		DirModeMask:             0755,
	}
}
