package jsonrpc

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
)

const (
	DefaultMaxFrameBytes     = 1 << 20
	DefaultMaxStreamChunk    = 64 * 1024
	DefaultMaxPendingBytes   = 8 * 1024 * 1024
	FrameHeaderSize          = 8
	MaxFrameAbsolute         = 64 * 1024 * 1024
)

var (
	ErrFrameTooLarge = errors.New("jsonrpc: frame too large")
	ErrShortRead     = errors.New("jsonrpc: short read")
	ErrClosed        = errors.New("jsonrpc: transport closed")
)

type Frame struct {
	Payload []byte
}

type Framer struct {
	mu            sync.Mutex
	r             io.Reader
	w             io.Writer
	maxFrameBytes int
	closed        bool
	closeMu       sync.Mutex
}

func NewFramer(r io.Reader, w io.Writer) *Framer {
	return &Framer{r: r, w: w, maxFrameBytes: DefaultMaxFrameBytes}
}

func (f *Framer) SetMaxFrameBytes(size int) {
	if size <= 0 || size > MaxFrameAbsolute {
		return
	}
	f.mu.Lock()
	f.maxFrameBytes = size
	f.mu.Unlock()
}

func (f *Framer) MaxFrameBytes() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.maxFrameBytes
}

func (f *Framer) isClosed() bool {
	f.closeMu.Lock()
	defer f.closeMu.Unlock()
	return f.closed
}

func (f *Framer) Close() {
	f.closeMu.Lock()
	f.closed = true
	f.closeMu.Unlock()
	if c, ok := f.w.(io.Closer); ok {
		_ = c.Close()
	}
	if c, ok := f.r.(io.Closer); ok {
		_ = c.Close()
	}
}

func (f *Framer) WriteFrame(payload []byte) error {
	if f.isClosed() {
		return ErrClosed
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(payload) > f.maxFrameBytes {
		return fmt.Errorf("%w: size=%d limit=%d", ErrFrameTooLarge, len(payload), f.maxFrameBytes)
	}
	var header [FrameHeaderSize]byte
	binary.BigEndian.PutUint64(header[:], uint64(len(payload)))
	if _, err := f.w.Write(header[:]); err != nil {
		return fmt.Errorf("jsonrpc: write header: %w", err)
	}
	if _, err := f.w.Write(payload); err != nil {
		return fmt.Errorf("jsonrpc: write payload: %w", err)
	}
	return nil
}

func (f *Framer) ReadFrame() (Frame, error) {
	if f.isClosed() {
		return Frame{}, ErrClosed
	}
	var header [FrameHeaderSize]byte
	if _, err := io.ReadFull(f.r, header[:]); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return Frame{}, ErrClosed
		}
		return Frame{}, fmt.Errorf("jsonrpc: read header: %w", err)
	}
	size := binary.BigEndian.Uint64(header[:])
	f.mu.Lock()
	limit := f.maxFrameBytes
	f.mu.Unlock()
	if size > uint64(limit) {
		_ = drainReader(f.r, size)
		return Frame{}, fmt.Errorf("%w: size=%d limit=%d", ErrFrameTooLarge, size, limit)
	}
	if size > uint64(MaxFrameAbsolute) {
		return Frame{}, fmt.Errorf("%w: absolute=%d", ErrFrameTooLarge, size)
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(f.r, payload); err != nil {
		return Frame{}, fmt.Errorf("jsonrpc: read payload: %w", err)
	}
	return Frame{Payload: payload}, nil
}

func drainReader(r io.Reader, size uint64) error {
	limited := io.LimitReader(r, int64(size))
	_, err := io.Copy(io.Discard, limited)
	return err
}

type MessageReader struct {
	framer *Framer
}

func NewMessageReader(framer *Framer) *MessageReader {
	return &MessageReader{framer: framer}
}

func (mr *MessageReader) Read() (*Envelope, error) {
	frame, err := mr.framer.ReadFrame()
	if err != nil {
		return nil, err
	}
	return DecodeEnvelope(frame.Payload)
}

type MessageWriter struct {
	framer *Framer
}

func NewMessageWriter(framer *Framer) *MessageWriter {
	return &MessageWriter{framer: framer}
}

func (mw *MessageWriter) Write(msg any) error {
	data, err := MarshalMessage(msg)
	if err != nil {
		return err
	}
	return mw.framer.WriteFrame(data)
}

func (mw *MessageWriter) WriteRaw(data []byte) error {
	return mw.framer.WriteFrame(data)
}

type ReadWriteCloser struct {
	framer *Framer
}

func NewTransport(r io.Reader, w io.Writer) *ReadWriteCloser {
	framer := NewFramer(r, w)
	return &ReadWriteCloser{framer: framer}
}

func (t *ReadWriteCloser) Framer() *Framer        { return t.framer }
func (t *ReadWriteCloser) Write(msg any) error    { return (&MessageWriter{framer: t.framer}).Write(msg) }
func (t *ReadWriteCloser) Read() (*Envelope, error) {
	return (&MessageReader{framer: t.framer}).Read()
}
func (t *ReadWriteCloser) Close() error {
	t.framer.Close()
	return nil
}
func (t *ReadWriteCloser) SetMaxFrameBytes(size int) { t.framer.SetMaxFrameBytes(size) }
func (t *ReadWriteCloser) MaxFrameBytes() int        { return t.framer.MaxFrameBytes() }
