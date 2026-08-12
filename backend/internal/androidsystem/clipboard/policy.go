package clipboard

import "unicode/utf8"

type Policy struct {
	MaxTextBytes int
}

func DefaultPolicy() Policy {
	return Policy{
		MaxTextBytes: MaxClipboardTextBytes,
	}
}

func (p Policy) validateWriteText(text string) error {
	if !utf8.ValidString(text) {
		return &clipboardError{code: CLIPBOARD_INPUT_TOO_LARGE, message: "invalid utf-8 input"}
	}
	if len(text) > p.MaxTextBytes {
		return &clipboardError{code: CLIPBOARD_INPUT_TOO_LARGE, message: "clipboard input exceeds maximum size"}
	}
	return nil
}

type clipboardError struct {
	code    string
	message string
}

func (e *clipboardError) Error() string  { return e.message }
func (e *clipboardError) Code() string   { return e.code }
