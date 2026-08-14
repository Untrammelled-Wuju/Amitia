package protocol

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type DedupKey string

func BuildDedupKey(parts ...string) DedupKey {
	joined := strings.Join(parts, "|")
	sum := sha256.Sum256([]byte(joined))
	return DedupKey(hex.EncodeToString(sum[:]))
}

func (k DedupKey) String() string {
	return string(k)
}

func (k DedupKey) IsEmpty() bool {
	return k == ""
}

type CommandDedupPort interface {
	Seen(
		ctx context.Context,
		sessionID runtimeidentity.RuntimeSessionID,
		key DedupKey,
	) (bool, error)

	Mark(
		ctx context.Context,
		sessionID runtimeidentity.RuntimeSessionID,
		key DedupKey,
	) error
}
