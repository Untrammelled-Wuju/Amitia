package package_security

import (
	"io"
	"math"
)

func limitReader(r io.Reader, limit int64) io.Reader {
	if limit >= math.MaxInt64 {
		return r
	}
	return io.LimitReader(r, limit+1)
}
