package task_runtime

func ownerCheckpointConfirmed(version int64, id string, cursorVersion int64, cursorID string) bool {
	return version > 0 && id != "" && version == cursorVersion && id == cursorID
}

func ownerTerminalConfirmed(expectedID string, terminal bool, cursorID string) bool {
	return terminal && expectedID != "" && cursorID == expectedID
}
