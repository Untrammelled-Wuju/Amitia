package clipboard

const (
	OperationStatus = "clipboard.status"
	OperationDetect = "clipboard.detect"
	OperationRead   = "clipboard.read"
	OperationWrite  = "clipboard.write"
	OperationClear  = "clipboard.clear"
)

func Operations() []string {
	return []string{
		OperationStatus,
		OperationDetect,
		OperationRead,
		OperationWrite,
		OperationClear,
	}
}
