package uitree

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
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

	dumpPath := fmt.Sprintf("/data/local/tmp/uitree_dump_%s.xml", uuid.NewString())
	result, err := s.executor.ExecuteArgs(ctx, "", []string{"shell", "uiautomator", "dump", dumpPath}, adb.InternalADBExecuteOptions{
		Timeout:   s.policy.SnapshotTimeout,
		MaxOutput: int64(s.policy.MaxOutputBytes),
	})
	if err != nil {
		return RawSnapshot{}, &Error{Code: UI_TREE_SNAPSHOT_FAILED, Message: err.Error()}
	}

	if result.TimedOut {
		return RawSnapshot{}, &Error{Code: UI_TREE_TIMEOUT, Message: "adb uiautomator dump timed out"}
	}

	readResult, err := s.executor.ExecuteArgs(ctx, "", []string{"shell", "cat", dumpPath}, adb.InternalADBExecuteOptions{
		Timeout:   s.policy.SnapshotTimeout,
		MaxOutput: int64(s.policy.MaxOutputBytes),
	})
	if err != nil {
		return RawSnapshot{}, &Error{Code: UI_TREE_SNAPSHOT_FAILED, Message: err.Error()}
	}

	if _, err := s.executor.ExecuteArgs(ctx, "", []string{"shell", "rm", "-f", dumpPath}, adb.InternalADBExecuteOptions{Timeout: 1 * time.Second}); err != nil {
		log.Printf("uitree: adb source: cleanup failed: %v", err)
	}

	parser := UiAutomatorXmlParser{}
	windows, nodes, err := parser.Parse(readResult.Stdout, SourceTypeADB)
	if err != nil {
		return RawSnapshot{}, err
	}

	now := time.Now().UnixMilli()
	return RawSnapshot{
		Source:      SourceTypeADB,
		Generation:  now,
		CapturedAt:  now,
		Truncated:   false,
		MultiWindow: false,
		StableRef:   true,
		RawWindows:  windows,
		RawNodes:    nodes,
	}, nil
}
