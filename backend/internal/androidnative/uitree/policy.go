package uitree

import "time"

const (
	DefaultMaxWindows         = 32
	DefaultMaxNodes           = 5000
	DefaultMaxDepth           = 64
	DefaultMaxChildrenPerNode = 512
	DefaultMaxNodeTextRunes   = 4096
	DefaultMaxDescriptionRunes = 4096
	DefaultMaxResourceIDRunes = 1024
	DefaultMaxClassNameRunes  = 512
	DefaultMaxOutputBytes     = 4 * 1024 * 1024
	DefaultSnapshotTimeout    = 5 * time.Second
	DefaultSnapshotTTL        = 10 * time.Second
	DefaultMaxSnapshots       = 4
	DefaultMaxFindLimit       = 100
)

type Policy struct {
	MaxWindows         int
	MaxNodes           int
	MaxDepth           int
	MaxChildrenPerNode int
	MaxNodeTextRunes   int
	MaxDescriptionRunes int
	MaxResourceIDRunes int
	MaxClassNameRunes  int
	MaxOutputBytes     int
	SnapshotTimeout    time.Duration
	SnapshotTTL        time.Duration
	MaxSnapshots       int
	MaxFindLimit       int
}

func DefaultPolicy() Policy {
	return Policy{
		MaxWindows:         DefaultMaxWindows,
		MaxNodes:           DefaultMaxNodes,
		MaxDepth:           DefaultMaxDepth,
		MaxChildrenPerNode: DefaultMaxChildrenPerNode,
		MaxNodeTextRunes:   DefaultMaxNodeTextRunes,
		MaxDescriptionRunes: DefaultMaxDescriptionRunes,
		MaxResourceIDRunes: DefaultMaxResourceIDRunes,
		MaxClassNameRunes:  DefaultMaxClassNameRunes,
		MaxOutputBytes:     DefaultMaxOutputBytes,
		SnapshotTimeout:    DefaultSnapshotTimeout,
		SnapshotTTL:        DefaultSnapshotTTL,
		MaxSnapshots:       DefaultMaxSnapshots,
		MaxFindLimit:       DefaultMaxFindLimit,
	}
}

func TruncateString(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes])
}

type snapshotRecord struct {
	snapshot   UITreeSnapshot
	createdAt  int64
	accessedAt int64
	nodeIndex  map[string]int
}
