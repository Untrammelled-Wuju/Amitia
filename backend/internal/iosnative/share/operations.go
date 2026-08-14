package share

const (
	OperationStatus        = "share.status"
	OperationSend          = "share.send"
	OperationPreviewSupported = "share.preview.supported"
	OperationReceivePending = "share.receive.pending"
	OperationReceiveConsume = "share.receive.consume"
	OperationReceivePeek    = "share.receive.peek"
	OperationReceiveDismiss = "share.receive.dismiss"
	OperationStagingCleanup = "share.staging.cleanup"
	OperationLimitedDelete  = "share.limited.delete"
)

func Operations() []string {
	return []string{
		OperationStatus,
		OperationSend,
		OperationPreviewSupported,
		OperationReceivePending,
		OperationReceiveConsume,
		OperationReceivePeek,
		OperationReceiveDismiss,
		OperationStagingCleanup,
		OperationLimitedDelete,
	}
}
