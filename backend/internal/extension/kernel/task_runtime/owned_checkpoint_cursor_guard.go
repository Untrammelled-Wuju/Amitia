package task_runtime

func ownedPendingCheckpointMatchesCursor(previousVersion, pendingVersion int64, previousID, pendingID string, committedVersion int64, committedID string, terminal bool) (bool, bool) {
	if terminal || previousVersion < 0 || pendingVersion != previousVersion+1 {
		return false, false
	}
	switch committedVersion {
	case previousVersion:
		return false, committedID == previousID
	case pendingVersion:
		return true, committedID == pendingID
	default:
		return false, false
	}
}
