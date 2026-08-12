//go:build linux && !android

package terminal

import "time"

func NewOutputBuffer(maxBytes int) *OutputBuffer {
	return &OutputBuffer{
		chunks:   make([]OutputChunk, 0, 64),
		maxBytes: maxBytes,
		startSeq: 1,
	}
}

func (b *OutputBuffer) Append(stream TerminalStream, data []byte, ts time.Time) OutputChunk {
	b.mu.Lock()
	defer b.mu.Unlock()

	chunk := OutputChunk{
		Sequence:  b.sequence + 1,
		Stream:    stream,
		Data:      data,
		Timestamp: ts,
	}
	b.sequence = chunk.Sequence

	b.chunks = append(b.chunks, chunk)
	b.size += len(data)

	for b.size > b.maxBytes && len(b.chunks) > 1 {
		removed := b.chunks[0]
		b.size -= len(removed.Data)
		b.chunks = b.chunks[1:]
		b.head++
		b.startSeq = b.chunks[0].Sequence
	}

	return chunk
}

func (b *OutputBuffer) Read(afterSequence uint64, maxBytes int) ([]OutputChunk, uint64, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.chunks) == 0 {
		return nil, b.sequence, false
	}

	startIdx := 0
	if afterSequence > 0 {
		for i, chunk := range b.chunks {
			if chunk.Sequence > afterSequence {
				startIdx = i
				break
			}
			if i == len(b.chunks)-1 {
				return nil, b.sequence, false
			}
		}
	}

	var result []OutputChunk
	totalBytes := 0
	for i := startIdx; i < len(b.chunks); i++ {
		chunk := b.chunks[i]
		if totalBytes+len(chunk.Data) > maxBytes && len(result) > 0 {
			break
		}
		result = append(result, chunk)
		totalBytes += len(chunk.Data)
	}

	truncated := false
	if afterSequence > 0 && afterSequence < b.startSeq {
		truncated = true
	}

	return result, b.sequence, truncated
}

func (b *OutputBuffer) LastSequence() uint64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.sequence
}

func (b *OutputBuffer) ChunkCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.chunks)
}

func (b *OutputBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.chunks = b.chunks[:0]
	b.size = 0
	b.head = 0
}
