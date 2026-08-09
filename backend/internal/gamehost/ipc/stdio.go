package ipc

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

const (
	stdioMaxFrameSize = 16 * 1024 * 1024
	stdioFrameHeaderSize = 4
)

type StdioTransport struct {
	reader     io.Reader
	writer     io.Writer
	closer     io.Closer
	writerMu   sync.Mutex
	readCloser io.Closer
	maxSize    int64
}

type StdioTransportConfig struct {
	Reader io.Reader
	Writer io.Writer
	Closer io.Closer
	MaxFrameSize int64
}

func NewStdioTransport(config StdioTransportConfig) *StdioTransport {
	maxSize := config.MaxFrameSize
	if maxSize <= 0 {
		maxSize = stdioMaxFrameSize
	}
	return &StdioTransport{
		reader:  config.Reader,
		writer:  config.Writer,
		closer:  config.Closer,
		maxSize: maxSize,
	}
}

func (t *StdioTransport) Send(ctx context.Context, envelope protocol.Envelope) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal envelope failed: %w", err)
	}

	if int64(len(data)) > t.maxSize {
		return fmt.Errorf("%w: frame size %d exceeds limit %d", ErrFrameTooLarge, len(data), t.maxSize)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	t.writerMu.Lock()
	defer t.writerMu.Unlock()

	if err := binary.Write(t.writer, binary.BigEndian, uint32(len(data))); err != nil {
		return fmt.Errorf("write frame header failed: %w", err)
	}

	if _, err := t.writer.Write(data); err != nil {
		return fmt.Errorf("write frame data failed: %w", err)
	}

	if flusher, ok := t.writer.(interface{ Flush() error }); ok {
		if err := flusher.Flush(); err != nil {
			return fmt.Errorf("flush writer failed: %w", err)
		}
	}

	return nil
}

func (t *StdioTransport) Receive(ctx context.Context) (protocol.Envelope, error) {
	if err := ctx.Err(); err != nil {
		return protocol.Envelope{}, err
	}

	headerBuf := make([]byte, stdioFrameHeaderSize)

	if err := t.readFull(headerBuf); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return protocol.Envelope{}, io.EOF
		}
		return protocol.Envelope{}, fmt.Errorf("read frame header failed: %w", err)
	}

	frameLen := binary.BigEndian.Uint32(headerBuf)
	if frameLen == 0 {
		return protocol.Envelope{}, fmt.Errorf("invalid frame: zero length")
	}
	if int64(frameLen) > t.maxSize {
		return protocol.Envelope{}, fmt.Errorf("%w: frame size %d exceeds limit %d", ErrFrameTooLarge, frameLen, t.maxSize)
	}

	data := make([]byte, frameLen)
	if err := t.readFull(data); err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return protocol.Envelope{}, io.EOF
		}
		return protocol.Envelope{}, fmt.Errorf("read frame data failed: %w", err)
	}

	var envelope protocol.Envelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return protocol.Envelope{}, fmt.Errorf("unmarshal envelope failed: %w", err)
	}

	return envelope, nil
}

func (t *StdioTransport) readFull(buf []byte) error {
	reader := t.reader
	if br, ok := reader.(*bufio.Reader); ok {
		_, err := io.ReadFull(br, buf)
		return err
	}
	_, err := io.ReadFull(reader, buf)
	return err
}

func (t *StdioTransport) Close() error {
	t.writerMu.Lock()
	defer t.writerMu.Unlock()

	if t.closer == nil {
		return nil
	}
	err := t.closer.Close()
	return err
}

var ErrFrameTooLarge = NewIPCError(IPCErrorLimit, domain.ErrResourceExhausted, "frame exceeds maximum size")
