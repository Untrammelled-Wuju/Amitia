package workbenchterminal

import "errors"

var ErrNotFound = errors.New("terminal session not found")
var ErrLimit = errors.New("terminal session limit reached")
var ErrInvalid = errors.New("invalid terminal request")
var ErrExited = errors.New("terminal process exited")

var ErrClosed = errors.New("terminal manager is shutting down")
var ErrUnavailable = errors.New("terminal process launcher unavailable")
