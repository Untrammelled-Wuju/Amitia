package workbenchterminal

import (
	"context"
	"errors"
	"regexp"
)

var ErrTaskBinding = errors.New("terminal task association is no longer authorized")
var taskIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

type TaskVerifier func(context.Context, string, string, string) error
