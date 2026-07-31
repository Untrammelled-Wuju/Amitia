package submission

import "fmt"

type SubmissionError struct {
	Code    string
	Message string
	Cause   error
}

func (e *SubmissionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *SubmissionError) Unwrap() error { return e.Cause }

func NewSubmissionError(code, message string, cause error) *SubmissionError {
	return &SubmissionError{Code: code, Message: message, Cause: cause}
}

const (
	ErrCodeReceiptAlreadyExists  = "SUBMISSION_RECEIPT_EXISTS"
	ErrCodeReceiptCreateFailed   = "SUBMISSION_RECEIPT_CREATE_FAILED"
	ErrCodeReceiptUpdateFailed   = "SUBMISSION_RECEIPT_UPDATE_FAILED"
	ErrCodeInvalidRequest        = "SUBMISSION_INVALID_REQUEST"
	ErrCodeResultEnvelopeInvalid = "SUBMISSION_RESULT_ENVELOPE_INVALID"
)
