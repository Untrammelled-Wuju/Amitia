package workbenchterminal

import (
	"errors"
	"strconv"
)

func parseTerminalStreamCursor(value string) (int64, error) {
	if value == "" || len(value) > 19 {
		return 0, ErrInvalid
	}
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return 0, ErrInvalid
		}
	}
	cursor, err := strconv.ParseInt(value, 10, 64)
	if err != nil || cursor < 0 {
		return 0, errors.Join(ErrInvalid, err)
	}
	return cursor, nil
}
