package interaction

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	return e.Code + ": " + e.Message
}

func NewError(code, message string) *Error {
	return &Error{
		Code:    code,
		Message: message,
	}
}
