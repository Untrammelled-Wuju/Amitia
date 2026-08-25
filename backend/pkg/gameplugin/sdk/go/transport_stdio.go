package sdk

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

const (
	StdioMaxFrameSize    = 16 * 1024 * 1024
	StdioFrameHeaderSize = 4
	StdioBinaryFrameFlag = uint32(1 << 31)
	stdioFrameLengthMask = ^StdioBinaryFrameFlag
)

type StdioTransport struct {
	reader   io.Reader
	writer   io.Writer
	closer   io.Closer
	writerMu sync.Mutex
	maxSize  int64
}

type StdioTransportConfig struct {
	Reader       io.Reader
	Writer       io.Writer
	Closer       io.Closer
	MaxFrameSize int64
}

func NewStdioTransport(config StdioTransportConfig) *StdioTransport {
	maxSize := config.MaxFrameSize
	if maxSize <= 0 {
		maxSize = StdioMaxFrameSize
	}
	reader := config.Reader
	if reader == nil {
		reader = os.Stdin
	}
	writer := config.Writer
	if writer == nil {
		writer = os.Stdout
	}
	return &StdioTransport{
		reader:  reader,
		writer:  writer,
		closer:  config.Closer,
		maxSize: maxSize,
	}
}

func NewDefaultStdioTransport() *StdioTransport {
	return NewStdioTransport(StdioTransportConfig{})
}

func (t *StdioTransport) Send(ctx context.Context, msg protocol.Envelope) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal envelope failed: %w", err)
	}

	if int64(len(data)) > t.maxSize {
		return fmt.Errorf("frame size %d exceeds limit %d", len(data), t.maxSize)
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

	return nil
}

func (t *StdioTransport) SendBinaryFrame(ctx context.Context, message protocol.Envelope, objectID string, offset int64, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(data) == 0 {
		return fmt.Errorf("binary frame payload must not be empty")
	}
	header := protocol.BinaryFrameHeader{
		Protocol:   message.Protocol,
		ID:         message.ID,
		RuntimeID:  message.RuntimeID,
		PluginID:   message.PluginID,
		ServiceID:  message.ServiceID,
		Generation: message.Generation,
		ObjectID:   objectID,
		Offset:     offset,
	}
	if err := header.Validate(); err != nil {
		return err
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return fmt.Errorf("marshal binary frame header failed: %w", err)
	}
	bodyLen := 4 + len(headerJSON) + len(data)
	if int64(bodyLen) > t.maxSize || uint64(bodyLen) > uint64(stdioFrameLengthMask) {
		return fmt.Errorf("binary frame size %d exceeds limit %d", bodyLen, t.maxSize)
	}

	t.writerMu.Lock()
	defer t.writerMu.Unlock()
	frameHeader := make([]byte, 4)
	binary.BigEndian.PutUint32(frameHeader, StdioBinaryFrameFlag|uint32(bodyLen))
	metaHeader := make([]byte, 4)
	binary.BigEndian.PutUint32(metaHeader, uint32(len(headerJSON)))
	for _, part := range [][]byte{frameHeader, metaHeader, headerJSON, data} {
		if _, err := t.writer.Write(part); err != nil {
			return fmt.Errorf("write binary frame failed: %w", err)
		}
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

	headerBuf := make([]byte, StdioFrameHeaderSize)

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
		return protocol.Envelope{}, fmt.Errorf("frame size %d exceeds limit %d", frameLen, t.maxSize)
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
	_, err := io.ReadFull(t.reader, buf)
	return err
}

func (t *StdioTransport) Close() error {
	t.writerMu.Lock()
	defer t.writerMu.Unlock()

	if t.closer == nil {
		return nil
	}
	return t.closer.Close()
}
