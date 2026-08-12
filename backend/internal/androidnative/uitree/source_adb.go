package uitree

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/androidnative/adb"
)

type ADBSource struct {
	executor adb.InternalADBExecutor
	policy   Policy
}

func NewADBSource(executor adb.InternalADBExecutor, policy Policy) *ADBSource {
	return &ADBSource{
		executor: executor,
		policy:   policy,
	}
}

func (s *ADBSource) Type() SourceType {
	return SourceTypeADB
}

func (s *ADBSource) Status(ctx context.Context) SourceStatus {
	if s.executor == nil {
		return SourceStatus{Type: SourceTypeADB, Available: false, Reason: "adb executor not configured"}
	}
	return SourceStatus{Type: SourceTypeADB, Available: true}
}

func (s *ADBSource) Snapshot(ctx context.Context, request SnapshotRequest) (RawSnapshot, error) {
	if s.executor == nil {
		return RawSnapshot{}, &Error{Code: UI_TREE_ADB_UNAVAILABLE, Message: "adb executor not configured"}
	}

	result, err := s.executor.ExecuteArgs(ctx, "", []string{"shell", "uiautomator", "dump", "/data/local/tmp/uitree_dump.xml"}, adb.InternalADBExecuteOptions{
		Timeout:   s.policy.SnapshotTimeout,
		MaxOutput: int64(s.policy.MaxOutputBytes),
	})
	if err != nil {
		return RawSnapshot{}, &Error{Code: UI_TREE_SNAPSHOT_FAILED, Message: err.Error()}
	}

	if result.TimedOut {
		return RawSnapshot{}, &Error{Code: UI_TREE_TIMEOUT, Message: "adb uiautomator dump timed out"}
	}

	readResult, err := s.executor.ExecuteArgs(ctx, "", []string{"shell", "cat", "/data/local/tmp/uitree_dump.xml"}, adb.InternalADBExecuteOptions{
		Timeout:   s.policy.SnapshotTimeout,
		MaxOutput: int64(s.policy.MaxOutputBytes),
	})
	if err != nil {
		return RawSnapshot{}, &Error{Code: UI_TREE_SNAPSHOT_FAILED, Message: err.Error()}
	}

	_, _ = s.executor.ExecuteArgs(ctx, "", []string{"shell", "rm", "-f", "/data/local/tmp/uitree_dump.xml"}, adb.InternalADBExecuteOptions{Timeout: 1 * time.Second})

	return RawSnapshot{
		Source:     SourceTypeADB,
		Generation: time.Now().UnixMilli(),
		CapturedAt: time.Now().UnixMilli(),
		Truncated:  false,
		MultiWindow: false,
		StableRef:  false,
		RawNodes: []map[string]any{
			{"xml": readResult.Stdout},
		},
	}, nil
}
