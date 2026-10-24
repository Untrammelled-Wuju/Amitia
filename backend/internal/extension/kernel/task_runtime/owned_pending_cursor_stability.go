package task_runtime

import (
	"context"
	"reflect"

	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func ownerPendingCursorSnapshotEqual(before *ownedTaskCommitCursor, beforeRevision int64, after *ownedTaskCommitCursor, afterRevision int64) bool {
	return beforeRevision == afterRevision && reflect.DeepEqual(before, after)
}

func verifyOwnerPendingCursorStable(ctx context.Context, data coordination.DataPort, scope coordination.ExecutionScope, run *TaskRun, cursor *ownedTaskCommitCursor, revision int64) error {
	latest, latestRevision, err := readTaskOwnerCommitCursor(ctx, data, scope, run)
	if err != nil {
		return err
	}
	if !ownerPendingCursorSnapshotEqual(cursor, revision, latest, latestRevision) {
		return NewTaskError(ErrTaskCheckpointIncompatible, "Owner检查点核验期间权威提交游标发生变化，禁止投影过期检查点")
	}
	return nil
}
