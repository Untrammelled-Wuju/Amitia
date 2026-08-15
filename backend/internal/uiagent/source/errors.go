package source

import "errors"

var (
	ErrPreciseUnavailable = errors.New("source editor: precise editing service unavailable")
	ErrTransactionFailed  = errors.New("source editor: transaction failed")
	ErrUnsupportedEdit    = errors.New("source editor: unsupported edit operation")
)
