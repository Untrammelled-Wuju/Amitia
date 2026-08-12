package uitree

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
)

func GenerateSnapshotID(generation int64, counter int64) string {
	return fmt.Sprintf("uis_%d_%d", generation, counter)
}

func GenerateNodeID(snapshotID string, windowID string, sourceRef string, resourceID string, className string, bounds Rect, depth int) string {
	h := sha1.New()
	fmt.Fprintf(h, "%s|%s|%s|%s|%s|%d,%d,%d,%d|%d",
		snapshotID, windowID, sourceRef, resourceID, className,
		bounds.Left, bounds.Top, bounds.Right, bounds.Bottom, depth)
	sum := h.Sum(nil)
	return "node_" + hex.EncodeToString(sum[:8])
}

func GenerateWindowID(sourceRef string, windowType WindowType, packageName string) string {
	h := sha1.New()
	fmt.Fprintf(h, "%s|%s|%s", sourceRef, windowType, packageName)
	sum := h.Sum(nil)
	return "win_" + hex.EncodeToString(sum[:8])
}
