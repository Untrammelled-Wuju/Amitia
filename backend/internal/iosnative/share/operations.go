package share

const (
	OperationStatus           = "share.status"
	OperationSend             = "share.send"
	OperationPreviewSupported = "share.preview.supported"
	OperationStagingCleanup   = "share.staging.cleanup"
	OperationLimitedDelete    = "share.limited.delete"
)

func Operations() []string {
	return []string{
		OperationStatus,
		OperationSend,
		OperationPreviewSupported,
		OperationStagingCleanup,
		OperationLimitedDelete,
	}
}
