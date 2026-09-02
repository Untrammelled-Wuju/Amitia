package uitree

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/androidnative/root"
)

type RootSource struct {
	executor root.InternalRootExecutor
	policy   Policy
}

func NewRootSource(executor root.InternalRootExecutor, policy Policy) *RootSource {
	return &RootSource{
		executor: executor,
		policy:   policy,
	}
}

func (s *RootSource) Type() SourceType {
	return SourceTypeRoot
}

func (s *RootSource) Status(ctx context.Context) SourceStatus {
	if s.executor == nil {
		return SourceStatus{Type: SourceTypeRoot, Available: false, Reason: "root executor not configured"}
	}
	return SourceStatus{Type: SourceTypeRoot, Available: true}
}

func (s *RootSource) Snapshot(ctx context.Context, request SnapshotRequest) (RawSnapshot, error) {
	if s.executor == nil {
		return RawSnapshot{}, &Error{Code: UI_TREE_ROOT_UNAVAILABLE, Message: "root executor not configured"}
	}

	dumpPath := fmt.Sprintf("/data/local/tmp/uitree_dump_%s.xml", uuid.NewString())
	execReq := root.ExecuteRequest{
		Executable: "uiautomator",
		Args:       []string{"dump", dumpPath},
		TimeoutMS:  int(s.policy.SnapshotTimeout.Milliseconds()),
		Mode:       "structured",
	}

	result, err := s.executor.ExecuteRoot(ctx, execReq, root.InternalExecuteOptions{
		Timeout:   s.policy.SnapshotTimeout.Milliseconds(),
		MaxOutput: int64(s.policy.MaxOutputBytes),
	})
	if err != nil {
		return RawSnapshot{}, &Error{Code: UI_TREE_SNAPSHOT_FAILED, Message: err.Error()}
	}

	if result.TimedOut {
		return RawSnapshot{}, &Error{Code: UI_TREE_TIMEOUT, Message: "root uiautomator dump timed out"}
	}

	readReq := root.ExecuteRequest{
		Executable: "cat",
		Args:       []string{dumpPath},
		TimeoutMS:  int(s.policy.SnapshotTimeout.Milliseconds()),
		Mode:       "structured",
	}

	readResult, err := s.executor.ExecuteRoot(ctx, readReq, root.InternalExecuteOptions{
		Timeout:   s.policy.SnapshotTimeout.Milliseconds(),
		MaxOutput: int64(s.policy.MaxOutputBytes),
	})
	if err != nil {
		return RawSnapshot{}, &Error{Code: UI_TREE_SNAPSHOT_FAILED, Message: err.Error()}
	}

	cleanReq := root.ExecuteRequest{
		Executable: "rm",
		Args:       []string{"-f", dumpPath},
		TimeoutMS:  1000,
		Mode:       "structured",
	}
	if _, err := s.executor.ExecuteRoot(ctx, cleanReq, root.InternalExecuteOptions{Timeout: 1000}); err != nil {
		log.Printf("uitree: root source: cleanup failed: %v", err)
	}

	parser := UiAutomatorXmlParser{}
	windows, nodes, err := parser.Parse(readResult.Stdout, SourceTypeRoot)
	if err != nil {
		return RawSnapshot{}, err
	}

	now := time.Now().UnixMilli()
	return RawSnapshot{
		Source:      SourceTypeRoot,
		Generation:  now,
		CapturedAt:  now,
		Truncated:   false,
		MultiWindow: false,
		StableRef:   true,
		RawWindows:  windows,
		RawNodes:    nodes,
	}, nil
}
