package interaction

import (
	"encoding/hex"
	"strings"
	"time"
)

func verifiedCodingTestProjection(evidence *CodingWorktreeReview) bool {
	if evidence == nil || evidence.TestState != "passed" || evidence.TestExitCode != 0 ||
		(evidence.TestProfile != "go" && evidence.TestProfile != "go-race") ||
		len(evidence.CheckpointRevision) != 64 || len(evidence.TestOutputDigest) != 64 ||
		strings.Trim(evidence.CheckpointRevision, "0123456789abcdef") != "" ||
		strings.Trim(evidence.TestOutputDigest, "0123456789abcdef") != "" {
		return false
	}
	if _, err := hex.DecodeString(evidence.CheckpointRevision); err != nil {
		return false
	}
	if _, err := hex.DecodeString(evidence.TestOutputDigest); err != nil {
		return false
	}
	started, err := time.Parse(time.RFC3339Nano, evidence.TestStartedAt)
	return err == nil && !started.IsZero()
}
