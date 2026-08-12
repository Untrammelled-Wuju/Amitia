package shell

import (
	"testing"
	"time"
)

func TestOutputBuffer_WriteStdout(t *testing.T) {
	buf := NewOutputBuffer(1024, 512, 2048)

	n, truncated := buf.WriteStdout([]byte("hello"))
	if n != 5 {
		t.Errorf("expected n=5, got %d", n)
	}
	if truncated {
		t.Error("should not be truncated")
	}

	if buf.StdoutString() != "hello" {
		t.Errorf("expected 'hello', got %q", buf.StdoutString())
	}
}

func TestOutputBuffer_WriteStderr(t *testing.T) {
	buf := NewOutputBuffer(1024, 512, 2048)

	n, truncated := buf.WriteStderr([]byte("error msg"))
	if n != 9 {
		t.Errorf("expected n=9, got %d", n)
	}
	if truncated {
		t.Error("should not be truncated")
	}

	if buf.StderrString() != "error msg" {
		t.Errorf("expected 'error msg', got %q", buf.StderrString())
	}
}

func TestOutputBuffer_StdoutTruncated(t *testing.T) {
	buf := NewOutputBuffer(10, 512, 2048)

	_, _ = buf.WriteStdout([]byte("12345"))
	_, truncated := buf.WriteStdout([]byte("67890abcde"))
	if !truncated {
		t.Error("expected truncation after exceeding stdout limit")
	}

	if !buf.IsStdoutTruncated() {
		t.Error("expected IsStdoutTruncated to return true")
	}
}

func TestOutputBuffer_CombinedLimitExceeded(t *testing.T) {
	buf := NewOutputBuffer(100, 100, 150)

	_, _ = buf.WriteStdout([]byte(makeStr('a', 100)))
	_, truncated := buf.WriteStderr([]byte(makeStr('b', 100)))
	if !truncated {
		t.Error("expected truncation after exceeding combined limit")
	}

	if !buf.IsLimitExceeded() {
		t.Error("expected IsLimitExceeded to return true")
	}
}

func TestOutputBuffer_DoneChannel(t *testing.T) {
	buf := NewOutputBuffer(10, 10, 20)

	select {
	case <-buf.Done():
		t.Error("should not be done yet")
	default:
	}

	_, _ = buf.WriteStdout([]byte("1234567890abcde"))

	select {
	case <-buf.Done():
	case <-time.After(time.Second):
		t.Error("expected done signal after truncation")
	}
}

func TestOutputBuffer_MarkComplete(t *testing.T) {
	buf := NewOutputBuffer(1024, 1024, 2048)

	buf.MarkComplete()

	select {
	case <-buf.Done():
	case <-time.After(time.Second):
		t.Error("expected done signal after MarkComplete")
	}
}

func makeStr(c byte, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = c
	}
	return string(b)
}
