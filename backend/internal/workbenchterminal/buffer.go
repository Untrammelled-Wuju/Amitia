package workbenchterminal

import "encoding/base64"

const bufferCapacity = 256 * 1024
const maxReadBytes = 64 * 1024

type outputBuffer struct {
	base int64
	data []byte
}

func (b *outputBuffer) append(p []byte) {
	b.data = append(b.data, p...)
	if len(b.data) > bufferCapacity {
		drop := len(b.data) - bufferCapacity
		b.base += int64(drop)
		copy(b.data, b.data[drop:])
		b.data = b.data[:bufferCapacity]
	}
}

func (b *outputBuffer) read(cursor int64) (string, int64, bool) {
	data, _, end, truncated := b.readWithStart(cursor)
	return data, end, truncated
}

func (b *outputBuffer) readWithStart(cursor int64) (string, int64, int64, bool) {
	truncated := cursor < b.base
	if cursor < b.base {
		cursor = b.base
	}
	end := b.base + int64(len(b.data))
	if cursor > end {
		cursor = end
	}
	if end-cursor > maxReadBytes {
		end = cursor + maxReadBytes
	}
	chunk := b.data[int(cursor-b.base):int(end-b.base)]
	return base64.StdEncoding.EncodeToString(chunk), cursor, end, truncated
}
