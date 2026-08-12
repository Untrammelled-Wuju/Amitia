//go:build linux && !android

package shell

import (
	"bytes"
	"sync"
)

type OutputBuffer struct {
	mu              sync.Mutex
	stdoutBuf       bytes.Buffer
	stderrBuf       bytes.Buffer
	stdoutLimit     int64
	stderrLimit     int64
	combinedLimit   int64
	currentCombined int64
	stdoutTruncated bool
	stderrTruncated bool
	done            chan struct{}
	doneOnce        sync.Once
	limitExceeded   bool
}

func NewOutputBuffer(stdoutLimit, stderrLimit, combinedLimit int64) *OutputBuffer {
	if stdoutLimit <= 0 {
		stdoutLimit = 1 * 1024 * 1024
	}
	if stderrLimit <= 0 {
		stderrLimit = 512 * 1024
	}
	if combinedLimit <= 0 {
		combinedLimit = stdoutLimit + stderrLimit + 512*1024
	}

	return &OutputBuffer{
		stdoutLimit:   stdoutLimit,
		stderrLimit:   stderrLimit,
		combinedLimit: combinedLimit,
		done:          make(chan struct{}),
	}
}

func (b *OutputBuffer) WriteStdout(data []byte) (int, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.limitExceeded || b.stdoutTruncated {
		return 0, true
	}

	remaining := b.stdoutLimit - int64(b.stdoutBuf.Len())
	if remaining <= 0 {
		b.stdoutTruncated = true
		return 0, true
	}

	combinedRemaining := b.combinedLimit - b.currentCombined
	if combinedRemaining <= 0 {
		b.limitExceeded = true
		b.doneOnce.Do(func() { close(b.done) })
		return 0, true
	}

	toWrite := int64(len(data))
	if toWrite > remaining {
		toWrite = remaining
		b.stdoutTruncated = true
	}
	if toWrite > combinedRemaining {
		toWrite = combinedRemaining
		b.limitExceeded = true
	}

	n, _ := b.stdoutBuf.Write(data[:toWrite])
	b.currentCombined += int64(n)

	if b.stdoutTruncated || b.limitExceeded {
		b.doneOnce.Do(func() { close(b.done) })
	}

	return n, b.stdoutTruncated || b.limitExceeded
}

func (b *OutputBuffer) WriteStderr(data []byte) (int, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.limitExceeded || b.stderrTruncated {
		return 0, true
	}

	remaining := b.stderrLimit - int64(b.stderrBuf.Len())
	if remaining <= 0 {
		b.stderrTruncated = true
		return 0, true
	}

	combinedRemaining := b.combinedLimit - b.currentCombined
	if combinedRemaining <= 0 {
		b.limitExceeded = true
		b.doneOnce.Do(func() { close(b.done) })
		return 0, true
	}

	toWrite := int64(len(data))
	if toWrite > remaining {
		toWrite = remaining
		b.stderrTruncated = true
	}
	if toWrite > combinedRemaining {
		toWrite = combinedRemaining
		b.limitExceeded = true
	}

	n, _ := b.stderrBuf.Write(data[:toWrite])
	b.currentCombined += int64(n)

	if b.stderrTruncated || b.limitExceeded {
		b.doneOnce.Do(func() { close(b.done) })
	}

	return n, b.stderrTruncated || b.limitExceeded
}

func (b *OutputBuffer) StdoutBytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.stdoutBuf.Bytes()
}

func (b *OutputBuffer) StderrBytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.stderrBuf.Bytes()
}

func (b *OutputBuffer) StdoutString() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.stdoutBuf.String()
}

func (b *OutputBuffer) StderrString() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.stderrBuf.String()
}

func (b *OutputBuffer) IsStdoutTruncated() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.stdoutTruncated
}

func (b *OutputBuffer) IsStderrTruncated() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.stderrTruncated
}

func (b *OutputBuffer) IsLimitExceeded() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.limitExceeded
}

func (b *OutputBuffer) StdoutSize() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return int64(b.stdoutBuf.Len())
}

func (b *OutputBuffer) StderrSize() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return int64(b.stderrBuf.Len())
}

func (b *OutputBuffer) CombinedSize() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.currentCombined
}

func (b *OutputBuffer) Done() <-chan struct{} {
	return b.done
}

func (b *OutputBuffer) MarkComplete() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.doneOnce.Do(func() { close(b.done) })
}
