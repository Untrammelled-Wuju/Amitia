package interaction

type TransactionBoundary string

const (
	TransactionBoundaryNone           TransactionBoundary = "none"
	TransactionBoundaryMessagePersist TransactionBoundary = "message_persist"
	TransactionBoundaryStateWrite     TransactionBoundary = "state_write"
	TransactionBoundaryOutboxCommit   TransactionBoundary = "outbox_commit"
	TransactionBoundaryAll            TransactionBoundary = "all"
)

type TransactionDefinition struct {
	Name       TransactionBoundary  `json:"name"`
	Stages     []TransactionStage   `json:"stages"`
	RollbackOn []TransactionBoundary `json:"rollbackOn"`
}

type TransactionStage struct {
	Name      TransactionBoundary `json:"name"`
	Critical  bool                `json:"critical"`
	Retryable bool                `json:"retryable"`
}

var DefaultTransactionBoundaries = []TransactionDefinition{
	{
		Name: TransactionBoundaryMessagePersist,
		Stages: []TransactionStage{
			{Name: TransactionBoundaryMessagePersist, Critical: true, Retryable: false},
		},
		RollbackOn: nil,
	},
	{
		Name: TransactionBoundaryStateWrite,
		Stages: []TransactionStage{
			{Name: TransactionBoundaryStateWrite, Critical: true, Retryable: true},
		},
		RollbackOn: nil,
	},
	{
		Name: TransactionBoundaryOutboxCommit,
		Stages: []TransactionStage{
			{Name: TransactionBoundaryOutboxCommit, Critical: false, Retryable: true},
		},
		RollbackOn: nil,
	},
	{
		Name: TransactionBoundaryAll,
		Stages: []TransactionStage{
			{Name: TransactionBoundaryMessagePersist, Critical: true, Retryable: false},
			{Name: TransactionBoundaryStateWrite, Critical: true, Retryable: true},
			{Name: TransactionBoundaryOutboxCommit, Critical: false, Retryable: true},
		},
		RollbackOn: []TransactionBoundary{TransactionBoundaryMessagePersist},
	},
}
