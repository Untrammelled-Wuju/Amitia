package task_runtime

func ownerInitialCheckpointAllowed(hasCursor, localCheckpoint bool, version int64) bool {
	return hasCursor || (!localCheckpoint && version == 1)
}
